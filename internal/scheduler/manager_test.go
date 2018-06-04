package scheduler_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io/ioutil"
	"log"
	"sync"
	"testing"
	"time"

	"github.com/apoydence/onpar"
	. "github.com/apoydence/onpar/expect"
	. "github.com/apoydence/onpar/matchers"
	"github.com/apoydence/triple-c/internal/git"
	"github.com/apoydence/triple-c/internal/scheduler"
)

type TM struct {
	*testing.T
	spyTaskCreator  *spyTaskCreator
	spyGitWatcher   *spyGitWatcher
	spyMetrics      *spyMetrics
	spyRepoRegistry *spyRepoRegistry
	m               *scheduler.Manager
}

func TestManager(t *testing.T) {
	t.Parallel()
	o := onpar.New()
	defer o.Run(t)

	o.BeforeEach(func(t *testing.T) TM {
		spyMetrics := newSpyMetrics()
		spyTaskCreator := newSpyTaskCreator()
		spyGitWatcher := newSpyGitWatcher()
		spyRepoRegistry := newSpyRepoRegistry()
		return TM{
			T:               t,
			spyMetrics:      spyMetrics,
			spyGitWatcher:   spyGitWatcher,
			spyTaskCreator:  spyTaskCreator,
			spyRepoRegistry: spyRepoRegistry,

			m: scheduler.NewManager(
				context.Background(),
				"some-guid",
				"some-branch",
				spyTaskCreator,
				spyGitWatcher.StartWatcher,
				spyRepoRegistry,
				func(key string) (string, bool) {
					if key == "KNOWN_KEY" {
						return "KNOWN_VALUE", true
					}
					return "", false
				},
				nil,
				spyMetrics,
				log.New(ioutil.Discard, "", 0),
			),
		}
	})

	o.Spec("it starts a task when a commit comes through", func(t TM) {
		t.m.Add(scheduler.MetaPlan{
			Plan: scheduler.Plan{
				RepoPaths: map[string]string{"some-repo": "some-path"},
				Tasks: []scheduler.Task{
					{
						Command: "some-command",
					},
				},
			},
		})

		Expect(t, t.spyGitWatcher.commit).To(Not(BeNil()))
		Expect(t, t.spyGitWatcher.branch).To(Equal("some-branch"))
		Expect(t, t.spyGitWatcher.repoName).To(Equal("some-path"))

		t.spyGitWatcher.commit("some-sha")
		Expect(t, t.spyTaskCreator.command).To(ContainSubstring("some-command"))
		Expect(t, t.spyTaskCreator.appGuid).To(Equal("some-guid"))

		dataName, err := base64.StdEncoding.DecodeString(t.spyTaskCreator.name)
		Expect(t, err).To(BeNil())

		var m map[string]interface{}
		Expect(t, json.Unmarshal(dataName, &m)).To(BeNil())
		Expect(t, m["sha"]).To(Equal("some-sha"))
		Expect(t, m["branch"]).To(Equal("some-branch"))
		Expect(t, m["task_index"]).To(Equal(0.0))

		Expect(t, t.spyMetrics.GetDelta("SuccessfulTasks")()).To(Equal(uint64(1)))
		Expect(t, t.spyMetrics.GetDelta("FailedTasks")()).To(Equal(uint64(0)))
	})

	o.Spec("it guards for certain branches", func(t TM) {
		t.m.Add(scheduler.MetaPlan{
			Plan: scheduler.Plan{
				RepoPaths: map[string]string{"some-repo": "some-path"},
				Tasks: []scheduler.Task{
					{
						Command:     "some-other-command",
						BranchGuard: "some-other-branch",
					},
					{
						Command:     "some-command",
						BranchGuard: "some-branch",
					},
				},
			},
		})

		Expect(t, t.spyGitWatcher.commit).To(Not(BeNil()))
		Expect(t, t.spyGitWatcher.branch).To(Equal("some-branch"))
		Expect(t, t.spyGitWatcher.repoName).To(Equal("some-path"))

		t.spyGitWatcher.commit("some-sha")
		Expect(t, t.spyTaskCreator.command).To(ContainSubstring("some-command"))
		Expect(t, t.spyTaskCreator.appGuid).To(Equal("some-guid"))
		Expect(t, t.spyMetrics.GetDelta("SuccessfulTasks")()).To(Equal(uint64(1)))
	})

	o.Spec("it handles multiple RepoPaths", func(t TM) {
		t.m.Add(scheduler.MetaPlan{
			Plan: scheduler.Plan{
				RepoPaths: map[string]string{"some-repo-path": "some-path", "some-other-repo": "some-other-path"},
				Tasks: []scheduler.Task{
					{
						Command: "some-command",
					},
				},
			},
		})

		Expect(t, t.spyGitWatcher.called).To(Equal(2))
	})

	o.Spec("it starts the next task", func(t TM) {
		t.m.Add(scheduler.MetaPlan{
			Plan: scheduler.Plan{
				RepoPaths: map[string]string{"some-repo": "some-path"},
				Tasks: []scheduler.Task{
					{
						Command: "some-command",
					},
					{
						Command: "some-other-command",
					},
				},
			},
		})

		Expect(t, t.spyGitWatcher.commit).To(Not(BeNil()))
		Expect(t, t.spyGitWatcher.branch).To(Equal("some-branch"))
		Expect(t, t.spyGitWatcher.repoName).To(Equal("some-path"))

		t.spyGitWatcher.commit("some-sha")
		Expect(t, t.spyTaskCreator.command).To(ContainSubstring("some-other-command"))
	})

	o.Spec("it does not start the next task if the first fails", func(t TM) {
		t.spyTaskCreator.err = errors.New("some-error")
		t.m.Add(scheduler.MetaPlan{
			Plan: scheduler.Plan{
				RepoPaths: map[string]string{"some-repo": "some-path"},
				Tasks: []scheduler.Task{
					{
						Command: "some-command",
					},
					{
						Command: "some-other-command",
					},
				},
			},
		})

		Expect(t, t.spyGitWatcher.commit).To(Not(BeNil()))
		Expect(t, t.spyGitWatcher.branch).To(Equal("some-branch"))
		Expect(t, t.spyGitWatcher.repoName).To(Equal("some-path"))

		t.spyGitWatcher.commit("some-sha")
		Expect(t, t.spyTaskCreator.command).To(Not(ContainSubstring("some-other-command")))
	})

	o.Spec("it starts a task once if the DoOnce is set", func(t TM) {
		t.m.Add(scheduler.MetaPlan{
			Plan: scheduler.Plan{
				RepoPaths: map[string]string{"some-repo": "some-path"},
				Tasks: []scheduler.Task{
					{
						Command: "some-command",
					},
				},
			},
			DoOnce: true,
		})

		Expect(t, t.spyGitWatcher.commit).To(Not(BeNil()))
		Expect(t, t.spyGitWatcher.branch).To(Equal("some-branch"))
		t.spyGitWatcher.commit("some-sha")
		t.spyGitWatcher.commit("some-other-sha")

		Expect(t, t.spyMetrics.GetDelta("SuccessfulTasks")()).To(Equal(uint64(1)))
	})

	o.Spec("it starts a task multiple times if the DoOnce is not set", func(t TM) {
		t.m.Add(scheduler.MetaPlan{
			Plan: scheduler.Plan{
				RepoPaths: map[string]string{"some-repo": "some-path"},
				Tasks: []scheduler.Task{
					{
						Command: "some-command",
					},
				},
			},
			DoOnce: false,
		})

		Expect(t, t.spyGitWatcher.commit).To(Not(BeNil()))
		Expect(t, t.spyGitWatcher.branch).To(Equal("some-branch"))
		t.spyGitWatcher.commit("some-sha")
		t.spyGitWatcher.commit("some-other-sha")

		Expect(t, t.spyMetrics.GetDelta("SuccessfulTasks")()).To(Equal(uint64(2)))
	})

	o.Spec("it sets the given environment variables", func(t TM) {
		t.m.Add(scheduler.MetaPlan{
			Plan: scheduler.Plan{
				RepoPaths: map[string]string{"some-repo": "some-path"},
				Tasks: []scheduler.Task{
					{
						Command: "some-command",
						Parameters: map[string]string{
							"SOME_VAR":       "some-value",
							"SOME_OTHER_VAR": "some-other-value",
							"LOOKUP":         "((KNOWN_KEY))",
							"DONT_LOOKUP":    "((UNKNOWN_KEY))",
						},
					},
				},
			},
		})

		Expect(t, t.spyGitWatcher.commit).To(Not(BeNil()))
		Expect(t, t.spyGitWatcher.branch).To(Equal("some-branch"))
		t.spyGitWatcher.commit("some-sha")
		Expect(t, t.spyTaskCreator.command).To(
			And(
				ContainSubstring("export SOME_VAR=some-value"),
				ContainSubstring("export SOME_OTHER_VAR=some-other-value"),
				ContainSubstring("export LOOKUP=KNOWN_VALUE"),
				Not(ContainSubstring("DONT_LOOKUP")),
			),
		)
	})

	o.Spec("it does not start a task when a commit comes through but there is a task for it already", func(t TM) {
		t.m.Add(scheduler.MetaPlan{
			Plan: scheduler.Plan{
				RepoPaths: map[string]string{"some-repo": "some-path"},
				Tasks: []scheduler.Task{
					{
						Command: "some-command",
					},
				},
			},
		})

		t.spyTaskCreator.listResults = []string{
			base64.StdEncoding.EncodeToString([]byte(`{"sha":"some-sha","branch":"some-branch"}`)),
			"some-other-name",
		}

		Expect(t, t.spyGitWatcher.commit).To(Not(BeNil()))
		Expect(t, t.spyGitWatcher.branch).To(Equal("some-branch"))
		t.spyGitWatcher.commit("some-sha")

		Expect(t, t.spyTaskCreator.called).To(Equal(0))
		Expect(t, t.spyMetrics.GetDelta("DedupedTasks")()).To(Equal(uint64(1)))
	})

	o.Spec("it does not dedupe commits on different branches", func(t TM) {
		t.m.Add(scheduler.MetaPlan{
			Plan: scheduler.Plan{
				RepoPaths: map[string]string{"some-repo": "some-path"},
				Tasks: []scheduler.Task{
					{
						Command: "some-command",
					},
				},
			},
		})

		t.spyTaskCreator.listResults = []string{
			base64.StdEncoding.EncodeToString([]byte(`{"sha":"some-sha","branch":"other-branch"}`)),
		}

		Expect(t, t.spyGitWatcher.commit).To(Not(BeNil()))
		Expect(t, t.spyGitWatcher.branch).To(Equal("some-branch"))
		t.spyGitWatcher.commit("some-sha")

		Expect(t, t.spyTaskCreator.called).To(Equal(1))
		Expect(t, t.spyMetrics.GetDelta("DedupedTasks")()).To(Equal(uint64(0)))
	})

	o.Spec("it increments FailedTasks when a task fails", func(t TM) {
		t.spyTaskCreator.err = errors.New("some-error")
		t.m.Add(scheduler.MetaPlan{
			Plan: scheduler.Plan{
				RepoPaths: map[string]string{"some-repo": "some-path"},
				Tasks: []scheduler.Task{
					{
						Command: "some-command",
					},
				},
			},
		})
		Expect(t, t.spyGitWatcher.commit).To(Not(BeNil()))
		t.spyGitWatcher.commit("some-sha")

		Expect(t, t.spyMetrics.GetDelta("SuccessfulTasks")()).To(Equal(uint64(0)))
		Expect(t, t.spyMetrics.GetDelta("FailedTasks")()).To(Equal(uint64(1)))
	})

	o.Spec("it increments FailedRepos when a repo fails to be fetched", func(t TM) {
		t.spyRepoRegistry.err = errors.New("some-err")
		t.m.Add(scheduler.MetaPlan{
			Plan: scheduler.Plan{
				RepoPaths: map[string]string{"some-repo": "some-path"},
				Tasks: []scheduler.Task{
					{
						Command: "some-command",
					},
				},
			},
		})

		Expect(t, t.spyMetrics.GetDelta("FailedRepos")()).To(Equal(uint64(1)))
		Expect(t, t.spyGitWatcher.ctx).To(BeNil())
	})

	o.Spec("it cancels the context when a task is removed", func(t TM) {
		t.m.Add(scheduler.MetaPlan{
			Plan: scheduler.Plan{
				RepoPaths: map[string]string{"some-repo": "some-path"},
				Tasks: []scheduler.Task{
					{
						Command: "some-command",
					},
				},
			},
		})

		t.m.Remove(scheduler.MetaPlan{
			Plan: scheduler.Plan{
				RepoPaths: map[string]string{"some-repo": "some-path"},
				Tasks: []scheduler.Task{
					{
						Command: "some-command",
					},
				},
			},
		})

		Expect(t, t.spyGitWatcher.ctx.Err()).To(Not(BeNil()))

		t.spyGitWatcher.commit("some-sha")
		Expect(t, t.spyTaskCreator.called).To(Equal(0))
	})

	o.Spec("it handles removing a task that never was added", func(t TM) {
		Expect(t, func() {
			t.m.Remove(scheduler.MetaPlan{
				Plan: scheduler.Plan{
					RepoPaths: map[string]string{"some-repo": "some-path"},
					Tasks: []scheduler.Task{
						{
							Command: "some-command",
						},
					},
				},
			})
		}).To(Not(Panic()))
	})

	o.Spec("it survives the race detector", func(t TM) {
		go func() {
			for i := 0; i < 100; i++ {
				t.m.Add(scheduler.MetaPlan{
					Plan: scheduler.Plan{
						RepoPaths: map[string]string{"some-repo": "some-path"},
						Tasks: []scheduler.Task{
							{
								Command: "some-command",
							},
						},
					},
				})
			}
		}()

		for i := 0; i < 2; i++ {
			go func() {
				for i := 0; i < 100; i++ {
					t.m.Remove(scheduler.MetaPlan{
						Plan: scheduler.Plan{
							RepoPaths: map[string]string{"some-repo": "some-path"},
							Tasks: []scheduler.Task{
								{
									Command: "some-command",
								},
							},
						},
					})
				}
			}()
		}

		t.m.Add(scheduler.MetaPlan{
			Plan: scheduler.Plan{
				RepoPaths: map[string]string{"some-repo": "some-path"},
				Tasks: []scheduler.Task{
					{
						Command: "some-command",
					},
				},
			},
		})
	})
}

