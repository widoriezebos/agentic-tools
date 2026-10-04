package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batchowner"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goalbudget"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/knownissues"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/rulings"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/seat/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/supervise"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/act"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/application"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/fleet"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/httpd"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/lifecycle"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/overview"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/partner"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/project"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/review"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/session"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/snapshot"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/stickies"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/uihome"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/web"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/workspace"
)

func runUIServe(args []string, stdout, stderr io.Writer) int {
	flags := newFlagSet("ui serve", stdout, stderr)
	repo := pathFlag(flags, "repo", "", "checkout path (default: the checkout that contains the installation)")
	root := flags.String("metasystem-root", "", "metasystem installation")
	listen := flags.String("listen", "", "loopback IP and port")
	readyFD := flags.Int("ready-fd", -1, "readiness pipe descriptor")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 0 || *readyFD < -1 {
		fmt.Fprintln(stderr, "invalid arguments for ui serve")
		return 2
	}

	var ready *os.File
	if *readyFD >= 0 {
		ready = os.NewFile(uintptr(*readyFD), "interface readiness")
		defer ready.Close()
	}
	// A launched child reports its refusal through the readiness pipe and
	// logs it on standard error.
	refuse := func(line string) int {
		if ready != nil {
			fmt.Fprintln(ready, "failed "+line)
			_ = ready.Close()
			ready = nil
		}
		fmt.Fprintln(stderr, line)
		return 1
	}
	listenSet := false
	flags.Visit(func(f *flag.Flag) {
		if f.Name == "listen" {
			listenSet = true
		}
	})
	roots, address, err := uiRootsAndListen("serve", *repo, *root, *listen, listenSet)
	if err != nil {
		return refuse(err.Error())
	}
	prober := identity.KernelProber{}
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer cancel()
	subject, err := config.UISubject(roots.Installation.Path("metasystem.conf"))
	if err != nil {
		return refuse(err.Error())
	}
	bundleDigest, bundle := interfaceBundle()

	// The one human-authority observation this process will ever make.
	//
	// It is taken here, before anything is served, because here is the
	// only moment this process has an ancestry that reaches the human:
	// `start ui` spawns the server and then waits for its readiness
	// line, so the launcher is still this process's parent, and the
	// launcher was run from the human's own terminal. The server itself
	// is started with setsid and so has no controlling terminal of its
	// own to walk from, which is why the walk starts at the parent —
	// exactly as the command edge's own human verbs walk from theirs.
	// Afterwards the launcher exits and nothing can be proven again; the
	// object is kept in memory for this server's life, and no later
	// request re-reads or re-parses it.
	authorityNow, nowErr := act.Now(roots.Installation.Path())
	if nowErr != nil {
		return refuse(nowErr.Error())
	}
	authority := act.Prove(roots.StateRoot.Path(), roots.Installation, act.ParentPID(), authorityNow)
	fmt.Fprintln(stderr, "interface authority: "+authority.Line())

	// The second way a human's acts reach the ledger: the seat's one-time
	// code, in a browser. It needs no ancestry and no terminal, so it is
	// the way in for a server that was started by anything else — and the
	// way in for the human who walked up to a server they did not start.
	// How long a session lasts and who it acts as are configuration, read
	// here, once, like every other ui. key.
	confPath := roots.Installation.Path("metasystem.conf")
	sessionLifetime, lifetimeErr := config.UISessionHours(confPath)
	if lifetimeErr != nil {
		return refuse(lifetimeErr.Error())
	}
	configuredHuman, humanErr := config.UIHuman(confPath)
	if humanErr != nil {
		return refuse(humanErr.Error())
	}
	// What the private store is kept to. A number this seat cannot resolve
	// refuses the server rather than being guessed at: a seat that widened
	// its store must not be given the default instead (g1-s54 D4).
	storeBounds, storeBoundsErr := config.UIStoreBounds(confPath)
	if storeBoundsErr != nil {
		return refuse(storeBoundsErr.Error())
	}
	// Where that store is. It is resolved here rather than inside the
	// Partner's own admission because the notepad lives under it too, so
	// Settings names it on a seat that serves no Partner. A home that
	// cannot be resolved is not a refusal: the seat keeps nothing outside
	// its checkout, and Settings says so in the reader's own words.
	storeHome, storeHomeErr := uihome.Home()
	storeProblem := ""
	if storeHomeErr != nil {
		storeProblem = "this account has no registry home the interface can read, so this seat keeps no private store: " + storeHomeErr.Error()
	}

	// The human's own notepad, under the account's registry home rather
	// than under this checkout's state root. A sticky is personal working
	// material and never a record: keeping it inside the checkout would
	// put it inside the Project Partner's native read grant and inside a
	// critic's read root, and "no seat reads your stickies" would be false
	// from the first one. A home this process cannot resolve is a build
	// with no notepad, which the routes say — it costs a human their
	// reminders and nothing about the rest of the interface, so it is not
	// a reason to refuse to serve.
	var notepad *stickies.Store
	if home, homeErr := stickies.Home(); homeErr == nil {
		notepad = stickies.New(home, roots.Checkout, time.Now)
	} else {
		fmt.Fprintln(stderr, "interface notepad: unavailable: "+homeErr.Error())
	}

	// The ledger reader answers requests from the accepted ref as it
	// stands. Its freshness loop is what carries that ref forward, and it
	// belongs to the process that owns the checkout: exclusivity is taken
	// inside Serve, so the loop waits on a gate that only Serve's readiness
	// opens, and a server refused the checkout fetches nothing. The loop's
	// own context is cancelled on every path out of this case, including a
	// listener that fails, and the process waits for a tick in flight to
	// finish its compare-and-swap before it exits.
	ledger := snapshot.New(roots.StateRoot.Path(), time.Now)
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
	// A launch that died while no server was running is reconciled once,
	// here, so the first page load after a restart reads it as failed
	// rather than as a launch nobody is running. Reading the records is
	// the only moment anything learns that a launch died.
	_, _ = launch.Reconcile(roots.Checkout, launch.Live, time.Now().UTC())
	// A launch record that changed is a cause of the `fleet` event, and
	// it rides the poll tick the presence owner already has: a launch
	// rewrites its record after every step, and a page that learned of it
	// only on the fetch cadence would show a clone finishing a minute
	// after it did.
	launchWatch := &fleet.LaunchWatch{
		Fingerprint: func() string { return fleet.FingerprintOf(launch.Dir(roots.Checkout)) },
		Announce:    presenceWatch.Announce,
	}
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
	launchStopped := make(chan struct{})
	go func() {
		defer close(launchStopped)
		if !owned.Wait(presenceContext) {
			return
		}
		ticks := time.NewTicker(fleet.PollInterval)
		defer ticks.Stop()
		launchWatch.Run(presenceContext, ticks.C)
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
			return uiAdvance(roots.StateRoot.Path(), endpoint)
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
	// The private store's housekeeping. It is built where the two owners
	// are — the journal's writer and the conversation owner — because
	// neither bound is applied to a file from outside: the journal rotates
	// through its own writer, and a transcript is trimmed by the
	// conversation that appends to it (g1-s54 D1).
	var housekeeping *storeKeeper
	partnerRefusal := ""
	if partnerRuntime != "" {
		// Where the conversation goes, before anything is admitted. It is
		// this account's own directory outside every checkout, because the
		// transcript is private sitting material an examiner must never read
		// and the state root lies inside the checkout a critic is handed
		// (g1-s53 D11). An account whose home cannot be resolved has no
		// Partner at all rather than a Partner whose every word lands in the
		// repository, and the routes say so in these words.
		conversations, homeErr := partner.Home()
		admitted, admitErr := partner.Admit(partnerRuntime, strings.Fields(partnerCommand), partnerModel, roots.Checkout)
		// A conversation written before the store moved out of the checkout
		// is carried into the new place, once, on the first open of this
		// workspace's directory. It is a no-op where the old place is gone,
		// and a failure is the reason the Partner is not served: an empty
		// history beside the old files still sitting in the checkout is the
		// finding the move answered, in a quieter form.
		var carryErr error
		if homeErr == nil {
			conversations = partner.Directory(conversations, roots.Checkout)
			if admitErr == nil {
				_, carryErr = partner.Carry(conversations,
					roots.Installation.Path(filepath.FromSlash(partner.LegacyRelative)))
			}
		}
		switch {
		case homeErr != nil:
			partnerRefusal = "this seat cannot keep the Partner's conversation outside the checkout, so it serves no Partner: " + homeErr.Error()
			fmt.Fprintln(stderr, "interface Partner: "+partnerRefusal)
		case admitErr != nil:
			partnerRefusal = admitErr.Error()
			fmt.Fprintln(stderr, "interface Partner: "+partnerRefusal)
		case carryErr != nil:
			partnerRefusal = "this seat serves no Partner rather than an empty history: " + carryErr.Error()
			fmt.Fprintln(stderr, "interface Partner: "+partnerRefusal)
		default:
			// The interface's own read tools, handed to the session at
			// session/new. A seat that cannot name its own executable gets
			// a Partner that reads the page and nothing beyond it, which is
			// the previous slice's Partner rather than no Partner at all.
			// And the file this seat marks each answer in, beside the
			// conversation: it is what lets the tool server tell one answer
			// from the next, so the fifty-first proposal of an answer is
			// refused at the call (R-130-ui, Astra F-06).
			if tools, toolsErr := partner.ToolsFor(roots.Checkout, roots.Installation.Path(), presenceRun,
				filepath.Join(conversations, "answer")); toolsErr != nil {
				fmt.Fprintln(stderr, "interface Partner: "+toolsErr.Error())
			} else {
				admitted.Tools = tools
			}
			// The wire journal is every frame of the runtime's life, which is
			// the conversation again in another form, so it is kept beside
			// the conversation and not in the checkout.
			host := partner.NewHost(admitted, roots.Checkout,
				filepath.Join(conversations, "wire.jsonl"))
			partnerService = partner.NewService(admitted, host,
				func(human string) (*partner.Conversation, error) {
					return partner.OpenConversation(conversations, human)
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
			housekeeping = &storeKeeper{
				journal: host.Journal(),
				wire:    int64(storeBounds.WireMB) << 20,
				trim: func() (int, error) {
					// The store's own files, so a transcript that grew
					// under an earlier run of this seat is bounded at
					// start, before anybody has spoken.
					humans, err := partner.Humans(conversations)
					if err != nil {
						return 0, err
					}
					return partnerService.Trim(partner.TrimBounds{
						Bytes: int64(storeBounds.ConversationMB) << 20,
						Age:   time.Duration(storeBounds.ConversationDays) * 24 * time.Hour,
					}, humans)
				},
				say: func(line string) { fmt.Fprintln(stderr, "interface store: "+line) },
			}
			partnerService.Announce(func(busy bool) {
				_ = lifecycle.Update(roots.Installation.Path(), func(r *lifecycle.Record) {
					r.Partner = partnerLine(admitted, busy)
				})
			})
			defer partnerService.Close()
			fmt.Fprintln(stderr, "interface Partner: "+partnerLine(admitted, false))
		}
	}

	// At start, and once a day while this server serves — inside the
	// interval this process owns the checkout and never outside it. The
	// first sweep waits on the same ownership gate the freshness loop
	// waits on, because a sweep before the lock is taken would trim a
	// transcript the incumbent server is appending to and rotate the
	// journal its runtime is writing: two processes, two Conversation
	// objects, two mutexes, one file. The tick is the caller's, as the
	// fleet fetch loop's is, so a test drives the cadence with a channel
	// it controls instead of waiting a day for it.
	var storeTicks <-chan time.Time
	if housekeeping != nil {
		sweeps := time.NewTicker(storeSweep)
		defer sweeps.Stop()
		storeTicks = sweeps.C
	}
	stopHousekeeping := startStoreHousekeeping(ctx, owned, housekeeping, storeTicks)
	defer stopHousekeeping()

	err = lifecycle.Serve(ctx, lifecycle.Options{
		Roots: roots, Listen: address, EngineBuild: supervise.BuildStamp, Prober: prober,
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
			seeded, _ := lifecycle.ReadSessions(roots.Installation.Path())
			sessions := session.New(session.Options{
				Root:     roots.StateRoot.Path(),
				Human:    firstNamed(authority.Human(), configuredHuman, seeded.Human),
				Lifetime: sessionLifetime,
				Secret:   func() (string, error) { return session.Secret(confPath) },
				Now:      func() time.Time { return time.Now().UTC() },
				// The floor outlives this run, because a code spent before
				// a restart is still spent after one. Reading it is part
				// of admitting a code and writing it is part of accepting
				// one: a seat that cannot do either signs nobody in.
				Floor: func() (int64, string, error) {
					read, err := lifecycle.ReadSessions(roots.Installation.Path())
					if err != nil {
						return 0, "", err
					}
					return read.LastStep, read.Human, nil
				},
				Record: func(lastStep int64, human string) error {
					return lifecycle.WriteSessions(roots.Installation.Path(),
						lifecycle.SessionFloor{LastStep: lastStep, Human: human})
				},
				// The live sessions go to this run's own record, which is
				// where `ui status` reads them. They are evidence, so a
				// record that cannot be written loses a line rather than
				// a sign-in.
				Lines: func(lines []string) {
					_ = lifecycle.Update(roots.Installation.Path(), func(r *lifecycle.Record) { r.Sessions = lines })
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
				return act.SignedIn(roots.StateRoot.Path(), roots.Installation, signed.Human, signed.Reference, signed.Proof)
			}
			fleetActs := uiFleetActs{checkout: roots.Checkout, owners: defaultIntentOwners}
			return httpd.New(httpd.Info{Checkout: rec.Checkout, Installation: roots.Installation, StartedAt: rec.StartedAt, EngineBuild: rec.EngineBuild, ExecutableDigest: rec.ExecutableDigest, BundleDigest: bundleDigest,
				Describe: func() (workspace.Workspace, error) {
					described, describeErr := workspace.Describe(
						workspace.Roots{Checkout: roots.Checkout, Installation: roots.Installation, StateRoot: roots.StateRoot},
						workspace.Record{EngineBuild: rec.EngineBuild, StartedAt: rec.StartedAt, ExecutableDigest: rec.ExecutableDigest},
						subject,
					)
					if describeErr != nil {
						return described, describeErr
					}
					// What the private store holds and what it is kept to,
					// on the read Settings already makes (g1-s54 D3). It is
					// measured per request, so a human watching the number
					// after a sweep sees the number after the sweep.
					store := workspace.DescribeStore(storeHome, storeProblem, roots.Checkout,
						workspace.StoreBounds{
							WireMB:           storeBounds.WireMB,
							ConversationMB:   storeBounds.ConversationMB,
							ConversationDays: storeBounds.ConversationDays,
						})
					described.Store = &store
					return described, nil
				},
				Observe: ledger.Observe,
				Fetch:   advance,
				// The Fleet page, and the holder flags the board and the
				// Overview carry with it. It reads the presence copy this
				// server fetched for itself and starts no fetch of its
				// own.
				Fleet: fleetReader(roots, presence.Succeeded, presence.State),
				Watch: presenceWatch,
				// The board panel: every seat of this host and how far its
				// work is, read and classified on request (D14-r2).
				Board: withProofLogs(batchowner.HostBoardSource(roots.Installation.Path())),
				// One machine of this fleet joining on this host. It is
				// the signed-in human's act and nothing weaker, which the
				// route checks for itself: a launch spends disk, a build
				// and their own authorization.
				Launch: launchStarter(roots),
				// A stopped launch put out of sight: the record is marked
				// and kept, and nothing on disk is deleted.
				DiscardLaunch: launchDiscarder(roots),
				// The landing lane card's Land now: landing run, run once
				// as the terminal runs it (ui_landnow.go's seam). The route
				// asks for the signed-in session itself.
				LandNow: func() (httpd.LandNowAnswer, error) {
					return landNowRun(roots)
				},
				// The fleet panel's Pause, Resume and Stop (ui_landnow.go's
				// uiFleetActs, R-142-ui): the route admits the person with
				// act.SignedIn, and each verb runs in this process with that
				// person answering its person seams. Stop is admitted first,
				// by machine stop's own reading, and never for this checkout.
				PauseLane:   fleetActs.pause,
				ResumeLane:  fleetActs.resume,
				AdmitStop:   fleetActs.admitStop,
				StopMachine: fleetActs.stop,
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
				// The review record a review sitting starts on, created by the
				// server with the head it resolved (g1-s65 D1), and the reads
				// its desk makes over the candidate's own tree (D4) — or, for
				// a sitting that shapes a record, over the checkout as it
				// stands, beneath the root the document reader opens (g1-s67
				// D2).
				CreateReview: func(asked project.NewReview) (project.Written, error) {
					return project.CreateReview(projectRoots(roots), asked, time.Now())
				},
				Review: &review.Owner{Git: gittree.Workspace{Dir: roots.Checkout}, Checkout: projectRoots(roots).Checkout},
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
				// answered with the document read again from disk. It runs
				// under the clone's one lock, the one a verdict holds from
				// its last read of a review record to its publication, so a
				// review record is never saved between the two (F-2 of read
				// 0096f159).
				EditDocument: func(id, source, revision string) (project.Document, error) {
					var edited project.Document
					err := authority.Holding(func() error {
						var editErr error
						edited, editErr = project.EditDocument(projectRoots(roots), id, source, revision, time.Now().UTC())
						return editErr
					})
					return edited, err
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
				// Not now, and back. Admitted from a signed-in browser
				// under R-125-m1u, at the three rows that ruling names
				// and at no others; every other refusal is the engine's
				// and reaches the page in its own words.
				Park: func(signed *session.Session, id, because string) error {
					hand, err := acting(signed)
					if err != nil {
						return err
					}
					if err := hand.Park(id, because); err != nil {
						return err
					}
					advance()
					return nil
				},
				Unpark: func(signed *session.Session, id string) error {
					hand, err := acting(signed)
					if err != nil {
						return err
					}
					if err := hand.Unpark(id); err != nil {
						return err
					}
					advance()
					return nil
				},
				// And the act that says a goal will never be worked,
				// with the reason and the successor carrying its work.
				// Admitted from a signed-in browser under R-128-ui; the
				// dependents rule, the successor rule and the reason
				// rule are the engine's, and reach the page in its own
				// words.
				Abandon: func(signed *session.Session, id, because, successor string) error {
					hand, err := acting(signed)
					if err != nil {
						return err
					}
					if err := hand.Abandon(id, because, successor); err != nil {
						return err
					}
					advance()
					return nil
				},
				// The one direct edit the master design admits: the
				// intent, the next step and the labels of a goal nobody
				// has approved. The state it must be in is the
				// mutation's own allowlist, so a goal approved or
				// claimed since the page read it is refused here rather
				// than rewritten.
				Edit: func(signed *session.Session, id string, edited act.Edited) error {
					hand, err := acting(signed)
					if err != nil {
						return err
					}
					if err := hand.Edit(id, edited); err != nil {
						return err
					}
					advance()
					return nil
				},
				// The review's verdict (g1-s69 D1, D2): goal review under the
				// signed-in session, with the record the page names resolved
				// beneath this checkout and read from disk as it now stands.
				Verdict: func(signed *session.Session, id string, asked act.Reviewed) (act.Recorded, error) {
					hand, err := acting(signed)
					if err != nil {
						return act.Recorded{}, err
					}
					asked.Record = filepath.Join(roots.Checkout, filepath.FromSlash(asked.Record))
					recorded, err := hand.Review(id, asked)
					if err != nil {
						return act.Recorded{}, err
					}
					advance()
					return recorded, nil
				},
				// The loop from the room (g1-s66 §6): Send to critique and
				// Answer the round run design review as the public verb runs,
				// behind the route's signed-in session rule, as a launch
				// does; the chain is read from the run store and a press
				// writes one row of the engine's own decisions file.
				DesignReview: func(_ *session.Session, design string, asked httpd.DesignAsked) (httpd.DesignAnswer, error) {
					return designReviewRun(roots, design, asked)
				},
				DesignLoop: func(design string) (httpd.DesignLoop, error) {
					return designLoopRead(roots, design)
				},
				DesignDecide: func(design string, round int64, row httpd.DesignRow) (httpd.DesignLoop, error) {
					return designDecide(roots, design, round, row)
				},
				// The goal's candidate, from the room's pill (D3): the three
				// app forms, run as the public verbs are run.
				Candidate: func(goal, at, action string) (httpd.Candidate, error) {
					return candidateRun(roots, goal, at, action)
				},
				BudgetDefaults: func() (map[string]goalbudget.Budget, error) {
					return tierBudgets(roots.Installation.Path())
				},
				// The landing gate (g1-s70 §6): the two settings through the
				// layered resolution the engine reads, the room's hold and
				// release, and the Decide sheet's decision, under the sign-in.
				LandingGate: func() (config.LandingGate, error) {
					return config.ResolveLandingGate(roots.Installation.Path("metasystem.conf"))
				},
				Sitting: func(signed *session.Session, id, record string, open bool) error {
					hand, err := acting(signed)
					if err != nil {
						return err
					}
					// The room names the record from the checkout; the act
					// resolves it in the installation's review home.
					if err := hand.Sitting(id, filepath.Join(roots.Checkout, filepath.FromSlash(record)), open); err != nil {
						return err
					}
					advance()
					return nil
				},
				LandWithoutSitting: func(signed *session.Session, id, tip, reason string) error {
					hand, err := acting(signed)
					if err != nil {
						return err
					}
					if err := hand.LandWithoutSitting(id, tip, reason); err != nil {
						return err
					}
					advance()
					return nil
				},
				// The steward's notification journal for this seat.
				// The steward writes it as a side effect of reaching the
				// operator; the interface reads it and nothing else of
				// the steward's, and never writes to it.
				NotificationJournal: notificationJournal(roots),
				// What this seat has been asked and has not answered,
				// and what this human has ruled. Both are read per
				// request for the reason every other reader is: a
				// question a seat opened while the server runs, and a
				// ruling appended to the register by hand, are read
				// without a restart. The register is read through the
				// steward's own package, so the page and the sweep
				// cannot disagree about what it says.
				// The walk is tolerant by its own contract: one
				// unreadable question file never hides the others, and
				// its path comes back on a second channel, which the
				// Fleet page names (uiAsks).
				Asks: uiAsks(roots),
				// The register is read from the INSTALLATION, because
				// that is where the kit's memory home is on every layout
				// this interface serves: a checkout that is not the
				// installation has no memory/ of its own, and reading
				// there answered an empty register for a file that exists.
				// The channel and the journal stay at the checkout, where
				// the steward writes them.
				Rulings: func() (rulings.Register, error) {
					return rulings.Read(roots.Installation.Path())
				},
				// And where that register is from the checkout, so that
				// every destination naming it opens in the reader, which
				// resolves against the checkout.
				RegisterPath: registerFromCheckout(roots),
				// The known-issues register, read from the INSTALLATION
				// for the rulings register's reason: that is where the
				// kit's memory home is on every layout this interface
				// serves. And where it is from the checkout, so that
				// "Open the register" opens in the reader, which resolves
				// against the checkout.
				KnownIssues: func() (knownissues.Register, error) {
					return knownissues.Read(roots.Installation.Path())
				},
				KnownIssuesPath: knownIssuesFromCheckout(roots),
				// The landing page's last-visit marker, beside this
				// server's own lifecycle state. It is the one thing the
				// interface writes for itself rather than for the
				// ledger: preference state about when a human last
				// looked, which no verb reads and losing which changes a
				// comparison window and nothing else.
				Visit: func(human string, now time.Time) (time.Time, bool, error) {
					return overview.Visit(roots.Installation.Path(), human, now)
				},
				// The Decisions page's own marker, in the same file,
				// through the same owner, under an entry of its own. A
				// read there must not move the landing page's boundary:
				// the two pages are read on different rhythms, and one
				// marker for both would hide from a human what they never
				// saw on the other.
				VisitDecisions: func(human string, now time.Time) (time.Time, bool, error) {
					return overview.VisitPage(roots.Installation.Path(), overview.PageDecisions, human, now)
				},
				// The Application page's own marker, under an entry of
				// its own, for the reason Decisions keeps one: a human
				// reads the three pages on three rhythms, and one marker
				// for all of them would hide from them what they never
				// saw on the others.
				VisitApplication: func(human string, now time.Time) (time.Time, bool, error) {
					return overview.VisitPage(roots.Installation.Path(), application.PageName, human, now)
				},
				// The notepad, resolved once above. It is the one piece of
				// state this interface keeps OUTSIDE the checkout, which
				// is the whole of why it is its own owner.
				Stickies: notepad,
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
			// Ownership is held from here, which is what the gate says: the
			// freshness loop, the presence fetch and the store's
			// housekeeping all start with this line and none of them
			// touches anything before it.
			owned.Open()
			if ready != nil {
				fmt.Fprintln(ready, "ready "+address)
				_ = ready.Close()
				ready = nil
			}
			fmt.Fprintf(stderr, "interface running at http://%s (pid %d)\n", address, os.Getpid())
		},
		// The loop ends while this process still holds the checkout, so a
		// tick in flight cannot advance the accepted ref after the next
		// server has taken the lock.
		Releasing: func() {
			stopLoop()
			<-loopStopped
			stopPresence()
			<-presenceStopped
			<-launchStopped
			// Housekeeping ends here too, and not after Serve returns: a
			// trim in flight when the successor takes the lock would be
			// replacing a transcript the next server's Conversation is
			// already appending to.
			stopHousekeeping()
		},
	})
	stopLoop()
	<-loopStopped
	stopPresence()
	<-presenceStopped
	<-launchStopped
	// And again for the path Releasing never ran on: a Serve that returned
	// early — another server already owns this checkout — opened no gate,
	// so housekeeping is still waiting at it and only its own cancellation
	// ends that wait.
	stopHousekeeping()
	if err != nil {
		return refuse(lifecycle.ServeFailure(roots.Installation.Path(), err))
	}
	return 0
}

// storeSweep is how often housekeeping asks. Once a day: the bounds are
// retention targets rather than disk ceilings, and what a long-running server
// adds to the store in a day is small against them (g1-s54 D1, D4).
const storeSweep = 24 * time.Hour

// uiAdvance is the one look at the canonical branch both human presses take:
// the act routes' own, after a publication, and the board's Refresh.
//
// It classifies this clone's transaction journal first. A push that landed and
// failed its confirmation leaves an entry at pushed, and the engine mutates
// nothing in this clone until somebody classifies it; a Refresh that only
// advanced the accepted ref showed a current board while every act stayed
// refused. The classification is the engine's own recovery rule, under the
// policy this interface can carry (act.Reconcile), and it is what the terminal's
// `goal recover` runs. A journal nothing is pushed in costs one directory read.
func uiAdvance(root string, endpoint goal.Endpoint) (goal.AdvanceResult, error) {
	if err := act.Reconcile(root, endpoint); err != nil {
		return goal.AdvanceResult{}, err
	}
	return goal.FetchAdvanceBounded(endpoint, snapshot.FetchBudget)
}

// startStoreHousekeeping starts one server's housekeeping and answers the stop
// the caller owes it: cancel its context and join its goroutine.
//
// Housekeeping's own context is why there is a function here rather than a
// goroutine inline. It is owed twice, and both are the point:
//
//   - inside Serve's Releasing, which runs while this process still owns the
//     checkout, so a trim or a rotation in flight cannot overlap the appends of
//     the server that takes the lock next;
//   - after Serve returns, for the path Releasing never ran on. A Serve that
//     refused because another server already runs opens no gate and calls no
//     Ready, so housekeeping is still waiting at the gate on a context that
//     ends only when this process does — and a caller that waited on it without
//     cancelling it first would hang there rather than print the refusal.
//
// Calling the stop twice cancels a cancelled context and reads a closed
// channel, which is why both callers may call it.

func startStoreHousekeeping(ctx context.Context, owned *snapshot.Gate, keeper *storeKeeper, tick <-chan time.Time) func() {
	housekeeping, stop := context.WithCancel(ctx)
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		if keeper == nil {
			return
		}
		keeper.Serve(housekeeping, owned, tick)
	}()
	return func() {
		stop()
		<-stopped
	}
}

// storeKeeper is the private store's housekeeping: the two bounds of g1-s54,
// asked of the owners that hold the files.
//
// It asks and never acts on a file itself. The wire journal rotates through its
// own writer, because a rename from outside would leave the runtime writing the
// renamed file; a transcript is trimmed by the conversation that appends to it,
// under the mutex the appends take. Housekeeping's whole part is the cadence.
//
// It is the fleet fetch loop's shape (internal/ui/fleet/fetch.go): one
// Consider, and a Run that considers on every tick the caller sends. The server
// sends a real ticker and a test sends a channel it controls, so the daily
// cadence is proven without a day passing. Serve is the two of them inside the
// one interval this process owns the checkout, which is the only interval in
// which either owner may touch a file.
//
// Nothing here touches a checkout or the state root. D2 is that housekeeping
// works only on what is under the account's home, and the two owners it asks
// are the only two writers of it.
type storeKeeper struct {
	// journal is the wire journal's own writer, or nil where this seat keeps
	// none; wire is the size it may reach, in bytes, and zero disables it.
	journal *partner.Journal
	wire    int64
	// trim asks the conversation owner to keep its transcripts within their
	// bounds, and answers how many messages went.
	trim func() (int, error)
	// say is where a sweep's own words go, which is this server's error stream.
	say func(string)
}

// Consider runs one sweep, asking both owners. A refusal from one does not stop
// the other: they hold different files, and the store is smaller for either.
func (k *storeKeeper) Consider() {
	if k.journal != nil {
		rotated, err := k.journal.Rotate(k.wire)
		switch {
		case err != nil:
			k.said(err.Error())
		case rotated:
			k.said("the Partner's wire journal passed its bound and was rotated; one previous is kept at " + k.journal.Previous())
		}
	}
	if k.trim != nil {
		cut, err := k.trim()
		if cut > 0 {
			k.said("this seat's conversations were trimmed from their oldest end: " +
				strconv.Itoa(cut) + " messages went; only what was saved to records survives")
		}
		if err != nil {
			k.said(err.Error())
		}
	}
}

// Serve is housekeeping's whole life under one server: it waits until this
// process owns the checkout, sweeps once, and sweeps again on every tick until
// its context ends.
//
// The first sweep is inside the ownership interval, not before it. Before the
// lock is taken there may be an incumbent server serving this same store, and a
// sweep then would trim the transcript it is appending to — the two processes
// hold two Conversation objects with two mutexes over one file — and rotate the
// journal its runtime writes through. A server that never wins the lock waits
// here until its context ends and sweeps nothing at all.
func (k *storeKeeper) Serve(ctx context.Context, owned *snapshot.Gate, tick <-chan time.Time) {
	if !owned.Wait(ctx) {
		return
	}
	k.Consider()
	k.Run(ctx, tick)
}

// Run considers on every tick until the context ends or the tick closes.
func (k *storeKeeper) Run(ctx context.Context, tick <-chan time.Time) {
	for {
		select {
		case <-ctx.Done():
			return
		case _, open := <-tick:
			if !open {
				return
			}
			k.Consider()
		}
	}
}

func (k *storeKeeper) said(line string) {
	if k.say == nil {
		return
	}
	k.say(line)
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

// uiRootsAndListen resolves the interface's checkout, installation and
// state root, and for the verbs that bind, its validated listen address.
func uiRootsAndListen(verb, repo, root, listen string, listenSet bool) (lifecycle.Roots, string, error) {
	metasystemRoot, err := upMetasystemRoot(root)
	if err != nil {
		return lifecycle.Roots{}, "", err
	}
	roots, err := lifecycle.ResolveRoots(repo, metasystemRoot)
	if err != nil {
		return lifecycle.Roots{}, "", err
	}
	listen, err = uiListen(verb, roots, listen, listenSet)
	if err != nil {
		return lifecycle.Roots{}, "", err
	}
	return roots, listen, nil
}

// uiListen is the validated listen address for the verbs that bind one.
func uiListen(verb string, roots lifecycle.Roots, listen string, listenSet bool) (string, error) {
	if verb != "start" && verb != "serve" && verb != "restart" {
		return listen, nil
	}
	listen, err := config.UIListen(roots.Installation.Path("metasystem.conf"), listen, listenSet)
	if err != nil {
		return "", err
	}
	return lifecycle.ValidateListen(listen)
}

// uiLifecycleResult is one interface lifecycle verb's result: the lines the
// verb prints and the typed facts behind them.
type uiLifecycleResult struct {
	Result  lifecycle.Result
	State   lifecycle.State          // status only
	Restart *lifecycle.RestartReport // restart only
	// Unchanged is a start or stop whose effect already held (R-129-ui).
	Unchanged bool
	// Decision is what a person does about a refusal, when one is known.
	Decision string
	// Seat is another machine of this computer whose interface the verb
	// acted on; nil when it acted on this seat only.
	Seat *uiSeat
	// Seats are the other machines of this computer the verb names, and
	// SeatsProblems why that list may be incomplete.
	Seats         []uiSeatView
	SeatsProblems []string
}

// uiLifecycleEffects are the process effects of the interface lifecycle:
// the prober that judges the recorded server, the spawn that launches one,
// and the engine it runs.
type uiLifecycleEffects struct {
	prober identity.Prober
	spawn  lifecycle.Spawn
	// send signals the recorded server (nil is syscall.Kill) and after is
	// the stop's lock-wait timer (nil is time.After).
	send  identity.SignalFunc
	after func(time.Duration) <-chan time.Time
	// seats reads the other machines of this computer; nil is none.
	seats func() uiSeatInventory
	// engine is the engine an installation carries, and listenFor the
	// validated listen address for a seat's roots.
	engine    func(installation string) (string, error)
	listenFor func(target lifecycle.Roots) (string, error)
	// listenProblem is why this seat's own listen address does not
	// resolve; a restart refuses on it only when it restarts this seat.
	listenProblem error
}

// uiLifecycleFor runs one interface lifecycle verb with the interface's own
// defaults; an error is a refusal before anything was done.
func uiLifecycleFor(verb string, roots lifecycle.Roots, options uiIntentOptions) (uiLifecycleResult, error) {
	return uiLifecycleForWith(verb, roots, options, uiLifecycleEffects{prober: identity.KernelProber{}, spawn: lifecycle.ExecSpawn, engine: uiInstallationEngine})
}

// uiLifecycleForWith is uiLifecycleFor with the process effects given. A
// restart may restart another seat at that seat's own address, so this
// seat's address that does not resolve refuses only its own restart.
func uiLifecycleForWith(verb string, roots lifecycle.Roots, options uiIntentOptions, effects uiLifecycleEffects) (uiLifecycleResult, error) {
	listen, err := uiListen(verb, roots, options.listen, options.listenSet)
	if err != nil && verb != "restart" {
		return uiLifecycleResult{}, err
	}
	effects.seats, effects.listenProblem = options.seats, err
	effects.listenFor = func(target lifecycle.Roots) (string, error) {
		return uiListen("restart", target, options.listen, options.listenSet)
	}
	return uiLifecycleRunWith(verb, roots, listen, options.waitSeconds, effects), nil
}

func uiLifecycleRunWith(verb string, roots lifecycle.Roots, listen string, waitSeconds int64, effects uiLifecycleEffects) uiLifecycleResult {
	prober := effects.prober
	stop := lifecycle.StopOptions{Prober: prober, Send: effects.send, After: effects.after, Wait: time.Duration(waitSeconds) * time.Second}
	// The other seats are read at most once per verb, and only when asked.
	var inventory *uiSeatInventory
	seats := func() uiSeatInventory {
		if inventory == nil {
			read := effects.seats()
			inventory = &read
		}
		return *inventory
	}
	var collision *uiSeatView
	var launched *uiLaunched
	start := func(target lifecycle.Roots, listen, engine string) lifecycle.Result {
		// Another machine of this computer holding the address is named
		// before anything is spawned, but only when the target runs no
		// interface: one running elsewhere keeps today's path whole.
		if effects.seats != nil {
			if own, err := lifecycle.Read(target.Installation.Path(), prober); err == nil && (own.State == lifecycle.Stopped || own.State == lifecycle.Stale) {
				if refusal, other := uiStartCollision(uiSeatsExcept(seats(), target), prober, listen); other != nil {
					collision = other
					return refusal
				}
			}
		}
		// The pair the new server was ready with is kept here: the record
		// it writes may already be gone when the verb reports.
		address, pid, err := lifecycle.Launch(lifecycle.LaunchSpec{
			Executable: engine,
			Args:       lifecycle.ServeArgs(target.Checkout, target.Installation.Path(), listen),
			Dir:        target.Checkout,
			LogPath:    filepath.Join(lifecycle.Dir(target.Installation.Path()), "server.log"),
		}, effects.spawn, 0)
		if err != nil {
			return lifecycle.Result{Lines: []string{err.Error()}, Code: 1}
		}
		launched = &uiLaunched{address: address, pid: pid}
		return lifecycle.Result{Lines: []string{fmt.Sprintf("interface running at http://%s (pid %d)", address, pid)}}
	}
	collided := func(result uiLifecycleResult) uiLifecycleResult {
		if collision != nil {
			result.Seats = []uiSeatView{*collision}
		}
		return result
	}
	switch verb {
	case "start":
		engine, refusal := uiEngineFor(effects.engine, roots, "")
		if refusal != nil {
			return *refusal
		}
		result, unchanged := lifecycle.StartOnce(roots.Installation.Path(), prober, listen, func() lifecycle.Result { return start(roots, listen, engine) })
		return collided(uiLifecycleResult{Result: result, Unchanged: unchanged})
	case "status":
		// The running server is compared with the engine a restart would
		// launch: the installation's, not the binary typed.
		digest := func() (string, error) {
			engine, err := effects.engine(roots.Installation.Path())
			if err != nil {
				return "", err
			}
			return uiFileDigest(engine)
		}
		result, state := lifecycle.StatusReport(roots.Installation.Path(), prober, digest)
		status := uiLifecycleResult{Result: result, State: state}
		if effects.seats != nil && (state == lifecycle.Stopped || state == lifecycle.Stale) {
			return uiStatusAcrossSeats(status, seats(), prober)
		}
		return status
	case "stop":
		result, unchanged := lifecycle.StopReport(roots.Installation.Path(), stop)
		if unchanged && effects.seats != nil {
			return uiStopAcrossSeats(result, seats(), stop)
		}
		return uiLifecycleResult{Result: result, Unchanged: unchanged}
	}
	if effects.seats != nil {
		if own, err := lifecycle.Read(roots.Installation.Path(), prober); err == nil && (own.State == lifecycle.Stopped || own.State == lifecycle.Stale) {
			if across, ok := uiRestartAcrossSeats(seats(), prober, func(other uiSeat, prefix string) uiLifecycleResult {
				listen, err := effects.listenFor(other.Roots)
				if err != nil {
					return uiLifecycleResult{Result: lifecycle.Result{Lines: []string{prefix + err.Error() + "; nothing was done"}, Code: 1}}
				}
				engine, refusal := uiEngineFor(effects.engine, other.Roots, prefix)
				if refusal != nil {
					return *refusal
				}
				launched = nil
				report := lifecycle.RestartReportFor(other.Roots.Installation.Path(), stop, func() lifecycle.Result { return start(other.Roots, listen, engine) })
				return collided(uiLifecycleResult{Result: report.Result, Restart: &report})
			}, func() *uiLaunched { return launched }); ok {
				return across
			}
		}
	}
	if effects.listenProblem != nil {
		return uiLifecycleResult{Result: lifecycle.Result{Lines: []string{effects.listenProblem.Error() + "; nothing was done"}, Code: 1}}
	}
	engine, refusal := uiEngineFor(effects.engine, roots, "")
	if refusal != nil {
		return *refusal
	}
	report := lifecycle.RestartReportFor(roots.Installation.Path(), stop, func() lifecycle.Result { return start(roots, listen, engine) })
	return collided(uiLifecycleResult{Result: report.Result, Restart: &report})
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

// registerFromCheckout is where the rulings register is RELATIVE TO THE
// CHECKOUT, which is the root the document reader opens a path against.
//
// The register itself is read from the installation, because that is the
// kit's memory home; the reader that opens a destination is the checkout's.
// On the self-hosted layout the two are the same directory and this answers
// "memory/rulings.md"; on the layout this interface most often serves — a
// checkout whose installation is a directory inside it — it answers
// "metasystem/memory/rulings.md", which is the path that opens.
//
// Where the installation is not under the checkout at all, no
// checkout-relative path names that file, so this answers nothing and the
// composer keeps its own default: a destination that cannot open is not
// improved by a path that walks out of the repository.
func registerFromCheckout(roots lifecycle.Roots) string {
	relative, err := filepath.Rel(roots.Checkout, rulings.Path(roots.Installation.Path()))
	if err != nil {
		return ""
	}
	slashed := filepath.ToSlash(relative)
	if slashed == ".." || strings.HasPrefix(slashed, "../") {
		return ""
	}
	return slashed
}

// The steward and the question channel keep this seat's records beneath the
// STATE ROOT, which in the self-hosted layout is a directory inside the Git
// checkout and not the checkout itself. Every reader of those records is
// handed the state root: one handed the checkout finds no file where one was
// written, and an absent file reads as "nothing yet" rather than as an error.

// notificationJournal is the journal the steward appends its notices to.
func notificationJournal(roots lifecycle.Roots) string {
	return steward.NotificationJournalPath(roots.Installation.Path())
}

// seatHealth is the steward's last recorded health verdict for this seat.
func seatHealth(roots lifecycle.Roots) *fleet.Health { return fleet.ReadHealth(roots.StateRoot.Path()) }

// openAsks is what `metasystem question list` reads: the channel's open
// questions, with an unreadable file left out rather than hiding the others.
func openAsks(roots lifecycle.Roots) []channel.Question {
	open, _ := uiAsks(roots)()
	return open
}

// knownIssuesFromCheckout is the same for the known-issues register, which is
// read from the same memory home and opened through the same reader.
func knownIssuesFromCheckout(roots lifecycle.Roots) string {
	relative, err := filepath.Rel(roots.Checkout, knownissues.Path(roots.Installation.Path()))
	if err != nil {
		return ""
	}
	slashed := filepath.ToSlash(relative)
	if slashed == ".." || strings.HasPrefix(slashed, "../") {
		return ""
	}
	return slashed
}

// uiAsks is the channel's open questions as the interface reads them, from
// the root `metasystem question list` reads: every open question the tolerant
// walk could read, and the records it could not beside them as an
// *httpd.UnreadQuestions, so a page shows what was asked and says what it could
// not read rather than reading it as nothing asked.
func uiAsks(roots lifecycle.Roots) func() ([]channel.Question, error) {
	return func() ([]channel.Question, error) {
		open, unreadable := channel.WalkOpenQuestions(roots.StateRoot.Path())
		if len(unreadable) > 0 {
			return open, &httpd.UnreadQuestions{Records: unreadable}
		}
		return open, nil
	}
}

// withProofLogs gives the board panel's source this computer's proof logs
// (fleet-panel-ux step 2, 2a.3); a build with no board reader stays one.
func withProofLogs(source *httpd.BoardSource) *httpd.BoardSource {
	if source != nil {
		source.ProofLog = uiProofLogs(batchowner.LandingLaneHome)
	}
	return source
}

// uiProofLogs is the proof-log seam: plain.ProofLog over the installation
// the lane record of this computer names, read on every request. No lane
// registered is no log served; a lane home it cannot find, or a record it
// cannot place, is said as such.
func uiProofLogs(home func() (string, error)) func(string) (string, error) {
	return func(attempt string) (string, error) {
		laneHome, err := home()
		if err != nil {
			return "", fmt.Errorf("this computer's landing lane can't be found: %w", err)
		}
		record, registered, err := lane.Read(laneHome)
		if err != nil {
			// A record that does not read is said, even when its paths would
			// still place a lane.
			return "", fmt.Errorf("this computer's landing lane record can't be read: %w", err)
		}
		if !registered {
			return "", fmt.Errorf("%w: no landing lane is registered on this computer", plain.ErrNoProofLog)
		}
		layout, err := record.Layout()
		if err != nil {
			return "", fmt.Errorf("this computer's landing lane record can't be read: %w", err)
		}
		return plain.ProofLog(string(layout.Install), attempt)
	}
}

// admitStop reads a machine's name for the fleet panel's Stop as machine
// stop reads it, from the checkout this server serves: discoverHostMachines
// (this checkout, the host registry's armed and stopped checkouts, the
// landing lane's), then matchMachine (fleet-panel-ux-step2 slice 2b, S2-02).
// nil admits the stop. Before any stop runs it refuses, in machine stop's own
// two lines, a name that is no machine of this computer, one that names
// several and one that runs on another computer; and, in its own, the
// checkout serving the page (S2-01): a stop run inside this server either
// ends it in the middle of the act or, since the stop leaves its own process
// standing, reports the machine stopped while this server still runs. Either
// way the page could not show the result, so a terminal stops that one.
//
// Whether the board names a seat is not the test: the board's seats are the
// armed checkouts only, so a machine just stopped leaves them, and a second
// tab's Stop would be refused here instead of reaching the verb's own "already
// stopped". A stopped machine is still this computer's machine.
func (acts uiFleetActs) admitStop(name string) (*httpd.LandNowAnswer, error) {
	command, found := findIntentCommand(machineStopVerb)
	if !found {
		return nil, errors.New("this engine has no metasystem machine stop, so the interface cannot stop a machine")
	}
	inv := &intentInvocation{command: command, raw: []string{"--repo", acts.checkout, "--", name}, stdout: io.Discard, stderr: io.Discard,
		cwd: acts.checkout, owners: acts.owners(), input: intentInput{values: map[string][]string{"repo": {acts.checkout}, "json": {"true"}}, args: []string{name}}}
	// A refusal of the verb's reading is said as the verb says it: rendered
	// with --json and read back as every act's answer is.
	refuse := func(result intentResult) (*httpd.LandNowAnswer, error) {
		answered, err := verbAnswer(machineStopVerb, command, nil, func(_ intentCommand, _ []string, stdout, stderr io.Writer) {
			inv.stdout, inv.stderr = stdout, stderr
			inv.render(result)
		})
		if err != nil {
			return nil, err
		}
		return &answered, nil
	}
	if problem := inv.selectLayoutRoot(); problem != nil {
		return refuse(*problem)
	}
	fleet := map[string]bool{}
	if report, err := inv.owners.processes.fleet(inv.layout.GitRoot, false, seatFleetNow()); err == nil {
		fleet = fleetNames(report)
	}
	matched, problem := inv.matchMachine(inv.discoverHostMachines(fleet), name)
	if problem != nil {
		return refuse(*problem)
	}
	if matched.This || matched.Checkout == acts.checkout || slices.Contains(matched.paths, acts.checkout) {
		return &httpd.LandNowAnswer{Outcome: intentRefused,
			Summary: matched.Name + " serves this page, so it is stopped at a terminal and not from here; nothing was stopped",
			Next: &httpd.LandNowNext{Argv: []string{"metasystem", "machine", "stop", matched.Name},
				Reason: "at a terminal on this computer: stopped from here, the stop would end the server showing this page before it could say what it did"}}, nil
	}
	return nil, nil
}
