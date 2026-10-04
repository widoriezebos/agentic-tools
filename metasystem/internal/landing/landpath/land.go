package landpath

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// LandRequest is one landing: the staged or named change, its message, and
// its declarations.
type LandRequest struct {
	// Root is the metasystem installation root the landing runs from.
	Root string
	// Delivered is the one plain sentence of what this landing delivers,
	// which Landed tells the channel once it is on main; empty tells
	// nothing.
	Delivered string
	// HeldEpoch is the lease epoch a caller that already holds the lease
	// passes to a carried landing ("human" or a claim epoch).
	HeldEpoch string
	// MessageFile names the commit message; Message is used instead when
	// MessageFile is "-" (the caller read it from standard input).
	MessageFile string
	Message     []byte
	StagedOnly  bool
	Pathspecs   []string

	AllowNewPlan, SkipTransport bool

	Chain, DirectFix, RevertOf string
	Goal                       string
	GoalSet                    bool
	RootJob, Tests             string
	TestReceipt                string
	Recertification            string
	Carried                    string

	OwnerLineage string
	// CommitOnly stops the landing after its commit: a change bound for the
	// landing lane is fetched, rebased, proved and pushed by the lane (U11b).
	CommitOnly bool
	// Stop, when set, receives what a landing that stopped tells the person:
	// the reason and the one command (stop.go).
	Stop *Stop
}

// landExit ends a landing with a status from anywhere below Land, as the
// former driver's exit did; Land recovers it after running its cleanup.
type landExit struct{ code int }

func exitLanding(code int) { panic(landExit{code}) }

// StopAtSeam ends the landing from an Owners.Seam with code, as a crash at
// that point would: the landing's exit cleanup runs and Land returns code.
func StopAtSeam(code int) { exitLanding(code) }

// Land runs one landing and returns the exit status the former driver
// returned. A landing that stops writes its two lines (the reason and the
// one command) to stderr and records them in request.Stop; the step log and
// every other background line go to details, which a command shows only with
// --verbose.
func Land(owners Owners, request LandRequest, details, stderr io.Writer) (status int) {
	// The driver still rebases and proves after its commit boundary returns.
	// Retain the warning until the whole landing has finished so its digest
	// cannot change the files being proved or pushed.
	recordDesign := owners.RecordDesign
	var design *landing.DesignObservation
	if recordDesign != nil {
		owners.RecordDesign = func(_, _ string, observed *landing.DesignObservation, _ io.Writer) {
			design = observed
		}
	}
	stop := request.Stop
	if stop == nil {
		stop = &Stop{}
	}
	d := &driver{owners: owners, request: request, details: details, stderr: stderr, stopped: stop}
	defer func() {
		if recovered := recover(); recovered != nil {
			exit, ok := recovered.(landExit)
			if !ok {
				panic(recovered)
			}
			status = exit.code
		}
		d.cleanup(status)
		if status == 0 && design != nil {
			recordDesign(request.Root, request.Goal, design, stderr)
		}
	}()
	return d.run()
}

type driver struct {
	owners  Owners
	request LandRequest
	// details is what only --verbose shows (the step log); stderr carries
	// the two lines of a stop.
	details, stderr io.Writer
	stopped         *Stop
	told            bool

	messageFile string
	ownedFile   string
	ownedDone   func()
	stepName    string
	stepOutput  bytes.Buffer
	branch      string
	gateWidth   string

	recertTarget, recertSourceRef, recertMergedRef string
	recertCandidateTree, recertCandidateCommit     string

	boot *BootSample

	// heldIndex is the index file as the landing found it, at heldIndexPath
	// under HEAD heldIndexHead, until the landing's commit (holdIndex).
	heldIndex, heldIndexPath, heldIndexHead string
	heldIndexArmed                          bool

	carriedLedgerTip, carriedRow, carriedEntry          string
	carriedBy, carriedPast, carriedWorkspace            string
	carriedWord, carriedConsumption, carriedReservation string
	carriedIntent, carriedCounselor, carriedSource      string
	carryAbandonArmed                                   bool
	carryStopReason                                     string
}

func (d *driver) git(args ...string) GitResult {
	return d.owners.Git(GitCall{Dir: d.request.Root, Args: args})
}

func (d *driver) gitOut(args ...string) (string, int) {
	result := d.git(args...)
	return strings.TrimSpace(string(result.Stdout)), result.Code
}

// gitTo runs git with both of its streams on out.
func (d *driver) gitTo(out io.Writer, args ...string) int {
	result := d.git(args...)
	out.Write(result.Stdout)
	out.Write(result.Stderr)
	return result.Code
}

func (d *driver) cleanup(status int) {
	if d.carryAbandonArmed && d.carriedRow != "" {
		why := d.carryStopReason
		if why == "" {
			why = fmt.Sprintf("carried landing exited before its push (status %d)", status)
		}
		if d.carriedEntry != "" {
			d.owners.GoalCarried(CarriedRequest{Root: d.request.Root, Entry: d.carriedEntry, Lineage: d.request.OwnerLineage})
		}
		output, code := d.owners.GoalCarrying(CarryingRequest{Root: d.request.Root, Goal: d.request.Goal,
			Abandon: d.carriedRow, Why: why, Lineage: d.request.OwnerLineage})
		if code != 0 {
			fmt.Fprintln(d.stderr, "the exception's hold on the goal couldn't be released either; --verbose shows why")
			writeDetails(d.details, "carried landing could not close its reservation: "+strings.TrimRight(output, "\n"))
		}
	}
	if status != 0 {
		d.restoreIndex()
	}
	if d.ownedFile != "" {
		d.owners.RemoveFile(d.ownedFile)
	}
	if d.ownedDone != nil {
		d.ownedDone()
	}
}

