package main

import (
	"context"
	"flag"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goalbudget"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/supervise"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/act"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/httpd"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/lifecycle"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/project"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/snapshot"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/web"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/workspace"
)

func runUIStart(args []string) int   { return runUI("start", args) }
func runUIServe(args []string) int   { return runUI("serve", args) }
func runUIStatus(args []string) int  { return runUI("status", args) }
func runUIStop(args []string) int    { return runUI("stop", args) }
func runUIRestart(args []string) int { return runUI("restart", args) }

func runUI(verb string, args []string) int {
	flags := flag.NewFlagSet("ui "+verb, flag.ContinueOnError)
	repo := pathFlag(flags, "repo", "", "checkout path (default: the checkout that contains the installation)")
	root := flags.String("metasystem-root", "", "metasystem installation")
	var listen string
	var waitSeconds int64 = 15
	readyFD := -1
	if verb == "start" || verb == "serve" || verb == "restart" {
		flags.StringVar(&listen, "listen", "", "loopback IP and port")
	}
	if verb == "stop" || verb == "restart" {
		flags.Int64Var(&waitSeconds, "wait-seconds", 15, "seconds to wait for the server to stop")
	}
	if verb == "serve" {
		flags.IntVar(&readyFD, "ready-fd", -1, "readiness pipe descriptor")
	}
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 0 || waitSeconds < 0 || waitSeconds > int64((1<<63-1)/time.Second) || readyFD < -1 {
		fmt.Fprintln(os.Stderr, "invalid arguments for ui "+verb)
		return 2
	}
	var ready *os.File
	if readyFD >= 0 {
		ready = os.NewFile(uintptr(readyFD), "interface readiness")
		defer ready.Close()
	}
	// A launched child reports its refusal to the launcher, which prints it
	// unchanged; a human verb prints the same line itself. serve keeps its own
	// output on standard error, where it logs.
	refuse := func(line string) int {
		if ready != nil {
			fmt.Fprintln(ready, "failed "+line)
			_ = ready.Close()
			ready = nil
		}
		if verb == "serve" {
			fmt.Fprintln(os.Stderr, line)
		} else {
			fmt.Println(line)
		}
		return 1
	}
	metasystemRoot, err := upMetasystemRoot(*root)
	if err != nil {
		return refuse(err.Error())
	}
	roots, err := lifecycle.ResolveRoots(*repo, metasystemRoot)
	if err != nil {
		return refuse(err.Error())
	}
	if verb == "start" || verb == "serve" || verb == "restart" {
		listenSet := false
		flags.Visit(func(f *flag.Flag) {
			if f.Name == "listen" {
				listenSet = true
			}
		})
		listen, err = config.UIListen(filepath.Join(roots.Installation, "metasystem.conf"), listen, listenSet)
		if err != nil {
			return refuse(err.Error())
		}
		listen, err = lifecycle.ValidateListen(listen)
		if err != nil {
			return refuse(err.Error())
		}
	}
	prober := identity.KernelProber{}
	stop := lifecycle.StopOptions{Prober: prober, Wait: time.Duration(waitSeconds) * time.Second}
	start := func() lifecycle.Result {
		executable, err := os.Executable()
		if err != nil {
			return lifecycle.Result{Lines: []string{"cannot launch the interface server: " + err.Error()}, Code: 1}
		}
		return lifecycle.StartResult(lifecycle.LaunchSpec{
			Executable: executable,
			Args:       lifecycle.ServeArgs(roots.Checkout, roots.Installation, listen),
			Dir:        roots.Checkout,
			LogPath:    filepath.Join(lifecycle.Dir(roots.StateRoot), "server.log"),
		}, lifecycle.ExecSpawn, 0)
	}
	switch verb {
	case "start":
		return printUIResult(start())
	case "status":
		return printUIResult(lifecycle.StatusResult(roots.StateRoot, prober, nil))
	case "stop":
		return printUIResult(lifecycle.StopResult(roots.StateRoot, stop))
	case "restart":
		return printUIResult(lifecycle.RestartResult(roots.StateRoot, stop, start))
	case "serve":
		ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
		defer cancel()
		subject, err := config.UISubject(filepath.Join(roots.Installation, "metasystem.conf"))
		if err != nil {
			return refuse(err.Error())
		}
		bundleDigest, bundle := interfaceBundle()

		// The one human-authority observation this process will ever make.
		//
		// It is taken here, before anything is served, because here is the
		// only moment this process has an ancestry that reaches the human:
		// `ui start` spawns the server and then waits for its readiness
		// line, so the launcher is still this process's parent, and the
		// launcher was run from the human's own terminal. The server itself
		// is started with setsid and so has no controlling terminal of its
		// own to walk from, which is why the walk starts at the parent —
		// exactly as the command edge's own human verbs walk from theirs.
		// Afterwards the launcher exits and nothing can be proven again; the
		// object is kept in memory for this server's life, and no later
		// request re-reads or re-parses it.
		authorityNow, nowErr := act.Now(roots.StateRoot)
		if nowErr != nil {
			return refuse(nowErr.Error())
		}
		authority := act.Prove(roots.StateRoot, roots.Installation, act.ParentPID(), authorityNow)
		fmt.Fprintln(os.Stderr, "interface authority: "+authority.Line())

		// The ledger reader answers requests from the accepted ref as it
		// stands. Its freshness loop is what carries that ref forward, and it
		// belongs to the process that owns the checkout: exclusivity is taken
		// inside Serve, so the loop waits on a gate that only Serve's readiness
		// opens, and a server refused the checkout fetches nothing. The loop's
		// own context is cancelled on every path out of this case, including a
		// listener that fails, and the process waits for a tick in flight to
		// finish its compare-and-swap before it exits.
		ledger := snapshot.New(roots.StateRoot, time.Now)
		owned := snapshot.NewGate()
		loopContext, stopLoop := context.WithCancel(ctx)
		defer stopLoop()
		loopStopped := make(chan struct{})
		go func() {
			defer close(loopStopped)
			if !owned.Wait(loopContext) {
				return
			}
			ledger.Run(loopContext, func(endpoint goal.Endpoint) (goal.AdvanceResult, error) {
				return goal.FetchAdvanceBounded(endpoint, snapshot.FetchBudget)
			}, snapshot.WallTimers{})
		}()

		// advance carries this clone's accepted ref forward at once, which a
		// human act needs and a cadence cannot give: the act has landed on
		// the canonical branch, and the board must not move the card until
		// this clone has accepted it.
		advance := func() {
			ledger.Advance(func(endpoint goal.Endpoint) (goal.AdvanceResult, error) {
				return goal.FetchAdvanceBounded(endpoint, snapshot.FetchBudget)
			})
		}

		err = lifecycle.Serve(ctx, lifecycle.Options{
			Roots: roots, Listen: listen, EngineBuild: supervise.BuildStamp, Prober: prober,
			Authority: authority.Line(),
			NewHandler: func(bound net.Addr, rec lifecycle.Record) http.Handler {
				return httpd.New(httpd.Info{Checkout: rec.Checkout, StartedAt: rec.StartedAt, EngineBuild: rec.EngineBuild, ExecutableDigest: rec.ExecutableDigest, BundleDigest: bundleDigest,
					Describe: func() (workspace.Workspace, error) {
						return workspace.Describe(
							workspace.Roots{Checkout: roots.Checkout, Installation: roots.Installation, StateRoot: roots.StateRoot},
							workspace.Record{EngineBuild: rec.EngineBuild, StartedAt: rec.StartedAt, ExecutableDigest: rec.ExecutableDigest},
							subject,
						)
					},
					Observe: ledger.Observe,
					Project: func() (project.Pane, error) {
						return project.ReadPane(projectRoots(roots), time.Now().UTC())
					},
					Document: func(id string) (project.Document, error) {
						return project.Read(projectRoots(roots), id, time.Now().UTC())
					},
					// The four writes, in the human's own checkout. Each one
					// reads the homes, writes one file atomically, and reads
					// them again, per request, for the reason the readers do:
					// what the browser is answered with is what the next read
					// of the checkout will say.
					CreateRecord: func(asked project.NewRecord) (project.Written, error) {
						return project.CreateRecord(projectRoots(roots), asked, time.Now())
					},
					SetStatus: func(id, status string) (project.Written, error) {
						return project.SetStatus(projectRoots(roots), id, status)
					},
					AskQuestion: func(asked project.NewQuestion) (project.Asked, error) {
						return project.AskQuestion(projectRoots(roots), asked, time.Now())
					},
					SetQuestionStatus: func(id, status string) (project.Asked, error) {
						return project.SetQuestionStatus(projectRoots(roots), id, status)
					},
					// The backlog's four acts. Each one publishes through
					// the engine in-process under the boot proof, and then
					// carries this clone's accepted ref forward, so the
					// payload the route answers with is the ledger as it now
					// stands rather than as it stood before the act.
					Authority: httpd.AuthorityInfo{
						Proven: authority.Proven(), Human: authority.Human(), Reason: authority.Reason(),
					},
					Approve: func(id string, budget goalbudget.Budget) error {
						if err := authority.Approve(id, budget); err != nil {
							return err
						}
						advance()
						return nil
					},
					Withdraw: func(id, reason string) error {
						if err := authority.Withdraw(id, reason); err != nil {
							return err
						}
						advance()
						return nil
					},
					SetPriority: func(id string, priority uint8, sequence *uint64) error {
						if err := authority.SetPriority(id, priority, sequence); err != nil {
							return err
						}
						advance()
						return nil
					},
					Open: func(opened act.Opened) error {
						if err := authority.Open(opened); err != nil {
							return err
						}
						advance()
						return nil
					},
					BudgetDefaults: func() (map[string]goalbudget.Budget, error) {
						return tierBudgets(roots.Installation)
					},
				}, bound, bundle)
			},
			Ready: func(address string) {
				owned.Open()
				if ready != nil {
					fmt.Fprintln(ready, "ready "+address)
					_ = ready.Close()
					ready = nil
				}
				fmt.Fprintf(os.Stderr, "interface running at http://%s (pid %d)\n", address, os.Getpid())
			},
			// The loop ends while this process still holds the checkout, so a
			// tick in flight cannot advance the accepted ref after the next
			// server has taken the lock.
			Releasing: func() {
				stopLoop()
				<-loopStopped
			},
		})
		stopLoop()
		<-loopStopped
		if err != nil {
			return refuse(lifecycle.ServeFailure(roots.StateRoot, err))
		}
		return 0
	}
	return 2
}

