package git

import (
	"context"
	"log"
	"time"
)

type Watcher struct {
	commit   func(SHA string)
	repo     Repo
	repoName string
	branch   string

	log *log.Logger
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
	log *log.Logger,
) {
	tracker := shaTracker.Register(ctx, repoName, branch)

	w := &Watcher{
		commit:   commit,
		repoName: repoName,
		branch:   branch,
		repo:     repo,
		log:      log,
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
	sha, err := w.repo.SHA(w.branch)
	if err != nil {
		w.log.Printf("failed to read SHA for %s on %s: %s", w.repoName, w.branch, err)
		return lastSHA
	}

	if sha == "" || sha == lastSHA {
		return lastSHA
	}

	w.commit(sha)
	return sha
}
