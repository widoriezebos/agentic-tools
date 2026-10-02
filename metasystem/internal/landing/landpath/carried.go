package landpath

import (
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
)

// The carried transaction: one human carry word lands exactly one commit on
// main, past exactly one recorded refusal, with a reservation, a local
// pre-push intent and a completed ledger record that together survive a
// crash at any point between them.

var fullCommitID = regexp.MustCompile(`^[0-9a-f]{40}$`)

// usingException is the command that lands under this exception again.
func (d *driver) usingException() []string {
	return []string{"metasystem", "work", "land", d.request.Goal, "--using-exception", d.request.Carried}
}

// exceptionGone is the command that records a new exception for the goal.
func (d *driver) exceptionGone() string {
	return "record a new one with metasystem work land " + d.request.Goal + " --exception CODE --reason R"
}

func (d *driver) goalFetchForCarry() {
	output, status := d.owners.GoalFetch(d.request.Root)
	if status != 0 {
		d.carryAsk("the goal records couldn't be fetched, so nothing was landed", d.usingException(), "once the goal remote answers", output)
	}
	output = strings.TrimRight(output, "\n")
	tip := output
	if _, after, found := strings.Cut(output, "tip="); found {
		tip = after
	}
	tip, _, _ = strings.Cut(tip, " ")
	if !fullCommitID.MatchString(tip) {
		d.carryAsk("the goal records came back incomplete, so nothing was landed", d.usingException(), repeat,
			"goal fetch returned no accepted ledger tip: "+output)
	}
	d.carriedLedgerTip = tip
}

func (d *driver) readCarryStatus() {
	status, output, code := d.owners.CarryStatus(d.request.Root, d.request.Carried, d.request.Goal, d.carriedLedgerTip)
	if code != 0 {
		d.carryAsk("exception "+d.request.Carried+" can't be read from the goal records, so nothing was landed", nil,
			"check the exception id (metasystem goal show "+d.request.Goal+")", output)
	}
	d.carriedWord, d.carriedConsumption, d.carriedReservation = status.Word, status.Consumption, status.Reservation
	d.carriedIntent, d.carriedCounselor, d.carriedPast = status.Intent, status.Counselor, status.Past
	d.carriedBy, d.carriedWorkspace, d.carriedSource = status.By, status.Workspace, status.Source
	if d.carriedWord == "" || d.carriedConsumption == "" || d.carriedReservation == "" {
		d.carryAsk("exception "+d.request.Carried+" reads incomplete in the goal records, so nothing was landed",
			[]string{"metasystem", "goal", "sync"}, "then repeat this command", "carry status was incomplete")
	}
}

func (d *driver) abandonReservation(row, why string) {
	d.owners.GoalCarrying(CarryingRequest{Root: d.request.Root, Goal: d.request.Goal, Abandon: row, Why: why, Lineage: d.request.OwnerLineage})
}

func (d *driver) mustGit(args ...string) string {
	result := d.git(args...)
	if result.Code != 0 {
		code := result.Code
		if code < 0 {
			code = 1
		}
		d.stop(code, "git "+args[0]+" failed: "+oneLine(string(result.Stderr)), nil, "fix what git reports (--verbose shows it), then repeat this command", string(result.Stderr))
		exitLanding(code)
	}
	return strings.TrimSpace(string(result.Stdout))
}

func (d *driver) atOriginMain() bool {
	return d.mustGit("rev-parse", "HEAD") == d.mustGit("rev-parse", "refs/remotes/origin/main")
}

// workspaceAsk stops when the candidate's files are not those the exception
// covered: a person replaces the exception for the files as they now are.
func (d *driver) workspaceAsk(candidate, tree, why string) {
	d.carryAsk("the files changed since exception "+d.request.Carried+" was recorded ("+why+"), so it no longer covers them",
		[]string{"metasystem", "work", "land", d.request.Goal, "--exception", d.carriedPast, "--replace-exception", d.request.Carried,
			"--reason", why, "--by", strings.TrimPrefix(d.carriedBy, "human:")}, "a person replaces the exception",
		fmt.Sprintf("word workspace=%s candidate workspace=%s (tree %s)", d.carriedWorkspace, candidate, tree))
}

