package git_test

import (
	"errors"
	"io/ioutil"
	"testing"

	"github.com/apoydence/onpar"
	. "github.com/apoydence/onpar/expect"
	. "github.com/apoydence/onpar/matchers"
	"github.com/apoydence/triple-c/internal/git"
)

type TRR struct {
	*testing.T
	r           *git.RepoRegistry
	spyExecutor *spyExecutor
}

func TestRepoRegistry(t *testing.T) {
	t.Parallel()
	o := onpar.New()
	defer o.Run(t)

	o.BeforeEach(func(t *testing.T) TRR {
		tmpDir, err := ioutil.TempDir("", "")
		Expect(t, err).To(BeNil())

		spyExecutor := newSpyExecutor()

		return TRR{
			T:           t,
			spyExecutor: spyExecutor,
			r:           git.NewRepoRegistry(tmpDir, spyExecutor, newSpyMetrics()),
		}
	})

	o.Spec("it returns a new repo for each new path", func(t TRR) {
		repo1, err := t.r.FetchRepo("some-path-1")
		Expect(t, err).To(BeNil())
		repo2, err := t.r.FetchRepo("some-path-2")
		Expect(t, err).To(BeNil())
		repo3, err := t.r.FetchRepo("some-path-1")
		Expect(t, err).To(BeNil())

		Expect(t, repo1).To(Not(Equal(repo2)))
		Expect(t, repo1).To(Equal(repo3))
	})

	o.Spec("it returns an error when creating the repo fails", func(t TRR) {
		t.spyExecutor.SetResults(
			"git clone some-path-1 c29tZS1wYXRoLTE",
			nil,
			errors.New("some-error"),
		)

		_, err := t.r.FetchRepo("some-path-1")
		Expect(t, err).To(Not(BeNil()))
	})

	o.Spec("it survives the race detector", func(t TRR) {
		go func() {
			for i := 0; i < 100; i++ {
				t.r.FetchRepo("some-path-1")
			}
		}()

		for i := 0; i < 100; i++ {
			t.r.FetchRepo("some-path-1")
		}
	})
}
