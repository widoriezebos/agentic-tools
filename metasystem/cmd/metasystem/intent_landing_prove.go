package main

// landing prove (plain lane step 3): runs the project's proof command,
// config key landing.prove.command, over the landing checkout's HEAD,
// detached so it outlives the agent's session, and records green or red for
// that exact tree in results.jsonl, which landing push reads. The detached
// start is gaterun.LaunchDetached; nothing else of the older lane runs.

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gaterun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/textui"
)

// proveCommandKey is the project's proof command, run in the lane checkout
// with LANDING_TREE and LANDING_COMMIT set; exit 0 is green.
const proveCommandKey = "landing.prove.command"

// laneAdmitted is a lane verb's admission: the registered lane and its
// installation.
type laneAdmitted struct {
	owners       laneVerbOwners
	home         string
	record       lane.Record
	layout       lane.Layout
	installation string
}

// laneCommand declares a lane verb whose run starts only once the
// registered lane was read.
func laneCommand(command intentCommand, run func(*intentInvocation, laneAdmitted) int) intentCommand {
	command.run = func(inv *intentInvocation) int {
		admitted, problem := inv.admitLane()
		if problem != nil {
			return inv.render(*problem)
		}
		return run(inv, admitted)
	}
	return command
}

// admitLane reads the registered lane and the installation landing set
// recorded for it, never guessed from the checkout again.
func (inv *intentInvocation) admitLane() (laneAdmitted, *intentResult) {
	owners, home, record, problem := inv.laneContext(true)
	if problem != nil {
		return laneAdmitted{}, problem
	}
	layout, err := record.Layout()
	if err != nil {
		return laneAdmitted{}, &intentResult{Outcome: intentFailed, code: 1, Targets: laneTargets(record.Root),
			Summary: "the landing lane's installation can't be found, so nothing was done",
			next:    inv.publicArgv("landing", "status", "--verbose"), nextReason: "shows the lane's checkout",
			Details: []string{"the lane at " + record.Root + " has no metasystem installation: " + err.Error()}}
	}
	return laneAdmitted{owners: owners, home: home, record: record, layout: layout, installation: string(layout.Install)}, nil
}

// lanePaused is the refusal of a lane verb while a person stopped the lane.
func (inv *intentInvocation) lanePaused(admitted laneAdmitted, what string) *intentResult {
	pause, paused := lane.ReadPause(admitted.home)
	if !paused {
		return nil
	}
	return &intentResult{Outcome: intentRefused, code: 1, Targets: laneTargets(admitted.record.Root),
		Summary: "the landing lane is stopped by " + pause.Who() + ", so nothing was " + what,
		next:    inv.publicArgv("landing", "start"), nextReason: "a person resumes it"}
}

// proveSeams are landing prove's effects: the test's, else this engine
// started detached.
func (owners laneVerbOwners) proveSeams() plain.ProveSeams {
	seams := owners.plainProve
	if seams.Executable == nil {
		seams.Executable = os.Executable
	}
	if seams.Launch == nil {
		seams.Launch = func(argv []string, dir, log string) (int64, error) {
			return gaterun.LaunchDetached(gaterun.DetachedLaunch{Argv: argv, Dir: dir, Log: log})
		}
	}
	return seams
}

func landingProveCommand() intentCommand {
	return laneCommand(intentCommand{
		object: "landing", action: "prove", audience: "both", summary: "prove the landing checkout's HEAD with the project's own command",
		usage: []string{"metasystem landing prove [--wait]"},
		details: []string{"Runs the shell command set as landing.prove.command in the lane checkout at HEAD, with LANDING_TREE and LANDING_COMMIT naming what it proves; exit 0 is green, anything else red.",
			"It starts in the background and the command returns at once, so it outlives the session that asked for it; the keeper wakes the landing agent when it ends. landing status shows it while it runs.",
			"Asked again while that tree is being proven, it starts nothing; while another tree is, it is refused. The result is kept for that exact tree in results.jsonl, which landing push reads.",
			"--wait proves in this command and says the result. Refused while the lane is stopped."},
		flags: []intentFlag{{name: "wait", usage: "prove here and wait for the result"},
			{name: "attempt", value: "ID", hidden: true, usage: "the attempt id a background start chose"}},
		maxArgs:  0,
		examples: []string{"metasystem landing prove"},
	}, runIntentLandingProve)
}

