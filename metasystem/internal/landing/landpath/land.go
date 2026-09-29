package landpath

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
	"io"
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
}

// landExit ends a landing with a status from anywhere below Land, as the
// former driver's exit did; Land recovers it after running its cleanup.
type landExit struct{ code int }

func exitLanding(code int) { panic(landExit{code}) }

// StopAtSeam ends the landing from an Owners.Seam with code, as a crash at
// that point would: the landing's exit cleanup runs and Land returns code.
func StopAtSeam(code int) { exitLanding(code) }

// Land runs one landing and returns the exit status the former driver
// returned. Progress goes to stdout; refusals and failed steps to stderr.
func Land(owners Owners, request LandRequest, stdout, stderr io.Writer) (status int) {
	d := &driver{owners: owners, request: request, stdout: stdout, stderr: stderr}
	defer func() {
		if recovered := recover(); recovered != nil {
			exit, ok := recovered.(landExit)
			if !ok {
				panic(recovered)
			}
			status = exit.code
		}
		d.cleanup(status)
	}()
	return d.run()
}

type driver struct {
	owners         Owners
	request        LandRequest
	stdout, stderr io.Writer

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
			fmt.Fprintf(d.stderr, "carried landing could not close its reservation: %s\n", strings.TrimRight(output, "\n"))
		}
	}
	if d.ownedFile != "" {
		d.owners.RemoveFile(d.ownedFile)
	}
	if d.ownedDone != nil {
		d.ownedDone()
	}
}

func (d *driver) refuse(code int, format string, args ...any) int {
	fmt.Fprintf(d.stderr, format+"\n", args...)
	return code
}

