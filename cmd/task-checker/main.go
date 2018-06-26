package main

import (
	"log"
	"net/http"

	logcache "code.cloudfoundry.org/go-log-cache"
	faas "github.com/apoydence/cf-faas"
	gocapi "github.com/apoydence/go-capi"
	"github.com/apoydence/triple-c/internal/handlers"
)

func main() {
	cfg, err := LoadConfig()
	if err != nil {
		log.Fatalf("invalid configuration: %s", err)
	}

	capiClient := gocapi.NewClient(
		cfg.VcapApplication.CAPIAddr,
		cfg.VcapApplication.ApplicationID,
		cfg.VcapApplication.SpaceID,
		http.DefaultClient,
	)

	logCacheClient := logcache.NewClient(
		cfg.VcapApplication.LogCacheAddr,
	)

	faas.Start(handlers.NewTaskChecker(
		capiClient,
		logCacheClient,
		http.DefaultClient,
	))
}
