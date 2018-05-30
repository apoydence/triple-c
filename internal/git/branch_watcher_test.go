package git_test

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
	"github.com/apoydence/triple-c/internal/git"
)

type TB struct {
	*testing.T
	spyBranchLister *spyBranchLister
	spyMetrics      *spyMetrics
	branches        [][]string
	mu              *sync.Mutex
}

func (t *TB) Branches() [][]string {
	t.mu.Lock()
	defer t.mu.Unlock()

	results := make([][]string, len(t.branches))
	copy(results, t.branches)

	return results
}

func TestBranchWatcher(t *testing.T) {
	t.Parallel()
	o := onpar.New()
	defer o.Run(t)

	o.BeforeEach(func(t *testing.T) *TB {
		return &TB{
			T:               t,
			spyBranchLister: newSpyBranchLister(),
			spyMetrics:      newSpyMetrics(),
			mu:              &sync.Mutex{},
		}
	})

	o.Spec("invokes the function with branch names", func(t *TB) {
		t.spyBranchLister.errs = []error{nil, nil}
		t.spyBranchLister.branches = [][]string{
			{"some-branch", "some-other-branch"},
			{"some-other-branch"},
		}

		startBranchWalker(t)

		Expect(t, t.Branches).To(ViaPolling(
			Equal([][]string{{"some-branch", "some-other-branch"}, {"some-other-branch"}})),
		)
	})

	o.Spec("stops watching when context is canceled", func(t *TB) {
		t.spyBranchLister.errs = []error{nil, nil, nil}

		t.spyBranchLister.branches = [][]string{
			{"some-branch"},
			{"some-branch"},
			{"some-other-branch"},
		}

		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		startBranchWalkerWithContext(ctx, t)

		Expect(t, t.Branches).To(Always(HaveLen(0)))
	})

	o.Spec("it keeps track of how many errors it has encountered", func(t *TB) {
		t.spyBranchLister.errs = []error{errors.New("some-error")}
		t.spyBranchLister.branches = [][]string{{}}

		startBranchWalker(t)

		Expect(t, t.spyMetrics.GetDelta("GithubErrs")).To(ViaPolling(Equal(uint64(1))))
		Expect(t, t.spyMetrics.GetDelta("GithubReads")()).To(Not(Equal(uint64(0))))
	})
}

func startBranchWalkerWithContext(ctx context.Context, t *TB) {
	git.StartBranchWatcher(
		ctx,
		t.spyBranchLister,
		func(branches []string) {
			t.mu.Lock()
			defer t.mu.Unlock()
			t.branches = append(t.branches, branches)
		},
		time.Nanosecond,
		t.spyMetrics,
		log.New(ioutil.Discard, "", 0),
	)
}

func startBranchWalker(t *TB) {
	git.StartBranchWatcher(
		context.Background(),
		t.spyBranchLister,
		func(branches []string) {
			t.mu.Lock()
			defer t.mu.Unlock()
			t.branches = append(t.branches, branches)
		},
		time.Nanosecond,
		t.spyMetrics,
		log.New(ioutil.Discard, "", 0),
	)
}

type spyBranchLister struct {
	mu sync.Mutex

	branches [][]string
	errs     []error
}

func newSpyBranchLister() *spyBranchLister {
	return &spyBranchLister{}
}

func (s *spyBranchLister) ListBranches() ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.branches) != len(s.errs) {
		panic("out of sync")
	}

	if len(s.branches) == 0 {
		return nil, nil
	}

	c, err := s.branches[0], s.errs[0]
	s.branches, s.errs = s.branches[1:], s.errs[1:]

	return c, err
}