// stop records why the landing stops (the first cause wins), writes its
// background to details and tells the person its two lines once.
func (d *driver) stop(code int, reason string, run []string, then string, details ...string) int {
	writeDetails(d.details, details...)
	d.stopped.said(reason, run, then)
	d.tell()
	return code
}

// failed stops for an owner's error: reason in plain words, the error in
// details.
func (d *driver) failed(code int, reason string, err error) int {
	cause := ""
	if err != nil {
		cause = err.Error()
	}
	then := ""
	if reason == notHolderReason {
		then = notHolderThen
	}
	if d.stopped.Reason == "" {
		d.stopped.cause = cause
	}
	return d.stop(code, reason, nil, then, cause)
}

// tell writes the recorded stop to the person's stream, once.
func (d *driver) tell() {
	if d.told || d.stopped.Reason == "" {
		return
	}
	d.told = true
	writeStop(d.stderr, *d.stopped)
}

// runStep logs a step to details, captures everything it writes, and logs
// that it finished.
func (d *driver) runStep(name string, step func(out io.Writer) int) int {
	d.stepName = name
	fmt.Fprintf(d.details, "step: %s\n", name)
	d.stepOutput.Reset()
	status := step(&d.stepOutput)
	if status == 0 {
		fmt.Fprintln(d.details, "  ok")
	}
	return status
}

func lastLines(data []byte, count int) []byte {
	trimmed := bytes.TrimSuffix(data, []byte("\n"))
	if len(trimmed) == 0 {
		return nil
	}
	lines := bytes.Split(trimmed, []byte("\n"))
	if len(lines) > count {
		lines = lines[len(lines)-count:]
	}
	return append(bytes.Join(lines, []byte("\n")), '\n')
}

func (d *driver) lastStepLine() string {
	return strings.TrimSuffix(string(lastLines(d.stepOutput.Bytes(), 1)), "\n")
}

// failStep reports a failed step: its cause (the refusal a step recorded,
// else the step and its last line) to the person, its retained log to
// details; then it ends the landing.
func (d *driver) failStep(status int) {
	cause := firstNonEmpty(d.stopped.cause, d.stopped.Reason, d.lastStepLine())
	d.carryStopReason = fmt.Sprintf("step %s failed with exit %d: %s", d.stepName, status, cause)
	fmt.Fprintf(d.details, "step failed: %s (exit %d)\n", d.stepName, status)
	reference, err := d.owners.OutputSpill(d.request.Root, "land", "log", d.stepOutput.Bytes())
	d.details.Write(lastLines(d.stepOutput.Bytes(), 40))
	if err == nil {
		fmt.Fprintln(d.details, reference)
	} else {
		fmt.Fprintf(d.details, "full step log not retained: %v\n", err)
	}
	last := oneLine(d.lastStepLine())
	if last == "" {
		last = fmt.Sprintf("it exited %d", status)
	}
	d.stopped.said("the landing stopped at "+d.stepName+": "+last, nil, "fix what stopped it (--verbose shows the step's log), then repeat this command")
	d.tell()
	exitLanding(status)
}

func (d *driver) requiredStep(name string, step func(out io.Writer) int) {
	if status := d.runStep(name, step); status != 0 {
		d.failStep(status)
	}
}

// carryAsk ends a carried landing with what the person reads: why it
// stopped, and the one command or the words that resolve it.
func (d *driver) carryAsk(reason string, run []string, then string, details ...string) {
	d.carryStopReason = reason
	d.stop(3, reason, run, then, details...)
	exitLanding(3)
}

func (d *driver) seam(point string) {
	if d.owners.Seam != nil {
		d.owners.Seam(point)
	}
}

// Usage is the landing's usage line, a detail of a refusal of its options.
const Usage = "Usage: metasystem work land [G] --message FILE (--staged | PATH...) [--chain J [--recertification R --test-receipt P] [--direct-fix register-carriage] | --direct-fix register-carriage | --direct-fix exact-revert --revert-of C | --direct-fix tier-1 --root-job J (--test-receipt P | --tests CMD)] [--allow-new-plan] [--skip-transport]"

// optionStop refuses options that do not combine; the usage is a detail.
func (d *driver) optionStop(reason, then string) int {
	return d.stop(2, reason, nil, then, Usage)
}

func (d *driver) run() int {
	request := d.request
	if request.Carried != "" && request.HeldEpoch == "" {
		epoch, err := d.owners.RequireHolder(request.Root, d.owners.CallerPID, nil)
		if err != nil {
			return d.failed(1, notHolderReason, err)
		}
		held := "human"
		if epoch != nil {
			held = strconv.FormatInt(*epoch, 10)
		}
		status := 0
		if err := d.owners.WithHeld(request.Root, d.owners.CallerPID, epoch, func() error {
			d.request.HeldEpoch = held
			status = d.held()
			return nil
		}); err != nil {
			return d.failed(1, notHolderReason, err)
		}
		return status
	}
	return d.held()
}

