package gitwatcher

import (
	"sync"
	"time"
)

type RepoRegistry struct {
	mu sync.Mutex

	m      map[string]*Repo
	tmpDir string

	exec    Executer
	metrics Metrics
}

func NewRepoRegistry(tmpDir string, e Executer, m Metrics) *RepoRegistry {
	return &RepoRegistry{
		tmpDir:  tmpDir,
		m:       make(map[string]*Repo),
		exec:    e,
		metrics: m,
	}
}

func (r *RepoRegistry) FetchRepo(repoPath string) (*Repo, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	repo, ok := r.m[repoPath]
	if ok {
		return repo, nil
	}

	repo, err := NewRepo(repoPath, r.tmpDir, time.Minute, r.exec, r.metrics)
	if err != nil {
		return nil, err
	}
	r.m[repoPath] = repo

	return repo, nil
}
