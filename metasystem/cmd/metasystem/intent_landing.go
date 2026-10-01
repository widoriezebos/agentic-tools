package main

// The landing object (batch-lane design U12, lane design r10): the landing
// lane on this computer, where every seat's work is proved and pushed.
// landing status reads it; landing set registers or moves it; landing stop
// and start pause and resume it, and unset takes it away. Each renders the
// one lane.View /api/board carries.

import (
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/cadence"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gaterun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/helm"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batchowner"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/kernel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/laneengine"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lock"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/textui"
)

// The landing verbs' refusal codes (register rows name their sites).
const (
	codeLandingLaneBusy    = "LANDING_LANE_BUSY"
	codeLandingLanePushing = "LANDING_LANE_PUSHING"
	// codeLandingOldOwnerHolds is landing set refused while the old owner
	// lineage holds claims (design r10 §5, R9-01).
	codeLandingOldOwnerHolds = "LANDING_LANE_OLD_OWNER_HOLDS"
)

// laneVerbOwners are the landing verbs' seams; the zero value is production.
type laneVerbOwners struct {
	home func() (string, error)
	// probe reads whether the lane's landing agent runs.
	probe    func(root string) (lane.OwnerProbe, error)
	records  func(root string) ([]batch.Record, error)
	validate func(root, seatRoot string, now time.Time) (string, error)
	by       func(installation string) string
	now      func() time.Time
	// person proves the person at the enrolled terminal of an installation
	// and names them.
	person func(root string) (string, error)
	// ready says whether the lane can run in a lane checkout: a
	// *lane.Refusal naming the fix when it cannot.
	ready func(root string) error
	// machine is a checkout's machine nickname.
	machine func(root string) (string, error)
	// helm reads whether a joined unit's seat is at the helm, which holds
	// its batch whole.
	helm func(seatRoot string) helm.State
	// installation is the metasystem installation of a lane checkout: the
	// module root, where the lane's enrollment lives.
	installation func(root string) (string, error)
	// engine admits this process as the lane's enrolled engine (K5): every
	// kernel verb calls it first.
	engine func(checkout, installation string, retry []string) (laneengine.Identity, error)
	// advance moves the lane to landed main's engine.
	advance func(laneengine.AdvanceRequest) (laneengine.AdvanceOutcome, error)
	// begin records a batch's canonical series (K4); prove runs one
	// subject of it (K6).
	begin func(kernel.BeginRequest) (kernel.BeginOutcome, error)
	prove func(kernel.ProveRequest) (batch.ProofAttempt, error)
	// unset runs landing unset's journaled steps for the person by.
	unset func(home, by string, force bool) (lane.UnsetReport, error)
	// laneHeld lists the goals the ledger, read from an installation,
	// shows held by the landing lane.
	laneHeld func(installation string) ([]string, error)
	// validation reads whether the standing validation is due, one of the
	// landing agent's wake reasons (A-a).
	validation func(root string, now time.Time) (bool, error)
	// finalization reads whether a reserved validation run awaits its
	// finalization in the host lane state at home, the landing agent's
	// finalization-pending wake reason (K-f).
	finalization func(home string) (bool, error)
	// evidence reads a batch's begin and proof records for landing
	// publish; nil reads none (K-b writes them).
	evidence lane.PublishEvidence
	// remoteURL is the landing checkout's origin.
	remoteURL func(root string) (string, error)
	// publish is the lane's publication boundary for a landing.
	publish func(home string, tuple lane.Tuple) error
	// oldOwnerClaims lists the goals the ledger, read from an
	// installation, shows claimed by the old owner lineage (R9-01).
	oldOwnerClaims func(installation string) ([]string, error)
	// agent proves the caller descends from the landing agent's launch that
	// holds the lane checkout (K7).
	agentCaller func(checkout string) error
	// returnMember runs one typed return (K8).
	returnMember func(batchowner.MemberReturn) (batchowner.MemberReturnReport, error)
	// validateRun runs one landing validate (K-f).
	validateRun func(laneValidateRequest) (gaterun.ValidateOutcome, error)
}

