package git

import (
	"context"
	"log"
	"time"
)

type BranchWatcher struct {
	lister   BranchLister
	callback func(branches []string)
	backoff  time.Duration

	gitReads func(delta uint64)
	gitErrs  func(delta uint64)
	log      *log.Logger
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

		gitReads: m.NewCounter("GitBranchReads"),
		gitErrs:  m.NewCounter("GitBranchErrs"),
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
	w.gitReads(1)

	branches, err := w.lister.ListBranches()

	if err != nil {
		w.log.Printf("failed to read branches: %s", err)
		w.gitErrs(1)
		return
	}

	if len(branches) == 0 {
		return
	}

	w.callback(branches)
}
