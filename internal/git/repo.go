package git

import (
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path"
	"strings"
	"sync"
	"time"
)

type Repo interface {
	SHA(branch string) (string, error)
	File(SHA, filePath string) (string, error)
	ListBranches() ([]string, error)
}

type repo struct {
	mu       sync.RWMutex
	exec     Executer
	repoPath string

	gitFetchSuccess func(uint64)
	gitFetchFail    func(uint64)

	gitSHASuccess func(uint64)
	gitSHAFailure func(uint64)

	gitFileSuccess func(uint64)
	gitFileFailure func(uint64)

	gitBranchesSuccess func(uint64)
	gitBranchesFailure func(uint64)
}

type Executer interface {
	Run(path string, commands ...string) ([]string, error)
}

type ExecutorFunc func(path string, commands ...string) ([]string, error)

func (f ExecutorFunc) Run(path string, commands ...string) ([]string, error) {
	return f(path, commands...)
}

type Metrics interface {
	NewCounter(name string) func(delta uint64)
}

func NewRepo(
	repoPath string,
	tmpPath string,
	interval time.Duration,
	e Executer,
	m Metrics,
) (Repo, error) {
	repoDirName := base64.RawURLEncoding.EncodeToString([]byte(repoPath))

	r := &repo{
		exec:     e,
		repoPath: path.Join(tmpPath, repoDirName),

		gitFetchSuccess:    m.NewCounter("GitFetchAllSuccess"),
		gitFetchFail:       m.NewCounter("GitFetchAllFailure"),
		gitSHASuccess:      m.NewCounter("GitSHASuccess"),
		gitSHAFailure:      m.NewCounter("GitSHAFailure"),
		gitFileSuccess:     m.NewCounter("GitFileSuccess"),
		gitFileFailure:     m.NewCounter("GitFileFailure"),
		gitBranchesSuccess: m.NewCounter("GitBranchesSuccess"),
		gitBranchesFailure: m.NewCounter("GitBranchesFailure"),
	}

	if !r.exists(r.repoPath) {
		_, err := r.exec.Run(
			tmpPath,
			"git", "clone", repoPath, repoDirName,
		)

		if err != nil {
			return nil, err
		}
	}

	go r.start(interval)

	return r, nil
}

func (r repo) SHA(branch string) (string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	results, err := r.exec.Run(
		r.repoPath,
		"git", "rev-parse", branch,
	)

	if err != nil {
		r.gitSHAFailure(1)
		return "", fmt.Errorf("|%s| %s: %s", r.repoPath, branch, err)
	}

	if len(results) == 0 {
		r.gitSHAFailure(1)
		return "", errors.New("empty results")
	}

	r.gitSHASuccess(1)
	return results[0], nil
}

func (r repo) File(SHA, filePath string) (string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	results, err := r.exec.Run(
		r.repoPath,
		"git", "show", fmt.Sprintf("%s:%s", SHA, filePath),
	)

	if err != nil {
		r.gitFileFailure(1)
		return "", err
	}

	r.gitFileSuccess(1)
	return strings.Join(results, "\n"), nil
}

func (r repo) ListBranches() ([]string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	results, err := r.exec.Run(
		r.repoPath,
		"git", "branch", "-a",
	)

	if err != nil {
		r.gitBranchesFailure(1)
		return nil, err
	}

	var branches []string
	for _, result := range results {
		result = strings.TrimSpace(result)
		if !strings.HasPrefix(result, "remotes/origin") || strings.Contains(result, "->") {
			continue
		}
		branches = append(branches, result)
	}

	r.gitBranchesSuccess(1)
	return branches, nil
}

func (r repo) start(interval time.Duration) {
	for {
		func() {
			defer time.Sleep(interval)

			r.mu.Lock()
			defer r.mu.Unlock()

			_, err := r.exec.Run(
				r.repoPath,
				"git", "fetch", "--all",
			)
			if err != nil {
				r.gitFetchFail(1)
				return
			}
			r.gitFetchSuccess(1)
		}()
	}
}

func (r repo) exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