// carryForwardStaged moves the staged candidate onto origin/main and refuses
// when that changed the carried workspace; release names the reservation to
// close on such an ask.
func (d *driver) carryForwardStaged(release string) {
	if !d.atOriginMain() {
		base := d.mustGit("rev-parse", "HEAD")
		wip := d.mustGit("commit-tree", d.mustGit("write-tree"), "-p", "HEAD", "-m", "carried wip")
		d.mustGit("update-ref", "HEAD", wip, base)
		d.stepOutput.Reset()
		if d.gitTo(&d.stepOutput, "rebase", "refs/remotes/origin/main") != 0 {
			conflicts, _ := d.gitOut("diff", "--name-only", "--diff-filter=U")
			conflicts = strings.Join(strings.Fields(conflicts), ",")
			d.mustGit("rebase", "--abort")
			d.mustGit("reset", "--soft", base)
			if release != "" {
				d.abandonReservation(release, "carried landing stopped at rebase conflict")
			}
			if conflicts == "" {
				conflicts = "unknown paths"
			}
			d.carryAsk("the change conflicts with main in "+conflicts+", so nothing was landed", nil,
				"merge origin/main by hand, stage the result, then repeat this command")
		}
		d.mustGit("reset", "--soft", "refs/remotes/origin/main")
	}
	tree := d.mustGit("write-tree")
	workspace, err := d.owners.Live().Workspace(d.request.Root, tree)
	if err != nil {
		workspace = ""
	}
	if workspace != d.carriedWorkspace {
		if release != "" {
			d.abandonReservation(release, "origin moved the carried workspace")
		}
		if d.carriedSource == "answer" {
			d.owners.ChannelAskCarry(d.request.Root, d.request.Goal,
				"carry workspace="+workspace+" goal="+d.request.Goal+" past="+d.carriedPast,
				"origin moved the candidate workspace after the channel carry word")
		}
		d.workspaceAsk(workspace, tree, "origin moved the carried workspace")
	}
}

func (d *driver) reserveCarry() {
	tree := d.mustGit("write-tree")
	output, status := d.owners.GoalCarrying(CarryingRequest{Root: d.request.Root, Goal: d.request.Goal, Ref: d.request.Carried,
		Tree: tree, By: d.carriedBy, Lineage: d.request.OwnerLineage})
	if status != 0 {
		d.carryAsk("the goal couldn't be reserved for this exception landing, so nothing was landed", nil, "fix what the goal records report (--verbose shows it), then repeat this command", output)
	}
	output = strings.TrimRight(output, "\n")
	row := strings.TrimPrefix(output, "carrying=")
	row, _, _ = strings.Cut(row, " ledger=")
	tip := output
	if index := strings.LastIndex(output, " ledger="); index >= 0 {
		tip = output[index+len(" ledger="):]
	}
	if row == "" || !fullCommitID.MatchString(tip) {
		d.carryAsk("the goal's reservation for this landing came back incomplete, so nothing was landed", d.usingException(), repeat,
			"reservation returned an incomplete row: "+output)
	}
	d.carriedRow, d.carriedLedgerTip = row, tip
	d.carryAbandonArmed = true
}

// trailerValue is the one value of a trailer key in a commit's message.
func (d *driver) trailerValue(commit, key string) (string, bool) {
	message, status := d.gitOut("log", "-1", "--format=%B", commit)
	if status != 0 {
		return "", false
	}
	var values []string
	for _, line := range strings.Split(message, "\n") {
		if value, found := strings.CutPrefix(line, key+": "); found && value != "" {
			values = append(values, value)
		}
	}
	if len(values) != 1 {
		return "", false
	}
	return values[0], true
}

func (d *driver) mustTrailer(commit, key string) string {
	value, ok := d.trailerValue(commit, key)
	if !ok {
		exitLanding(1)
	}
	return value
}