func (inv *intentInvocation) landing() laneVerbOwners {
	owners := inv.owners.landing
	if owners.home == nil {
		owners.home = batchowner.LandingLaneHome
	}
	if owners.probe == nil {
		owners.probe = newLandingAgent().probe
	}
	if owners.validate == nil {
		owners.validate = batchowner.ValidateLandingCheckout
	}
	if owners.by == nil {
		owners.by = batchowner.LandingLaneRegistrant
	}
	if owners.now == nil {
		owners.now = func() time.Time { return time.Now().UTC() }
	}
	if owners.person == nil {
		owners.person = provenPerson(humanauthority.KernelReader{}, func() int64 { return int64(os.Getppid()) }, goalCommandNow)
	}
	if owners.ready == nil {
		owners.ready = batchowner.LandingLaneReady
	}
	if owners.machine == nil {
		owners.machine = goal.ResolveMachine
	}
	if owners.helm == nil {
		owners.helm = helm.Active
	}
	if owners.engine == nil {
		owners.engine = laneengine.RequireSelf
	}
	if owners.installation == nil {
		owners.installation = func(root string) (string, error) {
			layout, err := inv.owners.resolver.ResolveLayout(root)
			return layout.InstallationRoot, err
		}
	}
	if owners.advance == nil {
		owners.advance = func(request laneengine.AdvanceRequest) (laneengine.AdvanceOutcome, error) {
			return laneengine.Advance(request, laneengine.ProductionConditions(request.Home, request.Checkout),
				laneengine.ProductionSteps(request.Checkout, request.Installation))
		}
	}
	if owners.begin == nil {
		owners.begin = func(request kernel.BeginRequest) (kernel.BeginOutcome, error) {
			return kernel.Begin(request, kernel.ProductionBeginSeams())
		}
	}
	if owners.prove == nil {
		owners.prove = func(request kernel.ProveRequest) (batch.ProofAttempt, error) {
			return kernel.Prove(request, kernel.ProductionProveSeams())
		}
	}
	if owners.laneHeld == nil {
		now := owners.now
		owners.laneHeld = func(installation string) ([]string, error) { return laneHeldGoals(installation, now()) }
	}
	if owners.unset == nil {
		now := owners.now
		owners.unset = func(home, by string, force bool) (lane.UnsetReport, error) {
			steps := batchowner.ProductionUnsetLane(home, by)
			agent := newLandingAgent()
			steps.Now, steps.Agent, steps.StopAgent = now, agent.running, agent.cancel
			return lane.Unset(home, by, now(), force, steps.Seams())
		}
	}
	if owners.validation == nil {
		owners.validation = lane.ValidationDue
	}
	if owners.finalization == nil {
		owners.finalization = cadence.FinalizationPending
	}
	if owners.validateRun == nil {
		owners.validateRun = productionLaneValidate
	}
	if owners.remoteURL == nil {
		owners.remoteURL = func(root string) (string, error) {
			out, err := batchowner.GitOutput(root, "remote", "get-url", "origin")
			return strings.TrimSpace(out), err
		}
	}
	if owners.publish == nil {
		owners.publish = func(home string, tuple lane.Tuple) error {
			return lane.Publish(home, tuple, lane.OpPublish, lane.AuthorityAgent)
		}
	}
	if owners.oldOwnerClaims == nil {
		now := owners.now
		owners.oldOwnerClaims = func(installation string) ([]string, error) { return laneOldOwnerClaims(installation, now()) }
	}
	if owners.agentCaller == nil {
		owners.agentCaller = laneAgentCaller
	}
	if owners.returnMember == nil {
		owners.returnMember = batchowner.ReturnMember
	}
	return owners
}

func landingIntentCommands() []intentCommand {
	byFlag := intentFlag{name: "by", value: "NAME", usage: "who acts (default: this seat's nickname)"}
	return []intentCommand{
		{
			object: "landing", action: "status", audience: "both", summary: "the landing lane on this computer: its checkout, its landing agent, the batch it proves and the next",
			usage: []string{"metasystem landing status [--verbose]"},
			details: []string{"One line: where the lane is, whether its landing agent runs, the batch it works on and the one collecting behind it.",
				"--verbose adds who registered the lane and when, the agent's pid, why the lane can't run when it can't, each batch's members and what the batch waits for.",
				"--json prints the same view the interface's Fleet page reads."},
			flags:    []intentFlag{intentVerboseFlag},
			maxArgs:  0,
			examples: []string{"metasystem landing status", "metasystem landing status --verbose"},
			run:      runIntentLandingStatus,
		},
		{
			object: "landing", action: "set", audience: "both", summary: "register or move this computer's landing lane",
			usage: []string{"metasystem landing set PATH [--by NAME]"},
			details: []string{"A person's act at an enrolled terminal: only landing set registers a lane, never a seat's landing.batch-root.",
				"PATH is a dedicated landing checkout, the top folder of a git clone whose MetaSystem installation is PATH or PATH/metasystem; every seat of this computer then lands through it, and a seat whose landing.batch-root names another is refused.",
				"PATH must have a machine nickname (git -C PATH config metasystem.goal.machine NAME): the lane's claims name it, so a checkout without it is refused.",
				"Each registration takes a new custody epoch. The same PATH again changes nothing. Moving the lane while a batch proves or pushes in it is refused with the way forward: wait, or pause it with landing stop first."},
			flags:    []intentFlag{byFlag},
			maxArgs:  1,
			examples: []string{"metasystem landing set /Users/wido/LocalStorage/GitHub/agentic-tools-landing"},
			run:      runIntentLandingSet,
		},
		{
			object: "landing", action: "start", audience: "both", summary: "resume the landing lane, so its landing agent runs when there is work",
			usage: []string{"metasystem landing start"},
			details: []string{"Ends a pause and forgets the last landing agent's cooldown: the lane checkout's steward then wakes the landing agent as soon as work is queued, a batch is unfinished, or a validation is due.",
				"When the lane can't run (the checkout has no machine nickname, or its supervision is not armed) nothing is changed and the one command that fixes it is named.",
				"A lane already running changes nothing."},
			maxArgs:  0,
			examples: []string{"metasystem landing start"},
			run:      runIntentLandingStart,
		},
		{
			object: "landing", action: "stop", audience: "both", summary: "pause the landing lane for maintenance until landing start",
			usage: []string{"metasystem landing stop [--by NAME]"},
			details: []string{"No lane operation runs and no landing agent starts until metasystem landing start; status shows who stopped it and when.",
				"Seats still join a stopped lane and wait in it; metasystem landing unset is the way back to each seat landing its own work.",
				"A lane already stopped changes nothing. While a batch is pushing to main, stop is refused with the way forward."},
			flags:    []intentFlag{byFlag},
			maxArgs:  0,
			examples: []string{"metasystem landing stop", "metasystem landing stop --by Wido"},
			run:      runIntentLandingStop,
		},
		{
			object: "landing", action: "unset", audience: "both", summary: "take this computer's landing lane away, so each seat lands its own work",
			usage: []string{"metasystem landing unset [--force] [--by NAME]"},
			details: []string{"A person's act at an enrolled terminal, never refused. It fences the lane (no new joins, no new lane work) and pauses it, lets a push under way finish, stops its landing agent and waits for a running proof.",
				"Then it finalizes the members already on main, returns every other member to its seat at the person's word, reads each return back (a goal on the ledger, a change by its recorded disposition) and unregisters the lane only when all are confirmed.",
				"When something still runs or a return is not confirmed, it stops, lists what is left, and the same command continues; --force goes past state that is unknown, never past work that runs."},
			flags:    []intentFlag{{name: "force", usage: "go past custody whose state is unknown"}, byFlag},
			maxArgs:  0,
			examples: []string{"metasystem landing unset", "metasystem landing unset --force"},
			run:      runIntentLandingUnset,
		},
		landingEngineCommand(),
		landingBeginCommand(),
		landingProveCommand(),
		landingPublishCommand(),
		landingReturnCommand(),
		landingValidateCommand(),
	}
}

