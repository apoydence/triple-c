package metrics

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type RepoInfo struct {
	Repo   string
	Branch string
	SHA    string
}

type SHATracker struct {
	mu sync.RWMutex
	m  map[string]*RepoInfo
}

func NewSHATracker() *SHATracker {
	return &SHATracker{
		m: make(map[string]*RepoInfo),
	}
}

func (t *SHATracker) RepoInfo() []RepoInfo {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var results []RepoInfo
	for _, v := range t.m {
		results = append(results, *v)
	}

	return results
}

func (t *SHATracker) Register(ctx context.Context, repoName, branch string) func(SHA string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	key := fmt.Sprintf("%s:%s:%d", repoName, branch, time.Now().UnixNano())
	t.m[key] = &RepoInfo{
		Repo:   repoName,
		Branch: branch,
	}

	go func() {
		<-ctx.Done()
		t.mu.Lock()
		defer t.mu.Unlock()
		delete(t.m, key)
	}()

	return func(SHA string) {
		t.mu.Lock()
		defer t.mu.Unlock()

		if ctx.Err() != nil {
			return
		}

		t.m[key].SHA = SHA
	}
}
