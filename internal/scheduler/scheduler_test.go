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
		t.s.SetPlans([]scheduler.MetaPlan{
			{
				Plan: scheduler.Plan{
					RepoPaths: map[string]string{"repo-a": "a"},
					Tasks:     []scheduler.Task{},
				},
			},
			{
				Plan: scheduler.Plan{
					RepoPaths: map[string]string{"repo-b": "b"},
					Tasks:     []scheduler.Task{},
				},
			},
		})

		Expect(t, t.spyTaskManager.adds).To(HaveLen(2))
		Expect(t, t.spyTaskManager.adds).To(Contain(
			scheduler.MetaPlan{
				Plan: scheduler.Plan{
					RepoPaths: map[string]string{"repo-a": "a"},
					Tasks:     []scheduler.Task{},
				},
			},
			scheduler.MetaPlan{
				Plan: scheduler.Plan{
					RepoPaths: map[string]string{"repo-b": "b"},
					Tasks:     []scheduler.Task{},
				},
			},
		))
	})

	o.Spec("it does not add the same task twice", func(t TS) {
		t.s.SetPlans([]scheduler.MetaPlan{
			{
				Plan: scheduler.Plan{
					RepoPaths: map[string]string{"repo-a": "a"},
					Tasks:     []scheduler.Task{},
				},
			},
		})
		t.s.SetPlans([]scheduler.MetaPlan{
			{
				Plan: scheduler.Plan{
					RepoPaths: map[string]string{"repo-a": "a"},
					Tasks:     []scheduler.Task{},
				},
			},
		})

		Expect(t, t.spyTaskManager.adds).To(HaveLen(1))
		Expect(t, t.spyTaskManager.adds).To(Contain(
			scheduler.MetaPlan{
				Plan: scheduler.Plan{
					RepoPaths: map[string]string{"repo-a": "a"},
					Tasks:     []scheduler.Task{},
				},
			},
		))
	})

	o.Spec("it does not keep track of DoOnce tasks", func(t TS) {
		t.s.SetPlans([]scheduler.MetaPlan{
			{
				Plan: scheduler.Plan{
					RepoPaths: map[string]string{"repo-a": "a"},
					Tasks:     []scheduler.Task{},
				},
				DoOnce: true,
			},
		})
		t.s.SetPlans([]scheduler.MetaPlan{
			{
				Plan: scheduler.Plan{
					RepoPaths: map[string]string{"repo-a": "a"},
					Tasks:     []scheduler.Task{},
				},
				DoOnce: true,
			},
		})

		Expect(t, t.spyTaskManager.adds).To(HaveLen(2))
	})

	o.Spec("it removes stale tasks", func(t TS) {
		t.s.SetPlans([]scheduler.MetaPlan{
			{
				Plan: scheduler.Plan{
					RepoPaths: map[string]string{"repo-a": "a"},
					Tasks:     []scheduler.Task{},
				},
			},
		})
		t.s.SetPlans([]scheduler.MetaPlan{
			{
				Plan: scheduler.Plan{
					RepoPaths: map[string]string{"repo-b": "b"},
					Tasks:     []scheduler.Task{},
				},
			},
		})

		Expect(t, t.spyTaskManager.removes).To(HaveLen(1))
		Expect(t, t.spyTaskManager.removes).To(Contain(
			scheduler.MetaPlan{
				Plan: scheduler.Plan{
					RepoPaths: map[string]string{"repo-a": "a"},
					Tasks:     []scheduler.Task{},
				},
			},
		))
	})
}

type spyTaskManager struct {
	mu      sync.Mutex
	adds    []scheduler.MetaPlan
	removes []scheduler.MetaPlan
}

func newSpyTaskManager() *spyTaskManager {
	return &spyTaskManager{}
}

func (s *spyTaskManager) Add(t scheduler.MetaPlan) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.adds = append(s.adds, t)
}

func (s *spyTaskManager) Remove(t scheduler.MetaPlan) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.removes = append(s.removes, t)
}

func (s *spyTaskManager) Adds() []scheduler.MetaPlan {
	s.mu.Lock()
	defer s.mu.Unlock()

	r := make([]scheduler.MetaPlan, len(s.adds))
	copy(r, s.adds)
	return r
}

func (s *spyTaskManager) Removes() []scheduler.MetaPlan {
	s.mu.Lock()
	defer s.mu.Unlock()

	r := make([]scheduler.MetaPlan, len(s.removes))
	copy(r, s.removes)
	return r
}
