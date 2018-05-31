package git_test

import (
	"errors"
	"io/ioutil"
	"os"
	"path"
	"strings"
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
			"git rev-parse some-branch",
			[]string{"some-sha"},
			nil,
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
			"git rev-parse some-branch",
			nil,
			errors.New("some-error"),
		)

		_, err := t.r.SHA("some-branch")
		Expect(t, err).To(Not(BeNil()))
	})

	o.Spec("it returns an error if fetching the SHA returns empty results", func(t TR) {
		t.spyExecutor.SetResults(
			"git rev-parse some-branch",
			nil,
			nil,
		)

		_, err := t.r.SHA("some-branch")
		Expect(t, err).To(Not(BeNil()))
	})

	o.Spec("it returns the file contents", func(t TR) {
		t.spyExecutor.SetResults(
			"git show some-sha:some-path",
			[]string{"some-contents", "some-other-contents"},
			nil,
		)

		sha, err := t.r.File("some-sha", "some-path")
		Expect(t, err).To(BeNil())
		Expect(t, sha).To(Equal("some-contents\nsome-other-contents"))

		Expect(t, t.spyExecutor.Paths()).To(Contain(path.Join(t.tmpDir, "c29tZS1wYXRo")))
		Expect(t, t.spyExecutor.Commands()).To(Contain([]string{
			"git", "show", "some-sha:some-path",
		}))
	})

	o.Spec("it returns an error if fetching the file contents fails", func(t TR) {
		t.spyExecutor.SetResults(
			"git show some-sha:some-path",
			nil,
			errors.New("some-error"),
		)

		_, err := t.r.File("some-sha", "some-path")
		Expect(t, err).To(Not(BeNil()))
	})

	o.Spec("it returns the branches", func(t TR) {
		t.spyExecutor.SetResults(
			"git branch -a",
			[]string{
				"branch-1",
				"remotes/origin/HEAD -> origin/master",
				"     remotes/origin/branch-1", "remotes/origin/branch-2 ",
			},
			nil,
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
			"git branch -a",
			nil,
			errors.New("some-error"),
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
		spyExecutor.SetResults(
			"git clone some-path c29tZS1wYXRo",
			nil,
			errors.New("some-error"),
		)

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
			"git fetch --all",
			nil,
			errors.New("some-error"),
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

	m map[string]struct {
		results []string
		err     error
	}
}

func newSpyExecutor() *spyExecutor {
	return &spyExecutor{
		m: make(map[string]struct {
			results []string
			err     error
		}),
	}
}

func (s *spyExecutor) Run(path string, commands ...string) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.paths = append(s.paths, path)
	s.commands = append(s.commands, commands)

	results, ok := s.m[strings.Join(commands, " ")]
	if !ok {
		return nil, nil
	}

	return results.results, results.err
}

func (s *spyExecutor) SetResults(command string, results []string, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.m[command] = struct {
		results []string
		err     error
	}{
		results: results,
		err:     err,
	}
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
