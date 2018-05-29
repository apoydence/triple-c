package gitwatcher

import (
	"context"
	"log"
	"time"
)

type BranchWatcher struct {
	lister   BranchLister
	callback func(branches []string)
	backoff  time.Duration

	githubReads func(delta uint64)
	githubErrs  func(delta uint64)
	log         *log.Logger
}

type BranchLister interface {
	ListBranches() ([]string, error)
}

func StartBranchWatcher(
	ctx context.Context,
	lister BranchLister,
	callback func(branches []string),
	backoff time.Duration,
	m Metrics,
	log *log.Logger,
) {
	w := &BranchWatcher{
		lister:   lister,
		callback: callback,
		log:      log,
		backoff:  backoff,

		githubReads: m.NewCounter("GithubReads"),
		githubErrs:  m.NewCounter("GithubErrs"),
	}

	go w.start(ctx)
}

func (w *BranchWatcher) start(ctx context.Context) {
	for {
		if ctx.Err() != nil {
			return
		}
		w.readFromGit()
	}
}

func (w *BranchWatcher) readFromGit() {
	defer time.Sleep(w.backoff)
	w.githubReads(1)

	branches, err := w.lister.ListBranches()

	if err != nil {
		w.log.Printf("failed to read branches from github: %s", err)
		w.githubErrs(1)
		return
	}

	if len(branches) == 0 {
		return
	}

	w.callback(branches)
}