func (d *driver) verifyLocalCarriedCommit(commit string) {
	tree, status := d.gitOut("rev-parse", commit+"^{tree}")
	if status != 0 {
		exitLanding(status)
	}
	workspace, err := d.owners.Live().Workspace(d.request.Root, tree)
	if err != nil {
		d.failed(1, "the files of the unfinished exception commit can't be read, so nothing was landed", err)
		exitLanding(1)
	}
	if workspace != d.carriedWorkspace {
		if d.carriedSource == "answer" {
			d.owners.ChannelAskCarry(d.request.Root, d.request.Goal,
				"carry workspace="+workspace+" goal="+d.request.Goal+" past="+d.carriedPast,
				"the recovered commit workspace differs from the channel carry word")
		}
		d.workspaceAsk(workspace, tree, "the recovered carried workspace changed")
	}
	message, status := d.gitOut("log", "-1", "--format=%B", commit)
	if status != 0 {
		exitLanding(status)
	}
	unfinished := "the unfinished exception commit " + shortID(commit) + " at HEAD isn't the one this exception made"
	undo := "undo it (git reset --soft HEAD^), then repeat this command"
	for _, key := range carriedKeys {
		if count := countLines(message, `(?m)^`+regexp.QuoteMeta(key)+`:`); count != 1 {
			d.stop(1, unfinished, nil, undo, fmt.Sprintf("local carried commit %s has %d %s trailers; expected exactly one", commit, count, key))
			exitLanding(1)
		}
	}
	carry, ok := d.trailerValue(commit, "Carry")
	if !ok {
		exitLanding(1)
	}
	if carry != d.request.Carried {
		d.stop(1, unfinished, nil, undo, fmt.Sprintf("local carried commit names Carry: %s, not %s", carry, d.request.Carried))
		exitLanding(1)
	}
	if countExact(message, "Goal-Item: "+d.request.Goal) != 1 {
		d.stop(1, unfinished, nil, undo, "local carried commit must have exactly one Goal-Item: "+d.request.Goal)
		exitLanding(1)
	}
}

// shortID is a commit's short form for line 1.
func shortID(commit string) string {
	if len(commit) > 12 {
		return commit[:12]
	}
	return commit
}

func (d *driver) rebaseRecovered() {
	if d.atOriginMain() {
		return
	}
	d.stepOutput.Reset()
	if d.gitTo(&d.stepOutput, "rebase", "refs/remotes/origin/main") != 0 {
		d.git("rebase", "--abort")
		d.carryAsk("the unfinished exception commit conflicts with main, so nothing was landed", nil,
			"rebase it onto origin/main by hand, then repeat this command")
	}
}

func (d *driver) ensureRecoveryReservation() {
	switch {
	case strings.HasPrefix(d.carriedReservation, "reservation: open:"):
		d.carriedRow = strings.TrimPrefix(d.carriedReservation, "reservation: open:")
	case strings.HasPrefix(d.carriedReservation, "reservation: expired:"):
		d.carryAsk("exception "+d.request.Carried+" has expired; its commit stays at HEAD", nil, d.exceptionGone())
	default:
		d.reserveCarry()
		d.stepOutput.Reset()
		if d.fetchOrigin(&d.stepOutput) != 0 {
			d.carryAsk("origin couldn't be fetched, so nothing was landed", d.usingException(), "once origin answers",
				d.stepOutput.String())
		}
		d.rebaseRecovered()
	}
}

// fieldAfter is the space-delimited value following marker in line.
func fieldAfter(line, marker string) (string, bool) {
	_, after, found := strings.Cut(line, marker)
	if !found {
		return "", false
	}
	value, _, _ := strings.Cut(after, " ")
	return value, true
}

func (d *driver) createCarriedIntent(commit string) {
	tree := d.mustGit("rev-parse", commit+"^{tree}")
	treeLine := d.mustTrailer(commit, "Carried-Tree")
	batteryLine := d.mustTrailer(commit, "Carried-Battery")
	judgeLine := d.mustTrailer(commit, "Carried-Judge")
	d.carriedPast = d.mustTrailer(commit, "Carried-Past")
	d.carriedBy = d.mustTrailer(commit, "Carried-By")
	d.carriedLedgerTip = d.mustTrailer(commit, "Carried-Ledger")
	workspace := strings.TrimPrefix(treeLine, "workspace=")
	workspace, _, _ = strings.Cut(workspace, " project=")
	d.carriedWorkspace = workspace
	battery, _, _ := strings.Cut(batteryLine, " ")
	missing, failing := "-", "-"
	if value, found := fieldAfter(batteryLine, " missing="); found {
		missing = value
	}
	if value, found := fieldAfter(batteryLine, " failing="); found {
		failing = value
	}
	judge, _, _ := strings.Cut(judgeLine, " ")
	digest, found := fieldAfter(judgeLine, " sha256=")
	if !found {
		digest = judge
	}
	request := CarryingRequest{Root: d.request.Root, Goal: d.request.Goal, Ref: d.request.Carried, Carrying: d.carriedRow,
		Commit: commit, Tree: tree, Workspace: workspace, Past: d.carriedPast, Battery: battery, Missing: missing, Failing: failing,
		Judge: judge, JudgeDigest: digest, Ledger: d.carriedLedgerTip, By: d.carriedBy, OwnerPID: d.owners.Getpid(),
		Lineage: d.request.OwnerLineage}
	if value, found := fieldAfter(judgeLine, " tree="); found {
		request.JudgeTree = value
	}
	if value, found := fieldAfter(judgeLine, " live-failure="); found {
		request.LiveFailure = value
	}
	output, status := d.owners.GoalCarrying(request)
	if status != 0 {
		d.stop(status, "the exception landing couldn't be recorded before its push, so nothing was pushed", d.usingException(), repeat, output)
		exitLanding(status)
	}
	entry := strings.TrimPrefix(strings.TrimRight(output, "\n"), "carrying=")
	entry, _, _ = strings.Cut(entry, " ledger=")
	if entry == "" {
		d.stop(1, "the exception landing couldn't be recorded before its push, so nothing was pushed", d.usingException(), repeat,
			"carried intent returned no entry")
		exitLanding(1)
	}
	d.carriedEntry = entry
}