func (d *driver) held() int {
	request := d.request
	if request.HeldEpoch != "" {
		if value, err := strconv.ParseInt(request.HeldEpoch, 10, 64); err == nil && value > 0 && request.HeldEpoch[0] != '0' {
			if _, err := d.owners.RequireHolder(request.Root, d.owners.CallerPID, &value); err != nil {
				return d.failed(1, notHolderReason, err)
			}
		} else {
			if request.HeldEpoch != "human" {
				return 2
			}
			if _, err := d.owners.RequireHolder(request.Root, d.owners.CallerPID, nil); err != nil {
				return d.failed(1, notHolderReason, err)
			}
		}
	}
	detail, err := d.owners.BrainFence(request.Root, "land")
	if err != nil {
		return d.failed(1, "this checkout's role couldn't be read, so nothing was landed", err)
	}
	if detail != "" {
		return d.stop(2, brainReason, nil, brainThen, detail)
	}
	if status := d.validate(); status != 0 {
		return status
	}
	d.messageFile = request.MessageFile
	if request.MessageFile == "-" {
		file, done, err := diskstore.ScratchFile("metasystem-land-message.")
		if err != nil {
			return d.failed(1, "the commit message couldn't be saved for the commit, so nothing was landed", err)
		}
		d.ownedFile, d.ownedDone = file.Name(), done
		_, writeErr := file.Write(request.Message)
		closeErr := file.Close()
		if writeErr != nil || closeErr != nil {
			return d.stop(1, "the commit message couldn't be saved for the commit, so nothing was landed", nil, "", fmt.Sprint(writeErr, closeErr))
		}
		d.messageFile = d.ownedFile
	} else if !d.owners.FileReadable(request.MessageFile) {
		return d.stop(2, "the commit message file can't be read, so nothing was landed", nil,
			"check the file named by --message, then repeat this command", "message file: "+request.MessageFile)
	}
	return d.land()
}

var chainIdentifier = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

func (d *driver) validate() int {
	request := d.request
	if request.MessageFile == "" {
		return d.optionStop("a landing needs its commit message, so nothing was landed", "name the message file with --message FILE")
	}
	if request.StagedOnly && len(request.Pathspecs) > 0 {
		return d.optionStop("--staged lands what is staged, so it takes no --path, so nothing was landed", "use --staged or --path, not both")
	}
	if !request.StagedOnly && len(request.Pathspecs) == 0 && request.Carried == "" {
		return d.optionStop("nothing to land: no --path and no --staged, so nothing was landed", "name the files with --path, or land what is staged with --staged")
	}
	if request.Tests != "" && request.TestReceipt != "" {
		return d.optionStop("--tests and --test-receipt don't go together, so nothing was landed", "drop --test-receipt for a tier-1 landing, or --tests for a receipted chain")
	}
	if request.TestReceipt != "" && request.Chain == "" && request.DirectFix != "tier-1" && request.Carried == "" {
		return d.optionStop("--test-receipt goes with --chain or --direct-fix tier-1, so nothing was landed", "add --chain J, or drop --test-receipt")
	}
	if request.Carried != "" && (!request.GoalSet || request.Goal == "") {
		return d.optionStop("an exception landing names its goal, so nothing was landed", "name the goal: metasystem work land G ...")
	}
	if request.Recertification != "" && request.Chain == "" {
		return d.optionStop("--recertification goes with --chain, so nothing was landed", "add --chain J with the chain's root job")
	}
	if request.Recertification != "" && request.DirectFix != "" && request.DirectFix != "register-carriage" {
		return d.optionStop("--recertification combines only with --direct-fix register-carriage, so nothing was landed", "drop --direct-fix, or use register-carriage")
	}
	if request.Recertification != "" && request.TestReceipt == "" {
		return d.optionStop("a recertified landing needs a fresh test receipt for this exact change, so nothing was landed", "add --test-receipt PATH")
	}
	if request.DirectFix == "tier-1" {
		if !request.GoalSet || request.Goal == "" || request.RootJob == "" || request.Tests == "" && request.TestReceipt == "" {
			return d.optionStop("--direct-fix tier-1 needs a goal, --root-job, and --test-receipt or --tests, so nothing was landed",
				"add the missing one: metasystem work land G --direct-fix tier-1 --root-job J --test-receipt PATH ...")
		}
	} else if request.RootJob != "" || request.Tests != "" {
		return d.optionStop("--root-job and --tests go only with --direct-fix tier-1, so nothing was landed", "drop them, or add --direct-fix tier-1")
	}
	if request.Chain != "" && chainIdentifier.MatchString(request.Chain) {
		d.gateWidth = d.owners.JobGateWidth(request.Root, request.Chain)
		if d.gateWidth == "full" && request.TestReceipt == "" {
			testRun := []string{"metasystem", "test", "run"}
			if request.GoalSet && request.Goal != "" {
				testRun = append(testRun, "--goal", request.Goal)
			}
			return d.stop(2, "chain "+request.Chain+" needs its full test run's receipt, so nothing was landed", testRun,
				"then pass its receipt with --test-receipt PATH", "the chain's gate width is full: it lands only with sufficient schema-2 testing evidence")
		}
	}
	return 0
}