type spyTaskCreator struct {
	called  int
	command string
	name    string
	appGuid string

	err error

	listAppGuid string
	listResults []string
	listErr     error
}

func newSpyTaskCreator() *spyTaskCreator {
	return &spyTaskCreator{}
}

func (s *spyTaskCreator) CreateTask(
	command string,
	name string,
	appGuid string,
) error {
	s.called++
	s.command = command
	s.name = name
	s.appGuid = appGuid

	return s.err
}

func (s *spyTaskCreator) ListTasks(appGuid string) ([]string, error) {
	s.listAppGuid = appGuid
	return s.listResults, s.listErr
}

type spyGitWatcher struct {
	called     int
	ctx        context.Context
	repoName   string
	branch     string
	commit     func(SHA string)
	interval   time.Duration
	repo       git.Repo
	shaTracker git.SHATracker
	log        *log.Logger
}

func newSpyGitWatcher() *spyGitWatcher {
	return &spyGitWatcher{}
}

func (s *spyGitWatcher) StartWatcher(
	ctx context.Context,
	repoName string,
	branch string,
	commit func(SHA string),
	interval time.Duration,
	repo git.Repo,
	shaTracker git.SHATracker,
	log *log.Logger,
) {
	s.called++
	s.ctx = ctx
	s.repoName = repoName
	s.branch = branch
	s.commit = commit
	s.interval = interval
	s.repo = repo
	s.shaTracker = shaTracker
	s.log = log
}

type spyMetrics struct {
	mu sync.Mutex
	m  map[string]uint64
}

func newSpyMetrics() *spyMetrics {
	return &spyMetrics{
		m: make(map[string]uint64),
	}
}

func (s *spyMetrics) NewCounter(name string) func(uint64) {
	return func(delta uint64) {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.m[name] += delta
	}
}

func (s *spyMetrics) GetDelta(name string) func() uint64 {
	return func() uint64 {
		s.mu.Lock()
		defer s.mu.Unlock()
		return s.m[name]
	}
}

type spyRepoRegistry struct {
	path string

	repo git.Repo
	err  error
}

func newSpyRepoRegistry() *spyRepoRegistry {
	return &spyRepoRegistry{}
}

func (s *spyRepoRegistry) FetchRepo(path string) (git.Repo, error) {
	s.path = path
	return s.repo, s.err
}