func runIntentLandingProve(inv *intentInvocation, admitted laneAdmitted) int {
	targets := laneTargets(admitted.record.Root)
	if refused := inv.lanePaused(admitted, "proven"); refused != nil {
		return inv.render(*refused)
	}
	command, _, err := config.Get(config.GetParams{Key: proveCommandKey, ConfPath: filepath.Join(admitted.installation, "metasystem.conf"), Default: "", DefaultSet: true})
	if err != nil || strings.TrimSpace(command) == "" {
		result := intentResult{Outcome: intentRefused, code: 1, Targets: targets,
			Summary: "this landing lane has no command that proves HEAD, so nothing was proven",
			next:    inv.publicArgv("settings", "set", proveCommandKey, "COMMAND"), nextReason: "run in the lane checkout"}
		if err != nil {
			result.Details = []string{err.Error()}
		}
		return inv.render(result)
	}
	seams := admitted.owners.proveSeams()
	checkout := string(admitted.layout.Checkout)
	if !inv.input.switched("wait") {
		running, already, err := plain.Start(admitted.installation, checkout, seams)
		if err != nil {
			return inv.render(landingProveRefusal(inv, targets, err))
		}
		result := intentResult{Outcome: intentConfirmed, Targets: targets, Data: running,
			Summary: "proving " + provedWords(running.Commit, running.Tree) + " in the background; end your turn, the keeper wakes the landing agent when it ends"}
		if already {
			result.Outcome = intentUnchanged
			result.Summary = "already proving " + provedWords(running.Commit, running.Tree) + " since " + lane.LocalText(running.Since)
		}
		return inv.render(result)
	}
	attempt := inv.input.text("attempt")
	output := os.Stdout
	if attempt == "" {
		// A person's --wait keeps the command's output in the lane's log,
		// as a detached proof does; this command's own output is its result.
		logs := filepath.Join(plain.Dir(admitted.installation), "proofs")
		if err := os.MkdirAll(logs, 0o755); err != nil {
			return inv.render(landingLaneFailure(targets, "the log of landing prove can't be created, so nothing was proven", err))
		}
		file, err := os.Create(filepath.Join(logs, time.Now().UTC().Format("20060102T150405.000000000Z")+".log"))
		if err != nil {
			return inv.render(landingLaneFailure(targets, "the log of landing prove can't be created, so nothing was proven", err))
		}
		defer file.Close()
		output = file
	}
	result, err := plain.Run(admitted.installation, checkout, command, attempt, output, seams)
	if err != nil {
		return inv.render(landingProveRefusal(inv, targets, err))
	}
	words := provedWords(result.Commit, result.Tree)
	if result.Result == plain.Green {
		summary := words + " is proven green; landing push may put it on main"
		return inv.render(intentResult{Outcome: intentConfirmed, Targets: targets, Data: result, Summary: summary, view: landingDone(summary, result.Log)})
	}
	return inv.render(intentResult{Outcome: intentFailed, code: 1, Targets: targets, Data: result,
		Summary: words + " is proven red; its log is " + result.Log,
		next:    inv.publicArgv("landing", "return", "GOAL", "--reason", "TEXT"), nextReason: "gives the goal that broke it back to its seat"})
}

// landingProveRefusal renders a prove that could not start or run.
func landingProveRefusal(inv *intentInvocation, targets []intentTarget, err error) intentResult {
	var busy *plain.Busy
	if errors.As(err, &busy) {
		return intentResult{Outcome: intentRefused, code: 1, Targets: targets, Summary: busy.Error(),
			next: inv.publicArgv("landing", "status"), nextReason: "shows when it ends"}
	}
	return landingLaneFailure(targets, "landing.prove.command could not run: "+oneLine(err.Error()), err)
}

func landingLaneFailure(targets []intentTarget, summary string, err error) intentResult {
	return intentResult{Outcome: intentFailed, code: 1, Targets: targets, Summary: summary, retry: "tries again", Details: []string{err.Error()}}
}

// provedWords names a proof's subject: its commit and tree.
func provedWords(commit, tree string) string {
	return shortLandingID(commit) + " (tree " + shortLandingID(tree) + ")"
}

// landingRunningProof is the lane's proof recorded running, as landing
// status shows it.
type landingRunningProof struct {
	Tree   string `json:"tree"`
	Commit string `json:"commit,omitempty"`
	Since  string `json:"since"`
	// Attempt and Log name the run; State is running while its process
	// runs, died when it ended without a result (the next prove runs it
	// again).
	Attempt string `json:"attempt"`
	Log     string `json:"log,omitempty"`
	State   string `json:"state"`
}

// readLandingRunningProof is the lane's running proof; nil when none is
// recorded running or it can't be read.
func readLandingRunningProof(install string, seams plain.ProveSeams) *landingRunningProof {
	running, recorded, alive, err := plain.ReadRunning(install, seams)
	if err != nil || !recorded {
		return nil
	}
	state := "running"
	if !alive {
		state = "died"
	}
	return &landingRunningProof{Attempt: running.Attempt, Tree: running.Tree, Commit: running.Commit, Since: running.Since, Log: running.Log, State: state}
}

// withRunningProof adds the lane's running proof to landing status's page.
func withRunningProof(view func(*textui.Page), running *landingRunningProof) func(*textui.Page) {
	if running == nil {
		return view
	}
	return func(page *textui.Page) {
		view(page)
		since := lane.LocalText(running.Since)
		if at, err := time.Parse(time.RFC3339, running.Since); err == nil {
			since = page.Env().Since(at)
		}
		words := "proving tree " + shortLandingID(running.Tree) + " as attempt " + running.Attempt + ", " + since
		if running.State == "died" {
			words = "proving tree " + shortLandingID(running.Tree) + " (attempt " + running.Attempt + ") died without a result; the next landing prove runs it again"
		}
		page.Section("Proving", "").Text(words)
	}
}