func (d *driver) land() int {
	request := d.request
	d.requiredStep("verify checks", d.verifyChecks)
	d.requiredStep("load recertification transport facts", d.loadRecertificationFacts)
	if request.Recertification != "" {
		d.requiredStep("fetch origin before recertified commit", d.fetchOrigin)
		if d.runStep("freeze recertified target before commit", d.checkRecertificationTarget) != 0 {
			d.parkRecertified("chain-recertification-target-moved", d.lastStepLine())
		}
	}
	if request.Carried != "" {
		d.runCarried()
		return 0
	}
	d.holdIndex()
	d.requiredStep("stage caller paths", d.stageChanges)
	d.requiredStep("receipt line for the landing", d.checkReceiptLine)
	if request.TestReceipt != "" {
		if request.Recertification != "" {
			if d.runStep("test receipt for staged candidate", d.checkSuppliedTestReceipt) != 0 {
				reason := "chain-recertification-test-command-refused"
				if d.gateWidth == "full" {
					reason = "chain-full-gate-refused"
				}
				d.parkRecertified(reason, d.lastStepLine())
			}
		} else {
			d.requiredStep("test receipt for staged candidate", d.checkSuppliedTestReceipt)
		}
	}
	d.requiredStep("tier-1 test receipt", d.createTestReceipt)
	if request.Recertification != "" {
		tree, status := d.gitOut("write-tree")
		if status != 0 {
			d.failStep(status)
		}
		d.recertCandidateTree = tree
		if d.runStep("recheck recertified target before commit", d.checkRecertificationTarget) != 0 {
			d.parkRecertified("chain-recertification-target-moved", d.lastStepLine())
		}
		if status := d.runStep("commit", d.commitChanges); status != 0 {
			if refusal := d.recertificationRefusal(); refusal != "" {
				d.parkRecertified(refusal, d.lastStepLine())
			}
			d.failStep(status)
		}
		if d.runStep("verify recertified commit parent and tree", d.verifyRecertifiedCommit) != 0 {
			d.parkRecertified("chain-recertification-target-moved", d.lastStepLine())
		}
		if d.runStep("verify clean after commit", d.requireCleanAfterCommit) != 0 {
			d.parkRecertified("chain-recertification-source-changed", d.lastStepLine())
		}
		if d.runStep("goal held at the rebased base", d.heldCheck) != 0 {
			detail := d.lastStepLine()
			writeDetails(d.details, detail)
			d.parkRecertified("chain-recertification-target-moved", detail)
		}
		d.details.Write(d.stepOutput.Bytes())
		if d.runStep("verify shared testing proof before recertified push", d.verifyCurrentTestingProof) != 0 {
			d.parkRecertified("chain-recertification-test-command-refused", d.lastStepLine())
		}
		d.requiredStep("landing gate before push", d.gateBeforePush)
		d.sampleBoot()
		d.recordRelease()
		if status := d.runStep("push recertified commit to origin (single attempt)", d.pushOrigin); status != 0 {
			if d.movingOriginRejection() {
				detail := d.lastStepLine()
				d.details.Write(d.stepOutput.Bytes())
				d.parkRecertified("chain-recertification-target-moved", detail)
			}
			d.failStep(status)
		}
		head, _ := d.gitOut("rev-parse", "HEAD")
		d.pushed(head)
		d.releaseLanded(head)
	} else {
		d.requiredStep("commit", d.commitChanges)
		d.requiredStep("verify clean after commit", d.requireCleanAfterCommit)
		if request.CommitOnly {
			// The landing lane fetches, rebases, proves and pushes it (U11b).
			return 0
		}
		d.requiredStep("fetch origin", d.fetchOrigin)
		d.requiredStep("rebase onto origin/"+d.branch, d.rebaseOrigin)
		d.requiredStep("goal held at the rebased base", d.heldCheck)
		d.details.Write(d.stepOutput.Bytes())
		d.requiredStep("verify shared testing proof after rebase", d.verifyCurrentTestingProof)
		const pushLimit = 3
		d.sampleBoot()
		for attempt := 1; attempt <= pushLimit; attempt++ {
			d.requiredStep("landing gate before push", d.gateBeforePush)
			d.recordRelease()
			status := d.runStep(fmt.Sprintf("push origin (attempt %d of %d)", attempt, pushLimit), d.pushOrigin)
			if status == 0 {
				break
			}
			if !d.movingOriginRejection() || attempt == pushLimit {
				d.failStep(status)
			}
			fmt.Fprintf(d.details, "retryable rejection: %s (exit %d)\n", d.stepName, status)
			d.details.Write(lastLines(d.stepOutput.Bytes(), 40))
			fmt.Fprintf(d.details, "origin moved during push; fetching and rebasing before retry %d of %d\n", attempt+1, pushLimit)
			d.requiredStep(fmt.Sprintf("fetch origin after push attempt %d", attempt), d.fetchOrigin)
			d.requiredStep(fmt.Sprintf("rebase onto origin/%s after push attempt %d", d.branch, attempt), d.rebaseOrigin)
			d.requiredStep("goal held at the rebased base", d.heldCheck)
			d.details.Write(d.stepOutput.Bytes())
			d.requiredStep("verify shared testing proof after retry rebase", d.verifyCurrentTestingProof)
		}
		head, _ := d.gitOut("rev-parse", "HEAD")
		d.pushed(head)
		d.releaseLanded(head)
	}
	if !request.SkipTransport {
		d.requiredStep("sync transport", d.syncTransport)
	}
	return 0
}

// recordRelease records the goal's release set for the commit about to be
// pushed (the staged route of disk-lifetimes Part B 3.6).
func (d *driver) recordRelease() {
	if d.owners.RecordRelease == nil || d.request.Goal == "" {
		return
	}
	head, status := d.gitOut("rev-parse", "HEAD")
	if status != 0 {
		return
	}
	if err := d.owners.RecordRelease(head, d.branch); err != nil {
		writeDetails(d.details, fmt.Sprintf("the goal's workspaces were not recorded for release (%v); they stay until the goal ends or metasystem work workspace --release", err))
	}
}

// releaseLanded releases the pushed commit's recorded set.
func (d *driver) releaseLanded(head string) {
	if d.owners.ReleaseLanded != nil && d.request.Goal != "" && head != "" {
		d.owners.ReleaseLanded(head)
	}
}

