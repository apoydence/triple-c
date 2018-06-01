package handlers

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/apoydence/triple-c/internal/metrics"
)

type Repos struct {
	l   RepoLister
	log *log.Logger
}

type RepoLister interface {
	RepoInfo() []metrics.RepoInfo
}

type RepoListerFunc func() []metrics.RepoInfo

func (f RepoListerFunc) RepoInfo() []metrics.RepoInfo {
	return f()
}

func NewRepos(l RepoLister, log *log.Logger) http.Handler {
	return &Repos{
		l:   l,
		log: log,
	}
}

func (b *Repos) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	if r.URL.Path != "/v1/repos" {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	var results struct {
		Repos map[string]map[string]struct {
			SHA string `json:"sha"`
		} `json:"repos"`
	}

	results.Repos = make(map[string]map[string]struct {
		SHA string `json:"sha"`
	})

	for _, info := range b.l.RepoInfo() {
		m, ok := results.Repos[info.Repo]
		if !ok {
			m = make(map[string]struct {
				SHA string `json:"sha"`
			})
			results.Repos[info.Repo] = m
		}

		m[info.Branch] = struct {
			SHA string `json:"sha"`
		}{
			SHA: info.SHA,
		}
	}

	data, err := json.Marshal(results)
	if err != nil {
		b.log.Panicf("failed to marshal results: %s", err)
	}

	w.Write(data)
}
