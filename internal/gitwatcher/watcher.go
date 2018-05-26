package gitwatcher

import (
	"context"
	"log"
	"math"
	"net/http"
	"time"

	"github.com/google/go-github/github"
)

type Watcher struct {
	owner  string
	repo   string
	lister CommitLister
	commit func(SHA string)

	githubReads func(delta uint64)
	githubErrs  func(delta uint64)
	log         *log.Logger
}

type Metrics interface {
	NewCounter(name string) func(delta uint64)
}

type CommitLister interface {
	ListCommits(
		ctx context.Context,
		owner string,
		repo string,
		opt *github.CommitsListOptions,
	) ([]*github.RepositoryCommit, *github.Response, error)
}

func StartWatcher(
	ctx context.Context,
	owner string,
	repo string,
	lister CommitLister,
	commit func(SHA string),
	m Metrics,
	log *log.Logger,
) {
	w := &Watcher{
		owner:  owner,
		repo:   repo,
		lister: lister,
		commit: commit,
		log:    log,

		githubReads: m.NewCounter("GithubReads"),
		githubErrs:  m.NewCounter("GithubErrs"),
	}

	go w.start(ctx)
}

func (w *Watcher) start(ctx context.Context) {
	w.log.Printf("starting git watcher for %s/%s", w.owner, w.repo)
	defer w.log.Printf("done watching for %s/%s", w.owner, w.repo)

	var last string
	for {
		if ctx.Err() != nil {
			return
		}
		last = w.readFromGithub(last)
	}
}

func (w *Watcher) readFromGithub(lastSHA string) string {
	w.log.Printf("Reading from Github (%s/%s)...", w.owner, w.repo)
	w.githubReads(1)

	ctx, _ := context.WithTimeout(context.Background(), 5*time.Second)
	commits, resp, err := w.lister.ListCommits(ctx, w.owner, w.repo, &github.CommitsListOptions{
		ListOptions: github.ListOptions{
			PerPage: 1,
			Page:    math.MaxInt64,
		},
	})

	if err != nil {
		w.log.Printf("failed to read commits from github: %s", err)
		w.githubErrs(1)
		return lastSHA
	}

	defer func() {
		if resp == nil || resp.Rate.Remaining == 0 {
			return
		}

		wait := time.Duration(int64(resp.Reset.Sub(time.Now())) / int64(resp.Rate.Remaining))
		w.log.Printf("Rate Limit: %v", wait)

		time.Sleep(wait)
	}()

	if resp.StatusCode != http.StatusOK {
		w.log.Printf("unexpected status code from github: %d", resp.StatusCode)
		w.githubErrs(1)
		return lastSHA
	}

	if len(commits) == 0 || commits[0].SHA == nil {
		return lastSHA
	}

	SHA := *commits[0].SHA
	if SHA == lastSHA {
		return SHA
	}

	w.commit(SHA)
	return SHA
}