func (d *driver) syncTransport(out io.Writer) int {
	return d.owners.SyncTransport(d.request.Root, d.branch, out, out)
}

func (d *driver) sampleBoot() {
	d.boot = nil
	if sample, err := d.owners.BootClock(); err == nil {
		d.boot = &sample
	}
}

// pushed is the one point where the landing path knows its commit is on
// origin: the goal's waiters are hinted, and a push to main tells the
// channel the request's plain sentence of what it delivered (Decision 7 of
// the blocked-agent-asks-the-human design); no sentence tells nothing, and
// a failed telling is a detail and stops nothing.
func (d *driver) pushed(commit string) {
	d.hintWaiters(commit)
	if d.owners.Landed == nil || d.branch != "main" || commit == "" || strings.TrimSpace(d.request.Delivered) == "" {
		return
	}
	if err := d.owners.Landed(d.request.Root, d.request.Delivered, commit); err != nil {
		writeDetails(d.details, "the channel was not told of the landing; the next landing or tick retries once: "+err.Error())
	}
}

func (d *driver) hintWaiters(publication string) {
	if d.request.Goal == "" {
		return
	}
	d.owners.NotifyGoal(d.request.Root, d.request.Goal, publication, d.boot)
}

var rulingRow = regexp.MustCompile(`^([-+])\| (R-[0-9]+[a-z]?) \|`)

// checkRulingsIDMints refuses a new rulings row whose id is not
// machine-suffixed; a rewritten historical row keeps its id.
func (d *driver) checkRulingsIDMints(out io.Writer) int {
	changed := d.git("diff", "--cached", "--name-only", "--")
	if changed.Code != 0 {
		return changed.Code
	}
	var offending []string
	seen := map[string]bool{}
	for _, path := range strings.Split(strings.TrimRight(string(changed.Stdout), "\n"), "\n") {
		if path != "metasystem/memory/rulings.md" && path != "memory/rulings.md" {
			continue
		}
		diff := d.git("diff", "--cached", "--no-ext-diff", "--no-textconv", "--", path)
		if diff.Code != 0 {
			return diff.Code
		}
		lines := strings.Split(string(diff.Stdout), "\n")
		removed := map[string]bool{}
		for _, line := range lines {
			if match := rulingRow.FindStringSubmatch(line); match != nil && match[1] == "-" {
				removed[match[2]] = true
			}
		}
		for _, line := range lines {
			match := rulingRow.FindStringSubmatch(line)
			if match == nil || match[1] != "+" || removed[match[2]] || seen[match[2]] {
				continue
			}
			seen[match[2]] = true
			offending = append(offending, match[2])
		}
	}
	if len(offending) > 0 {
		lines := []string{"new rulings ids must be machine-suffixed (R-<n>-<machine>); a rewritten historical row keeps its id:"}
		for _, id := range offending {
			lines = append(lines, "  "+id)
		}
		return d.stop(2, "new rulings rows need the machine in their id (R-123-m1e); "+firstPath(offending)+" has none", nil,
			"add the machine to the new ids in memory/rulings.md, then repeat this command", lines...)
	}
	return 0
}

func (d *driver) verifyChecks(out io.Writer) int {
	branch, status := d.gitOut("symbolic-ref", "--quiet", "--short", "HEAD")
	if status != 0 {
		return d.stop(2, "this checkout isn't on a branch, so nothing was landed", []string{"git", "switch", "main"}, repeat)
	}
	d.branch = branch
	if d.request.Carried != "" && branch != "main" {
		return d.stop(3, "an exception lands main, and this checkout is on "+branch, []string{"git", "switch", "main"}, repeat)
	}
	if status := d.checkRulingsIDMints(out); status != 0 {
		return status
	}
	if d.request.StagedOnly {
		return d.gitTo(out, "diff", "--cached", "--check", "--")
	}
	if d.git("diff", "--cached", "--quiet", "--").Code != 0 {
		return d.stop(2, "other files are already staged, so landing the named paths would take them along", nil,
			"land what is staged with --staged, or unstage it first (git restore --staged .)")
	}
	return d.gitTo(out, append([]string{"diff", "--check", "--"}, d.request.Pathspecs...)...)
}

func (d *driver) stageChanges(out io.Writer) int {
	if !d.request.StagedOnly {
		if len(d.request.Pathspecs) == 0 {
			return d.optionStop("nothing to land: no --path and no --staged, so nothing was landed", "name the files with --path, or land what is staged with --staged")
		}
		if status := d.gitTo(out, append([]string{"add", "--"}, d.request.Pathspecs...)...); status != 0 {
			return status
		}
	}
	if d.git("diff", "--cached", "--quiet", "--").Code == 0 {
		return d.stop(2, "nothing is staged, so there is nothing to land", nil, "stage the change (git add), then repeat this command")
	}
	var drift bytes.Buffer
	status := d.owners.Drift(d.request.Root, false, &drift, &drift)
	if status == 1 {
		var lines, paths []string
		for _, line := range strings.Split(strings.TrimRight(drift.String(), "\n"), "\n") {
			lines = append(lines, "  "+line)
			if fields := strings.Split(line, "\t"); len(fields) == 3 {
				paths = append(paths, fields[2])
			}
		}
		if len(paths) == 0 {
			paths = []string{"a file"}
		}
		kind := "untracked"
		if regexp.MustCompile(`(?m)^(unstaged|register-not-append)\t`).Match(drift.Bytes()) {
			kind = "unstaged"
		}
		return d.stop(2, "other changed files would stay behind uncommitted: "+firstPath(paths), []string{"git", "status", "--short"},
			"commit, stash or discard them, then repeat this command",
			append([]string{kind + " changes remain after staging; the landing needs a clean checkout after its commit:"}, lines...)...)
	}
	return status
}

