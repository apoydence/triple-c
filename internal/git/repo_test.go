package git_test

import (
	"errors"
	"io/ioutil"
	"os"
	"path"
	"sync"
	"testing"
	"time"

	"github.com/apoydence/onpar"
	. "github.com/apoydence/onpar/expect"
	. "github.com/apoydence/onpar/matchers"
	"github.com/apoydence/triple-c/internal/git"
)

type TR struct {
	*testing.T
	spyExecutor *spyExecutor
	spyMetrics  *spyMetrics
	r           *git.Repo
	tmpDir      string
}

func TestRepo(t *testing.T) {
	t.Parallel()
	o := onpar.New()
	defer o.Run(t)

	o.BeforeEach(func(t *testing.T) TR {
		tmpDir, err := ioutil.TempDir("", "")
		Expect(t, err).To(BeNil())
		spyExecutor := newSpyExecutor()
		spyMetrics := newSpyMetrics()

		r, err := git.NewRepo("some-path", tmpDir, 100*time.Millisecond, spyExecutor, spyMetrics)
		Expect(t, err).To(BeNil())

		return TR{
			T:           t,
			spyExecutor: spyExecutor,
			spyMetrics:  spyMetrics,
			tmpDir:      tmpDir,
			r:           r,
		}
	})

	o.Spec("it returns the latest SHA", func(t TR) {
		t.spyExecutor.SetResults(
			[][]string{{"some-sha"}},
			[]error{nil},
		)

		sha, err := t.r.SHA("some-branch")
		Expect(t, err).To(BeNil())
		Expect(t, sha).To(Equal("some-sha"))

		Expect(t, t.spyExecutor.Paths()).To(Contain(path.Join(t.tmpDir, "c29tZS1wYXRo")))
		Expect(t, t.spyExecutor.Commands()).To(Contain([]string{
			"git", "rev-parse", "some-branch",
		}))
	})

	o.Spec("it returns an error if fetching the SHA fails", func(t TR) {
		t.spyExecutor.SetResults(
			[][]string{nil},
			[]error{errors.New("some-error")},
		)

		_, err := t.r.SHA("some-branch")
		Expect(t, err).To(Not(BeNil()))
	})

	o.Spec("it returns an error if fetching the SHA returns empty results", func(t TR) {
		t.spyExecutor.SetResults(
			[][]string{nil},
			[]error{nil},
		)

		_, err := t.r.SHA("some-branch")
		Expect(t, err).To(Not(BeNil()))
	})

	o.Spec("it returns an error if fetching the file contents fails", func(t TR) {
		t.spyExecutor.SetResults(
			[][]string{nil},
			[]error{errors.New("some-error")},
		)

		_, err := t.r.File("some-sha", "some-path")
		Expect(t, err).To(Not(BeNil()))
	})

	o.Spec("it returns the file contents", func(t TR) {
		t.spyExecutor.SetResults(
			[][]string{{"some-contents", "some-other-contents"}},
			[]error{nil},
		)

		sha, err := t.r.File("some-sha", "some-path")
		Expect(t, err).To(BeNil())
		Expect(t, sha).To(Equal("some-contents\nsome-other-contents"))

		Expect(t, t.spyExecutor.Paths()).To(Contain(path.Join(t.tmpDir, "c29tZS1wYXRo")))
		Expect(t, t.spyExecutor.Commands()).To(Contain([]string{
			"git", "show", "some-sha:some-path",
		}))
	})

	o.Spec("it returns the branches", func(t TR) {
		t.spyExecutor.SetResults(
			[][]string{{"branch-1", "remotes/origin/HEAD -> origin/master", "     remotes/origin/branch-1", "remotes/origin/branch-2 "}},
			[]error{nil},
		)

		branches, err := t.r.ListBranches()
		Expect(t, err).To(BeNil())
		Expect(t, branches).To(Equal([]string{"remotes/origin/branch-1", "remotes/origin/branch-2"}))

		Expect(t, t.spyExecutor.Paths()).To(Contain(path.Join(t.tmpDir, "c29tZS1wYXRo")))
		Expect(t, t.spyExecutor.Commands()).To(Contain([]string{
			"git", "branch", "-a",
		}))
	})

	o.Spec("it returns an error if fetching the branches fails", func(t TR) {
		t.spyExecutor.SetResults(
			[][]string{nil},
			[]error{errors.New("some-error")},
		)

		_, err := t.r.ListBranches()
		Expect(t, err).To(Not(BeNil()))
	})

	o.Spec("it clones if dir doesn't yet exist", func(t TR) {
		Expect(t, t.spyExecutor.Paths()).To(Contain(t.tmpDir))
		Expect(t, t.spyExecutor.Commands()).To(Contain([]string{
			"git", "clone", "some-path", "c29tZS1wYXRo",
		}))
	})

	o.Spec("it does not clone if dir does exist", func(t TR) {
		tmpDir, err := ioutil.TempDir("", "")
		Expect(t, err).To(BeNil())
		Expect(t, os.Mkdir(path.Join(tmpDir, "c29tZS1wYXRo"), os.ModePerm)).To(BeNil())
		spyExecutor := newSpyExecutor()

		_, err = git.NewRepo("some-path", tmpDir, time.Millisecond, spyExecutor, t.spyMetrics)
		Expect(t, err).To(BeNil())

		Expect(t, spyExecutor.Commands()).To(Not(Contain([]string{
			"git", "clone", "some-path", "c29tZS1wYXRo",
		})))
	})

	o.Spec("it returns an error when cloning fails", func(t TR) {
		spyExecutor := newSpyExecutor()
		spyExecutor.errs = []error{errors.New("some-error")}
		spyExecutor.results = [][]string{nil}

		_, err := git.NewRepo("some-path", t.tmpDir, time.Millisecond, spyExecutor, t.spyMetrics)
		Expect(t, err).To(Not(BeNil()))
	})

	o.Spec("it periodically does a fetch all from the remote", func(t TR) {
		Expect(t, t.spyExecutor.Paths).To(ViaPolling(Contain(path.Join(t.tmpDir, "c29tZS1wYXRo"))))
		Expect(t, t.spyExecutor.Commands).To(ViaPolling(Contain([]string{
			"git", "fetch", "--all",
		})))

		Expect(t, t.spyMetrics.GetDelta("GitFetchAllSuccess")).To(ViaPolling(Not(Equal(uint64(0)))))
	})

	o.Spec("it reports a failure when fetching", func(t TR) {
		t.spyExecutor.SetResults(
			[][]string{nil, nil},
			[]error{nil, errors.New("some-error")},
		)
		Expect(t, t.spyExecutor.Paths).To(ViaPolling(Contain(t.tmpDir)))
		Expect(t, t.spyExecutor.Commands).To(ViaPolling(Contain([]string{
			"git", "fetch", "--all",
		})))

		Expect(t, t.spyMetrics.GetDelta("GitFetchAllFailure")).To(ViaPolling(Not(Equal(uint64(0)))))
	})
}

type spyExecutor struct {
	mu       sync.Mutex
	commands [][]string
	paths    []string

	results [][]string
	errs    []error
}

func newSpyExecutor() *spyExecutor {
	return &spyExecutor{}
}

func (s *spyExecutor) Run(path string, commands ...string) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.paths = append(s.paths, path)
	s.commands = append(s.commands, commands)

	if len(s.results) != len(s.errs) {
		panic("out of sync")
	}

	if len(s.results) == 0 {
		return nil, nil
	}

	r, e := s.results[0], s.errs[0]
	s.results, s.errs = s.results[1:], s.errs[1:]

	return r, e
}

func (s *spyExecutor) SetResults(results [][]string, errs []error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.results = results
	s.errs = errs
}

func (s *spyExecutor) Paths() []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	r := make([]string, len(s.paths))
	copy(r, s.paths)
	return r
}

func (s *spyExecutor) Commands() [][]string {
	s.mu.Lock()
	defer s.mu.Unlock()

	r := make([][]string, len(s.commands))
	copy(r, s.commands)
	return r
}
