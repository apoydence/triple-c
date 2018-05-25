package gitwatcher_test

import (
	"context"
	"errors"
	"io/ioutil"
	"log"
	"math"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/apoydence/onpar"
	. "github.com/apoydence/onpar/expect"
	. "github.com/apoydence/onpar/matchers"
	"github.com/apoydence/triple-c/internal/gitwatcher"
	"github.com/google/go-github/github"
)

type TW struct {
	*testing.T
	spyCommitLister *spyCommitLister
	spyMetrics      *spyMetrics
	shas            []string
	mu              *sync.Mutex
}

func (t *TW) Shas() []string {
	t.mu.Lock()
	defer t.mu.Unlock()

	results := make([]string, len(t.shas))
	copy(results, t.shas)

	return results
}

func TestWatcher(t *testing.T) {
	t.Parallel()
	o := onpar.New()
	defer o.Run(t)

	o.BeforeEach(func(t *testing.T) *TW {
		return &TW{
			T:               t,
			spyCommitLister: newSpyCommitLister(),
			spyMetrics:      newSpyMetrics(),
			mu:              &sync.Mutex{},
		}
	})

	o.Spec("invokes the function with the newest sha", func(t *TW) {
		sha1 := "some-sha"
		sha2 := "some-other-sha"
		t.spyCommitLister.errs = []error{nil, nil, nil}
		t.spyCommitLister.commits = [][]*github.RepositoryCommit{
			{{
				SHA: &sha1,
			}},
			{{
				SHA: &sha1,
			}},
			{{
				SHA: &sha2,
			}},
		}
		t.spyCommitLister.resps = []*github.Response{
			{
				Response: &http.Response{
					StatusCode: 200,
				},
			},
			{
				Response: &http.Response{
					StatusCode: 200,
				},
			},
			{
				Response: &http.Response{
					StatusCode: 200,
				},
			},
		}

		startWatcher(t)

		Expect(t, t.Shas).To(ViaPolling(Equal([]string{"some-sha", "some-other-sha"})))
	})

	o.Spec("handles empty responses and nil commits", func(t *TW) {
		t.spyCommitLister.errs = []error{nil, nil}
		t.spyCommitLister.commits = [][]*github.RepositoryCommit{
			{{
				SHA: nil,
			}},
			{{}},
		}
		t.spyCommitLister.resps = []*github.Response{
			{
				Response: &http.Response{
					StatusCode: 200,
				},
			},
			{
				Response: &http.Response{
					StatusCode: 200,
				},
			},
		}

		startWatcher(t)
	})

	o.Spec("requests information about the given repo", func(t *TW) {
		startWatcher(t)

		Expect(t, t.spyCommitLister.Owners).To(ViaPolling(Contain("some-owner")))
		Expect(t, t.spyCommitLister.Repos).To(ViaPolling(Contain("some-repo")))

		// Have a deadline set
		_, haveDeadline := t.spyCommitLister.Ctxs()[0].Deadline()
		Expect(t, haveDeadline).To(BeTrue())

		Expect(t, t.spyCommitLister.Opts()[0]).To(Not(BeNil()))
		Expect(t, t.spyCommitLister.Opts()[0].PerPage).To(Equal(1))
		Expect(t, t.spyCommitLister.Opts()[0].Page).To(Equal(math.MaxInt64))
	})

	o.Spec("it keeps track of how many errors it has encountered", func(t *TW) {
		t.spyCommitLister.errs = []error{errors.New("some-error")}
		t.spyCommitLister.commits = [][]*github.RepositoryCommit{nil}
		t.spyCommitLister.resps = []*github.Response{nil}

		startWatcher(t)

		Expect(t, t.spyMetrics.GetDelta("GithubErrs")).To(ViaPolling(Equal(uint64(1))))
		Expect(t, t.spyMetrics.GetDelta("GithubReads")()).To(Not(Equal(uint64(0))))
	})

	o.Spec("it keeps track of how many non-200s it has encountered", func(t *TW) {
		t.spyCommitLister.errs = []error{nil}
		t.spyCommitLister.commits = [][]*github.RepositoryCommit{nil}
		t.spyCommitLister.resps = []*github.Response{
			{
				Response: &http.Response{
					StatusCode: 500,
				},
			},
		}

		startWatcher(t)

		Expect(t, t.spyMetrics.GetDelta("GithubErrs")).To(ViaPolling(Equal(uint64(1))))
	})
}

func startWatcher(t *TW) {
	gitwatcher.StartWatcher(
		"some-owner",
		"some-repo",
		time.Millisecond,
		t.spyCommitLister,
		func(sha string) {
			t.mu.Lock()
			defer t.mu.Unlock()
			t.shas = append(t.shas, sha)
		},
		t.spyMetrics,
		log.New(ioutil.Discard, "", 0),
	)
}

type spyCommitLister struct {
	mu     sync.Mutex
	owners []string
	repos  []string
	ctxs   []context.Context
	opts   []*github.CommitsListOptions

	commits [][]*github.RepositoryCommit
	resps   []*github.Response
	errs    []error
}

func newSpyCommitLister() *spyCommitLister {
	return &spyCommitLister{}
}

func (s *spyCommitLister) ListCommits(
	ctx context.Context,
	owner string,
	repo string,
	opt *github.CommitsListOptions,
) ([]*github.RepositoryCommit, *github.Response, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.owners = append(s.owners, owner)
	s.repos = append(s.repos, repo)
	s.ctxs = append(s.ctxs, ctx)
	s.opts = append(s.opts, opt)

	if len(s.commits) != len(s.resps) || len(s.commits) != len(s.errs) {
		panic("out of sync")
	}

	if len(s.commits) == 0 {
		return nil, &github.Response{
			Response: &http.Response{StatusCode: 200},
		}, nil
	}

	c, r, err := s.commits[0], s.resps[0], s.errs[0]
	s.commits, s.resps, s.errs = s.commits[1:], s.resps[1:], s.errs[1:]

	return c, r, err
}

func (s *spyCommitLister) Owners() []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	results := make([]string, len(s.owners))
	copy(results, s.owners)

	return results
}

func (s *spyCommitLister) Repos() []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	results := make([]string, len(s.repos))
	copy(results, s.repos)

	return results
}

func (s *spyCommitLister) Ctxs() []context.Context {
	s.mu.Lock()
	defer s.mu.Unlock()

	results := make([]context.Context, len(s.ctxs))
	copy(results, s.ctxs)

	return results
}

func (s *spyCommitLister) Opts() []*github.CommitsListOptions {
	s.mu.Lock()
	defer s.mu.Unlock()

	results := make([]*github.CommitsListOptions, len(s.opts))
	copy(results, s.opts)

	return results
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
