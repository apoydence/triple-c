package handlers_test

import (
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/poy/onpar"
	. "github.com/poy/onpar/expect"
	. "github.com/poy/onpar/matchers"
	"github.com/poy/triple-c/internal/handlers"
	"github.com/poy/triple-c/internal/metrics"
)

type TB struct {
	*testing.T
	b             http.Handler
	recorder      *httptest.ResponseRecorder
	spyRepoLister *spyRepoLister
}

func TestRepos(t *testing.T) {
	t.Parallel()
	o := onpar.New()
	defer o.Run(t)

	o.BeforeEach(func(t *testing.T) TB {
		spyRepoLister := newSpyRepoLister()
		return TB{
			T:             t,
			b:             handlers.NewRepos(spyRepoLister, log.New(ioutil.Discard, "", 0)),
			recorder:      httptest.NewRecorder(),
			spyRepoLister: spyRepoLister,
		}
	})

	o.Spec("it returns a 405 for anything other than a GET", func(t TB) {
		req, err := http.NewRequest("PUT", "http://some.url/v1/repos", nil)
		Expect(t, err).To(BeNil())
		t.b.ServeHTTP(t.recorder, req)

		Expect(t, t.recorder.Code).To(Equal(http.StatusMethodNotAllowed))
	})

	o.Spec("it returns a 404 for non /v1/repos", func(t TB) {
		req, err := http.NewRequest("GET", "http://some.url/invalid", nil)
		Expect(t, err).To(BeNil())
		t.b.ServeHTTP(t.recorder, req)

		Expect(t, t.recorder.Code).To(Equal(http.StatusNotFound))
	})

	o.Spec("it returns all the repos and their SHAs", func(t TB) {
		t.spyRepoLister.results = []metrics.RepoInfo{
			{
				Repo:   "repo-1",
				Branch: "some-branch",
				SHA:    "sha-1",
			},
			{
				Repo:   "repo-1",
				Branch: "some-other-branch",
				SHA:    "sha-2",
			},
			{
				Repo:   "repo-2",
				Branch: "some-branch",
				SHA:    "sha-1",
			},
		}

		req, err := http.NewRequest("GET", "http://some.url/v1/repos", nil)
		Expect(t, err).To(BeNil())
		t.b.ServeHTTP(t.recorder, req)

		Expect(t, t.recorder.Code).To(Equal(http.StatusOK))
		Expect(t, t.recorder.Body.String()).To(MatchJSON(`{
			"repos": {
				"repo-1": {
					"some-branch":{
						"sha":"sha-1"
					},
					"some-other-branch":{
						"sha":"sha-2"
					}
				},
				"repo-2": {
					"some-branch":{
						"sha":"sha-1"
					}
				}
			}
		}`))
	})
}

type spyRepoLister struct {
	results []metrics.RepoInfo
}

func newSpyRepoLister() *spyRepoLister {
	return &spyRepoLister{}
}

func (s *spyRepoLister) RepoInfo() []metrics.RepoInfo {
	return s.results
}
