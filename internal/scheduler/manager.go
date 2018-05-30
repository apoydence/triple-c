package scheduler

import (
	"context"
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

			m.log.Printf("starting task for %s", SHA)
			defer m.log.Printf("done with task for %s", SHA)

			err := m.taskCreator.CreateTask(
				m.fetchRepo(t.RepoPath, t.Command, m.branch),
				"some-name",
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