func (d *driver) stagedProjectTree() (string, int) { return d.gitOut("write-tree") }

func (d *driver) stagedCandidateTree() (string, int) {
	tree, status := d.gitOut("write-tree")
	if status != 0 {
		return "", status
	}
	prefix, status := d.gitOut("rev-parse", "--show-prefix")
	if status != 0 {
		return "", status
	}
	if prefix != "" {
		return d.gitOut("rev-parse", tree+":"+strings.TrimSuffix(prefix, "/"))
	}
	return tree, 0
}

// checkReceiptLine relays the engine's decision whether the landing appends
// the RECEIPT line for its goal.
func (d *driver) checkReceiptLine(out io.Writer) int {
	tree, status := d.stagedProjectTree()
	if status != 0 {
		return status
	}
	goal := ""
	if d.request.GoalSet {
		goal = d.request.Goal
	}
	decision, err := d.owners.ReceiptLine(d.request.Root, tree, goal, d.request.DirectFix)
	if err != nil {
		return d.failed(1, "the receipt record couldn't be read, so nothing was landed", err)
	}
	if decision.Refused {
		detail := decision.Detail
		if detail == "" {
			detail = "the landing appends no RECEIPT line"
		}
		// Path mode staged the set itself; give the index back so the
		// retry is the same command with the ledger path added.
		if !d.request.StagedOnly && len(d.request.Pathspecs) > 0 {
			d.git(append([]string{"reset", "-q", "--"}, d.request.Pathspecs...)...)
		}
		if decision.Removed {
			return d.stop(2, "the change deletes the receipt record, so nothing was landed", nil,
				"restore the record and unstage its deletion, then repeat this command", detail)
		}
		then := "add the goal's receipt line, stage the record, then repeat this command"
		if decision.Command != "" {
			then = "add the line with " + decision.Command + ", stage the record, then repeat this command"
		}
		return d.stop(2, "the change touches code but adds no receipt line for it, so nothing was landed", nil, then, detail)
	}
	fmt.Fprintln(out, decision.Encoded)
	return 0
}

func (d *driver) checkSuppliedTestReceipt(out io.Writer) int {
	path := d.request.TestReceipt
	var receipt map[string]json.RawMessage
	var schema int
	readable := false
	if data, err := d.owners.ReadFile(path); err == nil && json.Unmarshal(data, &receipt) == nil {
		readable = true
		json.Unmarshal(receipt["schemaVersion"], &schema)
	}
	_, workspacePresent := receipt["workspace"]
	workspacePresent = workspacePresent && schema == 2
	var candidate string
	var status int
	if schema == 2 {
		candidate, status = d.stagedProjectTree()
	} else {
		candidate, status = d.stagedCandidateTree()
	}
	if status != 0 {
		return status
	}
	failure := ""
	var tree string
	switch {
	case !d.owners.FileExists(path):
		failure = "missing"
	case !d.owners.FileReadable(path):
		failure = "unreadable"
	case !readable || json.Unmarshal(receipt["tree"], &tree) != nil:
		failure = "no tree field"
	}
	testRun := []string{"metasystem", "test", "run"}
	if d.request.GoalSet && d.request.Goal != "" {
		testRun = append(testRun, "--goal", d.request.Goal)
	}
	if failure != "" {
		return d.stop(2, "the test receipt can't be read ("+failure+"), so nothing was landed", testRun,
			"makes a new one; pass it with --test-receipt, then repeat this command", "receipt: "+path)
	}
	if tree == candidate {
		return 0
	}
	mismatch := fmt.Sprintf("the receipt at %s names tree %s but the staged candidate is %s", path, tree, candidate)
	stale := "the test receipt is for other files than the staged change, so nothing was landed"
	if !workspacePresent {
		return d.stop(2, stale, testRun, "makes one for this change; pass it with --test-receipt, then repeat this command", mismatch)
	}
	receiptWorkspace, receiptErr := d.owners.Live().Workspace(d.request.Root, tree)
	candidateWorkspace, candidateErr := d.owners.Live().Workspace(d.request.Root, candidate)
	if receiptErr != nil {
		writeDetails(d.details, receiptErr.Error())
	} else if candidateErr != nil {
		writeDetails(d.details, candidateErr.Error())
	} else if receiptWorkspace == candidateWorkspace {
		return 0
	}
	var verify bytes.Buffer
	goal := ""
	if d.request.GoalSet {
		goal = d.request.Goal
	}
	if d.owners.Verify(VerifyRequest{Root: d.request.Root, Tree: candidate, Goal: goal}, &verify, &verify) == 0 {
		return 0
	}
	return d.stop(2, stale, testRun, "makes one for this change; pass it with --test-receipt, then repeat this command",
		mismatch, string(lastLines(verify.Bytes(), 20)))
}

func (d *driver) createTestReceipt(out io.Writer) int {
	if d.request.Tests == "" {
		return 0
	}
	tree, status := d.stagedCandidateTree()
	if status != 0 {
		return status
	}
	if status := d.owners.TestReceipt(d.request.Root, tree, d.request.Tests, out, out); status != 0 {
		return status
	}
	d.request.TestReceipt = filepath.Join(d.request.Root, "artifacts", "agents", "landing", "receipts", tree+".json")
	return 0
}