// laneContext is what every landing verb reads first: the home and the
// registered lane, or the result that ends the verb.
func (inv *intentInvocation) laneContext(needLane bool) (owners laneVerbOwners, home string, record lane.Record, problem *intentResult) {
	owners = inv.landing()
	home, err := owners.home()
	if err != nil {
		return owners, "", lane.Record{}, &intentResult{Outcome: intentFailed, code: 1,
			Summary: "the landing lane can't be read: this shell's home directory isn't known",
			next:    []string{"export", "HOME=PATH"}, nextReason: "then repeat this command",
			Details: []string{"no home for the landing lane: " + err.Error()}}
	}
	record, ok, err := lane.Read(home)
	if err != nil && !needLane {
		// An unreadable record is status's to report and landing set's to
		// replace (Register replaces it).
		return owners, home, lane.Record{}, nil
	}
	if err != nil {
		return owners, home, record, &intentResult{Outcome: intentFailed, code: 1, Summary: "this computer's landing lane record can't be read",
			next: inv.publicArgv("landing", "set", "PATH"), nextReason: "registers the landing checkout again, which replaces the record",
			Details: []string{err.Error()}}
	}
	if needLane && !ok {
		return owners, home, record, &intentResult{Outcome: intentRefused, code: 1,
			Summary: "no landing lane is registered on this computer; nothing was done",
			next:    inv.publicArgv("landing", "set", "PATH"), nextReason: "register the landing checkout every seat lands through"}
	}
	return owners, home, record, nil
}

func (inv *intentInvocation) laneView(owners laneVerbOwners, home string) lane.View {
	sources := lane.ViewSources{Home: home, Now: owners.now(), Owner: owners.probe, Records: owners.records, Ready: owners.ready, Helm: owners.helm,
		Validation: owners.validation, Finalization: func(string) (bool, error) { return owners.finalization(home) }}
	if inv.input.switched("verbose") {
		// The lane's spend is a full read of its proof store: only --verbose
		// pays for it (N-5).
		sources.Spend = batchowner.LaneSpend
	}
	return lane.BuildView(sources)
}

func laneTargets(root string) []intentTarget {
	return []intentTarget{{Kind: "landing-lane", ID: root}}
}

func runIntentLandingStatus(inv *intentInvocation) int {
	owners, home, _, problem := inv.laneContext(false)
	if problem != nil {
		return inv.render(*problem)
	}
	view := inv.laneView(owners, home)
	_, _, unreadable := lane.Read(home)
	result := intentResult{Outcome: intentConfirmed, Summary: view.Summary, Data: view, view: inv.landingStatusView(view, unreadable != nil)}
	if view.Root != nil {
		result.Targets = laneTargets(*view.Root)
	}
	if len(view.Owner.Fix) > 0 {
		result.next, result.nextReason = view.Owner.Fix, laneFixReason(view.Owner.Fix)
	}
	return inv.render(result)
}

