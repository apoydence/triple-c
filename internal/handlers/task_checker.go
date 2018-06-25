package handlers

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"path"
	"time"

	faas "github.com/apoydence/cf-faas"
	gocapi "github.com/apoydence/go-capi"
)

type TaskChecker struct {
	g TaskGetter
}

type TaskGetter interface {
	GetTask(ctx context.Context, guid string) (gocapi.Task, error)
}

func NewTaskChecker(g TaskGetter) *TaskChecker {
	return &TaskChecker{
		g: g,
	}
}

func (t *TaskChecker) Handle(r faas.Request) (faas.Response, error) {
	_, taskGuid := path.Split(r.Path)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	task, err := t.g.GetTask(ctx, taskGuid)
	if err != nil {
		return faas.Response{}, err
	}

	result := struct {
		State string `json:"state"`
	}{
		State: task.State,
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
