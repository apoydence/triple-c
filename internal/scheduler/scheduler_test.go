package scheduler_test

import (
	"sync"
	"testing"

	"github.com/apoydence/onpar"
	. "github.com/apoydence/onpar/expect"
	. "github.com/apoydence/onpar/matchers"
	"github.com/apoydence/triple-c/internal/scheduler"
)

type TS struct {
	*testing.T
	spyTaskManager *spyTaskManager
	s              *scheduler.Scheduler
}

func TestScheduler(t *testing.T) {
	t.Parallel()
	o := onpar.New()
	defer o.Run(t)

	o.BeforeEach(func(t *testing.T) TS {
		spyTaskManager := newSpyTaskManager()
		return TS{
			T:              t,
			spyTaskManager: spyTaskManager,
			s:              scheduler.New(spyTaskManager),
		}
	})

	o.Spec("it adds new tasks to the manager", func(t TS) {
		t.s.SetTasks([]scheduler.Task{
			{
				RepoOwner: "a",
			},
			{
				RepoOwner: "b",
			},
		})

		Expect(t, t.spyTaskManager.adds).To(HaveLen(2))
		Expect(t, t.spyTaskManager.adds).To(Contain(
			scheduler.Task{RepoOwner: "a"},
			scheduler.Task{RepoOwner: "b"},
		))
	})

	o.Spec("it does not add the same task twice", func(t TS) {
		t.s.SetTasks([]scheduler.Task{
			{
				RepoOwner: "a",
			},
		})
		t.s.SetTasks([]scheduler.Task{
			{
				RepoOwner: "a",
			},
		})

		Expect(t, t.spyTaskManager.adds).To(HaveLen(1))
		Expect(t, t.spyTaskManager.adds).To(Contain(
			scheduler.Task{RepoOwner: "a"},
		))
	})

	o.Spec("it removes stale tasks", func(t TS) {
		t.s.SetTasks([]scheduler.Task{
			{
				RepoOwner: "a",
			},
		})
		t.s.SetTasks([]scheduler.Task{
			{
				RepoOwner: "b",
			},
		})

		Expect(t, t.spyTaskManager.removes).To(HaveLen(1))
		Expect(t, t.spyTaskManager.removes).To(Contain(
			scheduler.Task{RepoOwner: "a"},
		))
	})
}

type spyTaskManager struct {
	mu      sync.Mutex
	adds    []scheduler.Task
	removes []scheduler.Task
}

func newSpyTaskManager() *spyTaskManager {
	return &spyTaskManager{}
}

func (s *spyTaskManager) Add(t scheduler.Task) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.adds = append(s.adds, t)
}

func (s *spyTaskManager) Remove(t scheduler.Task) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.removes = append(s.removes, t)
}

func (s *spyTaskManager) Adds() []scheduler.Task {
	s.mu.Lock()
	defer s.mu.Unlock()

	r := make([]scheduler.Task, len(s.adds))
	copy(r, s.adds)
	return r
}

func (s *spyTaskManager) Removes() []scheduler.Task {
	s.mu.Lock()
	defer s.mu.Unlock()

	r := make([]scheduler.Task, len(s.removes))
	copy(r, s.removes)
	return r
}
