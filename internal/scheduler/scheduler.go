package scheduler

type Scheduler struct {
	m TaskManager

	currentTasks []MetaTask
}

type Tasks struct {
	Tasks []Task `yaml:"tasks"`
}

type MetaTask struct {
	Task
	DoOnce bool
}

type Task struct {
	RepoPath   string            `yaml:"repo_path"`
	Command    string            `yaml:"command"`
	Parameters map[string]string `yaml:"parameters"`
}

type TaskManager interface {
	Add(t MetaTask)
	Remove(t MetaTask)
}

func New(m TaskManager) *Scheduler {
	return &Scheduler{
		m: m,
	}
}

func (s *Scheduler) SetTasks(ts []MetaTask) {
	var newCurrent []MetaTask

	for _, t := range ts {
		if s.findTask(t, s.currentTasks) {
			continue
		}
		if !t.DoOnce {
			newCurrent = append(newCurrent, t)
		}
		s.m.Add(t)
	}

	for _, t := range s.currentTasks {
		if s.findTask(t, ts) {
			continue
		}
		s.m.Remove(t)
	}

	s.currentTasks = newCurrent
}

func (s *Scheduler) findTask(t MetaTask, ts []MetaTask) bool {
	et := encodeTask(t)
	for _, tt := range ts {
		if encodeTask(tt) == et {
			return true
		}
	}

	return false
}
