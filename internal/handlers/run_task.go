package handlers

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"time"

	faas "github.com/apoydence/cf-faas"
)

const magicPrefix = "<--magic-identifier-->"

type RunTask struct {
	command        string
	d              Doer
	r              TaskRunner
	children       []string
	redirectFormat string
}

type Doer interface {
	Do(*http.Request) (*http.Response, error)
}

type TaskRunner interface {
	RunTask(command, name string) (string, error)
}

func NewRunTask(
	command string,
	d Doer,
	r TaskRunner,
	children []string,
	redirectFormat string, // e.g., http://some.addr/tasks/%s/lookup
) faas.Handler {
	return &RunTask{
		command:        command,
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

	taskGuid, err := r.r.RunTask(r.includeMagicIdentifier(), r.encodeTaskName(req))
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

func (r *RunTask) includeMagicIdentifier() string {
	return fmt.Sprintf(`
echo '%s |%d%d|'

%s
`, magicPrefix, time.Now().UnixNano(), rand.Int(), r.command)
}
