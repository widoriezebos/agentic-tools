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
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/helm"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batchowner"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lock"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/realpath"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/textui"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/verbresult"
)

// laneVerbOwners are the landing verbs' seams; the zero value is production.
type laneVerbOwners struct {
	home func() (string, error)
	// probe reads whether the lane's landing agent runs.
	probe    func(root string) (lane.OwnerProbe, error)
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
	// helm reads whether a checkout is at the helm.
	helm func(root string) helm.State
	// installation is the metasystem installation of a lane checkout: the
	// module root.
	installation func(root string) (string, error)
	// unset runs landing unset's journaled steps for the person by.
	unset func(home, by string, force bool) (lane.UnsetReport, error)
	// keeper is the landing agent's keeper of the lane checkout root, the
	// one the lane checkout's steward runs each tick (landing run).
	keeper func(home, root string) lane.AgentKeeper
	// plainProve are landing prove's effects; the zero value starts the
	// engine detached through gaterun.LaunchDetached.
	plainProve       plain.ProveSeams
	plainResolve     plain.ResolveSeams
	stopRegeneration func(string) error
	// push is landing push's push of the lane checkout's HEAD to main.
	push func(install, checkout string, now time.Time) (plain.PushOutcome, error)
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
	if owners.installation == nil {
		owners.installation = func(root string) (string, error) {
			layout, err := inv.owners.resolver.ResolveLayout(root)
			return layout.InstallationRoot, err
		}
	}
	if owners.unset == nil {
		now := owners.now
		owners.unset = func(home, by string, force bool) (lane.UnsetReport, error) {
			agent := newLandingAgent()
			return lane.Unset(home, by, now(), force, lane.UnsetSeams{Settle: settleLandingAgent(agent.running, agent.cancel)})
		}
	}
	if owners.keeper == nil {
		owners.keeper = func(home, root string) lane.AgentKeeper { return newLandingAgentKeeper(root, home, newLandingAgent()) }
	}
	if owners.stopRegeneration == nil {
		owners.stopRegeneration = plain.StopRegeneration
	}
	if owners.push == nil {
		owners.push = plain.Push
	}
	return owners
}

