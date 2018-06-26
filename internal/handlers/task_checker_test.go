package handlers_test

import (
	"context"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"
	"time"

	logcache "code.cloudfoundry.org/go-log-cache"
	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
	faas "github.com/apoydence/cf-faas"
	gocapi "github.com/apoydence/go-capi"
	"github.com/apoydence/onpar"
	. "github.com/apoydence/onpar/expect"
	. "github.com/apoydence/onpar/matchers"
	"github.com/apoydence/triple-c/internal/handlers"
)

type TC struct {
	*testing.T
	c                faas.Handler
	spyTaskGetter    *spyTaskGetter
	spyTaskLogReader *spyTaskLogReader
	spyDoer          *spyDoer
}

func TestTaskChecker(t *testing.T) {
	t.Parallel()
	o := onpar.New()
	defer o.Run(t)

	o.BeforeEach(func(t *testing.T) TC {
		spyTaskGetter := newSpyTaskGetter()
		spyTaskLogReader := newSpyTaskLogReader()
		spyDoer := newSpyDoer()
		return TC{
			T:                t,
			spyTaskGetter:    spyTaskGetter,
			spyTaskLogReader: spyTaskLogReader,
			spyDoer:          spyDoer,
			c:                handlers.NewTaskChecker(spyTaskGetter, spyTaskLogReader, spyDoer),
		}
	})

	o.Spec("it returns a 200 and the given state", func(t TC) {
		t.spyTaskGetter.task = gocapi.Task{
			Command:   `echo '<--magic-identifier--> |12345|'`,
			State:     "some-state",
			CreatedAt: time.Unix(1, 2),
			Links: map[string]gocapi.Links{
				"app": gocapi.Links{
					Href:   "http://app.xx",
					Method: "some-method",
				},
			},
		}

		t.spyTaskLogReader.envelopes = [][]*loggregator_v2.Envelope{
			{
				buildLogEnvelope("<--magic-identifier--> |12345|", "guid-a"),
				buildLogEnvelope("<--magic-identifier--> |12400|", "guid-b"),
			}, // Force logcache.Walk to be used
			{
				buildLogEnvelope("msg-1", "guid-a"),
				buildLogEnvelope("wrong msg", "guid-b"),
				buildLogEnvelope("msg-2", "guid-a"),
			},
		}
		t.spyTaskLogReader.errs = []error{nil, nil}

		t.spyDoer.m["some-method:http://app.xx"] = &http.Response{
			StatusCode: 200,
			Body:       ioutil.NopCloser(strings.NewReader(`{"guid":"app-guid"}`)),
		}

		resp, err := t.c.Handle(faas.Request{
			Path: "/v1/some-path/tasks/some-guid",
		})
		Expect(t, err).To(BeNil())

		Expect(t, resp.StatusCode).To(Equal(http.StatusOK))

		var state map[string]interface{}
		Expect(t, json.Unmarshal(resp.Body, &state)).To(BeNil())
		Expect(t, state["state"]).To(Equal("some-state"))
		Expect(t, state["logs"]).To(Equal([]interface{}{"msg-1", "msg-2"}))

		Expect(t, t.spyTaskGetter.ctx).To(Not(BeNil()))
		deadline, ok := t.spyTaskGetter.ctx.Deadline()
		Expect(t, ok).To(BeTrue())
		Expect(t, float64(deadline.Sub(time.Now()))).To(And(
			BeAbove(0),
			BeBelow(float64(time.Minute)),
		))

		Expect(t, t.spyTaskGetter.guid).To(Equal("some-guid"))

		Expect(t, t.spyTaskLogReader.ctxs[0]).To(Not(BeNil()))
		deadline, ok = t.spyTaskLogReader.ctxs[0].Deadline()
		Expect(t, ok).To(BeTrue())
		Expect(t, float64(deadline.Sub(time.Now()))).To(And(
			BeAbove(0),
			BeBelow(float64(time.Minute)),
		))

		Expect(t, t.spyTaskLogReader.sourceIDs[0]).To(Equal("app-guid"))
		Expect(t, t.spyTaskLogReader.starts[0]).To(Equal(time.Unix(1, 2)))
	})

	o.Spec("it returns an empty body if it can't find the magic line", func(t TC) {
		t.spyTaskGetter.task = gocapi.Task{
			Command:   `echo '<--magic-identifier--> |12345|'`,
			State:     "some-state",
			CreatedAt: time.Unix(1, 2),
			Links: map[string]gocapi.Links{
				"app": gocapi.Links{
					Href:   "http://app.xx",
					Method: "some-method",
				},
			},
		}

		t.spyTaskLogReader.envelopes = [][]*loggregator_v2.Envelope{
			{
				buildLogEnvelope("<--magic-identifier--> |12400|", "guid-b"),
				buildLogEnvelope("msg-1", "guid-a"),
				buildLogEnvelope("wrong msg", "guid-b"),
				buildLogEnvelope("msg-2", "guid-a"),
			},
		}
		t.spyTaskLogReader.errs = []error{nil}

		t.spyDoer.m["some-method:http://app.xx"] = &http.Response{
			StatusCode: 200,
			Body:       ioutil.NopCloser(strings.NewReader(`{"guid":"app-guid"}`)),
		}

		resp, err := t.c.Handle(faas.Request{
			Path: "/v1/some-path/tasks/some-guid",
		})
		Expect(t, err).To(BeNil())

		Expect(t, resp.StatusCode).To(Equal(http.StatusOK))

		var state map[string]interface{}
		Expect(t, json.Unmarshal(resp.Body, &state)).To(BeNil())
		Expect(t, state["state"]).To(Equal("some-state"))
		Expect(t, state["logs"]).To(Equal([]interface{}{"unable to find magic line"}))
	})

	o.Spec("it returns an error if the TaskGetter returns an error", func(t TC) {
		t.spyTaskGetter.err = errors.New("some-error")

		_, err := t.c.Handle(faas.Request{})
		Expect(t, err).To(Not(BeNil()))
	})

	o.Spec("it displays the error if reading from the app link returns an error", func(t TC) {
		t.spyTaskGetter.task = gocapi.Task{
			Command:   `echo '<--magic-identifier--> |12345|'`,
			State:     "some-state",
			CreatedAt: time.Unix(1, 2),
			Links: map[string]gocapi.Links{
				"app": gocapi.Links{
					Href:   "http://app.xx",
					Method: "some-method",
				},
			},
		}

		t.spyTaskLogReader.envelopes = [][]*loggregator_v2.Envelope{
			{
				buildLogEnvelope("<--magic-identifier--> |12345|", "guid-a"),
				buildLogEnvelope("<--magic-identifier--> |12400|", "guid-b"),
				buildLogEnvelope("msg-1", "guid-a"),
				buildLogEnvelope("wrong msg", "guid-b"),
				buildLogEnvelope("msg-2", "guid-a"),
			},
		}

		t.spyDoer.m["some-method:http://app.xx"] = &http.Response{
			StatusCode: 200,
			Body:       ioutil.NopCloser(strings.NewReader(`{"guid":"app-guid"}`)),
		}

		t.spyDoer.err = errors.New("some-error")

		resp, err := t.c.Handle(faas.Request{})
		Expect(t, err).To(BeNil())

		var state map[string]interface{}
		Expect(t, json.Unmarshal(resp.Body, &state)).To(BeNil())
		Expect(t, state["state"]).To(Equal("some-state"))
		Expect(t, state["logs"]).To(Equal([]interface{}{"some-error"}))
	})

	o.Spec("it displays the error if the task doesn't have the app link", func(t TC) {
		t.spyTaskGetter.task = gocapi.Task{
			Command:   `echo '<--magic-identifier--> |12345|'`,
			State:     "some-state",
			CreatedAt: time.Unix(1, 2),
		}

		t.spyTaskLogReader.envelopes = [][]*loggregator_v2.Envelope{
			{
				buildLogEnvelope("<--magic-identifier--> |12345|", "guid-a"),
				buildLogEnvelope("<--magic-identifier--> |12400|", "guid-b"),
				buildLogEnvelope("msg-1", "guid-a"),
				buildLogEnvelope("wrong msg", "guid-b"),
				buildLogEnvelope("msg-2", "guid-a"),
			},
		}

		resp, err := t.c.Handle(faas.Request{})
		Expect(t, err).To(BeNil())

		var state map[string]interface{}
		Expect(t, json.Unmarshal(resp.Body, &state)).To(BeNil())
		Expect(t, state["state"]).To(Equal("some-state"))
		Expect(t, state["logs"]).To(Equal([]interface{}{"unable to find parent app guid"}))
	})

	o.Spec("it displays the error if the JSON from the app link is invalid", func(t TC) {
		t.spyTaskGetter.task = gocapi.Task{
			Command:   `echo '<--magic-identifier--> |12345|'`,
			State:     "some-state",
			CreatedAt: time.Unix(1, 2),
			Links: map[string]gocapi.Links{
				"app": gocapi.Links{
					Href:   "http://app.xx",
					Method: "some-method",
				},
			},
		}

		t.spyTaskLogReader.envelopes = [][]*loggregator_v2.Envelope{
			{
				buildLogEnvelope("<--magic-identifier--> |12345|", "guid-a"),
				buildLogEnvelope("<--magic-identifier--> |12400|", "guid-b"),
				buildLogEnvelope("msg-1", "guid-a"),
				buildLogEnvelope("wrong msg", "guid-b"),
				buildLogEnvelope("msg-2", "guid-a"),
			},
		}

		t.spyDoer.m["some-method:http://app.xx"] = &http.Response{
			StatusCode: 200,
			Body:       ioutil.NopCloser(strings.NewReader(`invalid`)),
		}

		resp, err := t.c.Handle(faas.Request{})
		Expect(t, err).To(BeNil())

		var state map[string]interface{}
		Expect(t, json.Unmarshal(resp.Body, &state)).To(BeNil())
		Expect(t, state["state"]).To(Equal("some-state"))
		Expect(t, state["logs"]).To(Equal([]interface{}{"invalid character 'i' looking for beginning of value"}))
	})
}

