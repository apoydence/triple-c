package scheduler

type Scheduler struct {
	m TaskManager

	currentPlans []MetaPlan
}

type MetaPlan struct {
	Plan
	DoOnce bool
}

type Plans struct {
	Plans []Plan `yaml:"plans"`
}

type Plan struct {
	Name      string            `yaml:"name"`
	RepoPaths map[string]string `yaml:"repo_paths"`
	Tasks     []Task            `yaml:"tasks"`
}

type Task struct {
	Name       string            `yaml:"name"`
	Command    string            `yaml:"command"`
	Parameters map[string]string `yaml:"parameters"`
}

type TaskManager interface {
	Add(t MetaPlan)
	Remove(t MetaPlan)
}

func New(m TaskManager) *Scheduler {
	return &Scheduler{
		m: m,
	}
}

func (s *Scheduler) SetPlans(plans []MetaPlan) {
	var newCurrent []MetaPlan

	for _, t := range plans {
		if s.findPlan(t, s.currentPlans) {
			continue
		}
		if !t.DoOnce {
			newCurrent = append(newCurrent, t)
		}
		s.m.Add(t)
	}

	for _, t := range s.currentPlans {
		if s.findPlan(t, plans) {
			continue
		}
		s.m.Remove(t)
	}

	s.currentPlans = newCurrent
}

func (s *Scheduler) findPlan(plan MetaPlan, plans []MetaPlan) bool {
	ep := encodePlan(plan)
	for _, p := range plans {
		if encodePlan(p) == ep {
			return true
		}
	}

	return false
}
