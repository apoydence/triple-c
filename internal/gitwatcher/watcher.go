package gitwatcher

import (
	"context"
	"log"
	"time"
)

type Watcher struct {
	commit     func(SHA string)
	shaFetcher SHAFetcher
	branch     string

	gitReads func(delta uint64)
	gitErrs  func(delta uint64)
	log      *log.Logger
}

type SHAFetcher interface {
	SHA(branch string) (string, error)
}

type Metrics interface {
	NewCounter(name string) func(delta uint64)
}

func StartWatcher(
	ctx context.Context,
	branch string,
	commit func(SHA string),
	interval time.Duration,
	shaFetcher SHAFetcher,
	m Metrics,
	log *log.Logger,
) {
	w := &Watcher{
		commit:     commit,
		branch:     branch,
		shaFetcher: shaFetcher,
		log:        log,

		gitReads: m.NewCounter("GitReads"),
		gitErrs:  m.NewCounter("GitErrs"),
	}

	go w.start(ctx, interval)
}

func (w *Watcher) start(ctx context.Context, interval time.Duration) {
	var last string
	for {
		if ctx.Err() != nil {
			return
		}
		last = w.readSHA(last)
		time.Sleep(interval)
	}
}

func (w *Watcher) readSHA(lastSHA string) string {
	w.gitReads(1)

	sha, err := w.shaFetcher.SHA(w.branch)
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
