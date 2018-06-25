package handlers_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	faas "github.com/apoydence/cf-faas"
	gocapi "github.com/apoydence/go-capi"
	"github.com/apoydence/onpar"
	. "github.com/apoydence/onpar/expect"
	. "github.com/apoydence/onpar/matchers"
	"github.com/apoydence/triple-c/internal/handlers"
)

type TC struct {
	*testing.T
	c             faas.Handler
	spyTaskGetter *spyTaskGetter
}

func TestTaskChecker(t *testing.T) {
	t.Parallel()
	o := onpar.New()
	defer o.Run(t)

	o.BeforeEach(func(t *testing.T) TC {
		spyTaskGetter := newSpyTaskGetter()
		return TC{
			T:             t,
			spyTaskGetter: spyTaskGetter,
			c:             handlers.NewTaskChecker(spyTaskGetter),
		}
	})

	o.Spec("it returns a 200 and the given state", func(t TC) {
		t.spyTaskGetter.task = gocapi.Task{
			State: "some-state",
		}

		resp, err := t.c.Handle(faas.Request{
			Path: "/v1/some-path/tasks/some-guid",
		})
		Expect(t, err).To(BeNil())

		Expect(t, resp.StatusCode).To(Equal(http.StatusOK))

		var state map[string]interface{}
		Expect(t, json.Unmarshal(resp.Body, &state)).To(BeNil())
		Expect(t, state["state"]).To(Equal("some-state"))

		Expect(t, t.spyTaskGetter.ctx).To(Not(BeNil()))
		deadline, ok := t.spyTaskGetter.ctx.Deadline()
		Expect(t, ok).To(BeTrue())
		Expect(t, float64(deadline.Sub(time.Now()))).To(And(
			BeAbove(0),
			BeBelow(float64(time.Minute)),
		))

		Expect(t, t.spyTaskGetter.guid).To(Equal("some-guid"))
	})

	o.Spec("it returns an error if the TaskGetter returns an error", func(t TC) {
		t.spyTaskGetter.err = errors.New("some-error")

		_, err := t.c.Handle(faas.Request{})
		Expect(t, err).To(Not(BeNil()))
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
