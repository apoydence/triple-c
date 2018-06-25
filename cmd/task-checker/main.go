package main

import (
	"log"
	"net/http"

	faas "github.com/apoydence/cf-faas"
	gocapi "github.com/apoydence/go-capi"
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

	faas.Start(handlers.NewTaskChecker(
		client,
	))
}
