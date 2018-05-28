package gitwatcher

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

type Repo struct {
	mu       sync.RWMutex
	exec     Executer
	repoPath string

	gitFetchSuccess func(uint64)
	gitFetchFail    func(uint64)
}

type Executer interface {
	Run(path string, commands ...string) ([]string, error)
}

type ExecutorFunc func(path string, commands ...string) ([]string, error)

func (f ExecutorFunc) Run(path string, commands ...string) ([]string, error) {
	return f(path, commands...)
}

func NewRepo(
	repoPath string,
	tmpPath string,
	interval time.Duration,
	e Executer,
	m Metrics,
) (*Repo, error) {
	repoDirName := base64.RawURLEncoding.EncodeToString([]byte(repoPath))

	r := &Repo{
		exec:     e,
		repoPath: path.Join(tmpPath, repoDirName),

		gitFetchSuccess: m.NewCounter("GitFetchAllSuccess"),
		gitFetchFail:    m.NewCounter("GitFetchAllFailure"),
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

func (r *Repo) SHA() (string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	results, err := r.exec.Run(
		r.repoPath,
		"git", "rev-parse", "HEAD",
	)

	if err != nil {
		return "", err
	}

	if len(results) == 0 {
		return "", errors.New("empty results")
	}

	return results[0], nil
}

func (r *Repo) File(SHA, filePath string) (string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	results, err := r.exec.Run(
		r.repoPath,
		"git", "show", fmt.Sprintf("%s:%s", SHA, filePath),
	)

	if err != nil {
		return "", err
	}

	return strings.Join(results, "\n"), nil
}

func (r *Repo) start(interval time.Duration) {
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

func (r *Repo) exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
