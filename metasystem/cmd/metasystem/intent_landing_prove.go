package main

// landing prove (plain lane step 3): runs the project's proof command,
// config key proof.full, over the landing checkout's HEAD in a
// fresh worktree at that commit, detached so it outlives the agent's session, and records green or red for
// that exact tree in results.jsonl, which landing push reads. The detached
// start is gaterun.LaunchDetached; nothing else of the older lane runs.

import (
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gaterun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/textui"
)

// proveCommandKey is the project's proof command, run in a worktree of the
// lane checkout at the proven commit with LANDING_TREE and LANDING_COMMIT set; exit 0 is green.
const proveCommandKey = "proof.full"

var errProofDeclaration = errors.New("proof declaration is missing or invalid in the proven commit's metasystem.conf")

type proofDeclarationError struct {
	key string
	err error
}

func (e *proofDeclarationError) Error() string {
	return e.key + " is missing or invalid in the proven commit's metasystem.conf: " + e.err.Error()
}

func (e *proofDeclarationError) Unwrap() error { return errProofDeclaration }

func proofDeclarationRemedy(key string) string {
	return "declare " + key + " in metasystem.conf through a goal and land it on main"
}

// landingProofCommand reads the declaration from the tree the command proves.
func landingProofCommand(installation, checkout, commit, key string, git func(string, ...string) (string, error)) (string, error) {
	path, err := filepath.Rel(checkout, filepath.Join(installation, "metasystem.conf"))
	if err != nil {
		return "", &proofDeclarationError{key: key, err: err}
	}
	content, err := git(checkout, "show", commit+":"+filepath.ToSlash(path))
	if err != nil {
		return "", &proofDeclarationError{key: key, err: err}
	}
	command, _, err := config.CommittedContentLookup(content, key)
	if err == nil {
		err = config.SettingValueProblem(key, command)
	}
	if err != nil {
		return "", &proofDeclarationError{key: key, err: err}
	}
	return command, nil
}

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

// lanePaused holds automation outside the recorded selection under a pause.
func (inv *intentInvocation) lanePaused(admitted laneAdmitted, what string) *intentResult {
	pause, paused := lane.ReadPause(admitted.home)
	if !paused || (!inv.input.switched("trunk") && plain.PersonBatchContinuation(admitted.installation, admitted.record, admitted.home) != "") {
		return nil
	}
	return &intentResult{Outcome: intentRefused, code: 1, Targets: laneTargets(admitted.record.Root),
		Summary: "the landing lane is stopped by " + pause.Who() + ", so nothing was " + what,
		next:    inv.publicArgv("landing", "start"), nextReason: "a person resumes it"}
}

// proveSeams are landing prove's effects: the test's, else this engine
// started detached.
func (owners laneVerbOwners) proveSeams(installation string) plain.ProveSeams {
	seams := owners.plainProve
	if seams.Executable == nil {
		seams.Executable = os.Executable
	}
	if seams.Launch == nil {
		seams.Launch = func(argv []string, dir, log string) (int64, error) {
			return gaterun.LaunchDetached(gaterun.DetachedLaunch{Argv: argv, Dir: dir, Log: log})
		}
	}
	if seams.Git == nil {
		seams.Git = plain.Git
	}
	if seams.Judge == nil {
		seams.Judge = landingFlakeJudge(installation, seams.Git)
	}
	if seams.RecordFlake == nil {
		seams.RecordFlake = owners.landingFlakeRecorder(installation)
	}
	if seams.RecordMain == nil {
		seams.RecordMain = owners.landingIncidentRecorder(installation)
	}
	return seams
}