// interfaceBundle reports the digest of the source the embedded bundle was
// built from and the bundle itself. The manifest is the publish point: without
// one this executable carries no bundle, so the handler receives none and says
// so rather than serving half a build.
func interfaceBundle() (string, fs.FS) {
	manifest, err := web.ReadManifest()
	if err != nil {
		return "", nil
	}
	return manifest.SourceDigest, web.Dist()
}

func printUIResult(result lifecycle.Result) int {
	for _, line := range result.Lines {
		fmt.Println(line)
	}
	return result.Code
}

// tierBudgets is the project's budget law, by tier, as the approval sheet
// prefills from it. The law answers for every tier it knows; a configuration
// that cannot be read answers for none, and the sheet then asks the human for
// all five limits rather than showing a number nobody chose.
func tierBudgets(installation string) (map[string]goalbudget.Budget, error) {
	set, err := config.LoadTierBoxSet(filepath.Join(installation, "metasystem.conf"))
	if err != nil {
		return nil, err
	}
	budgets := map[string]goalbudget.Budget{}
	for tier := uint8(1); tier <= 3; tier++ {
		box, boxErr := set.TierBox(tier)
		if boxErr != nil {
			continue
		}
		budgets[strconv.Itoa(int(tier))] = box
	}
	return budgets, nil
}

// projectRoots carries the lifecycle's three roots to the reader that declares
// its own triple, which is the conversion this wiring exists for.
func projectRoots(roots lifecycle.Roots) project.Roots {
	return project.Roots{Checkout: roots.Checkout, Installation: roots.Installation, StateRoot: roots.StateRoot}
}
