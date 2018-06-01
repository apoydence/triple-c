package metrics_test

import (
	"context"
	"testing"

	"github.com/apoydence/onpar"
	. "github.com/apoydence/onpar/expect"
	. "github.com/apoydence/onpar/matchers"
	"github.com/apoydence/triple-c/internal/metrics"
)

type TS struct {
	*testing.T
	s *metrics.SHATracker
}

func TestSHATracker(t *testing.T) {
	t.Parallel()
	o := onpar.New()
	defer o.Run(t)

	o.BeforeEach(func(t *testing.T) TS {
		return TS{
			T: t,
			s: metrics.NewSHATracker(),
		}
	})

	o.Spec("it keeps track of SHAs", func(t TS) {
		t.s.Register(context.Background(), "some-name-1", "some-branch-1")("sha-1")
		t.s.Register(context.Background(), "some-name-2", "some-branch-2")("sha-2")

		ctx, cancel := context.WithCancel(context.Background())
		t.s.Register(ctx, "some-name-2", "some-branch-3")("sha-3")

		Expect(t, t.s.RepoInfo()).To(And(
			Contain(metrics.RepoInfo{
				Repo:   "some-name-1",
				Branch: "some-branch-1",
				SHA:    "sha-1",
			}),
			Contain(metrics.RepoInfo{
				Repo:   "some-name-2",
				Branch: "some-branch-2",
				SHA:    "sha-2",
			}),
			Contain(metrics.RepoInfo{
				Repo:   "some-name-2",
				Branch: "some-branch-3",
				SHA:    "sha-3",
			}),
		))

		cancel()

		Expect(t, t.s.RepoInfo).To(ViaPolling(Not(
			Contain(metrics.RepoInfo{
				Repo:   "some-name-2",
				Branch: "some-branch-3",
				SHA:    "sha-3",
			}),
		)))
	})

	o.Spec("it deals with the context being cancelled and the function being invoked", func(t TS) {
		ctx, cancel := context.WithCancel(context.Background())
		f := t.s.Register(ctx, "some-name", "some-branch")
		f("sha")

		Expect(t, t.s.RepoInfo).To(ViaPolling(
			Contain(metrics.RepoInfo{
				Repo:   "some-name",
				Branch: "some-branch",
				SHA:    "sha",
			}),
		))

		cancel()

		Expect(t, t.s.RepoInfo).To(ViaPolling(
			Not(Contain(metrics.RepoInfo{
				Repo:   "some-name",
				Branch: "some-branch",
				SHA:    "sha",
			})),
		))

		Expect(t, func() []metrics.RepoInfo {
			f("sha")
			return t.s.RepoInfo()
		}).To(Always(Not(
			Contain(metrics.RepoInfo{
				Repo:   "some-name",
				Branch: "some-branch",
				SHA:    "sha",
			}),
		)))
	})

	o.Spec("it survives the race detector", func(t TS) {
		for i := 0; i < 2; i++ {
			go func() {
				for i := 0; i < 100; i++ {
					ctx, cancel := context.WithCancel(context.Background())
					t.s.Register(ctx, "some-name", "some-branch")
					go cancel()
				}
			}()
		}

		go func() {
			for i := 0; i < 100; i++ {
				t.s.RepoInfo()
			}
		}()

		for i := 0; i < 100; i++ {
			t.s.RepoInfo()
		}
	})
}