var budgetExceptions = regexp.MustCompile(`(?m)^- BudgetExceptions: ([0-9]+)$`)

func (d *driver) printCarriedAdvisory(commit string) {
	judgeLine := d.mustTrailer(commit, "Carried-Judge")
	ordinary := d.mustTrailer(commit, "Landing-Provenance-Verdict")
	batteryLine := d.mustTrailer(commit, "Carried-Battery")
	liveFailure := "-"
	if value, found := fieldAfter(judgeLine, " live-failure="); found {
		liveFailure = value
	}
	sufficient, missing, failing := "false", "-", "-"
	if batteryLine == "green" {
		sufficient = "true"
	} else {
		if value, found := fieldAfter(batteryLine, " missing="); found {
			missing = value
		}
		if value, found := fieldAfter(batteryLine, " failing="); found {
			failing = value
		}
	}
	goalText := d.git("show", d.carriedLedgerTip+":./plans/goals/"+d.request.Goal+".md")
	if goalText.Code != 0 {
		d.stop(goalText.Code, "goal "+d.request.Goal+" can't be read from the goal records, so nothing was pushed", d.usingException(), repeat,
			string(goalText.Stderr))
		exitLanding(goalText.Code)
	}
	exceptions := 0
	if match := budgetExceptions.FindSubmatch(goalText.Stdout); match != nil {
		exceptions, _ = strconv.Atoi(string(match[1]))
	}
	finding := "carried:" + commit
	if batteryLine != "green" {
		finding += ":battery-red"
	}
	writeDetails(d.details, "carried reservation: "+d.carriedRow, "carried ledger: "+d.carriedLedgerTip,
		"carried judge: "+judgeLine, "carried live failure: "+liveFailure, "carried ordinary verdict: "+ordinary,
		fmt.Sprintf("carried testing result: sufficient=%s missing=%s failing=%s uncovered=- discrepancies=-", sufficient, missing, failing),
		"carried obligation finding: "+finding, fmt.Sprintf("carried exception count after this one: %d", exceptions+1))
}

func (d *driver) carriedStep(name string, request CarriedRequest) {
	request.Root, request.Lineage = d.request.Root, d.request.OwnerLineage
	d.requiredStep(name, func(out io.Writer) int {
		output, status := d.owners.GoalCarried(request)
		out.Write([]byte(output))
		return status
	})
}

func (d *driver) finishCarriedPublication(commit string) {
	d.requiredStep("goal held at the rebased base", d.heldCheck)
	d.createCarriedIntent(commit)
	d.printCarriedAdvisory(commit)
	d.seam("before-push")
	d.requiredStep("landing gate before push", d.gateBeforePush)
	d.sampleBoot()
	d.recordCarriedRelease(commit)
	if status := d.runStep("push carried commit to origin (single attempt)", d.pushOrigin); status != 0 {
		if d.movingOriginRejection() {
			d.carryAsk("main moved while this landed, so nothing was pushed", d.usingException(), "lands it on the new main",
				d.stepOutput.String())
		}
		d.failStep(status)
	}
	d.pushed(commit)
	d.carryAbandonArmed = false
	d.seam("after-push")
	d.carriedStep("complete carried goal record", CarriedRequest{Entry: d.carriedEntry})
	d.seam("after-record")
	d.releaseLanded(commit)
	if !d.request.SkipTransport {
		d.requiredStep("sync transport", d.syncTransport)
	}
}

