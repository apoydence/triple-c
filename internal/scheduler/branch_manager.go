package scheduler

import (
	"context"
	"sync"
)

type BranchManager struct {
	onStart func(ctx context.Context, branch string)
	mu      sync.Mutex
	ctxs    map[string]func()
}

func NewBranchManager(onStart func(ctx context.Context, branch string)) *BranchManager {
	return &BranchManager{
		onStart: onStart,
		ctxs:    make(map[string]func()),
	}
}

func (m *BranchManager) Add(branch string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.ctxs[branch]; ok {
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	m.ctxs[branch] = cancel
	m.onStart(ctx, branch)
}

func (m *BranchManager) Remove(branch string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if cancel, ok := m.ctxs[branch]; ok {
		cancel()
	}
}
