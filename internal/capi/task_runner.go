package capi

import (
	"context"
	"time"

	capi "github.com/apoydence/go-capi"
)

type TaskRunner struct {
	command string
	droplet string
	appName string
	c       Client
}

type Client interface {
	GetAppGuid(ctx context.Context, appName string) (string, error)
	RunTask(ctx context.Context, command, name, dropletGuid, appGuid string) (capi.Task, error)
	ListTasks(ctx context.Context, appGuid string, query map[string][]string) ([]capi.Task, error)
}

func NewTaskRunner(command, appName string, c Client) *TaskRunner {
	return &TaskRunner{
		command: command,
		appName: appName,
		c:       c,
	}
}

func (r *TaskRunner) RunTask(name string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	appGuid, err := r.c.GetAppGuid(ctx, r.appName)
	if err != nil {
		return "", err
	}

	tasks, err := r.c.ListTasks(ctx, appGuid, map[string][]string{
		"names": []string{name},
	})
	if err != nil {
		return "", err
	}

	for _, t := range tasks {
		if t.Name == name {
			return t.Guid, nil
		}
	}

	t, err := r.c.RunTask(ctx, r.command, name, r.droplet, appGuid)
	if err != nil {
		return "", err
	}

	return t.Guid, nil
}
