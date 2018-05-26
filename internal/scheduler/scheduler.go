package scheduler

type Scheduler struct {
	m TaskManager

	currentTasks []Task
}

type Tasks struct {
	Tasks []Task `yaml:"tasks"`
}

type Task struct {
	RepoOwner string `yaml:"repo_owner"`
	RepoName  string `yaml:"repo_name"`
	Command   string `yaml:"command"`
}

type TaskManager interface {
	Add(t Task)
	Remove(t Task)
}

func New(m TaskManager) *Scheduler {
	return &Scheduler{
		m: m,
	}
}

func (s *Scheduler) SetTasks(ts []Task) {
	var newCurrent []Task
	defer func() {
		s.currentTasks = newCurrent
	}()

	for _, t := range ts {
		if s.findTask(t, s.currentTasks) {
			continue
		}
		newCurrent = append(newCurrent, t)
		s.m.Add(t)
	}

	for _, t := range s.currentTasks {
		if s.findTask(t, ts) {
			continue
		}
		s.m.Remove(t)
	}
}

func (s *Scheduler) findTask(t Task, ts []Task) bool {
	for _, tt := range ts {
		if tt == t {
			return true
		}
	}

	return false
}