func landingIntentCommands() []intentCommand {
	byFlag := intentFlag{name: "by", value: "NAME", usage: "who acts (default: this seat's nickname)"}
	return []intentCommand{
		{
			object: "landing", action: "status", audience: "both", summary: "the landing lane on this computer: its checkout and its landing agent",
			usage: []string{"metasystem landing status [--verbose]"},
			details: []string{"One line: where the lane is and whether its landing agent runs.",
				"--verbose adds who registered the lane and when, the agent's pid and why the lane can't run when it can't.",
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
				"PATH is a dedicated landing checkout, the top folder of a git clone whose MetaSystem installation is PATH or PATH/metasystem; while it is registered, work land G on every seat of this computer hands the goal's branch to it (landing unset lets each seat land its own work).",
				"PATH must have a machine nickname (git -C PATH config metasystem.goal.machine NAME): the lane's claims name it, so a checkout without it is refused.",
				"Each registration takes a new custody epoch. The same PATH again changes nothing."},
			flags:    []intentFlag{byFlag},
			maxArgs:  1,
			examples: []string{"metasystem landing set /Users/wido/LocalStorage/GitHub/agentic-tools-landing"},
			run:      runIntentLandingSet,
		},
		{
			object: "landing", action: "start", audience: "both", summary: "resume the landing lane, so its landing agent runs when there is work",
			usage: []string{"metasystem landing start"},
			details: []string{"Ends a pause: the lane checkout's steward then wakes the landing agent at its next tick when there is work; metasystem landing run wakes it now.",
				"When the lane can't run (the checkout has no machine nickname, or its supervision is not armed) nothing is changed and the one command that fixes it is named.",
				"A lane already running changes nothing."},
			maxArgs:  0,
			examples: []string{"metasystem landing start"},
			run:      runIntentLandingStart,
		},
		{
			object: "landing", action: "run", audience: "both", summary: "start the landing agent now when the lane has queued work, instead of at the steward's next tick",
			usage: []string{"metasystem landing run [--json]"},
			details: []string{"Takes the same decision the lane checkout's steward takes each tick, under the lane's lock: it starts the landing agent when work is queued, the lane is not stopped and no landing agent runs.",
				"A landing agent already running, or a lane with nothing queued, changes nothing. A stopped lane, a lane checkout at the helm, or a lane that can't run is refused with the one command that resumes it.",
				"--json prints the outcome (started, running, idle, paused, held, failed) and the landing agent's session."},
			maxArgs:  0,
			examples: []string{"metasystem landing run"},
			run:      runIntentLandingRun,
		},
		{
			object: "landing", action: "stop", audience: "both", summary: "pause the landing lane for maintenance until landing start",
			usage: []string{"metasystem landing stop [--reason TEXT] [--by NAME]"},
			details: []string{"No landing agent starts until metasystem landing start; status shows who stopped it, when, and the reason it was given.",
				"metasystem landing unset is the way back to each seat landing its own work.",
				"Also ends a running regeneration, even when the lane is already stopped."},
			flags:    []intentFlag{{name: "reason", value: "TEXT", usage: "why it is stopped, kept with the stop and shown by status"}, byFlag},
			maxArgs:  0,
			examples: []string{"metasystem landing stop", "metasystem landing stop --reason 'goal-a is red twice on its own tests'"},
			run:      runIntentLandingStop,
		},
		{
			object: "landing", action: "unset", audience: "both", summary: "take this computer's landing lane away, so each seat lands its own work",
			usage: []string{"metasystem landing unset [--force] [--by NAME]"},
			details: []string{"A person's act at an enrolled terminal, never refused. It fences the lane and pauses it, stops its landing agent and then unregisters the lane.",
				"When the landing agent still runs, it stops and the same command continues; --force goes past state that is unknown, never past an agent that runs."},
			flags:    []intentFlag{{name: "force", usage: "go past state that is unknown"}, byFlag},
			maxArgs:  0,
			examples: []string{"metasystem landing unset", "metasystem landing unset --force"},
			run:      runIntentLandingUnset,
		},
		landingProveCommand(),
		landingPushCommand(),
		landingReturnCommand(),
		landingResolveCommand(),
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
	return lane.BuildView(lane.ViewSources{Home: home, Now: owners.now(), Owner: owners.probe, Ready: owners.ready, Wake: plain.KeeperWake(home)})
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
	record, _, unreadable := lane.Read(home)
	data := landingStatus(owners, home, record, view)
	summary := view.Summary
	if view.Root != nil {
		summary += "; " + landingQueueWords(data.Queue)
	}
	result := intentResult{Outcome: intentConfirmed, Summary: summary, Data: data,
		view: withPlainLane(withRunningProof(inv.landingStatusView(view, unreadable != nil, data.RunningProof), data.RunningProof), data)}
	if view.Root != nil {
		result.Targets = laneTargets(*view.Root)
	}
	if len(view.Owner.Fix) > 0 {
		result.next, result.nextReason = view.Owner.Fix, laneFixReason(view.Owner.Fix)
	}
	return inv.render(result)
}

// landingStatusData is landing status --json: the plain lane's status,
// the one reader /api/board's lane reads too (plain.ReadStatus).
type landingStatusData = plain.Status

func landingStatus(owners laneVerbOwners, home string, record lane.Record, view lane.View) landingStatusData {
	return plain.ReadStatus(home, record, view, owners.plainProve)
}

// landingQueueWords counts the queue's lines that need the lane, for the
// one line.
func landingQueueWords(queue []plain.Entry) string {
	waiting, returned := 0, 0
	for _, entry := range queue {
		switch entry.State {
		case plain.StateWaiting:
			waiting++
		case plain.StateReturned:
			returned++
		}
	}
	return fmt.Sprintf("%d waiting, %d returned in the queue", waiting, returned)
}

// withPlainLane adds the plain lane's queue, last proof and last push to
// landing status's page: the lines that wait or were returned, and with
// --verbose the landed ones too.
func withPlainLane(view func(*textui.Page), data landingStatusData) func(*textui.Page) {
	return func(page *textui.Page) {
		view(page)
		if data.Root == nil {
			return
		}
		if run := data.RunningRegeneration; run != nil {
			page.Section("Regenerating", "").Text(fmt.Sprintf("%s: %s (%s); log %s, %d bytes", run.Goal, strings.Join(run.Command, " "), run.State, run.Log, run.LogBytes))
		}
		rows := [][2]string{}
		for _, entry := range data.Queue {
			if (entry.State == plain.StateLanded || entry.State == plain.StateSuperseded) && !page.Verbose() {
				continue
			}
			state := entry.State
			if entry.State == plain.StateReturned {
				state += ": " + entry.Reason
			}
			rows = append(rows, [2]string{entry.Goal, entry.Branch + " at " + shortLandingID(entry.SHA) + " from " + entry.Seat + " · " + state})
		}
		if len(rows) > 0 {
			table := page.Section("Queue", "").Table(textui.Column{}, textui.Column{Flex: true, Wrap: true})
			for _, row := range rows {
				table.Row(textui.Plain(row[0]), textui.Plain(row[1]))
			}
		}
		if data.LastProof == nil && data.LastPush == nil {
			return
		}
		section := page.Section("Last", "")
		if proof := data.LastProof; proof != nil {
			section.KV("proven", textui.Plain(proof.Result+landingRedReason(proof.Reason)+" for "+provedWords(proof.Commit, proof.Tree)+", "+lane.LocalText(proof.At)))
		}
		if push := data.LastPush; push != nil {
			section.KV("push", textui.Plain(shortLandingID(push.Commit)+" (from "+shortLandingID(push.Old)+"), "+lane.LocalText(push.At)))
		}
	}
}

// landingStatusView is landing status's page (output-style §6.5): the
// headline says whether the lane runs, and what it proves while its agent
// is idle. --verbose adds the lane's checkout, who registered it and its
// landing agent.
func (inv *intentInvocation) landingStatusView(view lane.View, unreadable bool, running *plain.RunningProof) func(*textui.Page) {
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
		owner := view.Owner
		switch {
		case owner.State == lane.OwnerRunning:
			page.Headline("The landing lane's agent is at work")
		case owner.State == lane.OwnerStopped:
			by := "a person"
			if owner.StoppedBy != nil {
				by = *owner.StoppedBy
			}
			if owner.StoppedBecause != nil {
				by += " (" + *owner.StoppedBecause + ")"
			}
			page.Mark(textui.Stopped, "The landing lane is stopped by "+by+"; it lands nothing")
			page.Hint(textui.Hint{Argv: inv.publicArgv("landing", "start"), Reason: "resumes it"})
		case owner.State == lane.OwnerIdle && running != nil && running.State == "running":
			// The proof the agent started runs on after the agent's turn.
			page.Headline("The landing lane is proving tree " + shortLandingID(running.Tree) + "; its agent is not running")
		case owner.State == lane.OwnerIdle:
			// An idle lane runs no model: the keeper wakes the agent when
			// there is work (design r10 §3).
			page.Headline("The landing lane is idle; its agent starts when there is work")
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
		if owner.StoppedBecause != nil {
			section.KV("because", textui.Plain(*owner.StoppedBecause))
		}
		if owner.LastExit != nil {
			section.KV("why", textui.Plain(*owner.LastExit))
		}
		if owner.RetryHint != nil {
			section.KV("to fix", textui.Plain(*owner.RetryHint))
		}
	}
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

// startLane ends a person's pause and repairs an unreadable keeper record,
// so the keeper wakes the agent at once when there is work; unchanged when
// the lane was not paused. A lane that can't run is
// refused with its fix before anything is written.
func (inv *intentInvocation) startLane(owners laneVerbOwners, home string, record lane.Record) intentResult {
	targets := laneTargets(record.Root)
	if refused := inv.laneNotReady(owners, record.Root); refused != nil {
		return *refused
	}
	resumed, err := lane.ClearPause(home)
	if err == nil {
		err = lane.RepairAgentRecord(home)
	}
	if err != nil {
		return intentResult{Outcome: intentFailed, code: 1, Targets: targets, Summary: "the landing lane's state couldn't be saved, so nothing was started",
			next: inv.sameCommand(), nextReason: "tries again", Details: []string{"the landing lane's state could not be written: " + err.Error()}}
	}
	view := inv.laneView(owners, home)
	agent := []string{"its landing agent starts when there is work"}
	if resumed {
		summary := "resumed the landing lane at " + record.Root
		return intentResult{Outcome: intentConfirmed, Targets: targets, Data: view, Summary: summary, Details: agent, view: landingDone(summary, record.Root)}
	}
	summary := "the landing lane at " + record.Root + " is already running"
	return intentResult{Outcome: intentUnchanged, Targets: targets, Data: view, Summary: summary, Details: agent, view: landingDone(summary, record.Root)}
}

// landing run starts the landing agent now instead of at the lane
// checkout's steward's next tick. It runs the keeper's own step (the
// steward's KeepLandingLane) with the lane checkout as its steward, so the
// decision, the start claim under the lane flock and the launch are the
// keeper's; landing run only renders the step's outcome.

// landingRunData is landing run --json's data: the keeper step's outcome,
// the landing agent's session it started or found running, and the wake it
// started for.
type landingRunData struct {
	Outcome lane.AgentOutcome `json:"outcome"`
	Launch  string            `json:"launch,omitempty"`
	Root    string            `json:"root"`
	Reasons []string          `json:"reasons,omitempty"`
}

// wakeWords says why the landing agent was started as a person reads it;
// --json keeps the lane's own reasons.
func wakeWords(reasons []string) string {
	words := make([]string, 0, len(reasons))
	for _, reason := range reasons {
		switch reason {
		case plain.WakeQueued:
			reason = "queued work"
		case plain.WakeProofFinished:
			reason = "a finished test run"
		}
		words = append(words, reason)
	}
	return strings.Join(words, " and ")
}

func runIntentLandingRun(inv *intentInvocation) int {
	owners, home, record, problem := inv.laneContext(true)
	if problem != nil {
		return inv.render(*problem)
	}
	root := record.Root
	targets := laneTargets(root)
	// Outside the lane checkout the lane's own engine runs the step, so the
	// landing agent is always supervised by the lane's engine, as on a
	// steward tick, and no seat's restart takes its supervisor down.
	if !inv.insideLaneCheckout(root) {
		return inv.handOffLandingRun(record)
	}
	// The steward skips the keeper while the lane checkout is at the helm;
	// landing run does the same.
	if owners.helm(root).Active {
		return inv.render(intentResult{Outcome: intentRefused, code: 1, Targets: targets,
			Summary: "the landing checkout " + root + " is at the helm, so no landing agent was started",
			next:    []string{"metasystem", "helm", "return", "--repo", root}, nextReason: "gives it back to the machinery; then run metasystem landing run again"})
	}
	if refused := inv.laneNotReady(owners, root); refused != nil {
		return inv.render(*refused)
	}
	// A start asked for by name is not held by the agent's barren runs.
	keeper := owners.keeper(home, root)
	keeper.Explicit = true
	run := keeper.Run()
	data := landingRunData{Outcome: run.Outcome, Launch: run.Launch, Root: root, Reasons: run.Reasons}
	details := []string{run.Line}
	switch run.Outcome {
	case lane.AgentStarted:
		summary := "started the landing agent " + run.Launch + " for " + wakeWords(run.Reasons)
		return inv.render(intentResult{Outcome: intentConfirmed, Targets: targets, Data: data, Summary: summary, Details: details,
			next: inv.publicArgv("landing", "status"), nextReason: "shows what it lands",
			view: func(page *textui.Page) { page.Done(summary) }})
	case lane.AgentRunning:
		summary := "a landing agent is already running at " + root
		if run.Launch != "" {
			summary = "the landing agent " + run.Launch + " is already running"
		}
		why := "nothing to do; it is starting"
		if run.Launch != "" {
			why = "nothing to do; it is at work"
		}
		return inv.render(intentResult{Outcome: intentUnchanged, Targets: targets, Data: data, Summary: summary, Details: details, nextReason: why,
			view: func(page *textui.Page) { page.Done(summary); page.Hint(textui.Hint{Reason: why}) }})
	case lane.AgentIdle:
		summary := "the landing lane at " + root + " has no queued work, so no landing agent was started"
		why := "nothing to do; the lane is empty"
		return inv.render(intentResult{Outcome: intentUnchanged, Targets: targets, Data: data, Summary: summary, Details: details, nextReason: why,
			view: func(page *textui.Page) {
				page.Done(summary)
				page.Hint(textui.Hint{Reason: why})
			}})
	case lane.AgentPaused:
		summary := run.Line
		if pause, paused := lane.ReadPause(home); paused {
			summary = "the landing lane is stopped by " + pause.Who() + ", so no landing agent was started"
		}
		return inv.render(intentResult{Outcome: intentRefused, code: 1, Targets: targets, Data: data, Summary: summary, Details: details,
			next: inv.publicArgv("landing", "start"), nextReason: "resumes the lane; then run metasystem landing run again"})
	default:
		summary := run.Line
		if summary == "" {
			summary = "the landing agent at " + root + " was not started"
		}
		return inv.render(intentResult{Outcome: intentFailed, code: 1, Targets: targets, Data: data, Summary: summary, Details: details,
			next: inv.publicArgv("landing", "status", "--verbose"), nextReason: "shows the lane's state"})
	}
}

// insideLaneCheckout says whether landing run was called in the lane
// checkout: its working directory, or its --repo, is in it.
func (inv *intentInvocation) insideLaneCheckout(root string) bool {
	path := inv.cwd
	if inv.input.has("repo") {
		path = inv.callerPath(inv.input.text("repo"))
	}
	here, checkout := realpath.Resolve(path), realpath.Resolve(root)
	return here == checkout || strings.HasPrefix(here, checkout+string(filepath.Separator))
}

// handOffLandingRun runs landing run with the lane installation's own
// engine in the lane checkout. Its text passes through unchanged; with
// --json its JSON result is read and passed on.
func (inv *intentInvocation) handOffLandingRun(record lane.Record) int {
	targets := laneTargets(record.Root)
	layout, err := record.Layout()
	if err != nil {
		return inv.render(intentResult{Outcome: intentFailed, code: 1, Targets: targets,
			Summary: "the landing lane's installation can't be read, so no landing agent was started",
			next:    inv.publicArgv("landing", "set", record.Root), nextReason: "registers the landing checkout again, which records its installation",
			Details: []string{err.Error()}})
	}
	install := string(layout.Install)
	binary := filepath.Join(install, "bin", "metasystem")
	if info, err := os.Stat(binary); err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 {
		cause := "is missing"
		if err == nil {
			cause = "is not an executable file"
		}
		build := func(path string) string {
			// A path shown under ~ keeps its tilde outside the quotes, so
			// the shell still expands it.
			if rest, ok := strings.CutPrefix(path, "~/"); ok {
				return "cd ~/" + shellCommand([]string{rest}) + " && go run ./cmd/devgate build"
			}
			return "cd " + shellCommand([]string{path}) + " && go run ./cmd/devgate build"
		}
		summary := "the landing lane's engine bin/metasystem " + cause + ", so no landing agent was started"
		return inv.render(intentResult{Outcome: intentRefused, code: 1, Targets: targets, Summary: summary,
			next: []string{"sh", "-c", build(install)}, nextReason: "builds the lane's engine; then run metasystem landing run again",
			Details: []string{"the lane's engine " + binary + " " + cause}, viewsRefusal: true,
			view: func(page *textui.Page) {
				page.Refusal(summary, textui.Hint{Reason: build(page.Env().Path(install))})
			}})
	}
	args := []string{"landing", "run"}
	for _, flag := range []string{"json", "verbose"} {
		if inv.input.switched(flag) {
			args = append(args, "--"+flag)
		}
	}
	command := exec.Command(binary, args...)
	command.Dir = string(layout.Checkout)
	command.Stdin = os.Stdin
	command.Stderr = inv.stderr
	if inv.input.switched("json") {
		// The lane engine's envelope is read, never its text, and passed on.
		result, err := verbresult.Run(command, "landing run")
		if err != nil {
			return inv.render(intentResult{Outcome: intentFailed, code: 1, Targets: targets,
				Summary: "the landing lane's engine gave no result, so whether a landing agent started is unknown",
				next:    inv.publicArgv("landing", "status", "--verbose"), nextReason: "shows whether its landing agent runs",
				Details: []string{err.Error()}})
		}
		if err := verbresult.Write(inv.stdout, result); err != nil {
			return 1
		}
		return result.Exit
	}
	// Its text is the lane engine's own two lines, passed through unchanged.
	command.Stdout = inv.stdout
	runErr := command.Run()
	var exitErr *exec.ExitError
	switch {
	case errors.As(runErr, &exitErr):
		return exitErr.ExitCode()
	case runErr != nil:
		return inv.render(intentResult{Outcome: intentFailed, code: 1, Targets: targets,
			Summary: "the landing lane's engine could not be run, so no landing agent was started",
			next:    inv.sameCommand(), nextReason: "tries again", Details: []string{binary + ": " + runErr.Error()}})
	}
	return 0
}

func runIntentLandingStop(inv *intentInvocation) int {
	owners, home, record, problem := inv.laneContext(true)
	if problem != nil {
		return inv.render(*problem)
	}
	result, _ := inv.stopLane(owners, home, record, lane.PauseReason(inv.input.text("reason")))
	return inv.render(result)
}

// stopLane pauses the lane at a person's word.
func (inv *intentInvocation) stopLane(owners laneVerbOwners, home string, record lane.Record, reason string) (intentResult, bool) {
	targets := laneTargets(record.Root)
	if pause, paused := lane.ReadPause(home); paused {
		if err := stopLaneRegeneration(owners, record); err != nil {
			return landingLaneFailure(targets, "the running regeneration could not be stopped", err), false
		}
		result := intentResult{Outcome: intentUnchanged, Targets: targets, Summary: "the landing lane is already stopped by " + pause.Who() + " at " + lane.LocalText(pause.At) + "; metasystem landing start resumes it"}
		result.view = func(page *textui.Page) {
			done := "the landing lane is already stopped by " + pause.Who()
			if at, err := time.Parse(time.RFC3339, pause.At); err == nil {
				done += " " + page.Env().Since(at)
			}
			page.Done(done)
			page.Hint(textui.Hint{Argv: inv.publicArgv("landing", "start"), Reason: "resumes it"})
		}
		return result, true
	}
	by := inv.landingActor(owners)
	inv.sayWhenTheLaneLockIsHeld(home)
	if _, err := lane.SetPauseBecause(home, by, reason, owners.now()); err != nil {
		return intentResult{Outcome: intentFailed, code: 1, Targets: targets, Summary: "the landing lane couldn't be stopped",
			next: inv.sameCommand(), nextReason: "tries again", Details: []string{"the landing lane could not be stopped: " + err.Error()}}, false
	}
	if err := stopLaneRegeneration(owners, record); err != nil {
		return landingLaneFailure(targets, "the lane is paused, but its regeneration could not be stopped", err), false
	}
	// The way to seats landing their own work is unset, which line 2 names.
	who := lane.Pause{By: by, Reason: reason}.Who()
	return intentResult{Outcome: intentConfirmed, Targets: targets, Data: inv.laneView(owners, home),
		Summary: "stopped the landing lane for " + who + "; no landing agent starts until metasystem landing start",
		Details: []string{"the lane at " + record.Root + " starts no landing agent until metasystem landing start"},
		next:    inv.publicArgv("landing", "unset"), nextReason: "lets each seat land its own work instead",
		view: func(page *textui.Page) {
			page.Done("stopped the landing lane for " + who + "; no landing agent starts until it starts again")
		}}, true
}

// runIntentLandingUnset takes the lane away at a person's word (design r10
// §1): fence, stop the landing agent, then unregister. It is never refused
// to a person and it resumes: when the agent still runs, the same command
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
	switch {
	case report.NoLane:
		return inv.render(intentResult{Outcome: intentUnchanged, Targets: targets, Data: report,
			Summary: "no landing lane is registered on this computer; each seat lands its own work"})
	case report.Unregistered && report.CheckoutGone:
		summary := "unset the landing lane " + report.Record.Root + "; its checkout was gone. Each seat lands its own work now"
		return inv.render(intentResult{Outcome: intentConfirmed, Targets: targets, Data: report, Summary: summary})
	case report.Unregistered:
		summary := "unset this computer's landing lane " + report.Record.Root + "; each seat lands its own work now"
		return inv.render(intentResult{Outcome: intentConfirmed, Targets: targets, Data: report, Summary: summary,
			view: landingDone(summary, report.Record.Root)})
	case report.Stopped == lane.StepSettled:
		waiting := append(append([]string{}, report.Settlement.Live...), report.Settlement.Unknown...)
		result := intentResult{Outcome: intentInProgress, Targets: targets, Data: report,
			Summary: "the landing lane is fenced and waits before it is unregistered: " + strings.Join(waiting, "; "),
			next:    inv.publicArgv("landing", "unset"), nextReason: "continues once that work has ended",
			Details: []string{"nothing new starts in the lane; its record stays until its landing agent has ended"}}
		if len(report.Settlement.Live) == 0 {
			result.next, result.nextReason = inv.publicArgv("landing", "unset", "--force"), "goes past what cannot be known, once you have checked it"
		}
		return inv.render(result)
	}
	return inv.render(intentResult{Outcome: intentInProgress, Targets: targets, Data: report, Summary: "the landing lane is fenced and not unregistered yet",
		next: inv.publicArgv("landing", "unset"), nextReason: "continues the unset"})
}

// settleLandingAgent is landing unset's settle step: it stops the landing
// agent and reads whether it ended. A publication or other lane work no
// longer exists, so the agent is the one thing an unset waits for.
func settleLandingAgent(running func() (string, bool, error), stop func(string) error) func(lane.Layout) (lane.Settlement, error) {
	return func(lane.Layout) (lane.Settlement, error) {
		var settlement lane.Settlement
		id, live, err := running()
		switch {
		case err != nil:
			settlement.Unknown = append(settlement.Unknown, "whether the landing agent runs is unknown: "+err.Error())
		case live:
			if err := stop(id); err != nil {
				settlement.Unknown = append(settlement.Unknown, "the landing agent "+id+" could not be asked to end: "+err.Error())
			} else if after, still, err := running(); err != nil {
				settlement.Unknown = append(settlement.Unknown, "whether the landing agent ended is unknown: "+err.Error())
			} else if still {
				settlement.Live = append(settlement.Live, "the landing agent "+after+" still runs after it was asked to end")
			}
		}
		return settlement, nil
	}
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
// host flock: another lane step (a push to main among them) holds it, and
// the stop takes effect only when that finishes.
func (inv *intentInvocation) sayWhenTheLaneLockIsHeld(home string) {
	held, err := lock.File(lane.LockPath(home), 0o600, lock.TryExclusive)
	if err == nil {
		_ = held.Release()
		return
	}
	if lock.Busy(err) && !inv.input.switched("json") {
		page := textui.New(inv.textEnv(inv.stderr))
		page.Mark(textui.Running, "another landing step holds the lane; the stop takes effect when it finishes")
		_, _ = io.WriteString(inv.stderr, page.String())
	}
}

func stopLaneRegeneration(owners laneVerbOwners, record lane.Record) error {
	layout, err := record.Layout()
	if err != nil {
		return err
	}
	return owners.stopRegeneration(string(layout.Install))
}
