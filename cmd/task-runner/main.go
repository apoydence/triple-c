package main

import (
	"log"
	"net/http"

	faas "github.com/apoydence/cf-faas"
	gocapi "github.com/apoydence/go-capi"
	"github.com/apoydence/triple-c/internal/capi"
	"github.com/apoydence/triple-c/internal/handlers"
)

func main() {
	cfg, err := LoadConfig()
	if err != nil {
		log.Fatalf("invalid configuration: %s", err)
	}

	client := gocapi.NewClient(
		cfg.VcapApplication.CAPIAddr,
		cfg.VcapApplication.ApplicationID,
		cfg.VcapApplication.SpaceID,
		http.DefaultClient,
	)

	taskRunner := capi.NewTaskRunner(
		cfg.ScriptAppName,
		client,
	)

	faas.Start(handlers.NewRunTask(
		cfg.Command,
		http.DefaultClient,
		taskRunner,
		cfg.Children,
		"http://"+cfg.VcapApplication.ApplicationURIs[0]+cfg.Path+"/tasks/%s",
	))
}
