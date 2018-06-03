package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"expvar"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"time"

	envstruct "code.cloudfoundry.org/go-envstruct"
	"github.com/apoydence/triple-c/internal/capi"
	"github.com/apoydence/triple-c/internal/git"
	"github.com/apoydence/triple-c/internal/handlers"
	"github.com/apoydence/triple-c/internal/metrics"
	"github.com/apoydence/triple-c/internal/scheduler"
	"github.com/cloudfoundry-incubator/uaago"
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
		time.Second,
		capi.NewHTTPClient(
			&http.Client{
				Timeout: 5 * time.Second,
				Transport: &http.Transport{
					TLSHandshakeTimeout: 10 * time.Second,
					TLSClientConfig: &tls.Config{
						InsecureSkipVerify: cfg.SkipSSLValidation,
						MinVersion:         tls.VersionTLS12,
					},
				},
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

	tmpDir, err := ioutil.TempDir("", "")
	if err != nil {
		log.Fatalf("failed to create temp dir: %s", err)
	}
	log.Printf("temp dir is at: %s", tmpDir)

	execer := git.ExecutorFunc(func(path string, commands ...string) ([]string, error) {
		cmd := exec.Command(commands[0], commands[1:]...)
		cmd.Dir = path
		cmd.Env = []string{"GIT_TERMINAL_PROMPT=0"}

		out, err := cmd.Output()
		if err != nil {
			return nil, err
		}

		var lines []string
		scanner := bufio.NewScanner(bytes.NewReader(out))
		for scanner.Scan() {
			lines = append(lines, scanner.Text())
		}

		if scanner.Err() != nil {
			return nil, scanner.Err()
		}

		return lines, nil
	})

	shaTracker := metrics.NewSHATracker()

	repoRegistry := git.NewRepoRegistry(tmpDir, execer, m)
	configRepo, err := repoRegistry.FetchRepo(cfg.RepoPath)
	if err != nil {
		log.Fatalf("failed to get config repo (%s): %s", cfg.RepoPath, err)
	}

	startBranch := func(ctx context.Context, branch string) {
		go func() {
			log.Printf("Watching branch %s", branch)
			manager := scheduler.NewManager(
				ctx,
				cfg.VcapApplication.ApplicationID,
				branch,
				capi,
				git.StartWatcher,
				repoRegistry,
				os.LookupEnv,
				shaTracker,
				m,
				log,
			)
			sched := scheduler.New(manager)

			successfulConfig := m.NewCounter("SuccesssfulConifig")
			failConfig := m.NewCounter("FailedConifig")
			git.StartWatcher(
				ctx,
				cfg.RepoPath,
				branch,
				func(sha string) {
					var ts []scheduler.MetaPlan
					for _, plan := range fetchConfigFile(sha, cfg.ConfigPath, configRepo, failConfig, successfulConfig, log).Plans {
						if len(plan.RepoPaths) == 0 {
							continue
						}

						for _, t := range plan.Tasks {
							if t.Command == "" {
								log.Fatalf("invalid task: %+v", t)
							}
						}

						var doOnce bool
						for _, repoPath := range plan.RepoPaths {
							if repoPath == cfg.RepoPath {
								doOnce = true
								break
							}
						}

						ts = append(ts, scheduler.MetaPlan{
							Plan:   plan,
							DoOnce: doOnce,
						})
					}
					sched.SetPlans(ts)
				},
				time.Minute,
				configRepo,
				shaTracker,
				m,
				log,
			)
		}()
	}

	branchManager := scheduler.NewBranchManager(startBranch)
	branchSched := scheduler.NewBranchScheduler(branchManager)

	git.StartBranchWatcher(
		context.Background(),
		configRepo,
		func(branches []string) {
			branchSched.SetBranches(branches)
		},
		time.Minute,
		m,
		log,
	)

	repoHandler := handlers.NewRepos(shaTracker, log)
	http.Handle("/v1/repos", repoHandler)

	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", cfg.Port), nil))
}

func fetchConfigFile(SHA, filePath string, repo git.Repo, fail, succ func(uint64), log *log.Logger) scheduler.Plans {
	data, err := repo.File(SHA, filePath)
	if err != nil {
		log.Printf("failed to find config file: %s", err)
		fail(1)
		return scheduler.Plans{}
	}

	var t scheduler.Plans
	if err := yaml.Unmarshal([]byte(data), &t); err != nil {
		log.Printf("failed to find config file: %s", err)
		fail(1)
		return scheduler.Plans{}
	}

	succ(1)
	return t
}
