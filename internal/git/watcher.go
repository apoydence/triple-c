package git

import (
	"context"
	"log"
	"time"
)

type Watcher struct {
	commit func(SHA string)
	repo   Repo
	branch string

	gitReads func(delta uint64)
	gitErrs  func(delta uint64)
	log      *log.Logger
}

type Metrics interface {
	NewCounter(name string) func(delta uint64)
}

type SHATracker interface {
	Register(ctx context.Context, repoName, branch string) func(SHA string)
}

func StartWatcher(
	ctx context.Context,
	repoName string,
	branch string,
	commit func(SHA string),
	interval time.Duration,
	repo Repo,
	shaTracker SHATracker,
	m Metrics,
	log *log.Logger,
) {
	tracker := shaTracker.Register(ctx, repoName, branch)

	w := &Watcher{
		commit: commit,
		branch: branch,
		repo:   repo,
		log:    log,

		gitReads: m.NewCounter("GitReads"),
		gitErrs:  m.NewCounter("GitErrs"),
	}

	go w.start(ctx, interval, tracker)
}

func (w *Watcher) start(ctx context.Context, interval time.Duration, tracker func(SHA string)) {
	var last string
	for {
		if ctx.Err() != nil {
			return
		}
		last = w.readSHA(last)
		tracker(last)

		time.Sleep(interval)
	}
}

func (w *Watcher) readSHA(lastSHA string) string {
	w.gitReads(1)

	sha, err := w.repo.SHA(w.branch)
	if err != nil {
		w.log.Printf("failed to read SHA: %s", err)
		w.gitErrs(1)
		return lastSHA
	}

	if sha == "" || sha == lastSHA {
		return lastSHA
	}

	w.commit(sha)
	return sha
}
