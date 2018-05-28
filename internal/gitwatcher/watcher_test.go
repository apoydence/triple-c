package gitwatcher_test

import (
	"context"
	"errors"
	"io/ioutil"
	"log"
	"sync"
	"testing"
	"time"

	"github.com/apoydence/onpar"
	. "github.com/apoydence/onpar/expect"
	. "github.com/apoydence/onpar/matchers"
	"github.com/apoydence/triple-c/internal/gitwatcher"
)

type TW struct {
	*testing.T
	spySHAFetcher *spySHAFetcher
	spyMetrics    *spyMetrics
	shas          []string
	mu            *sync.Mutex
}

func (t *TW) Shas() []string {
	t.mu.Lock()
	defer t.mu.Unlock()

	results := make([]string, len(t.shas))
	copy(results, t.shas)

	return results
}

func TestWatcher(t *testing.T) {
	t.Parallel()
	o := onpar.New()
	defer o.Run(t)

	o.BeforeEach(func(t *testing.T) *TW {
		return &TW{
			T:             t,
			spySHAFetcher: newSpySHAFetcher(),
			spyMetrics:    newSpyMetrics(),
			mu:            &sync.Mutex{},
		}
	})

	o.Spec("invokes the function with the newest sha", func(t *TW) {
		t.spySHAFetcher.errs = []error{nil, nil, nil}
		t.spySHAFetcher.shas = []string{"sha1", "sha1", "sha2"}
		startWatcher(t)

		Expect(t, t.Shas).To(ViaPolling(Equal([]string{"sha1", "sha2"})))

		Expect(t, t.spyMetrics.GetDelta("GitErrs")).To(ViaPolling(Equal(uint64(0))))
		Expect(t, t.spyMetrics.GetDelta("GitReads")()).To(Not(Equal(uint64(0))))
	})

	o.Spec("stops watching when context is canceled", func(t *TW) {
		t.spySHAFetcher.errs = []error{nil, nil, nil}
		t.spySHAFetcher.shas = []string{"sha1", "sha1", "sha2"}

		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		startWatcherWithContext(ctx, t)

		Expect(t, t.Shas).To(Always(HaveLen(0)))
	})

	o.Spec("it keeps track of how many errors it has encountered", func(t *TW) {
		t.spySHAFetcher.errs = []error{errors.New("some-error")}
		t.spySHAFetcher.shas = []string{""}

		startWatcher(t)

		Expect(t, t.spyMetrics.GetDelta("GitErrs")).To(ViaPolling(Equal(uint64(1))))
		Expect(t, t.spyMetrics.GetDelta("GitReads")()).To(Not(Equal(uint64(0))))
	})
}

func startWatcherWithContext(ctx context.Context, t *TW) {
	gitwatcher.StartWatcher(
		ctx,
		func(sha string) {
			t.mu.Lock()
			defer t.mu.Unlock()
			t.shas = append(t.shas, sha)
		},
		time.Millisecond,
		t.spySHAFetcher,
		t.spyMetrics,
		log.New(ioutil.Discard, "", 0),
	)
}

func startWatcher(t *TW) {
	gitwatcher.StartWatcher(
		context.Background(),
		func(sha string) {
			t.mu.Lock()
			defer t.mu.Unlock()
			t.shas = append(t.shas, sha)
		},
		time.Millisecond,
		t.spySHAFetcher,
		t.spyMetrics,
		log.New(ioutil.Discard, "", 0),
	)
}

type spyMetrics struct {
	mu sync.Mutex
	m  map[string]uint64
}

func newSpyMetrics() *spyMetrics {
	return &spyMetrics{
		m: make(map[string]uint64),
	}
}

func (s *spyMetrics) NewCounter(name string) func(uint64) {
	return func(delta uint64) {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.m[name] += delta
	}
}

func (s *spyMetrics) GetDelta(name string) func() uint64 {
	return func() uint64 {
		s.mu.Lock()
		defer s.mu.Unlock()
		return s.m[name]
	}
}

type spySHAFetcher struct {
	mu   sync.Mutex
	shas []string
	errs []error
}

func newSpySHAFetcher() *spySHAFetcher {
	return &spySHAFetcher{}
}

func (s *spySHAFetcher) SHA() (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.shas) != len(s.errs) {
		panic("out of sync")
	}

	if len(s.shas) == 0 {
		return "", nil
	}

	sha, e := s.shas[0], s.errs[0]
	s.shas, s.errs = s.shas[1:], s.errs[1:]

	return sha, e
}
