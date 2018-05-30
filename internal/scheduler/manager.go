package scheduler

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"path"
	"sync"
	"time"

	"github.com/apoydence/triple-c/internal/git"
)

type Manager struct {
	log             *log.Logger
	m               Metrics
	successfulTasks func(delta uint64)
	failedTasks     func(delta uint64)
	failedRepos     func(delta uint64)
	dedupedTasks    func(delta uint64)
	appGuid         string
	branch          string

	taskCreator TaskCreator

	startWatcher GitWatcher
	repoRegistry RepoRegistry

	mu   sync.Mutex
	ctxs map[Task]func()
}

type GitWatcher func(
	ctx context.Context,
	branch string,
	commit func(SHA string),
	interval time.Duration,
	shaFetcher git.SHAFetcher,
	m git.Metrics,
	log *log.Logger,
)

type TaskCreator interface {
	CreateTask(
		command string,
		name string,
		appGuid string,
	) error

	ListTasks(appGuid string) ([]string, error)
}

type Metrics interface {
	NewCounter(name string) func(delta uint64)
}

type RepoRegistry interface {
	FetchRepo(path string) (*git.Repo, error)
}

func NewManager(
	ctx context.Context,
	appGuid string,
	branch string,
	tc TaskCreator,
	w GitWatcher,
	repoRegistry RepoRegistry,
	m Metrics,
	log *log.Logger,
) *Manager {

	successfulTasks := m.NewCounter("SuccessfulTasks")
	failedTasks := m.NewCounter("FailedTasks")
	dedupedTasks := m.NewCounter("DedupedTasks")
	failedRepos := m.NewCounter("FailedRepos")

	return &Manager{
		log:          log,
		startWatcher: w,
		repoRegistry: repoRegistry,
		appGuid:      appGuid,
		branch:       branch,
		m:            m,

		taskCreator: tc,

		successfulTasks: successfulTasks,
		failedTasks:     failedTasks,
		failedRepos:     failedRepos,
		dedupedTasks:    dedupedTasks,

		ctxs: make(map[Task]func()),
	}
}

func (m *Manager) Add(t Task) {
	m.log.Printf("Adding task: %+v", t)
	ctx, cancel := context.WithCancel(context.Background())
	m.ctxs[t] = cancel

	repo, err := m.repoRegistry.FetchRepo(t.RepoPath)
	if err != nil {
		m.log.Printf("failed to fetch repo %s: %s", t.RepoPath, err)
		m.failedRepos(1)
		return
	}

	m.startWatcher(
		ctx,
		m.branch,
		func(SHA string) {
			m.mu.Lock()
			_, ok := m.ctxs[t]
			m.mu.Unlock()
			if !ok {
				return
			}

			dupe, err := m.duplicate(SHA)
			if err != nil {
				m.log.Printf("failed deduping tasks: %s", err)
				return
			}

			if dupe {
				m.log.Printf("skipping task for %s", SHA)
				m.dedupedTasks(1)
				return
			}

			m.log.Printf("starting task for %s", SHA)
			defer m.log.Printf("done with task for %s", SHA)

			name, err := json.Marshal(struct {
				SHA string `json:"sha"`
			}{
				SHA: SHA,
			})
			if err != nil {
				log.Print(err)
				return
			}

			err = m.taskCreator.CreateTask(
				m.fetchRepo(t.RepoPath, t.Command, m.branch),
				base64.StdEncoding.EncodeToString(name),
				m.appGuid,
			)
			if err != nil {
				m.log.Printf("task for %s failed: %s", SHA, err)
				m.failedTasks(1)
				return
			}

			m.successfulTasks(1)
		},
		time.Minute,
		repo,
		m.m,
		m.log,
	)
}

func (m *Manager) duplicate(SHA string) (bool, error) {
	tasks, err := m.taskCreator.ListTasks(m.appGuid)
	if err != nil {
		return false, err
	}

	for _, t := range tasks {
		data, err := base64.StdEncoding.DecodeString(t)
		if err != nil {
			continue
		}

		var taskMeta struct {
			SHA string `json:"sha"`
		}
		if err := json.Unmarshal(data, &taskMeta); err != nil {
			continue
		}

		if taskMeta.SHA == SHA {
			return true, nil
		}
	}

	return false, nil
}

func (m *Manager) Remove(t Task) {
	m.mu.Lock()
	defer m.mu.Unlock()
	cancel, ok := m.ctxs[t]
	if !ok {
		return
	}

	delete(m.ctxs, t)
	cancel()
}

// fetchRepo adds the cloning of a repo to the given command
func (m *Manager) fetchRepo(repoPath, command, branch string) string {
	return fmt.Sprintf(`#!/bin/bash
set -ex

rm -rf %s
git clone %s


# If checking out fails, its fine. Move forward with the default branch.
set +e

pushd %s
  git checkout %s
  git submodule update --init --recursive
popd

set +x

%s
	`,
		path.Base(repoPath),
		repoPath,
		path.Base(repoPath),
		branch,
		command,
	)
}