func (d *driver) commitRequest() CommitRequest {
	request := d.request
	commit := CommitRequest{Root: request.Root, Chain: request.Chain, DirectFix: request.DirectFix, RevertOf: request.RevertOf,
		Goal: request.Goal, GoalSet: request.GoalSet, RootJob: request.RootJob, TestReceipt: request.TestReceipt,
		Recertification: request.Recertification, MessageFile: d.messageFile, OwnerLineage: request.OwnerLineage,
		AllowNewPlan: request.AllowNewPlan, LaneJoin: request.CommitOnly, Stop: d.stopped}
	if request.Carried != "" {
		commit.HeldEpoch = request.HeldEpoch
		commit.Carried, commit.LedgerTip, commit.CarriedBy, commit.CarriedPast = request.Carried, d.carriedLedgerTip, d.carriedBy, d.carriedPast
	}
	return commit
}

func (d *driver) commitChanges(out io.Writer) int {
	status := Commit(d.owners, d.commitRequest(), out, out)
	if status == 0 {
		d.heldIndexArmed = false
	}
	return status
}

// holdIndex records the index file as the landing finds it, before it
// stages anything, so a landing that refuses or fails before its commit
// gives the index back exactly (restoreIndex); the retry then starts from
// the index its caller left.
func (d *driver) holdIndex() {
	path, status := d.gitOut("rev-parse", "--git-path", "index")
	if status != 0 || path == "" {
		return
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(d.request.Root, path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	head, _ := d.gitOut("rev-parse", "--verify", "--quiet", "HEAD")
	d.heldIndex, d.heldIndexPath, d.heldIndexHead, d.heldIndexArmed = string(data), path, head, true
}

// restoreIndex puts the held index back when the landing ends before its
// commit. It writes only the index, under git's own index.lock, and never
// the working tree; a HEAD the landing moved keeps the index it has.
func (d *driver) restoreIndex() {
	if !d.heldIndexArmed {
		return
	}
	d.heldIndexArmed = false
	if head, _ := d.gitOut("rev-parse", "--verify", "--quiet", "HEAD"); head != d.heldIndexHead {
		return
	}
	if data, err := os.ReadFile(d.heldIndexPath); err == nil && string(data) == d.heldIndex {
		return
	}
	lock := d.heldIndexPath + ".lock"
	file, err := os.OpenFile(lock, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		fmt.Fprintln(d.stderr, indexNotRestored)
		writeDetails(d.details, fmt.Sprintf("the index could not be given back as the landing found it: %v", err))
		return
	}
	_, writeErr := file.WriteString(d.heldIndex)
	closeErr := file.Close()
	if writeErr == nil && closeErr == nil {
		writeErr = os.Rename(lock, d.heldIndexPath)
	}
	if writeErr != nil || closeErr != nil {
		os.Remove(lock)
		fmt.Fprintln(d.stderr, indexNotRestored)
		writeDetails(d.details, fmt.Sprintf("the index could not be given back as the landing found it: %v %v", writeErr, closeErr))
		return
	}
	writeDetails(d.details, "the staged files are as the landing found them")
}

// indexNotRestored is the line a person reads when the staged files could
// not be put back after a landing that stopped.
const indexNotRestored = "the staged files couldn't be put back as they were; check them with git status before repeating"

func (d *driver) requireCleanAfterCommit(out io.Writer) int {
	var drift bytes.Buffer
	status := d.owners.Drift(d.request.Root, true, &drift, &drift)
	if status == 1 {
		return d.stop(1, "committed, but other files are still changed on disk, so nothing was pushed", []string{"git", "status", "--short"},
			"commit, stash or discard them, then repeat this command", drift.String())
	}
	return status
}

func (d *driver) fetchOrigin(out io.Writer) int {
	return d.gitTo(out, "fetch", "--quiet", "origin", "+refs/heads/"+d.branch+":refs/remotes/origin/"+d.branch)
}

func (d *driver) rebaseOrigin(out io.Writer) int {
	return d.owners.Advance(d.request.Root, "refs/remotes/origin/"+d.branch, out, out)
}

func (d *driver) heldCheck(out io.Writer) int {
	base := "refs/remotes/origin/" + d.branch
	if d.request.Recertification != "" {
		base = d.recertTarget
	}
	return d.owners.Held(d.request.Root, base, "HEAD", "origin", "refs/heads/"+d.branch, out, out)
}

// gateBeforePush reads the goal's landing gate once more, immediately before
// a push (g1-s70 D2): admission is not the last word, so a hold or a changed
// human word that arrived while the landing proved, or while origin moved,
// stops the publication. A landing in no goal's name has no gate.
func (d *driver) gateBeforePush(out io.Writer) int {
	if d.request.Goal == "" || d.owners.LandingGate == nil {
		return 0
	}
	if err := d.owners.LandingGate(d.request.Root, d.request.Goal); err != nil {
		fmt.Fprintf(out, "%v\n", err)
		var gate *GateRefusal
		if errors.As(err, &gate) {
			if d.stopped.Reason == "" {
				d.stopped.cause = firstNonEmpty(gate.Cause, gate.Reason)
			}
			return d.stop(1, gate.Reason+", so nothing was pushed", gate.Run, gate.Then, gate.Details...)
		}
		d.stopped.cause = firstNonEmpty(d.stopped.cause, err.Error())
		return d.stop(1, "the goal may not land right now, so nothing was pushed", nil,
			"wait until it may (--verbose shows why), then repeat this command", err.Error())
	}
	return 0
}

func (d *driver) pushOrigin(out io.Writer) int {
	env := append(d.owners.Environ(), "LC_ALL=C")
	result := d.owners.Git(GitCall{Dir: d.request.Root, Env: env,
		Args: []string{"push", "--porcelain", "origin", "refs/heads/" + d.branch + ":refs/heads/" + d.branch}})
	out.Write(result.Stdout)
	out.Write(result.Stderr)
	return result.Code
}

var movingOrigin = regexp.MustCompile(`\[rejected\].*\((non-fast-forward|fetch first)\)|non-fast-forward|fetch first|cannot lock ref .*is at .*but expected`)

func (d *driver) movingOriginRejection() bool { return movingOrigin.Match(d.stepOutput.Bytes()) }

func (d *driver) loadRecertificationFacts(out io.Writer) int {
	if d.request.Recertification == "" {
		return 0
	}
	top, status := d.gitOut("rev-parse", "--show-toplevel")
	if status != 0 {
		return status
	}
	if strings.HasPrefix(d.request.Recertification, "/") {
		return d.optionStop("--recertification takes the record's path inside the repository, not an absolute path, so nothing was landed",
			"name it relative to the repository root")
	}
	var record struct {
		TargetCommit    string `json:"targetCommit"`
		SourceAnchorRef string `json:"sourceAnchorRef"`
		MergedAnchorRef string `json:"mergedAnchorRef"`
	}
	if data, err := d.owners.ReadFile(filepath.Join(top, d.request.Recertification)); err == nil {
		json.Unmarshal(data, &record)
	}
	d.recertTarget, d.recertSourceRef, d.recertMergedRef = record.TargetCommit, record.SourceAnchorRef, record.MergedAnchorRef
	if d.recertTarget == "" {
		// An explicitly selected but unreadable proof is still parked after
		// the evaluator names its refusal; the frozen local target is the
		// only target identity available for that record.
		head, status := d.gitOut("rev-parse", "HEAD^{commit}")
		if status != 0 {
			return status
		}
		d.recertTarget = head
	}
	return 0
}

func (d *driver) parkRecertified(reason, detail string) {
	request := ParkRequest{Root: d.request.Root, Chain: d.request.Chain, Target: d.recertTarget, Reason: reason, Detail: detail,
		Recertification: d.request.Recertification, CandidateCommit: d.recertCandidateCommit, CallerPID: d.owners.Getpid()}
	for _, ref := range []string{d.recertSourceRef, d.recertMergedRef} {
		if ref != "" {
			request.RecoveryRefs = append(request.RecoveryRefs, ref)
		}
	}
	output, err := d.owners.Park(request)
	if err == nil {
		writeDetails(d.details, "PARKED", "cause: "+reason, detail, output)
		d.stopped.Reason = ""
		d.stop(1, "chain "+d.request.Chain+" was set aside: "+recertificationCause(reason), nil,
			"land the chain again once that is fixed (--verbose shows the parked record)")
		exitLanding(1)
	}
	writeDetails(d.details, "PARK-FAILED cause="+reason, detail, output)
	d.stopped.Reason = ""
	d.stop(1, "chain "+d.request.Chain+" stopped ("+recertificationCause(reason)+") and couldn't be set aside either", nil,
		"fix the cause (--verbose shows it), then land the chain again")
	exitLanding(1)
}

// recertificationCause is a recertified landing's park reason in plain
// words.
func recertificationCause(reason string) string {
	switch reason {
	case "chain-recertification-target-moved":
		return "main moved while it was being landed"
	case "chain-recertification-test-command-refused", "chain-full-gate-refused":
		return "its tests haven't passed for this exact change"
	case "chain-recertification-source-changed":
		return "its files changed after the commit"
	}
	return "the landing check refused it"
}

func (d *driver) checkRecertificationTarget(out io.Writer) int {
	local, status := d.gitOut("rev-parse", "HEAD^{commit}")
	if status != 0 {
		return status
	}
	remote, status := d.gitOut("rev-parse", "refs/remotes/origin/"+d.branch+"^{commit}")
	if status != 0 {
		return status
	}
	if local != d.recertTarget || remote != d.recertTarget {
		fmt.Fprintf(out, "main moved: local=%s remote=%s expected=%s\n", local, remote, d.recertTarget)
		return 1
	}
	return 0
}

var recertificationCode = regexp.MustCompile(`.*code=([a-z][a-z0-9-]*)`)

func (d *driver) recertificationRefusal() string {
	for _, line := range strings.Split(d.stepOutput.String(), "\n") {
		if match := recertificationCode.FindStringSubmatch(line); match != nil {
			return match[1]
		}
	}
	return ""
}

func (d *driver) verifyRecertifiedCommit(out io.Writer) int {
	current, status := d.gitOut("rev-parse", "HEAD^{commit}")
	if status != 0 {
		return status
	}
	parent, status := d.gitOut("rev-parse", "HEAD^1")
	if status != 0 {
		return status
	}
	tree, status := d.gitOut("rev-parse", "HEAD^{tree}")
	if status != 0 {
		return status
	}
	if parent != d.recertTarget || tree != d.recertCandidateTree {
		fmt.Fprintf(out, "main moved: committed parent/tree %s/%s differ from %s/%s\n", parent, tree, d.recertTarget, d.recertCandidateTree)
		return 1
	}
	d.recertCandidateCommit = current
	return 0
}

func (d *driver) verifyCurrentTestingProof(out io.Writer) int {
	tree, status := d.gitOut("rev-parse", "HEAD^{tree}")
	if status != 0 {
		return status
	}
	goal := ""
	if d.request.GoalSet {
		goal = d.request.Goal
	}
	return d.owners.Verify(VerifyRequest{Root: d.request.Root, Tree: tree, Goal: goal}, out, out)
}
