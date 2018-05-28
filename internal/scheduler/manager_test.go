package scheduler_test

import (
	"context"
	"errors"
	"io/ioutil"
	"log"
	"sync"
	"testing"
	"time"

	"github.com/apoydence/onpar"
	. "github.com/apoydence/onpar/expect"
	. "github.com/apoydence/onpar/matchers"
	"github.com/apoydence/triple-c/internal/gitwatcher"
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
				"some-guid",
				spyTaskCreator,
				spyGitWatcher.StartWatcher,
				spyRepoRegistry,
				spyMetrics,
				log.New(ioutil.Discard, "", 0),
			),
		}
	})

	o.Spec("it starts a task when a commit comes through", func(t TM) {
		t.m.Add(scheduler.Task{
			RepoPath: "some-path",
			Command:  "some-command",
		})

		Expect(t, t.spyGitWatcher.commit).To(Not(BeNil()))
		t.spyGitWatcher.commit("some-sha")
		Expect(t, t.spyTaskCreator.command).To(ContainSubstring("some-command"))
		Expect(t, t.spyTaskCreator.name).To(Not(Equal("")))
		Expect(t, t.spyTaskCreator.appGuid).To(Equal("some-guid"))

		Expect(t, t.spyMetrics.GetDelta("SuccessfulTasks")()).To(Equal(uint64(1)))
		Expect(t, t.spyMetrics.GetDelta("FailedTasks")()).To(Equal(uint64(0)))
	})

	o.Spec("it increments FailedTasks when a task fails", func(t TM) {
		t.spyTaskCreator.err = errors.New("some-error")
		t.m.Add(scheduler.Task{
			RepoPath: "some-path",
			Command:  "some-command",
		})
		Expect(t, t.spyGitWatcher.commit).To(Not(BeNil()))
		t.spyGitWatcher.commit("some-sha")

		Expect(t, t.spyMetrics.GetDelta("SuccessfulTasks")()).To(Equal(uint64(0)))
		Expect(t, t.spyMetrics.GetDelta("FailedTasks")()).To(Equal(uint64(1)))
	})

	o.Spec("it increments FailedRepos when a repo fails to be fetched", func(t TM) {
		t.spyRepoRegistry.err = errors.New("some-err")
		t.m.Add(scheduler.Task{
			RepoPath: "some-path",
			Command:  "some-command",
		})

		Expect(t, t.spyMetrics.GetDelta("FailedRepos")()).To(Equal(uint64(1)))
		Expect(t, t.spyGitWatcher.ctx).To(BeNil())
	})

	o.Spec("it cancels the context when a task is removed", func(t TM) {
		t.m.Add(scheduler.Task{
			RepoPath: "some-path",
			Command:  "some-command",
		})

		t.m.Remove(scheduler.Task{
			RepoPath: "some-path",
			Command:  "some-command",
		})

		Expect(t, t.spyGitWatcher.ctx.Err()).To(Not(BeNil()))

		t.spyGitWatcher.commit("some-sha")
		Expect(t, t.spyTaskCreator.called).To(Equal(0))
	})

	o.Spec("it handles removing a task that never was added", func(t TM) {
		Expect(t, func() {
			t.m.Remove(scheduler.Task{
				RepoPath: "some-path",
				Command:  "some-command",
			})
		}).To(Not(Panic()))
	})
}

type spyTaskCreator struct {
	called  int
	command string
	name    string
	appGuid string

	err error
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

type spyGitWatcher struct {
	ctx        context.Context
	commit     func(SHA string)
	interval   time.Duration
	shaFetcher gitwatcher.SHAFetcher
	m          gitwatcher.Metrics
	log        *log.Logger
}

func newSpyGitWatcher() *spyGitWatcher {
	return &spyGitWatcher{}
}

func (s *spyGitWatcher) StartWatcher(
	ctx context.Context,
	commit func(SHA string),
	interval time.Duration,
	shaFetcher gitwatcher.SHAFetcher,
	m gitwatcher.Metrics,
	log *log.Logger,
) {
	s.ctx = ctx
	s.commit = commit
	s.interval = interval
	s.shaFetcher = shaFetcher
	s.m = m
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

	repo *gitwatcher.Repo
	err  error
}

func newSpyRepoRegistry() *spyRepoRegistry {
	return &spyRepoRegistry{}
}

func (s *spyRepoRegistry) FetchRepo(path string) (*gitwatcher.Repo, error) {
	s.path = path
	return s.repo, s.err
}