func landingProveCommand() intentCommand {
	return laneCommand(intentCommand{
		object: "landing", action: "prove", audience: "both", summary: "prove the landing checkout's HEAD with the project's own command",
		usage: []string{"metasystem landing prove [--impact|--gate|--trunk] [--wait]"},
		details: []string{"Runs the shell command set as proof.full in a fresh worktree of the lane checkout at HEAD's commit, from its installation folder, with LANDING_TREE and LANDING_COMMIT naming what it proves, LANDING_PROOF_SCOPE naming full, scoped or impact, LANDING_PROOF_BASE naming the base tree (empty for full), LANDING_PROOF_GROUPS naming space-separated declared group ids, and LANDING_PROOF_PACKAGES naming space-separated packages or package=TestA,TestB selections (both empty for full), and LANDING_ONLY naming a failed unit when it is checked again alone; exit 0 is green, anything else red. Changes not committed in the lane checkout are not seen.",
			"It starts in the background and the command returns at once, so it outlives the session that asked for it; the keeper wakes the landing agent when it ends. landing status shows it while it runs.",
			"A current green, or a tree that differs from it only in goal ledger files, is reported at once so landing push can follow in the same turn. An inherited or scoped green needs a full proof no more than an hour old. After record or ledger changes under a proven batch, the proof runs only the groups whose declared inputs cover what main gained, while that batch's full proof is under an hour old.",
			"Asked again while that tree is being proven, it starts nothing; while another tree is, it is refused. The result is kept for that exact tree in results.jsonl, which landing push reads.",
			"--gate runs committed proof.cheap after a merge, first recording a green baseline of its first parent, with that tree as LANDING_PROOF_BASE. Its result and one repeat per tree are kept in gates.jsonl; a green gate never authorizes landing push. --wait proves in this command and says the result. A recorded selection may continue through its standing pause; a direct person may request one proof while it stays stopped.",
			"--impact runs proof.cheap against the recorded batch base with fast-static-build first, and records its plan hash and environment without inheriting a full proof. When the batch decided full depth, an explicit impact green does not satisfy landing push.",
			"--trunk fetches origin/main and runs a fresh full check there, even after a green or red; the lane checkout stays where it is. --gate and --trunk cannot be used together."},
		flags: []intentFlag{{name: "impact", usage: "prove the batch with static checks and impact tests"}, {name: "trunk", usage: "fetch and freshly prove main in full"}, {name: "gate", usage: "check the last merge with proof.cheap"}, {name: "wait", usage: "prove here and wait for the result"},
			{name: "classify", value: "ATTEMPT", usage: "classify one saved red; a person supplies this act"},
			{name: "attempt", value: "ID", hidden: true, usage: "the attempt id a background start chose"}},
		maxArgs:  0,
		examples: []string{"metasystem landing prove"},
	}, runIntentLandingProve)
}