// runStep announces a step, captures everything it writes, and reports ok.
func (d *driver) runStep(name string, step func(out io.Writer) int) int {
	d.stepName = name
	fmt.Fprintf(d.stdout, "== STEP: %s\n", name)
	d.stepOutput.Reset()
	status := step(&d.stepOutput)
	if status == 0 {
		fmt.Fprintln(d.stdout, "-- ok")
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

// failStep reports a failed step with its retained log and ends the landing.
func (d *driver) failStep(status int) {
	d.carryStopReason = fmt.Sprintf("step %s failed with exit %d: %s", d.stepName, status, d.lastStepLine())
	fmt.Fprintf(d.stderr, "!! STEP FAILED: %s (exit %d)\n", d.stepName, status)
	reference, err := d.owners.OutputSpill(d.request.Root, "land", "log", d.stepOutput.Bytes())
	d.stderr.Write(lastLines(d.stepOutput.Bytes(), 40))
	if err == nil {
		fmt.Fprintln(d.stderr, reference)
	} else {
		fmt.Fprintf(d.stderr, "land: full step log not retained: %v\n", err)
	}
	exitLanding(status)
}

func (d *driver) requiredStep(name string, step func(out io.Writer) int) {
	if status := d.runStep(name, step); status != 0 {
		d.failStep(status)
	}
}

// carryAsk ends a carried landing with the exact ask a person reads.
func (d *driver) carryAsk(ask string) {
	d.carryStopReason = ask
	fmt.Fprintln(d.stderr, ask)
	exitLanding(3)
}

func (d *driver) seam(point string) {
	if d.owners.Seam != nil {
		d.owners.Seam(point)
	}
}

// Usage is the landing's usage line.
const Usage = "Usage: metasystem work land [G] --message FILE (--staged | PATH...) [--chain J [--recertification R --test-receipt P] [--direct-fix register-carriage] | --direct-fix register-carriage | --direct-fix exact-revert --revert-of C | --direct-fix tier-1 --root-job J (--test-receipt P | --tests CMD)] [--allow-new-plan] [--skip-transport]"

func (d *driver) usage(code int) int {
	fmt.Fprintln(d.stderr, Usage)
	return code
}

func (d *driver) run() int {
	request := d.request
	if request.Carried != "" && request.HeldEpoch == "" {
		epoch, err := d.owners.RequireHolder(request.Root, d.owners.CallerPID, nil)
		if err != nil {
			fmt.Fprintln(d.stderr, err)
			return 1
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
			fmt.Fprintln(d.stderr, err)
			return 1
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
				fmt.Fprintln(d.stderr, err)
				return 1
			}
		} else {
			if request.HeldEpoch != "human" {
				return 2
			}
			if _, err := d.owners.RequireHolder(request.Root, d.owners.CallerPID, nil); err != nil {
				fmt.Fprintln(d.stderr, err)
				return 1
			}
		}
	}
	detail, err := d.owners.BrainFence(request.Root, "land")
	if err != nil {
		return d.refuse(1, "land refused: brain fence failed")
	}
	if detail != "" {
		return d.refuse(2, "%s", detail)
	}
	if status := d.validate(); status != 0 {
		return status
	}
	d.messageFile = request.MessageFile
	if request.MessageFile == "-" {
		file, done, err := diskstore.ScratchFile("metasystem-land-message.")
		if err != nil {
			fmt.Fprintln(d.stderr, err)
			return 1
		}
		d.ownedFile, d.ownedDone = file.Name(), done
		_, writeErr := file.Write(request.Message)
		closeErr := file.Close()
		if writeErr != nil || closeErr != nil {
			fmt.Fprintln(d.stderr, writeErr, closeErr)
			return 1
		}
		d.messageFile = d.ownedFile
	} else if !d.owners.FileReadable(request.MessageFile) {
		return d.refuse(2, "land refused: commit message file is not readable: %s", request.MessageFile)
	}
	return d.land()
}

var chainIdentifier = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

func (d *driver) validate() int {
	request := d.request
	if request.MessageFile == "" {
		return d.usage(2)
	}
	if request.StagedOnly && len(request.Pathspecs) > 0 {
		return d.refuse(2, "land refused: --staged cannot be combined with paths")
	}
	if !request.StagedOnly && len(request.Pathspecs) == 0 && request.Carried == "" {
		return d.refuse(2, "land refused: name paths or choose --staged")
	}
	if request.Tests != "" && request.TestReceipt != "" {
		fmt.Fprintln(d.stderr, "land refused: --tests and --test-receipt cannot be combined; remove --test-receipt for a tier-1 landing, or remove --tests for a receipted chain landing")
		return d.usage(2)
	}
	if request.TestReceipt != "" && request.Chain == "" && request.DirectFix != "tier-1" && request.Carried == "" {
		fmt.Fprintln(d.stderr, "land refused: --test-receipt belongs with --chain or --direct-fix tier-1")
		return d.usage(2)
	}
	if request.Carried != "" && (!request.GoalSet || request.Goal == "") {
		return d.refuse(2, "land refused: --carried requires --goal")
	}
	if request.Recertification != "" && request.Chain == "" {
		return d.refuse(2, "land refused: --recertification requires --chain <root-job>")
	}
	if request.Recertification != "" && request.DirectFix != "" && request.DirectFix != "register-carriage" {
		return d.refuse(2, "land refused: --recertification combines only with the existing register-carriage class")
	}
	if request.Recertification != "" && request.TestReceipt == "" {
		return d.refuse(2, "land refused: a recertified landing requires a fresh --test-receipt for the actual candidate")
	}
	if request.DirectFix == "tier-1" {
		if !request.GoalSet || request.Goal == "" || request.RootJob == "" || request.Tests == "" && request.TestReceipt == "" {
			return d.refuse(2, "land refused: --direct-fix tier-1 requires --goal, --root-job, and either --test-receipt or legacy --tests")
		}
	} else if request.RootJob != "" || request.Tests != "" {
		return d.refuse(2, "land refused: --root-job and --tests belong only to --direct-fix tier-1")
	}
	if request.Chain != "" && chainIdentifier.MatchString(request.Chain) {
		d.gateWidth = d.owners.JobGateWidth(request.Root, request.Chain)
		if d.gateWidth == "full" && request.TestReceipt == "" {
			return d.refuse(2, "land refused: chain %s requires sufficient schema-2 testing evidence; run metasystem test run --goal <goal> and pass the receipt with --test-receipt", request.Chain)
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
			fmt.Fprintln(d.stderr, detail)
			d.parkRecertified("chain-recertification-target-moved", detail)
		}
		d.stdout.Write(d.stepOutput.Bytes())
		if d.runStep("verify shared testing proof before recertified push", d.verifyCurrentTestingProof) != 0 {
			d.parkRecertified("chain-recertification-test-command-refused", d.lastStepLine())
		}
		d.requiredStep("landing gate before push", d.gateBeforePush)
		d.sampleBoot()
		if status := d.runStep("push recertified commit to origin (single attempt)", d.pushOrigin); status != 0 {
			if d.movingOriginRejection() {
				detail := d.lastStepLine()
				d.stderr.Write(d.stepOutput.Bytes())
				d.parkRecertified("chain-recertification-target-moved", detail)
			}
			d.failStep(status)
		}
		head, _ := d.gitOut("rev-parse", "HEAD")
		d.hintWaiters(head)
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
		d.stdout.Write(d.stepOutput.Bytes())
		d.requiredStep("verify shared testing proof after rebase", d.verifyCurrentTestingProof)
		const pushLimit = 3
		d.sampleBoot()
		for attempt := 1; attempt <= pushLimit; attempt++ {
			d.requiredStep("landing gate before push", d.gateBeforePush)
			status := d.runStep(fmt.Sprintf("push origin (attempt %d of %d)", attempt, pushLimit), d.pushOrigin)
			if status == 0 {
				break
			}
			if !d.movingOriginRejection() || attempt == pushLimit {
				d.failStep(status)
			}
			fmt.Fprintf(d.stdout, "-- retryable rejection: %s (exit %d)\n", d.stepName, status)
			d.stdout.Write(lastLines(d.stepOutput.Bytes(), 40))
			fmt.Fprintf(d.stdout, "-- origin moved during push; fetching and rebasing before retry %d of %d\n", attempt+1, pushLimit)
			d.requiredStep(fmt.Sprintf("fetch origin after push attempt %d", attempt), d.fetchOrigin)
			d.requiredStep(fmt.Sprintf("rebase onto origin/%s after push attempt %d", d.branch, attempt), d.rebaseOrigin)
			d.requiredStep("goal held at the rebased base", d.heldCheck)
			d.stdout.Write(d.stepOutput.Bytes())
			d.requiredStep("verify shared testing proof after retry rebase", d.verifyCurrentTestingProof)
		}
		head, _ := d.gitOut("rev-parse", "HEAD")
		d.hintWaiters(head)
	}
	if !request.SkipTransport {
		d.requiredStep("sync transport", d.syncTransport)
	}
	return 0
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
		fmt.Fprintln(out, "land refused: new rulings ids must be machine-suffixed (R-<n>-<machine>); see the register header (a rewritten historical row must keep its id; only new mints need the suffix)")
		for _, id := range offending {
			fmt.Fprintf(out, "  %s\n", id)
		}
		return 2
	}
	return 0
}

