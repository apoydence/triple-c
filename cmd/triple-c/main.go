package main

import (
	"expvar"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/apoydence/triple-c/internal/capi"
	"github.com/apoydence/triple-c/internal/gitwatcher"
	"github.com/apoydence/triple-c/internal/metrics"
	"github.com/bradylove/envstruct"
	"github.com/cloudfoundry-incubator/uaago"
	"github.com/google/go-github/github"
)

func main() {
	log := log.New(os.Stderr, "", log.LstdFlags)
	log.Println("Starting triple-c...")
	defer log.Println("Closing triple-c...")

	cfg, err := LoadConfig()
	if err != nil {
		log.Fatalf("invalid configuration: %s", err)
	}

	envstruct.WriteReport(&cfg)

	uaaClient, err := uaago.NewClient(cfg.UAAAddr)
	if err != nil {
		log.Fatalf("invalid configuration: %s", err)
	}

	capi := capi.NewClient(
		cfg.VcapApplication.CAPIAddr,
		capi.NewHTTPClient(
			&http.Client{
				Timeout: 5 * time.Second,
			},
			capi.TokenFetcherFunc(func() (string, error) {
				rt, at, err := uaaClient.GetRefreshToken(cfg.ClientID, cfg.RefreshToken, cfg.SkipSSLValidation)
				if err != nil {
					return "", err
				}
				cfg.RefreshToken = rt
				return at, nil
			}),
		),
	)

	m := metrics.New(expvar.NewMap("TripleC"))

	failedTasks := m.NewCounter("FailedTasks")
	successfulTasks := m.NewCounter("SuccesssfulTasks")
	gitwatcher.StartWatcher(
		cfg.RepoOwner,
		cfg.RepoName,
		time.Second,
		github.NewClient(nil).Repositories,
		func(sha string) {
			log.Printf("Running task for %s", sha)
			defer log.Printf("Done with task for %s", sha)

			if err := capi.CreateTask(
				fetchRepo(cfg.RepoOwner, cfg.RepoName, cfg.Command),
				"some-name",
				cfg.VcapApplication.ApplicationID,
			); err != nil {
				log.Printf("failed to create event: %s", err)
				failedTasks(1)
				return
			}
			successfulTasks(1)
		},
		m,
		log,
	)

	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", cfg.Port), nil))
}

// fetchRepo adds the cloning of a repo to the given command
func fetchRepo(owner, repo, command string) string {
	return fmt.Sprintf(`#!/bin/bash
set -ex

git clone https://github.com/%s/%s --recursive

set +ex

%s
	`,
		owner,
		repo,
		command,
	)
}