// landingStatusView is landing status's page (output-style §6.5): the
// headline says whether the lane runs and what it lands; one row per batch
// says what it does, with its changes under it. --verbose adds the lane's
// checkout, who registered it, its landing agent and its spend.
func (inv *intentInvocation) landingStatusView(view lane.View, unreadable bool) func(*textui.Page) {
	return func(page *textui.Page) {
		env := page.Env()
		if view.Root == nil {
			if unreadable {
				page.Mark(textui.Unknown, "This computer's landing lane record can't be read")
				page.Hint(textui.Hint{Argv: inv.publicArgv("landing", "set", "PATH"), Reason: "registers the landing checkout again, which replaces it"})
				return
			}
			page.Headline("No landing lane is registered on this computer")
			page.Hint(textui.Hint{Argv: inv.publicArgv("landing", "set", "PATH"), Reason: "registers the landing checkout every seat lands through"})
			return
		}
		batchFact, nextFact := "", ""
		if view.Batch != nil {
			batchFact = textui.Count(len(view.Batch.Members), "change", "changes") + " " + view.Batch.State
		}
		if view.Next != nil {
			nextFact = textui.Count(len(view.Next.Members), "change", "changes") + " collecting next"
		}
		owner := view.Owner
		switch {
		case owner.State == lane.OwnerRunning:
			page.Headline("The landing lane's agent is at work", batchFact, nextFact)
		case owner.State == lane.OwnerStopped:
			by := "a person"
			if owner.StoppedBy != nil {
				by = *owner.StoppedBy
			}
			page.Mark(textui.Stopped, "The landing lane is stopped by "+by+"; it lands nothing")
			page.Hint(textui.Hint{Argv: inv.publicArgv("landing", "start"), Reason: "resumes it"})
		case owner.LastExit == nil && owner.RetryHint == nil:
			// An idle lane runs no model: the keeper wakes the agent when
			// there is work (design r10 §3).
			page.Headline("The landing lane is running; its agent starts when there is work", batchFact, nextFact)
		default:
			page.Mark(textui.Alert, "The landing lane can't run its agent")
			section := page.Section("", "")
			if owner.LastExit != nil {
				section.Text("why: " + *owner.LastExit)
			}
			if owner.RetryHint != nil && len(owner.Fix) == 0 {
				section.Text("to fix: " + *owner.RetryHint)
			}
		}
		if view.Batch != nil || view.Next != nil {
			table := page.Section("Batches", "").Table(textui.Column{}, textui.Column{Flex: true, Wrap: true})
			if current := view.Batch; current != nil {
				text := "batch " + current.ID
				if at, err := time.Parse(time.RFC3339, current.Since); err == nil {
					text += " " + env.Since(at)
				}
				if reason := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(current.Reason), ":")); reason != "" && reason != current.State {
					text += " · " + reason
				}
				state := textui.Running
				if current.State == lane.BatchHeld {
					state = textui.Alert
				}
				table.Row(textui.Marked(state, current.State), textui.Plain(text))
				if members := landingMembers(current.Members); members != "" {
					table.Row(textui.Plain(""), textui.Dim(members))
				}
				for _, change := range current.Returned {
					table.Row(textui.Plain(""), textui.Plain(change.Outcome+" "+change.Goal+" from "+change.Seat+": "+change.Reason))
				}
				if page.Verbose() {
					for _, waited := range current.WaitingFor {
						expected := ""
						if waited.Expected != nil {
							if at, err := time.Parse(time.RFC3339, *waited.Expected); err == nil {
								expected = ", expected " + env.Time(at)
							}
						}
						table.Row(textui.Plain(""), textui.Dim("waits for "+waited.Goal+" on "+waited.Seat+expected))
					}
				}
			}
			if next := view.Next; next != nil {
				table.Row(textui.Marked(textui.Running, lane.BatchCollecting), textui.Plain("batch "+next.ID+" · next"))
				if members := landingMembers(next.Members); members != "" {
					table.Row(textui.Plain(""), textui.Dim(members))
				}
			}
		}
		if !page.Verbose() {
			return
		}
		section := page.Section("Lane", env.Path(*view.Root))
		registered := "by " + landingText(view.RegisteredBy)
		if view.RegisteredAt != nil {
			if at, err := time.Parse(time.RFC3339, *view.RegisteredAt); err == nil {
				registered += ", " + env.Time(at)
			}
		}
		section.KV("registered", textui.Plain(registered))
		agentWords := []string{owner.State}
		if owner.Since != nil {
			if at, err := time.Parse(time.RFC3339, *owner.Since); err == nil {
				agentWords[0] += " " + env.Since(at)
			}
		}
		if owner.PID != nil {
			agentWords = append(agentWords, fmt.Sprintf("pid %d", *owner.PID))
		}
		section.KV("agent", textui.Plain(strings.Join(agentWords, " · ")))
		if owner.StoppedBy != nil {
			section.KV("stopped by", textui.Plain(*owner.StoppedBy))
		}
		if owner.LastExit != nil {
			section.KV("why", textui.Plain(*owner.LastExit))
		}
		if owner.RetryHint != nil {
			section.KV("to fix", textui.Plain(*owner.RetryHint))
		}
		if view.Spend != nil {
			section.KV("spend", textui.Plain(fmt.Sprintf("%s · %s · %s, charged to no goal", textui.Count(view.Spend.Attempts, "attempt", "attempts"),
				textui.Count(int(view.Spend.ReservedMinutes), "reserved minute", "reserved minutes"), view.Spend.Account)))
		}
	}
}

// landingMembers are a batch's changes and the seats they came from.
func landingMembers(members []lane.Member) string {
	var named []string
	for _, member := range members {
		named = append(named, member.Goal+" from "+member.Seat)
	}
	return strings.Join(named, ", ")
}

func landingText(text *string) string {
	if text == nil {
		return "no one recorded"
	}
	return *text
}

// landingDone is a landing act that held: its summary as one line, each
// of its paths shortened for the page.
func landingDone(summary string, paths ...string) func(*textui.Page) {
	return func(page *textui.Page) {
		text := summary
		for _, path := range paths {
			if path != "" {
				text = strings.ReplaceAll(text, path, page.Env().Path(path))
			}
		}
		page.Done(text)
	}
}

func (inv *intentInvocation) landingActor(owners laneVerbOwners) string {
	if by := strings.TrimSpace(inv.input.text("by")); by != "" {
		return by
	}
	if inv.resolveLayout() == nil {
		return owners.by(inv.layout.InstallationRoot)
	}
	return "a person at " + inv.cwd
}

// laneBusy names a batch of the lane that proves or pushes, for a move or a stop.
func laneBusy(view lane.View, pushingOnly bool) string {
	if view.Batch == nil || view.Owner.State == lane.OwnerStopped {
		return ""
	}
	if view.Batch.State == lane.BatchPushing || !pushingOnly && view.Batch.State == lane.BatchProving {
		return view.Batch.ID + " " + view.Batch.State
	}
	return ""
}