type spyTaskGetter struct {
	ctx  context.Context
	guid string

	task gocapi.Task
	err  error
}

func newSpyTaskGetter() *spyTaskGetter {
	return &spyTaskGetter{}
}

func (s *spyTaskGetter) GetTask(ctx context.Context, guid string) (gocapi.Task, error) {
	s.ctx = ctx
	s.guid = guid

	return s.task, s.err
}

type spyTaskLogReader struct {
	ctxs      []context.Context
	sourceIDs []string
	starts    []time.Time

	envelopes [][]*loggregator_v2.Envelope
	errs      []error
}

func newSpyTaskLogReader() *spyTaskLogReader {
	return &spyTaskLogReader{}
}

func (s *spyTaskLogReader) Read(ctx context.Context, sourceID string, start time.Time, opts ...logcache.ReadOption) ([]*loggregator_v2.Envelope, error) {
	s.ctxs = append(s.ctxs, ctx)
	s.sourceIDs = append(s.sourceIDs, sourceID)
	s.starts = append(s.starts, start)

	if len(s.envelopes) != len(s.errs) {
		panic("out of sync")
	}

	if len(s.envelopes) == 0 {
		return nil, nil
	}

	es, err := s.envelopes[0], s.errs[0]
	s.envelopes, s.errs = s.envelopes[1:], s.errs[1:]

	return es, err
}

func buildLogEnvelope(msg, index string) *loggregator_v2.Envelope {
	return &loggregator_v2.Envelope{
		Message: &loggregator_v2.Envelope_Log{
			Log: &loggregator_v2.Log{
				Payload: []byte(msg),
			},
		},
		Tags: map[string]string{
			"index": index,
		},
	}
}
