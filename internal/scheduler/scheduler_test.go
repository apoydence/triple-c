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
		t.s.SetTasks([]scheduler.MetaTask{
			{
				Task: scheduler.Task{RepoPath: "a"},
			},
			{
				Task: scheduler.Task{RepoPath: "b"},
			},
		})

		Expect(t, t.spyTaskManager.adds).To(HaveLen(2))
		Expect(t, t.spyTaskManager.adds).To(Contain(
			scheduler.MetaTask{Task: scheduler.Task{RepoPath: "a"}},
			scheduler.MetaTask{Task: scheduler.Task{RepoPath: "b"}},
		))
	})

	o.Spec("it does not add the same task twice", func(t TS) {
		t.s.SetTasks([]scheduler.MetaTask{
			{
				Task: scheduler.Task{RepoPath: "a"},
			},
		})
		t.s.SetTasks([]scheduler.MetaTask{
			{
				Task: scheduler.Task{RepoPath: "a"},
			},
		})

		Expect(t, t.spyTaskManager.adds).To(HaveLen(1))
		Expect(t, t.spyTaskManager.adds).To(Contain(
			scheduler.MetaTask{Task: scheduler.Task{RepoPath: "a"}},
		))
	})

	o.Spec("it does not keep track of DoOnce tasks", func(t TS) {
		t.s.SetTasks([]scheduler.MetaTask{
			{
				Task:   scheduler.Task{RepoPath: "a"},
				DoOnce: true,
			},
		})
		t.s.SetTasks([]scheduler.MetaTask{
			{
				Task:   scheduler.Task{RepoPath: "a"},
				DoOnce: true,
			},
		})

		Expect(t, t.spyTaskManager.adds).To(HaveLen(2))
	})

	o.Spec("it removes stale tasks", func(t TS) {
		t.s.SetTasks([]scheduler.MetaTask{
			{
				Task: scheduler.Task{RepoPath: "a"},
			},
		})
		t.s.SetTasks([]scheduler.MetaTask{
			{
				Task: scheduler.Task{RepoPath: "b"},
			},
		})

		Expect(t, t.spyTaskManager.removes).To(HaveLen(1))
		Expect(t, t.spyTaskManager.removes).To(Contain(
			scheduler.MetaTask{Task: scheduler.Task{RepoPath: "a"}},
		))
	})
}

type spyTaskManager struct {
	mu      sync.Mutex
	adds    []scheduler.MetaTask
	removes []scheduler.MetaTask
}

func newSpyTaskManager() *spyTaskManager {
	return &spyTaskManager{}
}

func (s *spyTaskManager) Add(t scheduler.MetaTask) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.adds = append(s.adds, t)
}

func (s *spyTaskManager) Remove(t scheduler.MetaTask) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.removes = append(s.removes, t)
}

func (s *spyTaskManager) Adds() []scheduler.MetaTask {
	s.mu.Lock()
	defer s.mu.Unlock()

	r := make([]scheduler.MetaTask, len(s.adds))
	copy(r, s.adds)
	return r
}

func (s *spyTaskManager) Removes() []scheduler.MetaTask {
	s.mu.Lock()
	defer s.mu.Unlock()

	r := make([]scheduler.MetaTask, len(s.removes))
	copy(r, s.removes)
	return r
}