func runIntentLandingSet(inv *intentInvocation) int {
	if len(inv.input.args) != 1 {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "landing set takes the landing checkout's path, and none was named; nothing was done",
			next: inv.publicArgv("landing", "set", "PATH")})
	}
	owners, home, record, problem := inv.laneContext(false)
	if problem != nil {
		return inv.render(*problem)
	}
	path := inv.callerPath(inv.input.args[0])
	seat := ""
	if inv.resolveLayout() == nil {
		seat = inv.layout.InstallationRoot
	}
	invalid := func(err error) int {
		return inv.render(intentResult{Outcome: intentRefused, code: 1, Targets: laneTargets(path),
			Summary: fmt.Sprintf("%s can't be the landing lane (%s); nothing was registered", path, oneLine(err.Error())),
			next:    inv.publicArgv("landing", "set", "PATH"), nextReason: "with a clone of this repository that no seat works in",
			Details: []string{"refused because: " + lane.CodeRegisterInvalid + ": " + err.Error()}})
	}
	root, err := owners.validate(path, seat, owners.now())
	if err != nil {
		return invalid(err)
	}
	// The lane's one layout is resolved here, once; every reader takes it
	// from the record.
	layout, err := lane.NewLayout(root)
	var refusal *lane.Refusal
	if errors.As(err, &refusal) {
		return invalid(errors.New(refusal.Message))
	}
	if err != nil {
		return invalid(err)
	}
	root = string(layout.Checkout)
	// A lane cannot run in a checkout without a machine nickname: its
	// claims name it, so the lane is never registered there.
	if _, err := owners.machine(root); err != nil {
		refusal := lane.NoMachineRefusal(root)
		return inv.render(intentResult{Outcome: intentRefused, code: 1, Targets: laneTargets(root),
			Summary: "the landing checkout has no machine nickname yet, so nothing was registered",
			next:    refusal.Argv, nextReason: "any one word, landing reads well; then run metasystem landing set " + root + " again",
			Details: []string{"refused because: " + refusal.Code + ": " + refusal.Message}})
	}
	if record.Root == root && record.Install == string(layout.Install) {
		view := inv.laneView(owners, home)
		return inv.render(intentResult{Outcome: intentUnchanged, Targets: laneTargets(root), Data: view, Summary: "this computer's landing lane is already " + root,
			view: landingDone("this computer's landing lane is already "+root, root)})
	}
	if record.Root != "" && record.Root != root {
		if busy := laneBusy(inv.laneView(owners, home), false); busy != "" {
			return inv.render(intentResult{Outcome: intentRefused, code: 1, Targets: laneTargets(record.Root),
				Summary: "the current landing lane is busy (batch " + busy + "), so it wasn't moved",
				next:    inv.publicArgv("landing", "stop"), nextReason: "pauses it; then run metasystem landing set " + root + " again",
				Details: []string{fmt.Sprintf("refused because: %s: batch %s in the current lane %s; moving the lane now would leave that batch behind", codeLandingLaneBusy, busy, record.Root),
					"metasystem landing stop pauses the lane; or wait until the batch lands (metasystem landing status shows it)"}})
		}
	}
	// The lane's claim identity activates with this registration, and only
	// once the old owner holds nothing: its claims are landed or returned
	// through its own authority first (design r10 §5, Astra R9-01).
	if refused := inv.oldOwnerHolds(owners, layout); refused != nil {
		return inv.render(*refused)
	}
	// Registering the host's lane decides where every seat lands: a
	// person's act at an enrolled terminal (design r10 §1). No seat and no
	// agent registers one.
	proveAt := seat
	if proveAt == "" {
		proveAt = inv.cwd
	}
	actor, err := owners.person(proveAt)
	if err != nil {
		refused := inv.personRefusal("", err, inv.input.text("by"))
		refused.Targets = laneTargets(root)
		refused.code = 3
		refused.Summary = "only a person may register the landing lane, and " + strings.TrimSuffix(refused.Summary, ", so nothing was done") + "; nothing was changed"
		refused.Details = append(refused.Details, "registering "+root+" as this computer's landing lane; the retry is metasystem landing set "+root)
		return inv.render(*refused)
	}
	if by := strings.TrimSpace(inv.input.text("by")); by != "" {
		actor = by
	}
	previous, changed, err := lane.Register(home, layout, actor, owners.now())
	if errors.As(err, &refusal) {
		return inv.render(intentResult{Outcome: intentRefused, code: 1, Targets: laneTargets(root), Summary: refusal.Message,
			next: refusal.Argv, nextReason: refusal.Fix, Details: []string{"refused because: " + refusal.Code}})
	}
	if err != nil {
		return inv.render(intentResult{Outcome: intentFailed, code: 1, Targets: laneTargets(root), Summary: "the landing lane couldn't be saved, so nothing was registered",
			next: inv.sameCommand(), nextReason: "tries again", Details: []string{"the landing lane could not be registered: " + err.Error()}})
	}
	view := inv.laneView(owners, home)
	if !changed {
		return inv.render(intentResult{Outcome: intentUnchanged, Targets: laneTargets(root), Data: view, Summary: "this computer's landing lane is already " + root,
			view: landingDone("this computer's landing lane is already "+root, root)})
	}
	summary := "this computer's landing lane is now " + root
	if previous.Root != "" {
		summary += " (it was " + previous.Root + ")"
	}
	registered, _, _ := lane.Read(home)
	result := intentResult{Outcome: intentConfirmed, Targets: laneTargets(root), Data: view, Summary: summary, view: landingDone(summary, root, previous.Root),
		Details: []string{fmt.Sprintf("installation %s, custody epoch %d", layout.Install, registered.CustodyEpoch)},
		next:    inv.publicArgv("landing", "start"), nextReason: "starts the lane; its steward then wakes the landing agent when there is work"}
	if len(view.Owner.Fix) > 0 {
		result.next, result.nextReason = view.Owner.Fix, laneFixReason(view.Owner.Fix)
	}
	return inv.render(result)
}

// laneFixReason is why a person runs a lane's fix, by the command it is.
func laneFixReason(argv []string) string {
	if len(argv) > 0 && argv[0] == "git" {
		return "name the landing checkout's machine once (any one word), then run metasystem landing start"
	}
	return "a person runs this at a terminal no agent started: it arms the landing checkout's supervision, whose steward wakes the landing agent"
}

