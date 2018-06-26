package handlers

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	logcache "code.cloudfoundry.org/go-log-cache"
	"code.cloudfoundry.org/go-log-cache/rpc/logcache_v1"
	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
	faas "github.com/apoydence/cf-faas"
	gocapi "github.com/apoydence/go-capi"
)

type TaskChecker struct {
	g TaskGetter
	r TaskLogReader
	d Doer
}

type TaskGetter interface {
	GetTask(ctx context.Context, guid string) (gocapi.Task, error)
}

type TaskLogReader interface {
	Read(
		ctx context.Context,
		sourceID string,
		start time.Time,
		opts ...logcache.ReadOption,
	) ([]*loggregator_v2.Envelope, error)
}

func NewTaskChecker(g TaskGetter, r TaskLogReader, d Doer) *TaskChecker {
	return &TaskChecker{
		g: g,
		r: r,
		d: d,
	}
}

func (t *TaskChecker) Handle(r faas.Request) (faas.Response, error) {
	taskGuid := r.URLVariables["task-guid"]

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	task, err := t.g.GetTask(ctx, taskGuid)
	if err != nil {
		return faas.Response{}, err
	}

	logs, err := t.fetchLogs(ctx, task)
	if err != nil {
		logs = []string{err.Error()}
	}

	result := struct {
		State string   `json:"state"`
		Logs  []string `json:"logs"`
	}{
		State: task.State,
		Logs:  logs,
	}
	data, err := json.Marshal(result)
	if err != nil {
		log.Panic(err)
	}

	return faas.Response{
		StatusCode: http.StatusOK,
		Body:       data,
	}, nil
}

func (t *TaskChecker) fetchLogs(ctx context.Context, task gocapi.Task) ([]string, error) {
	appGuid, err := t.fetchAppGuid(task)
	if err != nil {
		return nil, err
	}

	var es []*loggregator_v2.Envelope
	logcache.Walk(ctx, appGuid, func(e []*loggregator_v2.Envelope) bool {
		es = append(es, e...)
		return true
	}, t.r.Read,
		logcache.WithWalkStartTime(task.CreatedAt),
		logcache.WithWalkEndTime(time.Now()),
		logcache.WithWalkEnvelopeTypes(logcache_v1.EnvelopeType_LOG),
	)

	index, skipIdx := t.findIndex(task, es)
	if skipIdx < 0 {
		return nil, errors.New("unable to find magic line")
	}

	var results []string
	for i, e := range es {
		if i == skipIdx || e.GetTags()["index"] != index {
			continue
		}
		results = append(results, string(e.GetLog().GetPayload()))
	}

	return results, nil
}

func (t *TaskChecker) findIndex(task gocapi.Task, es []*loggregator_v2.Envelope) (string, int) {
	var magicLine string

	scanner := bufio.NewScanner(strings.NewReader(task.Command))
	for scanner.Scan() {
		if strings.HasPrefix(scanner.Text(), fmt.Sprintf("echo '%s", magicPrefix)) {
			magicLine = scanner.Text()[6 : len(scanner.Text())-1] // Trim echo ' and trailing '
			break
		}
	}

	if magicLine == "" {
		return "", -1
	}

	for i, e := range es {
		if string(e.GetLog().GetPayload()) == magicLine {
			return e.GetTags()["index"], i
		}
	}

	return "", -1
}

func (t *TaskChecker) fetchAppGuid(task gocapi.Task) (string, error) {
	link, ok := task.Links["app"]
	if !ok {
		return "", errors.New("unable to find parent app guid")
	}

	req, err := http.NewRequest(link.Method, link.Href, nil)
	if err != nil {
		return "", err
	}

	resp, err := t.d.Do(req)
	if err != nil {
		return "", err
	}

	var result struct {
		Guid string `json:"guid"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	return result.Guid, nil
}