func (d *driver) verifyChecks(out io.Writer) int {
	branch, status := d.gitOut("symbolic-ref", "--quiet", "--short", "HEAD")
	if status != 0 {
		fmt.Fprintln(out, "land refused: HEAD is not on a branch")
		return 2
	}
	d.branch = branch
	if d.request.Carried != "" && branch != "main" {
		fmt.Fprintf(out, "carry asks: the carried landing lands main; you are on %s\n", branch)
		return 3
	}
	if status := d.checkRulingsIDMints(out); status != 0 {
		return status
	}
	if d.request.StagedOnly {
		return d.gitTo(out, "diff", "--cached", "--check", "--")
	}
	if d.git("diff", "--cached", "--quiet", "--").Code != 0 {
		fmt.Fprintln(out, "land refused: path mode requires an empty index; use --staged for an existing staged set")
		return 2
	}
	return d.gitTo(out, append([]string{"diff", "--check", "--"}, d.request.Pathspecs...)...)
}

func (d *driver) stageChanges(out io.Writer) int {
	if !d.request.StagedOnly {
		if len(d.request.Pathspecs) == 0 {
			fmt.Fprintln(out, "land refused: name paths or choose --staged")
			return 2
		}
		if status := d.gitTo(out, append([]string{"add", "--"}, d.request.Pathspecs...)...); status != 0 {
			return status
		}
	}
	if d.git("diff", "--cached", "--quiet", "--").Code == 0 {
		fmt.Fprintln(out, "land refused: the caller-selected staging set is empty")
		return 2
	}
	var drift bytes.Buffer
	status := d.owners.Drift(d.request.Root, false, &drift, &drift)
	if status == 1 {
		if regexp.MustCompile(`(?m)^(unstaged|register-not-append)\t`).Match(drift.Bytes()) {
			fmt.Fprintln(out, "land refused: unstaged changes remain after staging; transport requires a clean tree after commit")
		} else {
			fmt.Fprintln(out, "land refused: untracked paths remain after staging; transport requires a clean tree after commit")
		}
		for _, line := range strings.Split(strings.TrimRight(drift.String(), "\n"), "\n") {
			fmt.Fprintf(out, "  %s\n", line)
		}
		return 2
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
		fmt.Fprintf(out, "landing receipt-line: %v\n", err)
		return 1
	}
	if decision.Refused {
		detail := decision.Detail
		if detail == "" {
			detail = "the landing appends no RECEIPT line"
		}
		fmt.Fprintf(out, "land refused: %s\n", detail)
		// Path mode staged the set itself; give the index back so the
		// retry is the same command with the ledger path added.
		if !d.request.StagedOnly && len(d.request.Pathspecs) > 0 {
			d.git(append([]string{"reset", "-q", "--"}, d.request.Pathspecs...)...)
		}
		return 2
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
	if failure != "" {
		fmt.Fprintf(out, "land refused: the receipt at %s cannot be read as a landing receipt (%s)\n", path, failure)
		return 2
	}
	if tree == candidate {
		return 0
	}
	mismatch := fmt.Sprintf("land refused: the receipt at %s names tree %s but the staged candidate is %s; make the receipt against this exact candidate", path, tree, candidate)
	if !workspacePresent {
		fmt.Fprintln(out, mismatch)
		return 2
	}
	receiptWorkspace, receiptErr := d.owners.Live().Workspace(d.request.Root, tree)
	candidateWorkspace, candidateErr := d.owners.Live().Workspace(d.request.Root, candidate)
	if receiptErr != nil {
		fmt.Fprintln(out, receiptErr)
	} else if candidateErr != nil {
		fmt.Fprintln(out, candidateErr)
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
	fmt.Fprintln(out, mismatch)
	out.Write(lastLines(verify.Bytes(), 20))
	return 2
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
		AllowNewPlan: request.AllowNewPlan}
	if request.Carried != "" {
		commit.HeldEpoch = request.HeldEpoch
		commit.Carried, commit.LedgerTip, commit.CarriedBy, commit.CarriedPast = request.Carried, d.carriedLedgerTip, d.carriedBy, d.carriedPast
	}
	return commit
}

func (d *driver) commitChanges(out io.Writer) int {
	return Commit(d.owners, d.commitRequest(), out, out)
}

func (d *driver) requireCleanAfterCommit(out io.Writer) int {
	var drift bytes.Buffer
	status := d.owners.Drift(d.request.Root, true, &drift, &drift)
	if status == 1 {
		fmt.Fprintln(out, "land refused: commit succeeded but the tree is not clean, so transport will not start")
		out.Write(drift.Bytes())
		return 1
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
		fmt.Fprintf(out, "%v; nothing was pushed\n", err)
		return 1
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
		fmt.Fprintln(out, "land refused: --recertification must be the canonical repository-relative path")
		return 2
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
		fmt.Fprintln(d.stdout, "PARKED")
		fmt.Fprintln(d.stdout, strings.TrimRight(output, "\n"))
		exitLanding(1)
	}
	fmt.Fprintf(d.stderr, "PARK-FAILED cause=%s\n", reason)
	fmt.Fprintln(d.stderr, strings.TrimRight(output, "\n"))
	exitLanding(1)
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
		fmt.Fprintf(out, "chain-recertification-target-moved: local=%s remote=%s expected=%s\n", local, remote, d.recertTarget)
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
		fmt.Fprintf(out, "chain-recertification-target-moved: committed parent/tree %s/%s differ from %s/%s\n", parent, tree, d.recertTarget, d.recertCandidateTree)
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
