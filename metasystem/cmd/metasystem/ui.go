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
	"strings"
	"syscall"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goalbudget"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/supervise"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/act"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/fleet"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/httpd"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/lifecycle"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/overview"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/partner"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/project"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/session"
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

		// The second way a human's acts reach the ledger: the seat's one-time
		// code, in a browser. It needs no ancestry and no terminal, so it is
		// the way in for a server that was started by anything else — and the
		// way in for the human who walked up to a server they did not start.
		// How long a session lasts and who it acts as are configuration, read
		// here, once, like every other ui. key.
		confPath := filepath.Join(roots.Installation, "metasystem.conf")
		sessionLifetime, lifetimeErr := config.UISessionHours(confPath)
		if lifetimeErr != nil {
			return refuse(lifetimeErr.Error())
		}
		configuredHuman, humanErr := config.UIHuman(confPath)
		if humanErr != nil {
			return refuse(humanErr.Error())
		}

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

		// The interface's own presence fetch, beside that loop and never
		// through it. It is started and stopped under the same ownership
		// gate, so a server refused the checkout fetches no presence either;
		// it attempts nothing while no browser holds the notifications stream
		// open, and at most once a minute when one does. Nothing it knows
		// survives this process: the page says so until its first attempt.
		// This run's own identifier. The metadata file the fetch owner writes
		// for the Partner's tool outlives this process; nothing the owner
		// knows does. So the file names the run that wrote it, and the tool
		// is handed the same name, which is how it tells this server's
		// fetches from a previous server's.
		presenceRun, runErr := goal.NewOperationULID()
		if runErr != nil {
			return refuse(runErr.Error())
		}
		presenceWatch := fleet.NewWatch()
		presence := fleetFetcher(roots, presenceWatch, presenceRun)
		presenceContext, stopPresence := context.WithCancel(ctx)
		defer stopPresence()
		presenceStopped := make(chan struct{})
		go func() {
			defer close(presenceStopped)
			if !owned.Wait(presenceContext) {
				return
			}
			ticks := time.NewTicker(fleet.PollInterval)
			defer ticks.Stop()
			presence.Run(presenceContext, ticks.C)
		}()

		// advance carries this clone's accepted ref forward at once, which a
		// human act needs and a cadence cannot give: the act has landed on
		// the canonical branch, and the board must not move the card until
		// this clone has accepted it. The board's Refresh needs the same
		// thing for the opposite reason — nothing has been published and a
		// human is asking whether this clone is current now — so it runs one
		// look through here too, bounded by the same budget.
		advance := func() {
			ledger.Advance(func(endpoint goal.Endpoint) (goal.AdvanceResult, error) {
				return goal.FetchAdvanceBounded(endpoint, snapshot.FetchBudget)
			})
		}

		// The Project Partner, where this seat named one. Naming a runtime is
		// what turns it on; it is also what closes the boot proof's path to
		// the act routes, so the flag travels to the handler whether or not
		// the runtime could be admitted. An admission that refuses is not a
		// server that refuses to start: the pages still read, and the
		// Partner's own routes answer 503 with the refusal's own words.
		partnerRuntime, partnerModel, partnerCommand, partnerErr := config.UIPartner(confPath)
		if partnerErr != nil {
			return refuse(partnerErr.Error())
		}
		var partnerService *partner.Service
		partnerRefusal := ""
		if partnerRuntime != "" {
			admitted, admitErr := partner.Admit(partnerRuntime, strings.Fields(partnerCommand), partnerModel, roots.Checkout)
			if admitErr != nil {
				partnerRefusal = admitErr.Error()
				fmt.Fprintln(os.Stderr, "interface Partner: "+partnerRefusal)
			} else {
				// The interface's own read tools, handed to the session at
				// session/new. A seat that cannot name its own executable gets
				// a Partner that reads the page and nothing beyond it, which is
				// the previous slice's Partner rather than no Partner at all.
				if tools, toolsErr := partner.ToolsFor(roots.Checkout, roots.Installation, presenceRun); toolsErr != nil {
					fmt.Fprintln(os.Stderr, "interface Partner: "+toolsErr.Error())
				} else {
					admitted.Tools = tools
				}
				host := partner.NewHost(admitted, roots.Checkout,
					filepath.Join(roots.StateRoot, filepath.FromSlash(partner.Relative), "wire.jsonl"))
				partnerService = partner.NewService(admitted, host,
					func(human string) (*partner.Conversation, error) {
						return partner.OpenConversation(roots.StateRoot, human)
					},
					partner.Facts{
						Observe: ledger.Observe,
						Document: func(id string) (project.Document, error) {
							return project.Read(projectRoots(roots), id, time.Now().UTC())
						},
						Project: func() (project.Pane, error) {
							return project.ReadPane(projectRoots(roots), time.Now().UTC())
						},
					}, func() time.Time { return time.Now().UTC() })
				partnerService.Announce(func(busy bool) {
					_ = lifecycle.Update(roots.StateRoot, func(r *lifecycle.Record) {
						r.Partner = partnerLine(admitted, busy)
					})
				})
				defer partnerService.Close()
				fmt.Fprintln(os.Stderr, "interface Partner: "+partnerLine(admitted, false))
			}
		}

		err = lifecycle.Serve(ctx, lifecycle.Options{
			Roots: roots, Listen: listen, EngineBuild: supervise.BuildStamp, Prober: prober,
			Authority: authority.Line(),
			NewHandler: func(bound net.Addr, rec lifecycle.Record) http.Handler {
				// Who this seat signs in as, in the order the answer is
				// trustworthy: the terminal that started this server, the
				// handle the seat configured, and last the one a browser
				// named on a seat that had neither, which an earlier run
				// recorded. A client-supplied name never displaces any of
				// them.
				//
				// This read seeds that last answer and nothing else, which is
				// why its error is dropped: it only decides whether the sheet
				// asks for a name. The floor the same file carries is read
				// again on every sign-in, where the same failure refuses the
				// sign-in rather than being guessed at.
				seeded, _ := lifecycle.ReadSessions(roots.StateRoot)
				sessions := session.New(session.Options{
					Root:     roots.StateRoot,
					Human:    firstNamed(authority.Human(), configuredHuman, seeded.Human),
					Lifetime: sessionLifetime,
					Secret:   func() (string, error) { return session.Secret(confPath) },
					Now:      func() time.Time { return time.Now().UTC() },
					// The floor outlives this run, because a code spent before
					// a restart is still spent after one. Reading it is part
					// of admitting a code and writing it is part of accepting
					// one: a seat that cannot do either signs nobody in.
					Floor: func() (int64, string, error) {
						read, err := lifecycle.ReadSessions(roots.StateRoot)
						if err != nil {
							return 0, "", err
						}
						return read.LastStep, read.Human, nil
					},
					Record: func(lastStep int64, human string) error {
						return lifecycle.WriteSessions(roots.StateRoot,
							lifecycle.SessionFloor{LastStep: lastStep, Human: human})
					},
					// The live sessions go to this run's own record, which is
					// where `ui status` reads them. They are evidence, so a
					// record that cannot be written loses a line rather than
					// a sign-in.
					Lines: func(lines []string) {
						_ = lifecycle.Update(roots.StateRoot, func(r *lifecycle.Record) { r.Sessions = lines })
					},
				})
				// acting is the hand one act publishes under: the browser
				// session that reached the route, or the boot proof when no
				// session did. A session carries its own proof, so the ledger
				// records which one it was.
				acting := func(signed *session.Session) (act.Authority, error) {
					if signed == nil {
						return authority, nil
					}
					return act.SignedIn(roots.StateRoot, signed.Human, signed.Reference, signed.Proof)
				}
				return httpd.New(httpd.Info{Checkout: rec.Checkout, StartedAt: rec.StartedAt, EngineBuild: rec.EngineBuild, ExecutableDigest: rec.ExecutableDigest, BundleDigest: bundleDigest,
					Describe: func() (workspace.Workspace, error) {
						return workspace.Describe(
							workspace.Roots{Checkout: roots.Checkout, Installation: roots.Installation, StateRoot: roots.StateRoot},
							workspace.Record{EngineBuild: rec.EngineBuild, StartedAt: rec.StartedAt, ExecutableDigest: rec.ExecutableDigest},
							subject,
						)
					},
					Observe: ledger.Observe,
					Fetch:   advance,
					// The Fleet page, and the holder flags the board and the
					// Overview carry with it. It reads the presence copy this
					// server fetched for itself and starts no fetch of its
					// own.
					Fleet: fleetReader(roots, presence.Succeeded, presence.State),
					Watch: presenceWatch,
					Project: func() (project.Pane, error) {
						return project.ReadPane(projectRoots(roots), time.Now().UTC())
					},
					Document: func(id string) (project.Document, error) {
						return project.Read(projectRoots(roots), id, time.Now().UTC())
					},
					// The five writes, in the human's own checkout. Each one
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
					AddRecordGoal: func(id, goal string) (project.Document, error) {
						return project.AddGoal(projectRoots(roots), id, goal, time.Now().UTC())
					},
					SetRecordGoals: func(id string, goals []string) (project.Document, error) {
						return project.SetGoals(projectRoots(roots), id, goals, time.Now().UTC())
					},
					AskQuestion: func(asked project.NewQuestion) (project.Asked, error) {
						return project.AskQuestion(projectRoots(roots), asked, time.Now())
					},
					SetQuestionStatus: func(id, status string) (project.Asked, error) {
						return project.SetQuestionStatus(projectRoots(roots), id, status)
					},
					// Editing one document in place. It is the same checkout
					// and the same boundary the read goes through, and it is
					// answered with the document read again from disk.
					EditDocument: func(id, source, revision string) (project.Document, error) {
						return project.EditDocument(projectRoots(roots), id, source, revision, time.Now().UTC())
					},
					PreviewDocument: func(source string) (project.Preview, error) {
						return project.PreviewDocument(source)
					},
					// The backlog's six acts. Each one publishes through
					// the engine in-process under the boot proof, and then
					// carries this clone's accepted ref forward, so the
					// payload the route answers with is the ledger as it now
					// stands rather than as it stood before the act.
					Authority: httpd.AuthorityInfo{
						Proven: authority.Proven(), Human: authority.Human(), Reason: authority.Reason(),
					},
					Sessions: sessions,
					Approve: func(signed *session.Session, id string, budget goalbudget.Budget) error {
						hand, err := acting(signed)
						if err != nil {
							return err
						}
						if err := hand.Approve(id, budget); err != nil {
							return err
						}
						advance()
						return nil
					},
					Withdraw: func(signed *session.Session, id, reason string) error {
						hand, err := acting(signed)
						if err != nil {
							return err
						}
						if err := hand.Withdraw(id, reason); err != nil {
							return err
						}
						advance()
						return nil
					},
					SetPriority: func(signed *session.Session, id string, priority uint8, sequence *uint64) error {
						hand, err := acting(signed)
						if err != nil {
							return err
						}
						if err := hand.SetPriority(id, priority, sequence); err != nil {
							return err
						}
						advance()
						return nil
					},
					Open: func(signed *session.Session, opened act.Opened) error {
						hand, err := acting(signed)
						if err != nil {
							return err
						}
						if err := hand.Open(opened); err != nil {
							return err
						}
						advance()
						return nil
					},
					Block: func(signed *session.Session, dependent, blocker string) error {
						hand, err := acting(signed)
						if err != nil {
							return err
						}
						if err := hand.Block(dependent, blocker); err != nil {
							return err
						}
						advance()
						return nil
					},
					Unblock: func(signed *session.Session, dependent, blocker string) error {
						hand, err := acting(signed)
						if err != nil {
							return err
						}
						if err := hand.Unblock(dependent, blocker); err != nil {
							return err
						}
						advance()
						return nil
					},
					BudgetDefaults: func() (map[string]goalbudget.Budget, error) {
						return tierBudgets(roots.Installation)
					},
					// The steward's notification journal, in this checkout.
					// The steward writes it as a side effect of reaching the
					// operator; the interface reads it and nothing else of
					// the steward's, and never writes to it.
					NotificationJournal: steward.NotificationJournalPath(roots.Checkout),
					// The landing page's last-visit marker, beside this
					// server's own lifecycle state. It is the one thing the
					// interface writes for itself rather than for the
					// ledger: preference state about when a human last
					// looked, which no verb reads and losing which changes a
					// comparison window and nothing else.
					Visit: func(human string, now time.Time) (time.Time, bool, error) {
						return overview.Visit(roots.StateRoot, human, now)
					},
					// The Project Partner. The flag travels whether or not the
					// runtime was admitted, because it decides the act routes'
					// policy and not only the Partner's own.
					Partner:           partnerService,
					PartnerConfigured: partnerRuntime != "",
					PartnerRefusal:    partnerRefusal,
					// What this interface is made of, joined from the half the
					// bundle carries and the half this seat resolves. It is
					// the same join the Partner's own tool server answers
					// from, so the page and the Partner cannot be told two
					// different things about this build.
					Interface: describeInterface(roots),
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
				stopPresence()
				<-presenceStopped
			},
		})
		stopLoop()
		<-loopStopped
		stopPresence()
		<-presenceStopped
		if err != nil {
			return refuse(lifecycle.ServeFailure(roots.StateRoot, err))
		}
		return 0
	}
	return 2
}

// partnerLine is the one line `ui status` prints about the Partner: which
// runtime answers on this seat, which model it runs, and whether it is
// answering right now.
func partnerLine(runtime partner.Runtime, busy bool) string {
	line := "Project Partner: " + runtime.Name
	if runtime.Model != "" {
		line += " (" + runtime.Model + ")"
	}
	if busy {
		return line + ", answering"
	}
	return line + ", idle"
}

// firstNamed is the first of these that names somebody. It is how the seat's
// three answers about who it is are ordered in one place rather than in three.
func firstNamed(candidates ...string) string {
	for _, candidate := range candidates {
		if named := strings.TrimSpace(candidate); named != "" {
			return named
		}
	}
	return ""
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
