package capi_test

import (
	"errors"
	"net/http"
	"testing"

	"github.com/apoydence/onpar"
	. "github.com/apoydence/onpar/expect"
	. "github.com/apoydence/onpar/matchers"
	"github.com/apoydence/triple-c/internal/capi"
)

type TH struct {
	*testing.T
	spyDoer          *spyDoer
	stubTokenFetcher *stubTokenFetcher
	c                *capi.HTTPClient
	rxReqs           []*http.Request

	req *http.Request
}

func TestHTTPClient(t *testing.T) {
	t.Parallel()
	o := onpar.New()
	defer o.Run(t)

	o.BeforeEach(func(t *testing.T) TH {
		spyDoer := newSpyDoer()
		stubTokenFetcher := newStubTokenFetcher()

		req, err := http.NewRequest("GET", "http://some.url", nil)
		if err != nil {
			panic(err)
		}

		return TH{
			T:                t,
			stubTokenFetcher: stubTokenFetcher,
			spyDoer:          spyDoer,
			c:                capi.NewHTTPClient(spyDoer, stubTokenFetcher),
			req:              req,
		}
	})

	o.Spec("it puts the Authorization header on each request", func(t TH) {
		t.stubTokenFetcher.token = "some-token"
		t.c.Do(t.req)
		Expect(t, t.spyDoer.req).To(Not(BeNil()))
		Expect(t, t.spyDoer.req.Header.Get("Authorization")).To(Equal("some-token"))

		t.c.Do(t.req)
		Expect(t, t.stubTokenFetcher.called).To(Equal(1))
	})

	o.Spec("it returns an error if the token fetcher returns an error", func(t TH) {
		t.stubTokenFetcher.err = errors.New("some-error")
		_, err := t.c.Do(t.req)
		Expect(t, err).To(Not(BeNil()))
	})

	o.Spec("it returns an error if the child-Doer returns an error", func(t TH) {
		t.spyDoer.err = errors.New("some-error")
		_, err := t.c.Do(t.req)
		Expect(t, err).To(Not(BeNil()))
	})
}

type stubTokenFetcher struct {
	called int
	token  string
	err    error
}

func newStubTokenFetcher() *stubTokenFetcher {
	return &stubTokenFetcher{}
}

func (s *stubTokenFetcher) GetToken() (string, error) {
	s.called++
	return s.token, s.err
}