// laneNotReady is the result that ends a start when the lane can't run:
// what is missing and the one command that fixes it; nil when it can.
func (inv *intentInvocation) laneNotReady(owners laneVerbOwners, root string) *intentResult {
	err := owners.ready(root)
	if err == nil {
		return nil
	}
	var refusal *lane.Refusal
	if !errors.As(err, &refusal) {
		return &intentResult{Outcome: intentFailed, code: 1, Targets: laneTargets(root),
			Summary: "whether the landing lane can run is unknown, so nothing was started",
			next:    inv.publicArgv("landing", "status", "--verbose"), nextReason: "shows the lane's state",
			Details: []string{"the lane at " + root + " could not be checked: " + err.Error()}}
	}
	code := 1
	if refusal.Code == lane.CodeUnarmed {
		// Arming a checkout is a person's act.
		code = 3
	}
	return &intentResult{Outcome: intentRefused, code: code, Targets: laneTargets(root),
		Summary: refusal.Message + "; nothing was started",
		next:    refusal.Argv, nextReason: laneFixReason(refusal.Argv),
		Details: []string{"refused because: " + refusal.Code + ": " + refusal.Message}}
}

func runIntentLandingStart(inv *intentInvocation) int {
	owners, home, record, problem := inv.laneContext(true)
	if problem != nil {
		return inv.render(*problem)
	}
	if refused := inv.laneResumable(owners, home, record); refused != nil {
		return inv.render(*refused)
	}
	return inv.render(inv.startLane(owners, home, record))
}

// laneResumable ends a start that must not run: while a person unsets the
// lane nothing starts it again, and only a person clears a pause (design
// r10 K2). nil when the verb may go on.
func (inv *intentInvocation) laneResumable(owners laneVerbOwners, home string, record lane.Record) *intentResult {
	targets := laneTargets(record.Root)
	if journal, fenced, _ := lane.ReadUnset(home); fenced {
		by := journal.By
		if by == "" {
			by = "a person"
		}
		return &intentResult{Outcome: intentRefused, code: 1, Targets: targets,
			Summary: "this computer's landing lane is being unset by " + by + ", so it wasn't started",
			next:    inv.publicArgv("landing", "unset"), nextReason: "finishes the unset; after it each seat lands its own work",
			Details: []string{"refused because: " + lane.CodeUnsetting}}
	}
	pause, paused := lane.ReadPause(home)
	if !paused {
		return nil
	}
	proveAt := inv.cwd
	if inv.resolveLayout() == nil {
		proveAt = inv.layout.InstallationRoot
	}
	if _, err := owners.person(proveAt); err != nil {
		refused := inv.personRefusal("", err, inv.input.text("by"))
		refused.Targets, refused.code = targets, 3
		refused.Summary = "only a person may resume the landing lane " + pause.By + " stopped, and " + strings.TrimSuffix(refused.Summary, ", so nothing was done") + "; it stays stopped"
		return refused
	}
	return nil
}

// startLane ends a person's pause and forgets the last landing agent's
// cooldown, so the keeper wakes the agent at once when there is work (design
// r10 §3); unchanged when the lane was not paused. A lane that can't run is
// refused with its fix before anything is written.
func (inv *intentInvocation) startLane(owners laneVerbOwners, home string, record lane.Record) intentResult {
	targets := laneTargets(record.Root)
	if refused := inv.laneNotReady(owners, record.Root); refused != nil {
		return *refused
	}
	resumed, err := lane.ClearPause(home)
	if err == nil {
		err = lane.ClearAgentCooldown(home)
	}
	if err != nil {
		return intentResult{Outcome: intentFailed, code: 1, Targets: targets, Summary: "the landing lane's state couldn't be saved, so nothing was started",
			next: inv.sameCommand(), nextReason: "tries again", Details: []string{"the landing lane's state could not be written: " + err.Error()}}
	}
	view := inv.laneView(owners, home)
	if resumed {
		summary := "resumed the landing lane at " + record.Root + "; its landing agent starts when there is work"
		return intentResult{Outcome: intentConfirmed, Targets: targets, Data: view, Summary: summary, view: landingDone(summary, record.Root)}
	}
	summary := "the landing lane at " + record.Root + " is already running; its landing agent starts when there is work"
	return intentResult{Outcome: intentUnchanged, Targets: targets, Data: view, Summary: summary, view: landingDone(summary, record.Root)}
}

func runIntentLandingStop(inv *intentInvocation) int {
	owners, home, record, problem := inv.laneContext(true)
	if problem != nil {
		return inv.render(*problem)
	}
	result, _ := inv.stopLane(owners, home, record)
	return inv.render(result)
}