func runIntentLandingProve(inv *intentInvocation, admitted laneAdmitted) int {
	targets := laneTargets(admitted.record.Root)
	if inv.input.switched("gate") && inv.input.switched("trunk") || inv.input.switched("impact") && (inv.input.switched("gate") || inv.input.switched("trunk")) {
		return inv.render(intentResult{Outcome: intentRefused, code: 1, Targets: targets,
			Summary: "--impact, --gate and --trunk cannot be used together, so nothing was proven"})
	}
	var person *plain.ActProvenance
	// The detached child claims the recorded admission; only a fresh public
	// invocation observes direct authority at the original calling checkout.
	if inv.input.text("attempt") == "" {
		observed, problem := inv.lanePerson("request this check", admitted.record.Root)
		if problem == nil {
			person = &plain.ActProvenance{Kind: "proof", Person: observed.Name, Root: observed.Root, CheckedAt: observed.At.UTC(),
				TerminalGeneration: observed.Proof.TerminalGeneration, TerminalRef: observed.Proof.TerminalRef, Destination: admitted.record}
		}
	} else if !inv.input.switched("wait") {
		return inv.render(landingProveRefusal(inv, targets, &plain.Refusal{Code: "LANE_PROOF_ADMISSION", Reason: "a background attempt must claim its admission with --wait", Next: "metasystem landing prove"}))
	}

	if inv.input.text("classify") != "" && (inv.input.switched("impact") || inv.input.switched("gate") || inv.input.switched("trunk") || inv.input.text("attempt") != "") {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Targets: targets, Summary: "--classify names a saved red and cannot be combined with another proof subject"})
	}
	if inv.input.text("classify") != "" && person == nil {
		_, problem := inv.lanePerson("classify this saved red", admitted.record.Root)
		return inv.render(*problem)
	}
	seams := admitted.owners.proveSeams(admitted.installation)
	seams = inv.laneBatchSeams(admitted.home, admitted.record, seams)
	if person != nil {
		seams.Person = person
		seams.FenceCheck = nil
	}
	seams.Trunk = inv.input.switched("trunk")
	checkout := string(admitted.layout.Checkout)
	seams.Gate = inv.input.switched("gate")
	seams.Impact = inv.input.switched("impact")
	key := proveCommandKey
	if seams.Impact {
		key = "proof.cheap"
	}
	if seams.Gate {
		key = "proof.cheap"
		if admitted.owners.plainProve.Judge == nil {
			seams.Judge = landingFlakeJudgeFor(admitted.installation, seams.Git, true)
		}
	}
	git := seams.Git
	if git == nil {
		git = plain.Git
	}
	seams.CommandForCommit = func(commit string) (string, error) {
		return landingProofCommand(admitted.installation, checkout, commit, key, git)
	}
	if classify := inv.input.text("classify"); classify != "" {
		seams.GateCommandForCommit = func(commit string) (string, error) {
			return landingProofCommand(admitted.installation, checkout, commit, "proof.cheap", git)
		}
		result, err := plain.ClassifyAttempt(admitted.installation, checkout, classify, seams)
		_ = plain.SyncPolicyQuestion(admitted.installation, admitted.owners.machine, admitted.owners.now())
		if err != nil {
			return inv.render(landingProveRefusal(inv, targets, err))
		}
		summary := "classified saved red " + classify + " at " + shortLandingID(result.Commit)
		if result.Cause != nil {
			summary += "; cause: " + result.Cause.Kind
		}
		if full := result.NextFull; full != nil {
			summary += "; its next full check is " + string(full.Result)
			if full.Trunk && full.Result == plain.Green {
				if err := admitted.owners.clearLandingIncidents(admitted.installation, *full); err != nil {
					return inv.render(landingLaneFailure(targets, "main passed its full check, but its incidents could not be cleared", err))
				}
			}
		}
		return inv.render(intentResult{Outcome: intentConfirmed, Targets: targets, Data: result, Summary: summary, next: inv.publicArgv("landing", "status"), nextReason: "shows the separate return or proof act"})
	}
	ref := "HEAD"
	if seams.Trunk {
		ref = "origin/main"
		if inv.input.text("attempt") == "" {
			if _, err := git(checkout, "fetch", "origin", "main"); err != nil {
				return inv.render(landingProveRefusal(inv, targets, err))
			}
		}
	}
	commit, err := git(checkout, "rev-parse", "--verify", ref+"^{commit}")
	if err != nil {
		return inv.render(landingProveRefusal(inv, targets, err))
	}
	attempt := inv.input.text("attempt")
	if attempt != "" {
		running, recorded, _, err := plain.ReadRunning(admitted.installation, seams)
		if err != nil {
			return inv.render(landingProveRefusal(inv, targets, err))
		}
		if recorded && running.Admission != nil && !seams.Gate && !seams.Trunk && !inv.input.switched("impact") {
			seams.Impact = running.Impact
		}
		if !recorded || running.Attempt != attempt || running.Admission == nil || (running.Admission.State != "launched" && running.Admission.State != "pending") || running.Gate != seams.Gate || running.Trunk != seams.Trunk || running.Impact != seams.Impact {
			return inv.render(landingProveRefusal(inv, targets, &plain.Refusal{Code: "LANE_PROOF_ADMISSION", Reason: "this background check has no matching unclaimed admission", Next: "metasystem landing prove"}))
		}
		commit = running.Commit
		seams.DepthScope = running.Admission.DepthScope
		seams.DepthReason = running.Admission.Reason
	}
	if attempt == "" && !seams.Gate && !seams.Trunk {
		impact, reason := batchDepth(admitted.installation, checkout, commit, seams)
		if reason != "" {
			seams.DepthScope = "full"
			if impact {
				seams.DepthScope = "impact"
			}
			seams.DepthReason = reason
			seams.Impact = seams.Impact || impact
		}
	}
	if seams.Impact {
		key = "proof.cheap"
	}
	command, err := seams.CommandForCommit(commit)
	if err != nil {
		return inv.render(landingProveRefusal(inv, targets, err))
	}
	if !inv.input.switched("wait") {
		settled, ok, err := plain.Settled(admitted.installation, checkout, seams)
		if err != nil {
			return inv.render(landingProveRefusal(inv, targets, err))
		}
		if ok {
			summary := provedWords(settled.Commit, settled.Tree) + " is already proven green"
			if settled.Reason != "" {
				summary += " (" + settled.Reason + ")"
			}
			next, reason := inv.publicArgv("landing", "push"), "puts it on main now, in this turn"
			if seams.Gate {
				summary = provedWords(settled.Commit, settled.Tree) + " already passed the cheap gate"
				next, reason = inv.publicArgv("landing", "status"), "shows the waiting goals to merge next; the completed batch still needs landing prove"
			}
			return inv.render(intentResult{Outcome: intentUnchanged, Targets: targets, Data: settled, Summary: summary,
				next: next, nextReason: reason})
		}
		running, already, err := plain.Start(admitted.installation, checkout, seams)
		_ = plain.SyncPolicyQuestion(admitted.installation, admitted.owners.machine, admitted.owners.now())
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
	_ = plain.SyncPolicyQuestion(admitted.installation, admitted.owners.machine, admitted.owners.now())
	if err != nil {
		return inv.render(landingProveRefusal(inv, targets, err))
	}
	words := provedWords(result.Commit, result.Tree)
	if result.Result == plain.Green {
		if result.Trunk {
			if err := admitted.owners.clearLandingIncidents(admitted.installation, result); err != nil {
				return inv.render(landingLaneFailure(targets, "main passed its full check, but its incidents could not be cleared", err))
			}
			summary := "main passed its full check; no push is needed"
			return inv.render(intentResult{Outcome: intentConfirmed, Targets: targets, Data: result, Summary: summary, view: landingDone(summary, result.Log)})
		}
		summary := words + " is proven green" + landingRedReason(result.Reason) + "; landing push may put it on main"
		if seams.Gate {
			summary = words + " passed the cheap gate" + landingRedReason(result.Reason) + "; the batch still needs landing prove before landing push"
		}
		return inv.render(intentResult{Outcome: intentConfirmed, Targets: targets, Data: result, Summary: summary, view: landingDone(summary, result.Log)})
	}
	return inv.render(intentResult{Outcome: intentFailed, code: 1, Targets: targets, Data: result,
		Summary: words + " is proven red" + landingRedReason(result.Reason) + "; its log is " + result.Log,
		next:    inv.publicArgv("landing", "status"), nextReason: "shows the cause and the waiting goals"})
}

