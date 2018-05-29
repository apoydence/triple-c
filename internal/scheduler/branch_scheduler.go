package scheduler

import "sync"

type BranchScheduler struct {
	mu sync.Mutex
	m  BranchTaskManager

	currentBranches []string
}

type BranchTaskManager interface {
	Add(branch string)
	Remove(branch string)
}

func NewBranchScheduler(m BranchTaskManager) *BranchScheduler {
	return &BranchScheduler{
		m: m,
	}
}

func (s *BranchScheduler) SetBranches(branches []string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var newCurrent []string
	defer func() {
		s.currentBranches = newCurrent
	}()

	for _, t := range branches {
		if s.findBranch(t, s.currentBranches) {
			continue
		}
		newCurrent = append(newCurrent, t)
		s.m.Add(t)
	}

	for _, t := range s.currentBranches {
		if s.findBranch(t, branches) {
			continue
		}
		s.m.Remove(t)
	}
}

func (s *BranchScheduler) findBranch(t string, branches []string) bool {
	for _, tt := range branches {
		if tt == t {
			return true
		}
	}

	return false
}
