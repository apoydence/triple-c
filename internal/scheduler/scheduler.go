package scheduler

type Scheduler struct {
	m TaskManager

	currentTasks []Task
}

type Tasks struct {
	Tasks []Task `yaml:"tasks"`
}

type Task struct {
	RepoPath   string            `yaml:"repo_path"`
	Command    string            `yaml:"command"`
	Parameters map[string]string `yaml:"parameters"`
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
	et := encodeTask(t)
	for _, tt := range ts {
		if encodeTask(tt) == et {
			return true
		}
	}

	return false
}