// stopLane pauses the lane at a person's word; a batch pushing to main is
// never paused mid-push.
func (inv *intentInvocation) stopLane(owners laneVerbOwners, home string, record lane.Record) (intentResult, bool) {
	targets := laneTargets(record.Root)
	if pause, paused := lane.ReadPause(home); paused {
		result := intentResult{Outcome: intentUnchanged, Targets: targets, Summary: "the landing lane is already stopped by " + pause.By + " at " + lane.LocalText(pause.At) + "; metasystem landing start resumes it"}
		result.view = func(page *textui.Page) {
			done := "the landing lane is already stopped by " + pause.By
			if at, err := time.Parse(time.RFC3339, pause.At); err == nil {
				done += " " + page.Env().Since(at)
			}
			page.Done(done)
			page.Hint(textui.Hint{Argv: inv.publicArgv("landing", "start"), Reason: "resumes it"})
		}
		return result, true
	}
	if busy := laneBusy(inv.laneView(owners, home), true); busy != "" {
		return intentResult{Outcome: intentRefused, code: 1, Targets: targets,
			Summary: "the landing lane is busy (batch " + busy + " to main), so it wasn't stopped",
			next:    inv.publicArgv("landing", "status"), nextReason: "shows when it has landed; then run metasystem landing stop again",
			Details: []string{fmt.Sprintf("refused because: %s: batch %s to main now; stopping mid-push would leave main and the batch's record out of step", codeLandingLanePushing, busy)}}, false
	}
	by := inv.landingActor(owners)
	inv.sayWhenTheLaneLockIsHeld(home)
	if _, err := lane.SetPause(home, by, owners.now()); err != nil {
		return intentResult{Outcome: intentFailed, code: 1, Targets: targets, Summary: "the landing lane couldn't be stopped",
			next: inv.sameCommand(), nextReason: "tries again", Details: []string{"the landing lane could not be stopped: " + err.Error()}}, false
	}
	// Stop holds seats (design r10 §1): their work joins and waits, and the
	// way to seats landing their own work is unset, which line 2 names.
	return intentResult{Outcome: intentConfirmed, Targets: targets, Data: inv.laneView(owners, home),
		Summary: "stopped the landing lane for " + by + "; seats' work waits in it until metasystem landing start",
		Details: []string{"the lane at " + record.Root + " runs no lane operation and starts no landing agent until metasystem landing start"},
		next:    inv.publicArgv("landing", "unset"), nextReason: "lets each seat land its own work instead",
		view: func(page *textui.Page) {
			page.Done("stopped the landing lane for " + by + "; seats' work waits in it until it starts again")
		}}, true
}

// runIntentLandingUnset takes the lane away at a person's word (design r10
// §1): fence, settle, reconcile, return and confirm, then unregister. It is
// never refused to a person and it resumes: when work still runs or a
// return is not confirmed, it lists what is left and the same command
// continues.
func runIntentLandingUnset(inv *intentInvocation) int {
	owners, home, _, problem := inv.laneContext(false)
	if problem != nil {
		return inv.render(*problem)
	}
	proveAt := inv.cwd
	if inv.resolveLayout() == nil {
		proveAt = inv.layout.InstallationRoot
	}
	by, err := owners.person(proveAt)
	if err != nil {
		refused := inv.personRefusal("", err, inv.input.text("by"))
		refused.code = 3
		refused.Summary = "only a person may unset the landing lane, and " + strings.TrimSuffix(refused.Summary, ", so nothing was done") + "; nothing was changed"
		return inv.render(*refused)
	}
	if named := strings.TrimSpace(inv.input.text("by")); named != "" {
		by = named
	}
	report, err := owners.unset(home, by, inv.input.switched("force"))
	var refusal *lane.Refusal
	if errors.As(err, &refusal) && len(refusal.Argv) > 0 {
		return inv.render(intentResult{Outcome: intentRefused, code: 1, Data: report, Summary: refusal.Message,
			next: refusal.Argv, nextReason: refusal.Fix, Details: []string{"refused because: " + refusal.Code}})
	}
	if err != nil {
		summary := "the landing lane's unset could not start (" + oneLine(err.Error()) + "); nothing was changed"
		if _, fenced, _ := lane.ReadUnset(home); fenced {
			summary = "the landing lane's unset could not go on (" + oneLine(err.Error()) + "); the lane stays fenced"
		}
		return inv.render(intentResult{Outcome: intentFailed, code: 1, Data: report, Summary: summary,
			next: inv.sameCommand(), nextReason: "tries again once the cause is fixed",
			Details: []string{"landing unset: " + err.Error()}})
	}
	targets := []intentTarget{}
	if report.Record.Root != "" {
		targets = laneTargets(report.Record.Root)
	}
	var details []string
	for _, entry := range report.Unresolved {
		member := entry.Member
		if member == "" {
			member = "its members"
		}
		details = append(details, "batch "+entry.Batch+", "+member+": "+entry.Reason)
	}
	switch {
	case report.NoLane:
		return inv.render(intentResult{Outcome: intentUnchanged, Targets: targets, Data: report,
			Summary: "no landing lane is registered on this computer; each seat lands its own work"})
	case report.Unregistered && report.CheckoutGone:
		return inv.render(inv.goneLaneUnset(owners, report, targets, proveAt))
	case report.Unregistered:
		summary := "unset this computer's landing lane " + report.Record.Root + "; each seat lands its own work now"
		return inv.render(intentResult{Outcome: intentConfirmed, Targets: targets, Data: report, Summary: summary,
			view: landingDone(summary, report.Record.Root)})
	case report.Stopped == lane.StepSettled:
		waiting := append(append([]string{}, report.Settlement.Live...), report.Settlement.Unknown...)
		result := intentResult{Outcome: intentInProgress, Targets: targets, Data: report,
			Summary: "the landing lane is fenced and waits before returning its members: " + strings.Join(waiting, "; "),
			next:    inv.publicArgv("landing", "unset"), nextReason: "continues once that work has ended",
			Details: append(details, "nothing new starts in the lane; its record stays until every member is returned")}
		if len(report.Settlement.Live) == 0 {
			result.next, result.nextReason = inv.publicArgv("landing", "unset", "--force"), "goes past what cannot be known, once you have checked it"
		}
		return inv.render(result)
	}
	summary := fmt.Sprintf("the landing lane is fenced; %s not confirmed returned yet", textui.Count(len(report.Unresolved), "member is", "members are"))
	if len(report.Unresolved) != 0 {
		summary += " (" + report.Unresolved[0].Member + ": " + report.Unresolved[0].Reason + ")"
	}
	return inv.render(intentResult{Outcome: intentInProgress, Targets: targets, Data: report, Summary: summary,
		next: inv.publicArgv("landing", "unset"), nextReason: "returns them again and unregisters the lane once all are confirmed",
		Details: details})
}

