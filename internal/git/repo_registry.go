package git

import (
	"sync"
	"time"
)

type RepoRegistry struct {
	mu sync.Mutex

	m      map[string]Repo
	tmpDir string

	exec    Executer
	metrics Metrics
}

func NewRepoRegistry(tmpDir string, e Executer, m Metrics) *RepoRegistry {
	return &RepoRegistry{
		tmpDir:  tmpDir,
		m:       make(map[string]Repo),
		exec:    e,
		metrics: m,
	}
}

func (r *RepoRegistry) FetchRepo(repoPath string) (Repo, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	repo, ok := r.m[repoPath]
	if ok {
		return repo, nil
	}

	repo, err := NewRepo(repoPath, r.tmpDir, 15*time.Second, r.exec, r.metrics)
	if err != nil {
		return nil, err
	}
	r.m[repoPath] = repo

	return repo, nil
}

func (r *RepoRegistry) ListRepos() []Repo {
	r.mu.Lock()
	defer r.mu.Unlock()

	var repos []Repo
	for _, k := range r.m {
		repos = append(repos, k)
	}

	return repos
}