// landingProveRefusal renders a prove that could not start or run.
func landingProveRefusal(inv *intentInvocation, targets []intentTarget, err error) intentResult {
	var batch *plain.Refusal
	if errors.As(err, &batch) {
		return intentResult{Outcome: intentRefused, code: 1, Targets: targets, Summary: batch.Reason, Decision: batch.Next, Details: []string{batch.Code}}
	}
	if errors.Is(err, errProofDeclaration) {
		key := proveCommandKey
		if inv.input.switched("gate") {
			key = "proof.cheap"
		}
		return intentResult{Outcome: intentRefused, code: 1, Targets: targets,
			Summary:  key + " is missing or invalid in the proven commit's metasystem.conf, so nothing was proven",
			Decision: proofDeclarationRemedy(key), Details: []string{err.Error()}}
	}
	var noRepeat *plain.NoRepeat
	if errors.As(err, &noRepeat) {
		return intentResult{Outcome: intentRefused, code: 1, Targets: targets, Summary: noRepeat.Error(),
			next: proofRetryArgv(inv), nextReason: "a person requests one execution while the prior allowance stays spent"}
	}
	var busy *plain.Busy
	if errors.As(err, &busy) {
		return intentResult{Outcome: intentRefused, code: 1, Targets: targets, Summary: busy.Error(),
			next: inv.publicArgv("landing", "status"), nextReason: "shows when it ends"}
	}
	return landingLaneFailure(targets, "the full check command could not run: "+oneLine(err.Error()), err)
}

func landingLaneFailure(targets []intentTarget, summary string, err error) intentResult {
	return intentResult{Outcome: intentFailed, code: 1, Targets: targets, Summary: summary, retry: "tries again", Details: []string{err.Error()}}
}

// provedWords names a proof's subject: its commit and tree.
func provedWords(commit, tree string) string {
	return shortLandingID(commit) + " (tree " + shortLandingID(tree) + ")"
}

// withRunningProof adds the lane's running proof to landing status's page.
func withRunningProof(view func(*textui.Page), running *plain.RunningProof) func(*textui.Page) {
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
		command := "metasystem landing prove"
		if running.Gate {
			command += " --gate"
		} else if running.Trunk {
			command += " --trunk"
		}
		switch running.State {
		case "died":
			words = "check of tree " + shortLandingID(running.Tree) + " (attempt " + running.Attempt + ") died without a result; retry: " + command
		case "pending":
			words = "check admission pending for tree " + shortLandingID(running.Tree) + " (attempt " + running.Attempt + "); retry: " + command + " after admission ends"
		case "failed":
			words = "check admission failed for tree " + shortLandingID(running.Tree) + " (attempt " + running.Attempt + "); retry: " + command
		}

		page.Section("Proving", "").Text(words)
	}
}

// landingRedReason is a red result's recorded reason, in brackets: how the
// proof command ended, or why it could not run.
func landingRedReason(reason string) string {
	if reason == "" {
		return ""
	}
	return " (" + reason + ")"
}

func proofRetryArgv(inv *intentInvocation) []string {
	argv := inv.publicArgv("landing", "prove")
	if inv.input.switched("gate") {
		argv = append(argv, "--gate")
	}
	if inv.input.switched("trunk") {
		argv = append(argv, "--trunk")
	}
	return argv
}