// goneLaneUnset is the unset of a lane whose checkout was gone: no batch
// could be read, so the goals the ledger still shows the lane holding are
// named on the page itself, with the command that releases each.
func (inv *intentInvocation) goneLaneUnset(owners laneVerbOwners, report lane.UnsetReport, targets []intentTarget, installation string) intentResult {
	summary := "unset the landing lane " + report.Record.Root + "; its checkout was gone. Each seat lands its own work now"
	held, err := owners.laneHeld(installation)
	result := intentResult{Outcome: intentConfirmed, Targets: targets, Data: report, Summary: summary}
	switch {
	case err != nil:
		result.Summary += "; which goals it still held can't be read (" + oneLine(err.Error()) + ")"
		result.next, result.nextReason = inv.publicArgv("goal", "list", "--fetch"), "shows the claims; release each the lane still holds with metasystem goal release G"
	case len(held) != 0:
		result.Summary += fmt.Sprintf("; the ledger still shows it holding %s: %s", textui.Count(len(held), "goal", "goals"), strings.Join(held, ", "))
		result.next = inv.publicArgv("goal", "release", held[0], "--reason", "the landing lane was unset")
		result.nextReason = "gives the goal back; the same for each other goal named"
	}
	return result
}

// laneHeldGoals are the goals the ledger, fetched at installation, shows
// handed to a landing batch or claimed by a landing lineage: the lane's
// claim identity or the old owner's.
func laneHeldGoals(installation string, now time.Time) ([]string, error) {
	return ledgerClaims(installation, now, func(claim *goal.ClaimRecord) bool {
		return claim.HandedOver.Batch != "" || claim.Lineage == lane.ClaimLineage || claim.Lineage == lane.OldOwnerLineage
	})
}

// laneOldOwnerClaims are the goals the ledger, fetched at installation,
// shows claimed by the deleted batch owner's lineage (lane.OldOwnerLineage).
func laneOldOwnerClaims(installation string, now time.Time) ([]string, error) {
	return ledgerClaims(installation, now, func(claim *goal.ClaimRecord) bool { return claim.Lineage == lane.OldOwnerLineage })
}

func ledgerClaims(installation string, now time.Time, held func(*goal.ClaimRecord) bool) ([]string, error) {
	endpoint, err := goal.ResolveEndpoint(installation)
	if err != nil {
		return nil, err
	}
	projection, err := goal.Project(endpoint, true, now)
	if err != nil {
		return nil, err
	}
	var ids []string
	for id, file := range projection.Tree.Live {
		if file.Claimed != nil && held(file.Claimed) {
			ids = append(ids, id)
		}
	}
	slices.Sort(ids)
	return ids, nil
}

// oldOwnerHolds refuses a registration while the ledger shows the deleted
// batch owner's lineage holding a goal (design r10 §5, Astra R9-01): the
// lane's claim identity is activated only once no such claim remains. It
// names each goal and a person's release, the one way left to settle it
// (the old owner and its authority are gone); an unreadable ledger
// registers nothing either.
func (inv *intentInvocation) oldOwnerHolds(owners laneVerbOwners, layout lane.Layout) *intentResult {
	root := string(layout.Checkout)
	held, err := owners.oldOwnerClaims(string(layout.Install))
	if err != nil {
		return &intentResult{Outcome: intentFailed, code: 1, Targets: laneTargets(root),
			Summary: "which goals the old landing owner still holds can't be read, so nothing was registered",
			retry:   "tries again", Details: []string{"reading the ledger in " + string(layout.Install) + ": " + err.Error()}}
	}
	if len(held) == 0 {
		return nil
	}
	return &intentResult{Outcome: intentRefused, code: 1, Targets: laneTargets(root),
		Summary:    fmt.Sprintf("the old landing owner still holds %s (%s), so nothing was registered", textui.Count(len(held), "goal", "goals"), strings.Join(held, ", ")),
		next:       inv.publicArgv("goal", "release", held[0], "--reason", "the landing lane changes owner"),
		nextReason: "frees it for its seat to claim again; the same for each goal named, then run landing set again",
		Details: []string{"refused because: " + codeLandingOldOwnerHolds + ": the new lane's claim identity activates only once no goal is claimed under lineage " + lane.OldOwnerLineage,
			"a person gives each back: metasystem goal release G --reason TEXT",
			"a release leaves the goal unclaimed: its seat claims it again to land it"}}
}

// statusLaneLine is the lane's line in status's board block: landing
// status's headline, when this computer has a lane.
func (inv *intentInvocation) statusLaneLine() string {
	if view := inv.statusLane(); view != nil {
		return view.Summary
	}
	return ""
}

// statusLane is the lane's view for status, when this computer has a lane.
func (inv *intentInvocation) statusLane() *lane.View {
	owners := inv.landing()
	home, err := owners.home()
	if err != nil {
		return nil
	}
	if view := inv.laneView(owners, home); view.Root != nil {
		return &view
	}
	return nil
}

// sayWhenTheLaneLockIsHeld prints one plain line before a stop waits for the
// host flock: a landing engine advance holds it while it re-arms, and the
// stop takes effect only when that finishes.
func (inv *intentInvocation) sayWhenTheLaneLockIsHeld(home string) {
	held, err := lock.File(lane.LockPath(home), 0o600, lock.TryExclusive)
	if err == nil {
		_ = held.Release()
		return
	}
	if lock.Busy(err) && !inv.input.switched("json") {
		page := textui.New(inv.textEnv(inv.stderr))
		page.Mark(textui.Running, "the lane's engine is being changed; the stop takes effect when that finishes")
		_, _ = io.WriteString(inv.stderr, page.String())
	}
}
