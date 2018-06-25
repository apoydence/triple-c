package handlers

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	faas "github.com/apoydence/cf-faas"
)

type RunTask struct {
	d              Doer
	r              TaskRunner
	children       []string
	redirectFormat string
}

type Doer interface {
	Do(*http.Request) (*http.Response, error)
}

type TaskRunner interface {
	RunTask(name string) (string, error)
}

type TaskRunnerFunc func() (string, error)

func (f TaskRunnerFunc) RunTask() (string, error) {
	return f()
}

func NewRunTask(
	d Doer,
	r TaskRunner,
	children []string,
	redirectFormat string, // e.g., http://some.addr/tasks/%s/lookup
) faas.Handler {
	return &RunTask{
		d:              d,
		r:              r,
		children:       children,
		redirectFormat: redirectFormat,
	}
}

func (r *RunTask) Handle(req faas.Request) (faas.Response, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	for _, child := range r.children {
		req, err := http.NewRequest(http.MethodGet, child, nil)
		if err != nil {
			return faas.Response{}, err
		}
		req = req.WithContext(ctx)

		resp, err := r.d.Do(req)
		if err != nil {
			return faas.Response{}, err
		}

		if resp.StatusCode != http.StatusOK {
			return faas.Response{
				StatusCode: resp.StatusCode,
			}, nil
		}
	}

	taskGuid, err := r.r.RunTask(r.encodeTaskName(req))
	if err != nil {
		return faas.Response{}, err
	}

	return faas.Response{
		StatusCode: http.StatusFound,
		Header: http.Header{
			"Location": []string{fmt.Sprintf(r.redirectFormat, taskGuid)},
		},
	}, nil
}

func (r *RunTask) encodeTaskName(req faas.Request) string {
	req.Headers = nil
	data, err := json.Marshal(req)
	if err != nil {
		log.Panic(err)
	}

	return base64.StdEncoding.EncodeToString(data)
}
