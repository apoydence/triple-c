package scheduler

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/apoydence/triple-c/internal/gitwatcher"
)

type Manager struct {
	log             *log.Logger
	m               Metrics
	successfulTasks func(delta uint64)
	failedTasks     func(delta uint64)
	appGuid         string

	taskCreator TaskCreator

	startWatcher GitWatcher
	lister       gitwatcher.CommitLister

	mu   sync.Mutex
	ctxs map[Task]func()
}

type GitWatcher func(
	ctx context.Context,
	owner string,
	repo string,
	lister gitwatcher.CommitLister,
	commit func(SHA string),
	backoff time.Duration,
	m gitwatcher.Metrics,
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

func NewManager(
	appGuid string,
	tc TaskCreator,
	lister gitwatcher.CommitLister,
	w GitWatcher,
	m Metrics,
	log *log.Logger,
) *Manager {

	successfulTasks := m.NewCounter("SuccessfulTasks")
	failedTasks := m.NewCounter("FailedTasks")

	return &Manager{
		log:          log,
		startWatcher: w,
		appGuid:      appGuid,
		m:            m,
		lister:       lister,

		taskCreator:     tc,
		successfulTasks: successfulTasks,
		failedTasks:     failedTasks,
		ctxs:            make(map[Task]func()),
	}
}

func (m *Manager) Add(t Task) {
	log.Printf("Adding task: %+v", t)
	ctx, cancel := context.WithCancel(context.Background())
	m.ctxs[t] = cancel
	m.startWatcher(
		ctx,
		t.RepoOwner,
		t.RepoName,
		m.lister,
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
				m.fetchRepo(t.RepoOwner, t.RepoName, t.Command),
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
func (m *Manager) fetchRepo(owner, repo, command string) string {
	return fmt.Sprintf(`#!/bin/bash
set -ex

rm -rf %s
git clone https://github.com/%s/%s --recursive

set +ex

%s
	`,
		repo,
		owner,
		repo,
		command,
	)
}
