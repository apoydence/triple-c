package git

import (
	"context"
	"log"
	"time"
)

type Watcher struct {
	commit     func(SHA string)
	shaFetcher SHAFetcher

	gitReads func(delta uint64)
	gitErrs  func(delta uint64)
	log      *log.Logger
}

type SHAFetcher interface {
	SHA() (string, error)
	Name() string
	CurrentBranch() string
}

type Metrics interface {
	NewCounter(name string) func(delta uint64)
}

type SHATracker interface {
	Register(ctx context.Context, repoName, branch string) func(SHA string)
}

func StartWatcher(
	ctx context.Context,
	commit func(SHA string),
	interval time.Duration,
	shaFetcher SHAFetcher,
	shaTracker SHATracker,
	m Metrics,
	log *log.Logger,
) {
	tracker := shaTracker.Register(ctx, shaFetcher.Name(), shaFetcher.CurrentBranch())

	w := &Watcher{
		commit:     commit,
		shaFetcher: shaFetcher,
		log:        log,

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

	sha, err := w.shaFetcher.SHA()
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
