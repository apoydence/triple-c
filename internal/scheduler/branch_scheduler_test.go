package scheduler_test

import (
	"sync"
	"testing"

	"github.com/poy/onpar"
	. "github.com/poy/onpar/expect"
	. "github.com/poy/onpar/matchers"
	"github.com/poy/triple-c/internal/scheduler"
)

type TB struct {
	*testing.T
	spyBranchTaskManager *spyBranchTaskManager
	s                    *scheduler.BranchScheduler
}

func TestBranchScheduler(t *testing.T) {
	t.Parallel()
	o := onpar.New()
	defer o.Run(t)

	o.BeforeEach(func(t *testing.T) TB {
		spyBranchTaskManager := newSpyBranchTaskManager()
		return TB{
			T:                    t,
			spyBranchTaskManager: spyBranchTaskManager,
			s:                    scheduler.NewBranchScheduler(spyBranchTaskManager),
		}
	})

	o.Spec("it adds new tasks to the manager", func(t TB) {
		t.s.SetBranches([]string{"a", "b"})

		Expect(t, t.spyBranchTaskManager.adds).To(HaveLen(2))
		Expect(t, t.spyBranchTaskManager.adds).To(Contain("a", "b"))
	})

	o.Spec("it does not add the same task twice", func(t TB) {
		t.s.SetBranches([]string{"a"})
		t.s.SetBranches([]string{"a"})

		Expect(t, t.spyBranchTaskManager.adds).To(HaveLen(1))
		Expect(t, t.spyBranchTaskManager.adds).To(Contain("a"))
	})

	o.Spec("it removes stale tasks", func(t TB) {
		t.s.SetBranches([]string{"a"})
		t.s.SetBranches([]string{"b"})

		Expect(t, t.spyBranchTaskManager.removes).To(HaveLen(1))
		Expect(t, t.spyBranchTaskManager.removes).To(Contain("a"))
	})

	o.Spec("it survives the race detector", func(t TB) {
		go func() {
			for i := 0; i < 100; i++ {
				t.s.SetBranches([]string{"a"})
			}
		}()
		for i := 0; i < 100; i++ {
			t.s.SetBranches([]string{"a"})
		}
	})
}

type spyBranchTaskManager struct {
	mu      sync.Mutex
	adds    []string
	removes []string
}

func newSpyBranchTaskManager() *spyBranchTaskManager {
	return &spyBranchTaskManager{}
}

func (s *spyBranchTaskManager) Add(t string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.adds = append(s.adds, t)
}

func (s *spyBranchTaskManager) Remove(t string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.removes = append(s.removes, t)
}

func (s *spyBranchTaskManager) Adds() []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	r := make([]string, len(s.adds))
	copy(r, s.adds)
	return r
}

func (s *spyBranchTaskManager) Removes() []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	r := make([]string, len(s.removes))
	copy(r, s.removes)
	return r
}
