package capi_test

import (
	"errors"
	"net/http"
	"testing"

	"github.com/poy/onpar"
	. "github.com/poy/onpar/expect"
	. "github.com/poy/onpar/matchers"
	"github.com/poy/triple-c/internal/capi"
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

	o.Spec("it fetches a new token and tries again (once) for a non-200 status code", func(t TH) {
		t.spyDoer.m["GET:http://some.url"] = &http.Response{
			StatusCode: 403,
		}
		_, err := t.c.Do(t.req)
		Expect(t, err).To(BeNil())

		Expect(t, t.stubTokenFetcher.called).To(Equal(2))
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
