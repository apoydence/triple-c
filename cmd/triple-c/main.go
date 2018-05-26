package main

import (
	"context"
	"expvar"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/apoydence/triple-c/internal/capi"
	"github.com/apoydence/triple-c/internal/gitwatcher"
	"github.com/apoydence/triple-c/internal/metrics"
	"github.com/apoydence/triple-c/internal/scheduler"
	"github.com/bradylove/envstruct"
	"github.com/cloudfoundry-incubator/uaago"
	"github.com/google/go-github/github"
	"gopkg.in/yaml.v2"
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

	manager := scheduler.NewManager(
		cfg.VcapApplication.ApplicationID,
		capi,
		github.NewClient(nil).Repositories,
		gitwatcher.StartWatcher,
		m,
		log,
	)
	sched := scheduler.New(manager)

	go func() {
		successfulConfig := m.NewCounter("SuccesssfulConifig")
		failConfig := m.NewCounter("FailedConifig")
		gitwatcher.StartWatcher(
			context.Background(),
			cfg.RepoOwner,
			cfg.RepoName,
			github.NewClient(nil).Repositories,
			func(sha string) {
				var ts []scheduler.Task
				for _, t := range fetchConfigFile(sha, cfg, failConfig, successfulConfig, log).Tasks {
					ts = append(ts, t)
				}
				sched.SetTasks(ts)
			},
			time.Minute,
			m,
			log,
		)
	}()

	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", cfg.Port), nil))
}

func fetchConfigFile(SHA string, cfg Config, fail, succ func(uint64), log *log.Logger) scheduler.Tasks {
	path := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/%s",
		cfg.RepoOwner,
		cfg.RepoName,
		SHA,
		cfg.ConfigPath,
	)
	log.Printf("Reading config from %s", path)
	resp, err := http.Get(path)
	if err != nil {
		log.Printf("failed to find config file: %s", err)
	}

	defer func() {
		io.Copy(ioutil.Discard, resp.Body)
		resp.Body.Close()
	}()

	if resp.StatusCode != 200 {
		log.Printf("failed to find config file: %d", resp.StatusCode)
		fail(1)
		return scheduler.Tasks{}
	}

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Printf("failed to find config file: %s", err)
		fail(1)
		return scheduler.Tasks{}
	}

	var t scheduler.Tasks
	if err := yaml.Unmarshal([]byte(data), &t); err != nil {
		log.Printf("failed to find config file: %s", err)
		fail(1)
		return scheduler.Tasks{}
	}

	succ(1)
	return t
}