// recordCarriedRelease records, before the carried commit's single push,
// the commit about to be pushed for the goal's release set (the exception
// route of disk-lifetimes Part B 3.6); a failure to record is reported and
// never stops the landing.
func (d *driver) recordCarriedRelease(commit string) {
	if d.owners.RecordRelease == nil || d.request.Goal == "" {
		return
	}
	if err := d.owners.RecordRelease(commit, d.branch); err != nil {
		writeDetails(d.details, fmt.Sprintf("the goal's workspaces were not recorded for release (%v); they stay until the goal ends or metasystem work workspace --release", err))
	}
}

func (d *driver) runCarried() {
	if d.runStep("fetch origin for carried landing", d.fetchOrigin) != 0 {
		d.carryAsk("origin couldn't be fetched, so nothing was landed", d.usingException(), "once origin answers", d.stepOutput.String())
	}
	d.goalFetchForCarry()
	d.readCarryStatus()
	switch {
	case strings.HasPrefix(d.carriedConsumption, "superseded:"):
		replacement := strings.TrimPrefix(d.carriedConsumption, "superseded:")
		d.carryAsk("exception "+d.request.Carried+" was replaced by "+replacement, []string{"metasystem", "work", "land", d.request.Goal, "--using-exception", replacement}, "")
	case strings.HasPrefix(d.carriedConsumption, "ledger:"):
		if d.carriedCounselor == "counselor: missing" {
			d.carriedStep("repair carried counselor record", CarriedRequest{RepairCounselor: true, Ref: d.request.Carried})
		}
		if !d.request.SkipTransport {
			d.requiredStep("sync transport", d.syncTransport)
		}
		writeDetails(d.details, "already recorded in the goal ledger as "+strings.TrimPrefix(d.carriedConsumption, "ledger:"))
		return
	case strings.HasPrefix(d.carriedConsumption, "origin:"):
		commit := strings.TrimPrefix(d.carriedConsumption, "origin:")
		writeDetails(d.details, "already landed as "+commit+"; completing the record")
		if entry, found := strings.CutPrefix(d.carriedIntent, "carrying:"); found {
			d.carriedEntry = entry
			d.carriedStep("complete carried goal record", CarriedRequest{Entry: entry})
		} else {
			d.carriedStep("rebuild carried goal record", CarriedRequest{Goal: d.request.Goal, Ref: d.request.Carried, RebuildFromCommit: commit})
		}
		if !d.request.SkipTransport {
			d.requiredStep("sync transport", d.syncTransport)
		}
		return
	case strings.HasPrefix(d.carriedConsumption, "local:"):
		d.rebaseRecovered()
		d.ensureRecoveryReservation()
		d.carryAbandonArmed = true
		commit := d.mustGit("rev-parse", "HEAD")
		d.verifyLocalCarriedCommit(commit)
		d.finishCarriedPublication(commit)
		return
	}
	switch d.carriedWord {
	case "ok":
	case "expired":
		d.carryAsk("exception "+d.request.Carried+" has expired, so nothing was landed", nil, d.exceptionGone())
	case "missing":
		d.carryAsk("goal "+d.request.Goal+" has no exception "+d.request.Carried+", so nothing was landed", []string{"metasystem", "goal", "sync"},
			"then repeat this command if the exception was recorded elsewhere")
	case "unproven":
		d.carryAsk("exception "+d.request.Carried+" wasn't recorded by a person at an enrolled terminal, so nothing was landed", nil,
			"record it again from your own terminal: metasystem work land "+d.request.Goal+" --exception CODE --reason R")
	default:
		d.carryAsk("exception "+d.request.Carried+" is in a state this landing doesn't know, so nothing was landed", nil,
			d.exceptionGone()+" (--verbose shows its state)", "carry word state: "+d.carriedWord)
	}
	if d.carriedConsumption != "none" {
		d.carryAsk("exception "+d.request.Carried+" is in a state this landing doesn't know, so nothing was landed", nil,
			d.exceptionGone()+" (--verbose shows its state)", "consumption state: "+d.carriedConsumption)
	}
	d.holdIndex()
	d.requiredStep("stage caller paths", d.stageChanges)
	d.requiredStep("receipt line for the landing", d.checkReceiptLine)
	d.carryForwardStaged("")
	d.seam("carry-forward")
	d.reserveCarry()
	d.seam("reservation")
	d.requiredStep("fetch origin after carry reservation", d.fetchOrigin)
	d.carryForwardStaged(d.carriedRow)
	d.seam("second-carry-forward")
	d.requiredStep("commit", d.commitChanges)
	commit := d.mustGit("rev-parse", "HEAD")
	d.verifyLocalCarriedCommit(commit)
	d.finishCarriedPublication(commit)
}
