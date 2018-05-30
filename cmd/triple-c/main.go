package main

import (
	"bufio"
	"bytes"
	"context"
	"expvar"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"time"

	"github.com/apoydence/triple-c/internal/capi"
	"github.com/apoydence/triple-c/internal/git"
	"github.com/apoydence/triple-c/internal/metrics"
	"github.com/apoydence/triple-c/internal/scheduler"
	"github.com/bradylove/envstruct"
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

	repoRegistry := git.NewRepoRegistry(tmpDir, execer, m)
	configRepo, err := repoRegistry.FetchRepo(cfg.RepoPath)
	if err != nil {
		log.Fatalf("failed to get config repo (%s): %s", cfg.RepoPath, err)
	}

	startBranch := func(ctx context.Context, branch string) {
		go func() {
			log.Printf("Watch branch %s", branch)
			manager := scheduler.NewManager(
				ctx,
				cfg.VcapApplication.ApplicationID,
				branch,
				capi,
				git.StartWatcher,
				repoRegistry,
				m,
				log,
			)
			sched := scheduler.New(manager)

			successfulConfig := m.NewCounter("SuccesssfulConifig")
			failConfig := m.NewCounter("FailedConifig")
			git.StartWatcher(
				ctx,
				branch,
				func(sha string) {
					var ts []scheduler.Task
					for _, t := range fetchConfigFile(sha, cfg.ConfigPath, configRepo, failConfig, successfulConfig, log).Tasks {
						if t.Command == "" || t.RepoPath == "" {
							log.Printf("invalid task: %+v", t)
							continue
						}
						ts = append(ts, t)
					}
					sched.SetTasks(ts)
				},
				time.Minute,
				configRepo,
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

	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", cfg.Port), nil))
}

func fetchConfigFile(SHA, filePath string, repo *git.Repo, fail, succ func(uint64), log *log.Logger) scheduler.Tasks {
	data, err := repo.File(SHA, filePath)
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
