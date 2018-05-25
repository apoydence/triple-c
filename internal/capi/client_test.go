package capi_test

import (
	"bytes"
	"errors"
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/apoydence/onpar"
	. "github.com/apoydence/onpar/expect"
	. "github.com/apoydence/onpar/matchers"
	"github.com/apoydence/triple-c/internal/capi"
)

type TC struct {
	*testing.T
	spyDoer *spyDoer
	c       *capi.Client
}

func TestClient(t *testing.T) {
	t.Parallel()
	o := onpar.New()
	defer o.Run(t)

	o.BeforeEach(func(t *testing.T) TC {
		spyDoer := newSpyDoer()
		return TC{
			T:       t,
			spyDoer: spyDoer,
			c:       capi.NewClient("http://some-addr.com", spyDoer),
		}
	})

	o.Spec("it hits CAPI correct", func(t TC) {
		err := t.c.CreateTask("some-command", "some-name", "some-guid")
		Expect(t, err).To(BeNil())

		Expect(t, t.spyDoer.req.Method).To(Equal("POST"))
		Expect(t, t.spyDoer.req.URL.String()).To(Equal("http://some-addr.com/v3/apps/some-guid/tasks"))
		Expect(t, t.spyDoer.req.Header.Get("Content-Type")).To(Equal("application/json"))
		Expect(t, t.spyDoer.body).To(MatchJSON(`{"command":"some-command"}`))
	})

	o.Spec("it returns an error if a non-202 is received", func(t TC) {
		t.spyDoer.resp = &http.Response{
			StatusCode: 500,
			Body:       ioutil.NopCloser(bytes.NewReader(nil)),
		}
		err := t.c.CreateTask("some-command", "some-name", "some-guid")
		Expect(t, err).To(Not(BeNil()))
	})

	o.Spec("it returns an error if the addr is invalid", func(t TC) {
		t.c = capi.NewClient("::invalid", t.spyDoer)
		err := t.c.CreateTask("some-command", "some-name", "some-guid")
		Expect(t, err).To(Not(BeNil()))
	})

	o.Spec("it returns an error if the request fails", func(t TC) {
		t.spyDoer.err = errors.New("some-error")
		err := t.c.CreateTask("some-command", "some-name", "some-guid")
		Expect(t, err).To(Not(BeNil()))
	})
}

type spyDoer struct {
	req  *http.Request
	body []byte

	resp *http.Response
	err  error
}

func newSpyDoer() *spyDoer {
	return &spyDoer{
		resp: &http.Response{
			StatusCode: 202,
			Body:       ioutil.NopCloser(bytes.NewReader(nil)),
		},
	}
}

func (s *spyDoer) Do(req *http.Request) (*http.Response, error) {
	s.req = req

	if req.Body != nil {
		body, err := ioutil.ReadAll(req.Body)
		if err != nil {
			panic(err)
		}
		s.body = body
	}

	return s.resp, s.err
}
