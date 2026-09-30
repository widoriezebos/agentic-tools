package main

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/authority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/board"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/ownercall"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batchowner"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/landpath"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/project"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/realpath"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/validate"
)

// The delivery commands review a subject, fold a review's accepted findings
// into a follow-up, close a finished chain and land a goal. Each one resolves
// the named subject to its recorded evidence and hands it to the existing
// owner: the delegate boundary for critic dispatch and follow-ups, the goal
// branch read for a unit commit, the delegate lifecycle for the whole chain close, and
// the batch join or the hand land-prep and land-push for landing. The owners
// keep their authority, proof and retry identity; these commands never judge
// a finding, certify a standalone result or conclude a goal.

func intentDeliveryCommands() []intentCommand {
	goalFlag := intentFlag{name: "goal", value: "G", usage: "with --commit, or feedback on changes: the goal the subject serves"}
	return []intentCommand{
		{
			object: "work", action: "review", primary: true, audience: "both", summary: "independently review a goal's built work, a job, a run or a commit",
			usage: []string{reviewGoalUsage, reviewSubmitUsage, reviewFindingUsage,
				reviewJobUsage, reviewCommitUsage, reviewRunUsage, reviewChangesUsage, reviewDiffUsage, reviewCheckUsage},
			helpForms: reviewHelpForms(),
			details: []string{
				"work review G records the goal's built result as the candidate and requests its independent examination: the one named with --work,",
				"or the only work item whose result is built. Repeat the same command to see the examination's progress and findings.",
				"An examination with no findings completes by itself: the review closes and its read is collected and published.",
				"Findings stop at the author's decision: the review writes a decisions file bound to this exact examination; decide",
				"each finding (accepted, refuted, out-of-scope or noted) and run work review G --dispositions FILE. An accepted material",
				"finding needs a fix first: work revise G --after N --brief FILE --dispositions FILE, then review the new attempt.",
				"An examination round that fails without findings is retried with work review G --retry N (its round number): the same",
				"review chain examines the same subject once more under its round limit; repeating it rejoins that retry.",
				"work review G --finding F --test NAME discharges a finding's obligation with the test that proves it resolved; the",
				"examination is inferred when the goal has exactly one, else named with --review R. The session holding G",
				"discharges in its own name; anyone else's discharge is a person's act.",
				"G --changes or G --patch PATCH submits work a person or agent wrote as the goal's work (named --work, default main): claims the goal,",
				"commits the change on the goal branch in the goal's worktree, publishes it and requests the same independent examination built work",
				"gets; decisions, closing, collection and landing continue exactly as for built work. --changes takes this checkout's tracked and",
				"untracked changes; system records and the named brief stay behind. From the goal's own worktree the edits are committed in place.",
				"Repeating the command rejoins the committed version. A correction names the version it replaces: --after COMMIT (from status).",
				"j2:J: a completed implementer job is reviewed by the code-critic lane; its goal and subject come from the job record.",
				"work review j2:J --dispositions FILE records your decisions on that review's findings and completes the review.",
				"--commit SHA --goal G reads one committed version of a goal's work; with --dispositions FILE it decides its findings, closes and collects.",
				"Without a goal, --changes or --patch PATCH asks independent readers for feedback only: no goal review, approval or landing follows,",
				"and the checkout is not changed. The result's read:REF drives work status, work wait and work stop; the same request rejoins its reads,",
				"and --retry N asks for one new attempt after attempt N failed or was stopped.",
				"run:RUN commits the newest round on its goal branch, replacing an earlier round commit, and requests independent review; --model names its critic.",
				"A design is reviewed with design review FILE.",
				"--check-only asks no critic: j2:J --stage review|recertify|merge checks the job's review boundary, and --findings RETURN",
				"--dispositions FILE checks that every finding of a round is decided (against the chain's register when j2:ROOT names it).",
			},
			flags: []intentFlag{
				goalFlag,
				{name: "work", value: "NAME", usage: "with G: the goal's named work to review"},
				{name: "changes", usage: "with G: submit this checkout's current changes as the goal's work; without G: feedback on them"},
				{name: "patch", value: "PATCH", usage: "with G: submit this patch file as the goal's work; without G: feedback on it"},
				{name: "commit", value: "SHA", usage: "review one committed version of a goal's work, with --goal"},
				{name: "dispositions", value: "FILE", usage: "the author's decisions, in the bound file the review wrote"},
				{name: "retry", value: "N", usage: "examine the subject once more after examination N failed without findings"},
				{name: "after", value: "COMMIT", usage: "with --changes or --patch: the version (commit) being corrected"},
				{name: "finding", value: "F", usage: "with G: discharge this finding's obligation with --test (or the fixture proof)"},
				{name: "test", value: "NAME", usage: "with --finding: the test that proves the finding resolved"},
				{name: "review", aliases: []string{"chain"}, value: "R", advanced: true, usage: "with --finding: the examination, when the goal has more than one"},
				{name: "implementation-chain", value: "J", advanced: true, usage: "with --finding, a fixture obligation: the implementation chain carrying the fix"},
				{name: "artifact", value: "PATH", advanced: true, usage: "with --finding, a fixture obligation: the changed artifact"},
				{name: "result", value: "RUN", advanced: true, usage: "with --finding, a fixture obligation: the retained governed test result"},
				{name: "critic", value: "ROOT", advanced: true, usage: "with --finding, a fixture obligation: the clean code-critic root"},
				intentByFlag, intentLineageFlag, intentFixtureFlag,
				{name: "brief", value: "FILE", usage: "a submission's, feedback request's or commit review's brief"},
				{name: "tool-calls", value: "N", usage: "job review: the reader's maximum tool calls, stated in its brief"},
				{name: "model", value: "MODEL", usage: "run and commit review: the critic model, subject to roster authorization; kept with the read"},
				{name: "effort", value: "VALUE", hidden: true, usage: "refused: every review's reasoning effort is set by its hazard class's configuration obligations"},
				{name: "check-only", usage: "check without asking a critic: j2:J's boundary at --stage, or a round's --findings against --dispositions"},
				{name: "stage", value: "STAGE", advanced: true, usage: "with --check-only and j2:J: review, recertify or merge"},
				{name: "test-command", value: "COMMAND", advanced: true, usage: "with --check-only --stage recertify: the explicit recertification test command"},
				{name: "recertification", value: "RECORD", advanced: true, usage: "with --check-only --stage merge: the recertification proof of the same job"},
				{name: "findings", value: "RETURN", advanced: true, usage: "with --check-only: the critic return whose findings must all be decided in --dispositions"},
			},
			maxArgs: 1,
			accepts: []string{refGoal, refJ2, refRun},
			examples: []string{"metasystem work review verbs-match-intent", "metasystem work review verbs-match-intent --work discovery", "metasystem work review j2:impl-01 --tool-calls 40",
				"metasystem work review --commit 3f2a9c1 --goal verbs-match-intent"},
			run: runIntentReview,
		},
		{
			object: "work", action: "revise", audience: "agent", summary: "correct a goal's work with a brief: one new attempt, reviewed again",
			usage: []string{"metasystem work revise G [--work NAME] [--after N] --brief FILE [--dispositions FILE]", "metasystem work revise j2:R --dispositions FILE --brief FILE", "metasystem work revise run:RUN --brief FILE"},
			details: []string{
				"Every attempt of a work item has a number N, whether it passed or failed; --after N names the attempt being corrected.",
				"The request (work, N and the brief's exact bytes) is kept before anything starts, so repeating it, even after a lost",
				"response or a failure, reaches the same new attempt and never spends another. Without --after an identical earlier",
				"request is rejoined first; otherwise the newest attempt is corrected. A request against an older attempt is refused.",
				"When the new attempt fails, the same brief with --after of that attempt's number deliberately makes one more attempt.",
				"Without --work: the only work item that failed, else the only one that finished. The result must be reviewed again.", sentBackReviseDetail,
				"--dispositions is the decisions file work review G wrote for an examination of attempt R (R may be before N). Its findings",
				"and decisions are frozen with the request and stay the builder's input across failed corrections; a file answering an",
				"examination that a later attempt's completed examination superseded is refused.",
				"work revise j2:R continues a finished job review's implementer chain with the author's decisions on every finding.",
				"work revise run:RUN corrects the run work build returned: the run's own plan and proof build one follow-up round under the round limit",
				"the run started with, read it again and await judgement; work review run:RUN then reviews the new round afresh.",
			},
			flags: []intentFlag{
				{name: "work", value: "NAME", usage: "the goal's named work to correct"},
				{name: "after", value: "N", usage: "the attempt being corrected (default: rejoin the same request, else the newest attempt)"},
				intentBriefFlag,
				{name: "dispositions", value: "FILE", usage: "the bound decisions file of the reviewed examination"},
			},
			maxArgs:  1,
			accepts:  []string{refGoal, refJ2, refRun},
			examples: []string{"metasystem work revise verbs-match-intent --brief fix.md", "metasystem work revise j2:crit-01 --dispositions r1-dispositions.md --brief r1-fix.md", "metasystem work revise verbs-match-intent --work discovery --after 2 --brief fix.md"},
			run:      runIntentReviseSentBack,
		},
		{
			object: "work", action: "land", primary: true, audience: "both", summary: "land a goal's reviewed work",
			usage: []string{"metasystem work land G [--through COMMIT]", "metasystem work land G --queue-only", "metasystem work land j2:J",
				"metasystem work land G --exception CODE --reason TEXT --by NAME [--expires 2h] [--replace-exception ID [--transfer]]",
				"metasystem work land G --using-exception ID",
				"metasystem work land [G] --message FILE (--staged | --path P...) [--chain J | --direct-fix CLASS ...]"},
			administrationUsage: []string{"metasystem work land G --exception CODE --reason TEXT --by NAME --upgrade-goals [--expires 2h] [--replace-exception ID [--transfer]]"},
			details: []string{
				"--queue-only marks the held goal built and waiting to land, and nothing else: no proof runs, no read is collected and nothing is pushed.",
				"--exception is a person's explicit act, never implied by work land G: the goal's whole landing candidate is computed, the",
				"exception past exactly one refusal code (or one group:NAME) is recorded with the enrolled person's proof, and the carried",
				"landing delivers it from this checkout's main. Repeating the request rejoins the recorded exception; a changed candidate",
				"needs --replace-exception. --using-exception ID lands under an exception already recorded, locally or through the channel.",
				"The claim leaves the one-claim quota and its elapsed fence until it lands; each machine has one landing slot.",
				"With landing.batch-root configured, the goal branch (or the certified chain) joins the landing batch, which proves and pushes it.",
				"Without it, the read-clean goal branch is proved on its landing candidate, prepared and pushed by hand.",
				"With the batch configured, a selection holding a unit read from a reader record is refused with the critic read that admits it",
				"(metasystem work review --commit SHA --goal G); it lands by hand only when G is the fix goal of an open trunk red on main",
				"(metasystem incident list names it; metasystem incident claim E --goal G).",
				"Missing reads, proof or approval refuse with the missing input; no other route is tried instead.",
				"A repeat reuses the retained receipt and prepared landing; a moved endpoint starts from a new proof. The goal is not concluded: that stays goal done G.",
				"--message lands a hand-made change instead: the named paths (or the staged set) are staged and committed through the commit",
				"boundary with the landing's declarations. With a landing lane the commit is pinned and joins the lane as a change member",
				"(change:<first 12 of the commit>): it rides on the proof of the goal members it joins, a batch of changes alone is proved on",
				"the lane's account, and it lands with a Landing-Change trailer; the same command reads its membership until it lands, and",
				"an ejected change is given back: its commit is undone with its changes kept, to fix and land again.",
				"Without a lane, and with --local or --recertification, it is rebased onto origin,",
				"proved against retained delivery proof, and pushed by hand.",
			},
			flags: append([]intentFlag{
				{name: "through", value: "COMMIT", usage: "land a human-approved prefix ending at this unit commit"},
				{name: "queue-only", usage: "only mark the held goal waiting to land, for a later work land G"},
				{name: "exception", value: "CODE", advanced: true, usage: "a person's exception: the one refusal code or group:NAME this landing is carried past"},
				{name: "reason", value: "TEXT", advanced: true, usage: "with --exception: why"},
				{name: "by", value: "NAME", advanced: true, usage: "with --exception: the person deciding, at the enrolled terminal"},
				{name: "expires", value: "DURATION", advanced: true, usage: "with --exception: how long it stays usable (default 2h, at most 4h)"},
				{name: "replace-exception", value: "ID", advanced: true, usage: "with --exception: the unused exception this one replaces"},
				{name: "transfer", advanced: true, usage: "with --replace-exception: take over an exception recorded on another seat"},
				{name: "upgrade-goals", advanced: true, usage: "with --exception: raise the goal ledger's format in the same act"},
				{name: "using-exception", value: "ID", advanced: true, usage: "land under this already recorded exception (local or answered through the channel)"},
				intentLineageFlag,
			}, stagedLandingFlags...),
			maxArgs:  1,
			accepts:  []string{refGoal, refJ2},
			examples: []string{"metasystem work land verbs-match-intent", "metasystem work land verbs-match-intent --queue-only", "metasystem work land verbs-match-intent --through 3f2a9c1", "metasystem work land j2:impl-01"},
			run:      runIntentLand,
		},
		{
			object: "work", action: "finish", audience: "both", summary: "record a finished job with nothing to review or land as complete",
			usage: []string{"metasystem work finish j2:J", "metasystem work finish j2:J --evidence R", "metasystem work finish j2:J --dispositions FILE"},
			details: []string{
				"For a job whose result is findings, not code: an investigation, or a read that is not reviewed further. Use the reference work status printed.",
				"The job's authority, its members' results and their durability are checked before its records are completed. It concludes no goal,",
				"lands nothing and grants no approval. A reviewed job completes inside work review, and landed work inside work land.",
				"--evidence R reconciles read R's evidence into the job before it completes; a review chain names its author's --dispositions.",
				"An already completed job is reported unchanged.",
			},
			flags: []intentFlag{
				{name: "evidence", value: "R", usage: "a read whose evidence is reconciled into the job before it completes"},
				{name: "dispositions", value: "FILE", advanced: true, usage: "a review chain's Markdown dispositions table"},
			},
			maxArgs:  1,
			accepts:  []string{refJ2},
			examples: []string{"metasystem work finish j2:inv-01", "metasystem work finish j2:crit-01 --evidence crit-02"},
			run:      runIntentWorkFinish,
		},
	}
}

// intentProcess is one owner subprocess and what it returned.
type intentProcess struct {
	argv []string
	dir  string
}

type intentProcessResult struct {
	stdout, stderr []byte
	code           int
	err            error
}

// intentDeliveryOwners are the owners the delivery commands call. Production
// uses defaultIntentDeliveryOwners; tests give each invocation its own.
type intentDeliveryOwners struct {
	// recordWriter asks the record-writer authority owner whether this
	// engine may write the named chain's records, before anything writes.
	recordWriter func(root, job string) (cause string, err error)
	process      func(intentProcess) intentProcessResult
	// landCarried runs one carried landing through the landing path, which
	// reads gate immediately before its push and records and runs release
	// around it, and returns what it printed and its exit status.
	landCarried func(request landpath.LandRequest, gate func(root, goalID string) error, release carriedRelease) intentProcessResult
	// closeOwner runs the delegate lifecycle's close command (the whole
	// chain close) for an installation root.
	closeOwner  func(root string, args []string) intentProcessResult
	executable  func() (string, error)
	branchRead  func([]string) (branch.BranchReadResult, int, error)
	branchState func(root, goalID string) (intentBranchState, error)
	landPrep    func([]string) (goalBranchLandPrepOutcome, int, error)
	landPush    func([]string) (branch.PreparedLanding, string, int, error)
	// landCandidate composes the pending landing and returns the candidate
	// tree its receipt must prove.
	landCandidate func([]string) (goalBranchLandPrepOutcome, int, error)
	sweep         func(root, goalID, landing string) error
	// batchUnit finds the batch member a land request names; branchTip is the
	// live goal branch, or empty once the branch is gone.
	batchUnit   func(landingRoot string, request batchowner.BatchJoinRequest, branchTip string) (batch.Record, batch.Unit, bool, error)
	publishRead func(root, goalID, unit string) (branch.PublishReadResult, error)
	batchRoot   func(root string, now time.Time) (string, bool, error)
	// boardView reads the host board for a one-shot view of the checkout,
	// checking its cards against the goal ledger at ledgerRoot (the state
	// root); nil reads the host this command runs on.
	boardView func(ledgerRoot string, now time.Time) board.View
	batchJoin func(batchowner.BatchJoinRequest) (batch.Record, error)
	now       func() time.Time
	// landingGate evaluates the landing gate for a goal at a branch tip
	// against a fresh ledger (g1-s70 D2); nil selects the production gate.
	landingGate func(inv *intentInvocation, goalID, tip string) (string, error)
	// branchTip reads a goal branch's tip at origin; nil reads origin.
	branchTip func(root, goalID string) (string, error)
	// redOnMain names the open red-on-main entry whose fix goal is goalID,
	// or "" when the goal fixes none; nil reads the synced ledger.
	redOnMain func(inv *intentInvocation, goalID string) (string, error)
	// recordLanded writes the holder's landed line after a confirmed
	// publication; nil selects the ledger's own act.
	recordLanded func(inv *intentInvocation, goalID string) error
	foldUnitHook func(inv *intentInvocation, run string) int
	// calls are the owner functions public commands call in this process
	// (intent_owner_calls.go); nil selects the production owners.
	calls *intentOwnerCalls
	// rebind carries the goal's review-round limit onto the critic
	// registers a review of a chain continues; nil selects the dispatch
	// owner.
	rebind func(root, job string) (map[string]string, error)
	// landPath runs one landing through the landing path; nil runs
	// landpath.Land.
	landPath func(landpath.Owners, landpath.LandRequest, io.Writer, io.Writer) int
	// changeHeld runs held over a change's one commit on the seat; nil runs
	// the landing path's held owner (U11b).
	changeHeld func(root, base, commit, branch string) (string, int)
	// changeJoin joins a change the seat committed to the landing lane; nil
	// runs the production join.
	changeJoin func(batchowner.ChangeJoinRequest) (batch.Record, error)
	// changeUnit reads a change's membership in the lane's batches; nil reads
	// the lane checkout's store, and any unreadable record is an error.
	changeUnit func(landingRoot, id string) (batch.Record, batch.Unit, bool, error)
	// changeAdvance moves the seat's branch onto origin after its change
	// landed; nil fetches and advances.
	changeAdvance func(root, branch string) error
}

// intentBranchState is the goal branch as the landing owners read it.
type intentBranchState struct {
	EndpointTip, BranchTip string
	Status                 branch.Status
	// Sources is each read unit's attestation source kind, in branch order.
	Sources []string
}

// carriedRelease is the exception route's release set around the carried
// push (disk-lifetimes Part B 3.6): record names the commit about to be
// pushed, landed releases the set once the push succeeded; nil records and
// releases nothing.
type carriedRelease struct {
	record func(commit, branch string) error
	landed func(commit string)
}

// apply gives the landing path the release owners.
func (r carriedRelease) apply(owners *landpath.Owners) {
	owners.RecordRelease, owners.ReleaseLanded = r.record, r.landed
}

// landCarriedInProcess runs one carried landing in this process.
func landCarriedInProcess(request landpath.LandRequest, gate func(root, goalID string) error, release carriedRelease) intentProcessResult {
	owners := landingPathOwners()
	owners.LandingGate = gate
	release.apply(&owners)
	return landCarriedWithOwners(owners, request)
}

func landCarriedWithOwners(owners landpath.Owners, request landpath.LandRequest) intentProcessResult {
	var stdout, stderr bytes.Buffer
	code := landpath.Land(owners, request, &stdout, &stderr)
	return intentProcessResult{stdout: stdout.Bytes(), stderr: stderr.Bytes(), code: code}
}

func defaultIntentDeliveryOwners() *intentDeliveryOwners {
	return &intentDeliveryOwners{
		recordWriter: recordWriterPreflight,
		process:      runIntentOwnerProcess,
		landCarried:  landCarriedInProcess,
		closeOwner:   inProcessCloseOwner,
		executable:   os.Executable,
		branchRead: func(args []string) (branch.BranchReadResult, int, error) {
			return goalBranchReadRun(args, goalBranchReadDependencies{})
		},
		branchState: productionIntentBranchState,
		landPrep: func(args []string) (goalBranchLandPrepOutcome, int, error) {
			return goalBranchLandPrepRun(args, goalBranchLandPrepDependencies{Prepare: branch.PrepareLanding,
				LoadContract: func(root string) (testpolicy.Contract, error) {
					_, contract, _, err := testrun.LoadContract(root)
					return contract, err
				}})
		},
		landCandidate: func(args []string) (goalBranchLandPrepOutcome, int, error) {
			return goalBranchLandPrepRun(args, goalBranchLandPrepDependencies{CandidateOnly: true, Prepare: branch.PrepareLanding})
		},
		sweep:       goalBranchSweepLanded,
		batchUnit:   productionIntentBatchUnit,
		publishRead: goalBranchPublishRead,
		landPush:    goalBranchLandPushRun,
		batchRoot:   productionIntentBatchRoot,
		batchJoin: func(request batchowner.BatchJoinRequest) (batch.Record, error) {
			return batchowner.ExecuteBatchJoin(request, batchowner.BatchJoinDependenciesForCommand())
		},
		now: func() time.Time { return time.Now().UTC() },
	}
}

func (inv *intentInvocation) delivery() *intentDeliveryOwners {
	if inv.owners.delivery == nil {
		inv.owners.delivery = defaultIntentDeliveryOwners()
	}
	return inv.owners.delivery
}

// inProcessCloseOwner runs the delegate lifecycle's close in this process.
func inProcessCloseOwner(root string, args []string) intentProcessResult {
	stdout, stderr, code := delegateInProcess(root)(args...)
	return intentProcessResult{stdout: []byte(stdout), stderr: []byte(stderr), code: code}
}

// runIntentOwnerProcess runs one owner with its own output pipes; the
// caller reads only the owner's structured output from them.
func runIntentOwnerProcess(process intentProcess) intentProcessResult {
	command := exec.Command(process.argv[0], process.argv[1:]...)
	command.Dir = process.dir
	command.Stdin = nil
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	err := command.Run()
	result := intentProcessResult{stdout: stdout.Bytes(), stderr: stderr.Bytes(), code: commandExitCode(err)}
	var exit *exec.ExitError
	if err != nil && !errors.As(err, &exit) {
		result.err = err
	}
	return result
}

func productionIntentBranchState(root, goalID string) (intentBranchState, error) {
	endpoint, err := branch.MainEndpoint(root)
	if err != nil {
		return intentBranchState{}, err
	}
	endpointTip, err := goalBranchEndpointTip(root, endpoint)
	if err != nil {
		return intentBranchState{}, err
	}
	branchTip, present, err := goalBranchOriginTip(root, endpoint, goalID)
	if err != nil {
		return intentBranchState{}, err
	}
	if !present {
		return intentBranchState{EndpointTip: endpointTip}, nil
	}
	status, err := branch.InspectStatus(root, endpointTip, branchTip, goalID)
	if err != nil {
		return intentBranchState{}, err
	}
	state := intentBranchState{EndpointTip: endpointTip, BranchTip: branchTip, Status: status}
	for _, unit := range status.Units[:status.Prefix] {
		attestation, err := branch.ValidateAttestationAt(root, branchTip, endpointTip, goalID, unit.Unit, unit.Commit)
		if err != nil {
			return intentBranchState{}, fmt.Errorf("unit %s has no valid attestation: %w", unit.Unit, err)
		}
		state.Sources = append(state.Sources, attestation.Source.Kind)
	}
	return state, nil
}

// productionIntentBatchUnit finds the batch member this land request names
// in the landing checkout's store: a chain by id, a goal branch by its
// recorded selection (the whole goal, or the prefix ending at --through).
func productionIntentBatchUnit(landingRoot string, request batchowner.BatchJoinRequest, branchTip string) (batch.Record, batch.Unit, bool, error) {
	store := batch.NewStore(landingRoot, identity.KernelProber{})
	paths, err := filepath.Glob(filepath.Join(landingRoot, "artifacts", "agents", "landing-batches", "*.json"))
	if err != nil {
		return batch.Record{}, batch.Unit{}, false, err
	}
	var landedRecord batch.Record
	var landedUnit batch.Unit
	landed := false
	for _, path := range paths {
		record, err := store.Load(strings.TrimSuffix(filepath.Base(path), ".json"))
		if err != nil {
			return batch.Record{}, batch.Unit{}, false, err
		}
		for _, unit := range record.Units {
			if !intentBatchMember(unit, request, branchTip) {
				continue
			}
			switch unit.State {
			case batch.UnitJoining, batch.UnitJoined, batch.UnitReturnPending:
				return record, unit, true, nil
			case batch.UnitLanded:
				landedRecord, landedUnit, landed = record, unit, true
			}
		}
	}
	return landedRecord, landedUnit, landed, nil
}

// intentBatchMember reports whether a batch unit is exactly the member a
// land request asks for. A goal-branch member records its builds (its chain
// field is the branch tip it joined at) and its selection: the whole goal,
// or the last unit commit of an approved prefix.
func intentBatchMember(unit batch.Unit, request batchowner.BatchJoinRequest, branchTip string) bool {
	if unit.GoalID != request.GoalID {
		return false
	}
	if request.ChainID != "" {
		return unit.Chain == request.ChainID
	}
	if len(unit.Builds) == 0 {
		return false
	}
	if request.Last {
		// A whole-goal member is this work only while the goal branch is the
		// tip it joined at; a newer branch is fresh work. With the branch
		// gone, the retained member is the retry's answer.
		return unit.GoalLast && (branchTip == "" || unit.BranchTip == branchTip)
	}
	return !unit.GoalLast && unit.Builds[len(unit.Builds)-1].Commit == request.Through
}

// productionIntentBatchRoot reports whether this installation lands through
// a lane and, when it does, the checkout: its own landing.batch-root against
// the host's one landing lane (U12, landing_lane.go).
func productionIntentBatchRoot(root string, now time.Time) (string, bool, error) {
	return batchowner.ProductionLandingLaneSeams().BatchRoot(root, now)
}

// ---- shared job-store reads

func (inv *intentInvocation) jobRecord(id string) (map[string]any, error) {
	return inv.jobRecordAt(inv.layout.InstallationRoot, id)
}

func (inv *intentInvocation) jobRecordAt(installation, id string) (map[string]any, error) {
	if !validIntentJobID(id) {
		return nil, fmt.Errorf("%q is not a job id", id)
	}
	record, err := dispatchcore.ReadRecordObject(filepath.Join(installation, "artifacts", "agents", "jobs", id+".json"))
	if err != nil {
		return nil, fmt.Errorf("job %s has no readable record (%v)", id, err)
	}
	return record, nil
}

func validIntentJobID(id string) bool {
	if id == "" || len(id) > 128 {
		return false
	}
	for _, r := range id {
		if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' || r == '_' || r == '.') {
			return false
		}
	}
	return !strings.Contains(id, "..")
}

func recordText(record map[string]any, field string) string {
	value, _ := record[field].(string)
	return value
}

func recordRound(record map[string]any) int64 {
	switch value := record["round"].(type) {
	case float64:
		return int64(value)
	case json.Number:
		n, _ := value.Int64()
		return n
	}
	return 0
}

func criticRole(role string) bool {
	return role == "design-critic" || role == "code-critic" || role == "warden"
}

// newestRound is the chain's highest-numbered round record.
func (inv *intentInvocation) newestRound(root string) (map[string]any, error) {
	return inv.newestRoundAt(inv.layout.InstallationRoot, root)
}

func (inv *intentInvocation) newestRoundAt(installation, root string) (map[string]any, error) {
	records, err := dispatchcore.ChainRecords(installation, root)
	if err != nil {
		return nil, err
	}
	var newest map[string]any
	for _, record := range records {
		if newest == nil || recordRound(record) > recordRound(newest) {
			newest = record
		}
	}
	if newest == nil {
		return nil, fmt.Errorf("job %s has no readable chain", root)
	}
	return newest, nil
}

func (inv *intentInvocation) returnPath(root string, round int64) string {
	return inv.returnPathAt(inv.layout.InstallationRoot, root, round)
}

func (inv *intentInvocation) returnPathAt(installation, root string, round int64) string {
	return filepath.Join(installation, "artifacts", "agents", root, "rounds", fmt.Sprint(round), "return.json")
}

type intentFinding struct {
	ID       string `json:"id"`
	Material bool   `json:"material"`
	Title    string `json:"title,omitempty"`
}

func readIntentFindings(path string) ([]intentFinding, string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, "", err
	}
	var result struct {
		Findings []intentFinding `json:"findings"`
		Verdict  string          `json:"verdict"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, "", err
	}
	return result.Findings, result.Verdict, nil
}

func jobTarget(id string) intentTarget { return intentTarget{Kind: "job", ID: id} }

// dispatchJobID decodes the job reference status prints into the raw
// dispatch job id its owner reads. A qualified or ambiguous reference that
// does not resolve, or a launch, is refused; an unqualified id no store
// knows is passed on for the owner's own report.
func (inv *intentInvocation) dispatchJobID(ref, verb string) (string, *intentResult) {
	job, problem := inv.resolveJob(ref, verb)
	qualified := strings.HasPrefix(ref, launchJobPrefix) || strings.HasPrefix(ref, dispatchJobPrefix)
	switch {
	case problem != nil && qualified:
		return "", problem
	case problem != nil:
		if data, _ := problem.Data.(map[string]any); data["candidates"] != nil {
			return "", problem
		}
		return ref, nil
	case job.kind == "launch":
		return "", &intentResult{Outcome: intentRefused, code: 2, Targets: []intentTarget{{Kind: "job", ID: jobReference(job)}},
			Summary: fmt.Sprintf("%s is a launch, and %s takes a dispatch job; nothing was done", jobReference(job), verb),
			next:    inv.publicArgv("work", "status", "--all"), nextReason: "lists the dispatch jobs with their j2: references"}
	}
	return job.id, nil
}

func (inv *intentInvocation) sameCommand() []string {
	return append(append([]string{"metasystem"}, inv.command.words()...), inv.raw...)
}

// ---- review

// runIntentReview reviews what its target names: a goal's work (or a
// submission, or a finding's discharge), a dispatch job, a unit run, one
// commit with --commit, or with no target feedback on changes.
func runIntentReview(inv *intentInvocation) int {
	if inv.input.switched("check-only") {
		return runIntentReviewCheckOnly(inv)
	}
	for _, only := range []string{"stage", "test-command", "recertification", "findings"} {
		if inv.input.has(only) {
			return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: fmt.Sprintf("--%s only goes with --check-only; nothing was done", only),
				next: inv.typedArgvWith("--check-only"), nextReason: "checks without asking a critic"})
		}
	}
	manual := inv.input.has("changes") || inv.input.has("patch")
	if inv.input.has("commit") {
		if len(inv.input.args) > 0 || manual {
			return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "--commit names the subject itself, so it takes no other subject; nothing was done",
				next:       inv.publicArgv("work", "review", "--commit", inv.input.text("commit"), "--goal", chooseUnitValue(inv.input.text("goal"), "GOAL")),
				nextReason: "reviews the commit alone"})
		}
		if problem := inv.reviewCommonChecks("commit"); problem != nil {
			return inv.render(*problem)
		}
		if result := inv.selectRoot(); result != nil {
			return inv.render(*result)
		}
		return inv.render(inv.reviewCommit(inv.input.text("commit")))
	}
	if len(inv.input.args) == 0 {
		if !manual {
			return inv.render(intentResult{Outcome: intentRefused, code: 2,
				Summary: "work review needs what to review; nothing was done",
				next:    inv.typedArgvFor("GOAL"), nextReason: "a goal's work; or j2:JOB, run:RUN, --commit SHA, or --changes for feedback"})
		}
		if inv.input.has("changes") && inv.input.has("patch") {
			return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "give --changes or --patch, not both; nothing was done",
				next: withoutSwitch(inv.sameCommand(), "changes"), nextReason: "reviews the patch"})
		}
		return runIntentReviewDiagnostic(inv, inv.input.text("patch"))
	}
	if kind, _ := splitReference(inv.input.args[0]); kind == refJ2 || kind == refRun {
		// Options no owner of this subject takes refuse before it is read.
		if problem := inv.reviewCommonChecks(kind); problem != nil {
			return inv.render(*problem)
		}
	}
	ref, problem := inv.resolveWorkRef(inv.input.args[0], inv.command.accepts)
	if problem != nil {
		return inv.render(*problem)
	}
	if ref.kind == refGoal {
		if manual {
			return runIntentReviewManual(inv, ref.id)
		}
		return runIntentReviewGoal(inv, ref.id)
	}
	if manual || inv.input.has("work") {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: fmt.Sprintf("--changes, --patch and --work are for a goal's review, not %s; nothing was done", ref.qualified()),
			next: inv.typedArgvLess("changes", "patch", "work"), nextReason: "reviews " + ref.qualified()})
	}
	if problem := inv.reviewCommonChecks(ref.kind); problem != nil {
		return inv.render(*problem)
	}
	if result := inv.selectRoot(); result != nil {
		return inv.render(*result)
	}
	if ref.kind == refRun {
		return inv.render(inv.reviewUnit(ref.id))
	}
	subject := ref.id
	if record, err := inv.jobRecord(subject); err == nil && criticRole(recordText(record, "role")) {
		return inv.render(inv.closeCriticJob(subject))
	}
	result := inv.reviewJob(subject)
	if inv.input.has("dispositions") {
		return inv.render(inv.closeJobReview(subject, result))
	}
	return inv.render(result)
}

// reviewCommonChecks refuses the options no owner of a job, run or commit
// review takes, before anything is read.
func (inv *intentInvocation) reviewCommonChecks(kind string) *intentResult {
	for _, number := range []string{"retry"} {
		if value, err := strconv.Atoi(inv.input.text(number)); inv.input.has(number) && (err != nil || value < 1) {
			return &intentResult{Outcome: intentRefused, code: 2, Summary: fmt.Sprintf("--%s takes a review round number such as 1; nothing was done", number),
				next: append(inv.typedArgvLess(number), "--"+number, "1"), nextReason: "the round that failed"}
		}
	}
	if inv.input.has("effort") {
		return &intentResult{Outcome: intentRefused, code: 2,
			Summary: "a critic's effort is set by the project's configuration, not by --effort; nothing was done",
			next:    inv.typedArgvLess("effort"), nextReason: "the roster and the change's reach decide the effort"}
	}
	if inv.input.has("model") && kind != "commit" && kind != refRun {
		return &intentResult{Outcome: intentRefused, code: 2,
			Summary: "a job review's critic is chosen by the roster, not by --model; nothing was done",
			next:    inv.typedArgvLess("model"), nextReason: "--model is only for reviewing a run or a commit"}
	}
	return nil
}

// runIntentReviewCheckOnly checks without asking a critic: an implementer
// job's review boundary at a stage (j2:J --stage S), or that a critic
// round's findings are all decided (--findings RETURN --dispositions FILE,
// against the chain's register when j2:ROOT names it). Its output and exit
// code are the checks' own: 0 passes, 1 fails, 2 is a usage mistake.
func runIntentReviewCheckOnly(inv *intentInvocation) int {
	allowed := []string{"check-only", "stage", "test-command", "recertification", "findings", "dispositions", "repo", "json"}
	for name := range inv.input.values {
		if !slices.Contains(allowed, name) {
			return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: fmt.Sprintf("--check-only asks no critic, so --%s doesn't apply; nothing was done", name),
				next: inv.typedArgvLess(name), nextReason: "checks without --" + name})
		}
	}
	root := inv.cwd
	if inv.input.has("repo") {
		root = inv.textPath(inv.input.text("repo"))
	}
	job := ""
	if len(inv.input.args) == 1 {
		kind, id := splitReference(inv.input.args[0])
		if kind != "" && kind != refJ2 {
			return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: fmt.Sprintf("--check-only checks a dispatch job, not %s; nothing was done", inv.input.args[0]),
				next: inv.publicArgv("work", "status", "--all"), nextReason: "lists the dispatch jobs with their j2: references"})
		}
		job = id
		if kind == "" {
			job = inv.input.args[0]
		}
	}
	if inv.input.has("findings") || inv.input.has("dispositions") {
		for _, other := range []string{"stage", "test-command", "recertification"} {
			if inv.input.has(other) {
				return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: fmt.Sprintf("--%s checks a job, and --findings with --dispositions checks decisions; give one; nothing was done", other),
					next: inv.typedArgvLess(other), nextReason: "checks the decisions"})
			}
		}
		args := []string{"--findings", inv.flagPath("findings"), "--dispositions", inv.flagPath("dispositions")}
		if job != "" {
			args = append(args, "--repo", root, "--root-job", job)
		}
		return runValidateCritiqueClosed(args, inv.stdout, inv.stderr)
	}
	if job == "" || !inv.input.has("stage") {
		retry := inv.sameCommand()
		if job == "" {
			retry = append(inv.typedArgvFor("j2:JOB"), "--stage", "review")
		} else if !inv.input.has("stage") {
			retry = append(retry, "--stage", "review")
		}
		return inv.render(intentResult{Outcome: intentRefused, code: 2,
			Summary: "--check-only needs a job and a stage, or --findings and --dispositions; nothing was done",
			next:    retry, nextReason: "the stage is review, recertify or merge"})
	}
	args := []string{"--root", root, "--stage", inv.input.text("stage"), "--job", job}
	for _, name := range []string{"test-command", "recertification"} {
		if inv.input.has(name) {
			args = append(args, "--"+name, inv.input.text(name))
		}
	}
	return runValidateConformance(args, inv.stdout, inv.stderr)
}

// runIntentWorkFinish completes one finished dispatch job's records through
// the close owner, with its authority and durability checks.
func runIntentWorkFinish(inv *intentInvocation) int {
	if len(inv.input.args) != 1 {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "work finish needs the job to finish; nothing was done",
			next: inv.publicArgv("work", "status", "--all"), nextReason: "lists the jobs with their j2: references"})
	}
	ref, problem := inv.resolveWorkRef(inv.input.args[0], inv.command.accepts)
	if problem != nil {
		return inv.render(*problem)
	}
	return runIntentDoneJob(inv, ref.qualified())
}

// intentDesignRecord is a design page's identifying header.
type intentDesignRecord struct {
	ID, Status string
	Goals      []string
}

func readIntentDesignRecord(path string) (intentDesignRecord, []byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return intentDesignRecord{}, nil, err
	}
	var record intentDesignRecord
	kind := ""
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for lines := 0; scanner.Scan() && lines < 40; lines++ {
		key, value, found := strings.Cut(strings.TrimPrefix(strings.TrimSpace(scanner.Text()), "- "), ":")
		if !found {
			continue
		}
		value = strings.TrimSpace(value)
		switch key {
		case "Kind":
			kind = value
		case "Id":
			record.ID = value
		case "Status":
			record.Status = value
		case "Goals":
			for _, id := range strings.Split(value, ",") {
				if id = strings.TrimSpace(id); id != "" {
					record.Goals = append(record.Goals, id)
				}
			}
		}
	}
	if kind != "design" || record.ID == "" {
		return record, data, fmt.Errorf("%s is not a goal's design document; metasystem design write writes one", path)
	}
	return record, data, nil
}

// reviewBriefFacts are the parts of a review brief that come from recorded
// state: the goal's approved review rounds and the caller's reader budget.
func (inv *intentInvocation) reviewBriefFacts(targets []intentTarget, goalID string) (int64, int, *intentResult) {
	calls := 0
	if _, err := fmt.Sscanf(inv.input.text("tool-calls"), "%d", &calls); err != nil || calls < 1 {
		return 0, 0, &intentResult{Targets: targets, Outcome: intentRefused, code: 2,
			Summary: "the review needs the critic's tool-call budget, and none is configured; nothing was done",
			next:    append(inv.typedArgvLess("tool-calls"), "--tool-calls", "30"), nextReason: "30 is an example budget"}
	}
	projection, _, failed := inv.projection()
	if failed != nil {
		failed.Targets = targets
		return 0, 0, failed
	}
	file := projection.Tree.Live[goalID]
	if file == nil || file.Approved == nil || file.Budget == nil || file.Budget.ReviewRoundLimit < 1 {
		return 0, 0, &intentResult{Targets: targets, Outcome: intentRefused, code: 1,
			Summary: fmt.Sprintf("goal %s has no approved budget for review rounds; nothing was reviewed", goalID),
			next:    inv.publicArgv("goal", "approve", goalID), nextReason: "a person approves it with a budget; then repeat this command"}
	}
	return file.Budget.ReviewRoundLimit, calls, nil
}

// reviewBrief renders the engine's review-brief.md template for one
// subject. Every value is recorded state or the caller's explicit input.
const designCritiqueRounds = 2

func reviewBrief(mode, chain, goalID string, rounds int64, calls int, threat, scope, copyPath, contents, findings string, checklist []string) string {
	lines := []string{
		"Working Mode: " + mode,
		"Orchestrator Identity: metasystem work review",
		"",
		"# Review brief: " + chain,
		"",
		fmt.Sprintf("Round budget: %d focused rounds, goal %s's approved review-round limit; exhaustion follows the critique skills' budget rules.", rounds, goalID),
		"",
		"Threat model: " + threat,
		"",
		"Scope: " + scope,
		"",
		"## Prepared copy",
		"",
		"Path: " + copyPath,
		"",
		"Contents: " + contents,
		"",
		"## Checklist",
		"",
		"Batch independent reads: when several files or ranges are needed and none depends on another's content, request them all in one turn, never one per turn.",
		"",
		"No single command may wait longer than 240 seconds; run a longer one in the background with its output to a file and poll the file. A wait that outlives the host turn continues with the wait command its result prints.",
		"",
	}
	for index, item := range checklist {
		lines = append(lines, fmt.Sprintf("%d. %s", index+1, item))
	}
	lines = append(lines, "",
		"## Tool-call budget",
		"",
		fmt.Sprintf("Maximum reader tool calls: %d", calls),
		"",
		"Stop when this number is reached. In the findings file, list every checklist item or part of an item that the budget did not allow you to check.",
		"",
		"## Findings artifact and return shape",
		"",
		"Write findings to: "+findings,
		"",
		"Inside that file, number findings from most severe to least severe. Each finding names the file, rule, and concrete failure it causes. If there are no material findings, record AGREE and any non-gating observations there.",
		"")
	return strings.Join(lines, "\n")
}

func (inv *intentInvocation) reviewDesign(file string) intentResult {
	path := file
	if !filepath.IsAbs(path) {
		path = filepath.Join(inv.cwd, path)
	}
	target := []intentTarget{{Kind: "design", ID: file}}
	roots, err := project.ResolveRoots(inv.layout.InstallationRoot)
	if err != nil {
		return intentResult{Targets: target, Outcome: intentRefused, code: 1, Summary: "the project's design folders can't be read, so nothing was reviewed",
			next: inv.publicArgv("settings", "check"), nextReason: "checks the project's configuration", Details: []string{err.Error()}}
	}
	resolved, err := filepath.EvalSymlinks(path)
	checkout, checkoutErr := filepath.EvalSymlinks(roots.Checkout)
	git, gitErr := filepath.EvalSymlinks(inv.layout.GitRoot)
	var checkoutRel, gitRel string
	if err == nil && checkoutErr == nil && gitErr == nil {
		checkoutRel, err = filepath.Rel(checkout, resolved)
		if err == nil {
			gitRel, err = filepath.Rel(git, resolved)
		}
	}
	checkoutRel, gitRel = filepath.ToSlash(checkoutRel), filepath.ToSlash(gitRel)
	home, inHome := project.HomeFor(roots, checkoutRel)
	if err != nil || checkoutErr != nil || gitErr != nil || !inHome || home.Kind != project.KindDesign || strings.HasPrefix(gitRel, "..") {
		homes := []string{}
		for _, candidate := range project.Homes(roots) {
			if candidate.Kind == project.KindDesign {
				homes = append(homes, candidate.Rel)
			}
		}
		return intentResult{Targets: target, Outcome: intentRefused, code: 2,
			Summary:  fmt.Sprintf("%s is not in one of this project's design folders (%s); nothing was reviewed", file, strings.Join(homes, ", ")),
			Decision: "move the page into " + strings.Join(homes, " or ") + ", then run metasystem design review on it"}
	}
	if !strings.HasPrefix(gitRel, "metasystem/") && !dispatchcore.RepositoryDesignPath(gitRel) {
		return intentResult{Targets: target, Outcome: intentRefused, code: 2,
			Summary:  fmt.Sprintf("design %s lies outside metasystem/ and plans/designs/; nothing was reviewed", gitRel),
			Decision: "move the page under plans/designs/, then run metasystem design review on it"}
	}
	record, data, err := readIntentDesignRecord(resolved)
	if err != nil {
		return intentResult{Targets: target, Outcome: intentRefused, code: 2, Summary: err.Error(),
			Decision: "nothing to do; name a page whose head says Kind: design and names its goal"}
	}
	goalID := inv.input.text("goal")
	if goalID == "" {
		if len(record.Goals) != 1 {
			return intentResult{Targets: target, Outcome: intentRefused, code: 2,
				Summary: fmt.Sprintf("design %s names %d goals, and a review serves one; nothing was reviewed", record.ID, len(record.Goals)),
				next:    append(inv.typedArgvLess("goal"), "--goal", firstOr(record.Goals, "GOAL")), nextReason: "or another goal the design names"}
		}
		goalID = record.Goals[0]
	}
	rounds, calls, refused := inv.reviewBriefFacts(target, goalID)
	if refused != nil {
		return *refused
	}
	digest := sha256.Sum256(data)
	dir := filepath.Join(inv.layout.InstallationRoot, "artifacts", "agents", "intent-review",
		"design-"+strings.ToLower(record.ID)+"-"+hex.EncodeToString(digest[:6]))
	outputs := filepath.Join(dir, "outputs.md")
	brief := filepath.Join(dir, "brief.md")
	lineCount := bytes.Count(data, []byte("\n"))
	if lineCount == 0 {
		lineCount = 1
	}
	// A design critique has at most two rounds: the register raises a second
	// round's unresolved findings to a person (finding_register.go).
	rounds = min(rounds, designCritiqueRounds)
	briefText := reviewBrief("design-critique", "design "+record.ID, goalID, rounds, calls,
		"the threat model the design page states for itself, and goal "+goalID+"'s intent; a true finding outside it closes as out-of-scope.",
		fmt.Sprintf("design record %s at %s (status %s) and its declared outputs; the implementation is out of scope.", record.ID, gitRel, record.Status),
		filepath.Join(git, filepath.FromSlash(gitRel)),
		fmt.Sprintf("design page %s, SHA-256 %s", gitRel, hex.EncodeToString(digest[:])),
		filepath.Join(dir, "findings.md"),
		[]string{fmt.Sprintf("`%s:1-%d` — the whole design under skills/design-critique/SKILL.md: missing work, false premises and first-use failures", gitRel, lineCount)})
	designPath := filepath.Join(git, filepath.FromSlash(gitRel))
	plan := designReviewPlan{targets: target, goalID: goalID, recordID: record.ID, design: designPath, subject: hex.EncodeToString(digest[:]), brief: brief,
		inputs: map[string]string{brief: briefText, outputs: gitRel + "\n"}}
	// An existing chain is decided before anything is written: a Send that
	// rejoins a running examination writes nothing, so the brief it admitted,
	// which states its reader budget, keeps its bytes.
	if decided := inv.reviewDesignChain(plan); decided != nil {
		return *decided
	}
	if err := writeIntentInputs(dir, plan.inputs); err != nil {
		return intentResult{Targets: target, Outcome: intentFailed, Summary: "the review's brief can't be written, so nothing was reviewed",
			next: inv.sameCommand(), nextReason: "try again; --verbose shows the cause", Details: []string{err.Error()}}
	}
	if len(dispatchcore.DesignCritiqueChains(inv.layout.InstallationRoot, goalID, designPath)) == 0 {
		// The first paid critique needs the goal's claim; an approved goal
		// nobody holds is claimed lawfully, with no build worktree.
		if refused := inv.acquireDesignCritiqueClaim(goalID); refused != nil {
			return *refused
		}
	}
	chainsBefore := len(dispatchcore.DesignCritiqueChains(inv.layout.InstallationRoot, goalID, designPath))
	result := inv.dispatchReview(target, []string{"--role", "design-critic", "--brief", brief, "--goal", goalID,
		"--destructive-reach", "DESIGN-BEARING", "--outputs", outputs, "--design", gitRel})
	if chainsBefore == 0 {
		inv.recordFirstDesignExamination(plan)
	}
	return result
}

// writeIntentInputs writes generated review inputs once. The same subject
// generates the same bytes, so the dispatch identity derived from the brief
// is the same on every repeat.
func writeIntentInputs(dir string, files map[string]string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for path, content := range files {
		if existing, err := os.ReadFile(path); err == nil && string(existing) == content {
			continue
		}
		temporary, err := os.CreateTemp(dir, ".input-*")
		if err != nil {
			return err
		}
		_, writeErr := temporary.WriteString(content)
		closeErr := temporary.Close()
		if writeErr == nil {
			writeErr = closeErr
		}
		if writeErr == nil {
			writeErr = os.Rename(temporary.Name(), path)
		}
		if writeErr != nil {
			_ = os.Remove(temporary.Name())
			return writeErr
		}
	}
	return nil
}

func (inv *intentInvocation) reviewJob(job string) intentResult {
	target := []intentTarget{jobTarget(job)}
	record, err := inv.jobRecord(job)
	if err != nil {
		return intentResult{Targets: target, Outcome: intentRefused, code: 2, Summary: err.Error() + "; nothing was reviewed",
			next: inv.publicArgv("work", "status", "--all"), nextReason: "lists the jobs with their references"}
	}
	if role := recordText(record, "role"); role != "implementer" {
		return intentResult{Targets: target, Outcome: intentRefused, code: 2,
			Summary:  fmt.Sprintf("job %s is a %s job, and only a builder's job is reviewed; nothing was reviewed", job, role),
			Decision: "nothing to do; review the job that built the change instead"}
	}
	if status := recordText(record, "status"); status != "completed" {
		outcome := intentRefused
		if !dispatchcore.TerminalStatus(status) {
			outcome = intentInProgress
		}
		return intentResult{Targets: target, Outcome: outcome, Summary: fmt.Sprintf("job %s is %s; a review reads a completed job", job, status),
			Data: map[string]any{"job": job, "status": status}}
	}
	goalID := inv.input.text("goal")
	if goalID == "" {
		goalID = recordText(record, "goalId")
	}
	if goalID == "" {
		return intentResult{Targets: target, Outcome: intentRefused, code: 2,
			Summary: fmt.Sprintf("job %s records no goal; nothing was reviewed", job),
			next:    inv.typedArgvWith("--goal", "GOAL"), nextReason: "names the goal the job serves"}
	}
	rounds, calls, refused := inv.reviewBriefFacts(target, goalID)
	if refused != nil {
		return *refused
	}
	dir := filepath.Join(inv.layout.InstallationRoot, "artifacts", "agents", "intent-review", "job-"+job)
	brief := filepath.Join(dir, "brief.md")
	briefText := reviewBrief("code-critique", "job "+job, goalID, rounds, calls,
		"the threat model of the design and brief job "+job+" implements; a true finding outside it closes as out-of-scope.",
		fmt.Sprintf("implementer job %s's recorded diff against its brief; unchanged code is out of scope.", job),
		recordText(record, "workspaceRoot"),
		fmt.Sprintf("job %s: base %s, branch %s", job, recordText(record, "baseSha"), recordText(record, "branch")),
		filepath.Join(dir, "findings.md"),
		[]string{"`artifacts/agents/jobs/" + job + ".json` and the job's round diff — brief conformance first, then defects, under skills/code-critique/SKILL.md"})
	if err := writeIntentInputs(dir, map[string]string{brief: briefText}); err != nil {
		return intentResult{Targets: target, Outcome: intentFailed, Summary: "the review's brief can't be written, so nothing was reviewed",
			next: inv.sameCommand(), nextReason: "try again; --verbose shows the cause", Details: []string{err.Error()}}
	}
	return inv.dispatchReview(target, []string{"--role", "code-critic", "--reviews", job, "--brief", brief, "--goal", goalID,
		"--destructive-reach", "MECHANICAL"})
}

// dispatchReview asks the delegate boundary for the critic round and reads
// the round's recorded state. The boundary replays a request it already
// holds, so a repeat collects instead of dispatching again.
func (inv *intentInvocation) dispatchReview(targets []intentTarget, args []string) intentResult {
	outcome, result := inv.delegate(targets, args)
	if result != nil {
		return *result
	}
	return inv.collectReview(targets, outcome)
}

// delegate calls the delegate boundary in this process and reads its typed
// outcome. The installation is the selected engine's, as the former
// `internal delegate` child derived it from its own executable; the
// dispatch script starts in the installation with no input.
func (inv *intentInvocation) delegate(targets []intentTarget, args []string) (delegateOutcome, *intentResult) {
	owners := inv.delivery()
	binary, err := owners.executable()
	if err != nil {
		return delegateOutcome{}, &intentResult{Targets: targets, Outcome: intentFailed, Summary: "the running metasystem program can't be found, so no critic was started",
			next: inv.sameCommand(), nextReason: "try again; --verbose shows the cause", Details: []string{err.Error()}}
	}
	if len(args) > 1 && args[0] == "--follow-up" {
		if problem := inv.rebindCritiqueBudget(targets, args[1]); problem != nil {
			return delegateOutcome{}, problem
		}
	}
	request := delegateRequest{rootOverride: os.Getenv("METASYSTEM_DELEGATE_ROOT"), engine: binary, args: args, dir: inv.layout.InstallationRoot}
	ran := ownerCall(func(stdout, stderr io.Writer) int { return inv.ownerCalls().delegate(request, stdout, stderr) })
	var outcome delegateOutcome
	if ran.err != nil || json.Unmarshal(bytes.TrimSpace(ran.stdout), &outcome) != nil || outcome.Outcome == "" {
		detail := "the delegate boundary returned no typed outcome"
		if ran.err != nil {
			detail = ran.err.Error()
		}
		return outcome, &intentResult{Targets: targets, Outcome: intentInProgress,
			Summary:    "whether the critic started isn't known yet; it is never started twice",
			Data:       map[string]any{"exitCode": ran.code},
			next:       inv.sameCommand(),
			nextReason: "recovers the same request; it is not dispatched again", Details: []string{detail}}
	}
	switch {
	case outcome.JobID != "" && !strings.HasPrefix(outcome.Outcome, "REFUSED"):
		return outcome, nil
	case outcome.Outcome == "RECONCILING" || outcome.Outcome == "IN-PROGRESS" || outcome.Outcome == "BOUND":
		return outcome, &intentResult{Targets: targets, Outcome: intentInProgress,
			Summary: "the dispatch is " + strings.ToLower(outcome.Outcome) + " and names no job yet", Data: map[string]any{"delegate": outcome},
			next: inv.sameCommand(), nextReason: "collects the same dispatch once it names its job"}
	}
	summary := strings.TrimSpace(outcome.Detail)
	if summary == "" {
		summary = "the critic was not started"
	}
	return outcome, &intentResult{Targets: targets, Outcome: intentRefused, code: max(ran.code, 1),
		Summary: summary, Data: map[string]any{"delegate": outcome}, next: inv.sameCommand(), nextReason: "once that is settled",
		Details: []string{strings.TrimSpace(outcome.Outcome + ": " + outcome.Detail)}}
}

// rebindCritiqueBudget carries the goal's current review-round limit onto
// the critic registers a review of root continues, before the follow-up or
// the close reads it: root itself when it is a critic's, else every code
// critic or warden reviewing its implementation chain. A limit a person
// raised then takes effect without a separate step; a rebind that fails is
// refused in its own words, and nothing is continued or closed.
func (inv *intentInvocation) rebindCritiqueBudget(targets []intentTarget, root string) *intentResult {
	rebind := inv.delivery().rebind
	if rebind == nil {
		rebind = dispatchcore.CritiqueChainBudgetRebind
	}
	if _, err := rebind(inv.layout.InstallationRoot, root); err != nil {
		return &intentResult{Targets: targets, Outcome: intentRefused, code: 1,
			Summary: "the goal's review-round limit can't be applied to this review, so nothing was continued or closed",
			next:    inv.sameCommand(), nextReason: "try again; --verbose shows the cause", Details: []string{fmt.Sprintf("chain %s: %v", root, err)}}
	}
	return nil
}

func (inv *intentInvocation) collectReview(targets []intentTarget, outcome delegateOutcome) intentResult {
	job := outcome.JobID
	targets = append(targets, jobTarget(job))
	record, err := inv.jobRecord(job)
	if err != nil {
		return intentResult{Targets: targets, Outcome: intentInProgress, Summary: "the dispatch named job " + job + " but its record is not readable yet",
			Data: map[string]any{"delegate": outcome}, next: inv.sameCommand(), nextReason: "collects the same review"}
	}
	root := job
	if parent := recordText(record, "parentJob"); parent != "" {
		root = parent
	}
	status := recordText(record, "status")
	data := map[string]any{"delegate": outcome, "job": job, "status": status}
	switch {
	case !dispatchcore.TerminalStatus(status):
		return intentResult{Targets: targets, Outcome: intentInProgress, Summary: fmt.Sprintf("review job %s is %s", job, status),
			Data: data, next: inv.sameCommand(), nextReason: "collects the review when its round is finished"}
	case status != "completed":
		return intentResult{Targets: targets, Outcome: intentFailed, Summary: fmt.Sprintf("review job %s ended %s (%s)", job, status, recordText(record, "reason")), Data: data,
			next: inv.sameCommand(), nextReason: "asks for the review again"}
	}
	path := inv.returnPath(root, recordRound(record))
	findings, verdict, err := readIntentFindings(path)
	if err != nil {
		return intentResult{Targets: targets, Outcome: intentFailed, Summary: fmt.Sprintf("review job %s finished, but its findings can't be read", job), Data: data,
			next: inv.publicArgv("work", "status", qualifiedJob(job)), nextReason: "shows the job", Details: []string{err.Error()}}
	}
	material := 0
	for _, finding := range findings {
		if finding.Material {
			material++
		}
	}
	data["findings"], data["verdict"], data["return"] = findings, verdict, path
	result := intentResult{Targets: targets, Outcome: intentConfirmed, Data: data,
		Summary: fmt.Sprintf("review %s returned %d findings, %d material", job, len(findings), material)}
	if len(findings) > 0 {
		result.Decision = fmt.Sprintf("the author decides each finding of %s in a dispositions file, then corrects or closes the review", job)
		if len(targets) > 0 && targets[0].Kind == "job" {
			reviewed := targets[0].ID
			result.text = []string{"correct: " + shellCommand(inv.publicArgv("work", "revise", qualifiedJob(job), "--dispositions", "FILE", "--brief", "FILE")),
				"close: " + shellCommand(inv.publicArgv("work", "review", qualifiedJob(reviewed), "--dispositions", "FILE"))}
		} else {
			result.text = []string{"close: " + shellCommand(inv.publicArgv("work", "finish", qualifiedJob(job), "--dispositions", "FILE"))}
		}
	} else if len(targets) > 0 && targets[0].Kind == "job" {
		result.next, result.nextReason = inv.publicArgv("work", "review", qualifiedJob(targets[0].ID), "--dispositions", "FILE"), "close the review with the author's (empty) decisions"
	}
	return result
}

func (inv *intentInvocation) reviewCommit(unit string) intentResult {
	goalID := inv.input.text("goal")
	targets := []intentTarget{{Kind: "commit", ID: unit}}
	if goalID == "" {
		return intentResult{Targets: targets, Outcome: intentRefused, code: 2,
			Summary: "a commit review needs the goal whose branch holds the commit; nothing was reviewed",
			next:    inv.typedArgvWith("--goal", "GOAL"), nextReason: "names the goal"}
	}
	targets = append(targets, intentTarget{Kind: "goal", ID: goalID})
	args := []string{"--root", inv.layout.InstallationRoot, "--goal", goalID, "--unit", unit}
	if inv.input.has("brief") {
		args = append(args, "--brief", inv.callerPath(inv.input.text("brief")))
	}
	if inv.input.has("model") {
		args = append(args, "--model", inv.input.text("model"))
	}
	if inv.input.has("retry") {
		args = append(args, "--retry", inv.input.text("retry"))
	}
	return inv.commitReview(targets, inv.layout.InstallationRoot, goalID, unit, args)
}

// criticClosure reports what a terminal commit critic still needs before its
// read can be collected: the author's dispositions and the chain's close.
func (inv *intentInvocation) criticClosure(targets []intentTarget, root, unit, goalID, rootJob string) *intentResult {
	targets = append(targets, jobTarget(rootJob))
	record, err := inv.jobRecordAt(root, rootJob)
	if err != nil {
		return &intentResult{Targets: targets, Outcome: intentFailed, Summary: err.Error(),
			next: inv.sameCommand(), nextReason: "try again"}
	}
	if _, closed, err := dispatchcore.ReadClosure(record); err != nil {
		return &intentResult{Targets: targets, Outcome: intentFailed, Summary: fmt.Sprintf("the record of critic %s is damaged, so the review can't continue", rootJob),
			next: inv.publicArgv("system", "check"), nextReason: "diagnoses the record store", Details: []string{err.Error()}}
	} else if closed {
		return nil
	}
	data := map[string]any{"rootJob": rootJob, "state": "terminal-unclosed"}
	if round, err := inv.newestRoundAt(root, rootJob); err == nil {
		if _, _, err := readIntentFindings(inv.returnPathAt(root, rootJob, recordRound(round))); err != nil {
			// The examination ended without a return: there are no findings to
			// decide; the same review examines the subject once more.
			again := append(inv.canonicalReviewArgv(targets, goalID, unit), "--retry", fmt.Sprint(recordRound(round)))
			data["failedRound"] = recordRound(round)
			return &intentResult{Targets: targets, Outcome: intentFailed, code: 1, Data: data,
				Summary: fmt.Sprintf("review round %d of critic %s ended without findings to decide", recordRound(round), rootJob),
				next:    again, nextReason: "reviews once more, once the old round has stopped"}
		}
		if findings, verdict, err := readIntentFindings(inv.returnPathAt(root, rootJob, recordRound(round))); err == nil {
			material := 0
			for _, finding := range findings {
				if finding.Material {
					material++
				}
			}
			data["findings"], data["material"], data["verdict"] = findings, material, verdict
		}
	}
	return &intentResult{Targets: targets, Outcome: intentInProgress, Data: data,
		Summary:    fmt.Sprintf("critic %s has read unit %s; its findings await your decisions", rootJob, unit),
		next:       append(slices.DeleteFunc(inv.sameCommand(), func(word string) bool { return word == "--json" }), "--dispositions", "FILE"),
		nextReason: "FILE decides every finding of " + rootJob}
}

// ---- revise job

func (inv *intentInvocation) foldReview(review string) intentResult {
	targets := []intentTarget{{Kind: "review", ID: review}}
	if !inv.input.has("dispositions") || !inv.input.has("brief") {
		retry := inv.sameCommand()
		if !inv.input.has("dispositions") {
			retry = append(retry, "--dispositions", "FILE")
		}
		if !inv.input.has("brief") {
			retry = append(retry, "--brief", "BRIEF")
		}
		return intentResult{Targets: targets, Outcome: intentRefused, code: 2,
			Summary: "revising after a review needs your decisions and the follow-up brief; nothing was done",
			next:    retry, nextReason: "FILE decides the findings; BRIEF says what the follow-up does"}
	}
	round, result := inv.finishedReview(targets, review)
	if result != nil {
		return *result
	}
	returnPath := inv.returnPath(review, recordRound(round))
	if violations := validate.CritiqueClosed(returnPath, inv.flagPath("dispositions")); len(violations) > 0 {
		return joinRefusal(targets, review, violations, inv.sameCommand())
	}
	root, _ := inv.jobRecord(review)
	subject := recordText(root, "reviews")
	if recordText(root, "role") == "design-critic" || subject == "" {
		return intentResult{Targets: targets, Outcome: intentRefused, code: 2,
			Summary:  fmt.Sprintf("review %s reviewed a design, and a design has no build to follow up; nothing was done", review),
			Decision: "revise the design with these decisions yourself, then run metasystem design review on it again"}
	}
	if strings.HasPrefix(subject, "commit:") {
		return intentResult{Targets: targets, Outcome: intentRefused, code: 2,
			Summary:  fmt.Sprintf("review %s read a commit, and its fix is a new commit on the goal branch; nothing was done", review),
			Decision: "commit the fix on the goal branch, then run metasystem work review --commit SHA --goal " + chooseUnitValue(inv.input.text("goal"), "GOAL")}
	}
	implementer, err := inv.jobRecord(subject)
	if err != nil {
		return intentResult{Targets: targets, Outcome: intentRefused, code: 1, Summary: err.Error() + "; nothing was done",
			next: inv.publicArgv("work", "status", "--all"), nextReason: "lists the jobs with their references"}
	}
	subjectRoot := subject
	if parent := recordText(implementer, "parentJob"); parent != "" {
		subjectRoot = parent
	}
	targets = append(targets, jobTarget(subjectRoot))
	message, err := inv.composeFoldMessage(review, recordRound(round), subject, subjectRoot, returnPath)
	if err != nil {
		return intentResult{Targets: targets, Outcome: intentFailed, Summary: "the follow-up brief can't be put together, so nothing was done",
			next: inv.sameCommand(), nextReason: "try again; --verbose shows the cause", Details: []string{err.Error()}}
	}
	outcome, refused := inv.delegate(targets, []string{"--follow-up", subjectRoot, "--brief", message})
	if refused != nil {
		return *refused
	}
	return intentResult{Targets: append(targets, jobTarget(outcome.JobID)), Outcome: intentInProgress,
		Summary: fmt.Sprintf("follow-up %s of chain %s is %s for review %s", outcome.JobID, subjectRoot, strings.ToLower(outcome.Headline), review),
		Data:    map[string]any{"delegate": outcome, "review": review, "reviewRound": recordRound(round), "chain": subjectRoot, "message": message},
		next:    []string{"metasystem", "work", "review", qualifiedJob(outcome.JobID)}, nextReason: "reviews the follow-up once it completes"}
}

// composeFoldMessage writes the follow-up message once: the caller's brief,
// then review round's exact return and the author's dispositions, bound to
// the review, its round and its subject. The inputs are copied, never
// changed, and the directory name carries their digest, so the same inputs
// give the same message and the delegate's request identity repeats.
func (inv *intentInvocation) composeFoldMessage(review string, round int64, subject, subjectRoot, returnPath string) (string, error) {
	brief, err := os.ReadFile(inv.flagPath("brief"))
	if err != nil {
		return "", fmt.Errorf("the follow-up brief is unreadable: %v", err)
	}
	returned, err := os.ReadFile(returnPath)
	if err != nil {
		return "", fmt.Errorf("review %s round %d return is unreadable: %v", review, round, err)
	}
	dispositions, err := os.ReadFile(inv.flagPath("dispositions"))
	if err != nil {
		return "", fmt.Errorf("the dispositions are unreadable: %v", err)
	}
	digest := func(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }
	identity := sha256.Sum256([]byte(strings.Join([]string{review, fmt.Sprint(round), subject, digest(brief), digest(returned), digest(dispositions)}, "\n")))
	dir := filepath.Join(inv.layout.InstallationRoot, "artifacts", "agents", "intent-fold",
		fmt.Sprintf("%s-r%d-%s", review, round, hex.EncodeToString(identity[:6])))
	body := strings.Join([]string{
		strings.TrimRight(string(brief), "\n"),
		"",
		fmt.Sprintf("# Review %s round %d: its findings and the author's dispositions", review, round),
		"",
		fmt.Sprintf("Reviewed subject: job %s of chain %s.", subject, subjectRoot),
		fmt.Sprintf("Review return: %s, SHA-256 %s.", returnPath, digest(returned)),
		fmt.Sprintf("Dispositions: %s, SHA-256 %s.", inv.flagPath("dispositions"), digest(dispositions)),
		"Fold every finding the author accepted; a refuted, noted or out-of-scope finding is not work for this round.",
		"",
		"## Review return",
		"",
		"```json",
		strings.TrimRight(string(returned), "\n"),
		"```",
		"",
		"## Author's dispositions",
		"",
		strings.TrimRight(string(dispositions), "\n"),
		"",
	}, "\n")
	message := filepath.Join(dir, "message.md")
	return message, writeIntentInputs(dir, map[string]string{message: body})
}

// textPath is a caller-relative path made absolute from the directory the
// command runs in.
func (inv *intentInvocation) textPath(path string) string {
	if path != "" && !filepath.IsAbs(path) {
		path = filepath.Join(inv.cwd, path)
	}
	return path
}

func (inv *intentInvocation) flagPath(name string) string {
	path := inv.input.text(name)
	if path != "" && !filepath.IsAbs(path) {
		path = filepath.Join(inv.cwd, path)
	}
	return path
}

// finishedReview returns the newest round of a completed critic chain.
func (inv *intentInvocation) finishedReview(targets []intentTarget, review string) (map[string]any, *intentResult) {
	root, err := inv.jobRecord(review)
	if err != nil {
		return nil, &intentResult{Targets: targets, Outcome: intentRefused, code: 2, Summary: err.Error() + "; nothing was done",
			next: inv.publicArgv("work", "status", "--all"), nextReason: "lists the jobs with their references"}
	}
	if !criticRole(recordText(root, "role")) {
		return nil, &intentResult{Targets: targets, Outcome: intentRefused, code: 2,
			Summary:  fmt.Sprintf("job %s is a %s job, not a review; nothing was done", review, recordText(root, "role")),
			Decision: "nothing to do; name the review's job instead (metasystem work status --all lists them)"}
	}
	if parent := recordText(root, "parentJob"); parent != "" {
		return nil, &intentResult{Targets: targets, Outcome: intentRefused, code: 2,
			Summary:    fmt.Sprintf("job %s is one round of review %s; nothing was done", review, parent),
			next:       replaceWord(inv.sameCommand(), review, qualifiedJob(parent)),
			nextReason: "names the review itself"}
	}
	round, err := inv.newestRound(review)
	if err != nil {
		return nil, &intentResult{Targets: targets, Outcome: intentRefused, code: 1, Summary: "the review's newest round can't be read; nothing was done",
			next: inv.sameCommand(), nextReason: "try again; --verbose shows the cause", Details: []string{err.Error()}}
	}
	if status := recordText(round, "status"); status != "completed" {
		outcome := intentRefused
		if !dispatchcore.TerminalStatus(status) {
			outcome = intentInProgress
		}
		result := &intentResult{Targets: targets, Outcome: outcome,
			Summary: fmt.Sprintf("review %s round %d is %s, and decisions answer a finished round; nothing was done", review, recordRound(round), status),
			next:    inv.publicArgv("work", "wait", qualifiedJob(review)), nextReason: "waits for the round to finish"}
		if outcome == intentRefused {
			result.next, result.nextReason = inv.publicArgv("work", "status", qualifiedJob(review)), "shows how the round ended"
		}
		return nil, result
	}
	return round, nil
}

func joinRefusal(targets []intentTarget, review string, violations []string, retry []string) intentResult {
	return intentResult{Targets: targets, Outcome: intentRefused, code: 1,
		Summary:    fmt.Sprintf("the decisions don't match review %s's findings (%d problems, listed above); nothing was done", review, len(violations)),
		text:       violations,
		Data:       map[string]any{"violations": violations},
		next:       retry,
		nextReason: "after giving every finding exactly one decision row"}
}

// ---- close

func (inv *intentInvocation) closeChain(job string) intentResult {
	targets := []intentTarget{jobTarget(job)}
	root, err := inv.jobRecord(job)
	if err != nil {
		return intentResult{Targets: targets, Outcome: intentRefused, code: 2, Summary: err.Error() + "; nothing was closed",
			next: inv.publicArgv("work", "status", "--all"), nextReason: "lists the jobs with their references"}
	}
	if parent := recordText(root, "parentJob"); parent != "" {
		return intentResult{Targets: targets, Outcome: intentRefused, code: 2,
			Summary: fmt.Sprintf("job %s is a round of chain %s", job, parent), next: inv.publicArgv("work", "finish", qualifiedJob(parent)), nextReason: "work finish names the chain's root"}
	}
	if closed, _ := root["chainClosed"].(bool); closed {
		return intentResult{Targets: targets, Outcome: intentUnchanged, Summary: fmt.Sprintf("chain %s is already closed", job), Data: map[string]any{"chainClosed": true}}
	}
	if evidence := inv.input.text("evidence"); evidence != "" && !validIntentJobID(evidence) {
		return intentResult{Targets: targets, Outcome: intentRefused, code: 2, Summary: fmt.Sprintf("--evidence %q is not a job id; nothing was closed", evidence),
			next: inv.typedArgvLess("evidence"), nextReason: "or --evidence with the id of the job that holds the review evidence"}
	}
	writer := inv.delivery().recordWriter
	if writer == nil {
		writer = recordWriterPreflight
	}
	if cause, err := writer(inv.layout.InstallationRoot, job); err != nil {
		if cause == "record-writer-refused" {
			return intentResult{Targets: targets, Outcome: intentRefused, code: 1, Data: map[string]any{"cause": cause},
				Summary:  fmt.Sprintf("closing %s writes its records, which this shell may not do; nothing was closed", job),
				Decision: "a person, the checkout's lease holder or the job itself runs " + shellCommand(inv.sameCommand()),
				Details:  []string{err.Error()}}
		}
		return intentResult{Targets: targets, Outcome: intentRefused, code: 1, Data: map[string]any{"cause": cause},
			Summary: fmt.Sprintf("who may close %s can't be told from this shell; nothing was closed", job),
			next:    inv.sameCommand(), nextReason: "try again where the checkout's owner can be read", Details: []string{err.Error()}}
	}
	if criticRole(recordText(root, "role")) {
		if !inv.input.has("dispositions") {
			return intentResult{Targets: targets, Outcome: intentRefused, code: 2,
				Summary: fmt.Sprintf("review %s closes only with your decisions on its findings; nothing was closed", job),
				next:    inv.typedArgvWith("--dispositions", "FILE"), nextReason: "FILE decides every finding"}
		}
		round, result := inv.finishedReview(targets, job)
		if result != nil {
			return *result
		}
		if violations := validate.CritiqueClosedWithRegister(inv.returnPath(job, recordRound(round)), inv.flagPath("dispositions"),
			inv.layout.InstallationRoot, job); len(violations) > 0 {
			return joinRefusal(targets, job, violations, inv.sameCommand())
		}
	} else if inv.input.has("dispositions") {
		return intentResult{Targets: targets, Outcome: intentRefused, code: 2,
			Summary: fmt.Sprintf("%s is a %s job with no findings to decide; nothing was closed", job, recordText(root, "role")),
			next:    inv.typedArgvLess("dispositions"), nextReason: "closes it; the decisions go with its review's job"}
	} else if newest, err := inv.newestRound(job); err == nil && !dispatchcore.TerminalStatus(recordText(newest, "status")) {
		return intentResult{Targets: targets, Outcome: intentInProgress,
			Summary: fmt.Sprintf("chain %s still has round %s %s", job, recordText(newest, "jobId"), recordText(newest, "status")),
			next:    inv.publicArgv("work", "wait", qualifiedJob(job)), nextReason: "waits for it; then close again"}
	}
	if problem := inv.rebindCritiqueBudget(targets, job); problem != nil {
		return *problem
	}
	argv := []string{"close", "--job", job}
	if evidence := inv.input.text("evidence"); evidence != "" {
		argv = append(argv, "--reconcile-evidence", evidence)
	}
	closeOwner := inv.delivery().closeOwner
	if closeOwner == nil {
		closeOwner = inProcessCloseOwner
	}
	ran := closeOwner(inv.layout.InstallationRoot, argv)
	after, readErr := inv.jobRecord(job)
	closed := false
	if readErr == nil {
		closed, _ = after["chainClosed"].(bool)
	}
	data := map[string]any{"exitCode": ran.code, "chainClosed": closed}
	if closed {
		return intentResult{Targets: targets, Outcome: intentConfirmed, Data: data, Summary: fmt.Sprintf("chain %s is closed", job)}
	}
	var fenced delegateOutcome
	if json.Unmarshal(bytes.TrimSpace(ran.stdout), &fenced) == nil && fenced.Outcome != "" {
		data["owner"] = fenced
		return intentResult{Targets: targets, Outcome: intentRefused, code: max(ran.code, 1), Data: data,
			Summary: "the close was refused before it started: " + chooseUnitValue(fenced.Detail, "see --verbose"),
			next:    inv.sameCommand(), nextReason: "once that is settled", Details: []string{fenced.Outcome + " " + fenced.Detail}}
	}
	summary := fmt.Sprintf("the close owner stopped (exit %d) and chain %s is not closed", ran.code, job)
	if ran.err != nil {
		summary = fmt.Sprintf("the close owner could not run: %v; chain %s is not closed", ran.err, job)
	}
	data["ownerMessage"] = nonEmptyLines(string(ran.stderr))
	return intentResult{Targets: targets, Outcome: intentRefused, code: max(ran.code, 1), Data: data, Summary: summary,
		text: nonEmptyLines(string(ran.stderr)),
		next: inv.sameCommand(), nextReason: "after resolving what is named above"}
}

func nonEmptyLines(text string) []string {
	var lines []string
	for _, line := range strings.Split(text, "\n") {
		if strings.TrimSpace(line) != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

// ---- land

func runIntentLand(inv *intentInvocation) int {
	if inv.input.has("message") {
		return runIntentLandStaged(inv)
	}
	for _, option := range stagedLandingOptions {
		if inv.input.has(option) {
			return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "--" + option + " only goes with landing a hand-made change (--message); nothing was done",
				next: inv.typedArgvLess(option), nextReason: "lands without --" + option})
		}
	}
	if len(inv.input.args) != 1 {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "work land needs one goal, or one dispatch job; nothing was landed",
			next: inv.typedArgvFor("GOAL"), nextReason: "names the goal; a job is named as j2:JOB"})
	}
	ref, problem := inv.resolveWorkRef(inv.input.args[0], inv.command.accepts)
	if problem != nil {
		return inv.render(*problem)
	}
	if ref.kind == refJ2 {
		if inv.input.switched("queue-only") || inv.input.has("lineage") {
			return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "--queue-only and --lineage are for a goal, and a job lands whole; nothing was done",
				next: inv.typedArgvLess("queue-only", "lineage"), nextReason: "lands the job"})
		}
		if inv.input.has("through") {
			return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "--through is for a goal, and a job lands whole; nothing was landed",
				next: inv.typedArgvLess("through"), nextReason: "lands the job"})
		}
		for _, other := range append([]string{"using-exception"}, exceptionOptions...) {
			if inv.input.has(other) {
				return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "an exception lands a goal, not a job; nothing was done",
					next: inv.typedArgvLess(append([]string{"using-exception"}, exceptionOptions...)...), nextReason: "lands the job without an exception"})
			}
		}
		if result := inv.selectRoot(); result != nil {
			return inv.render(*result)
		}
		return inv.render(inv.landJob(ref.id))
	}
	args := []string{ref.id}
	if inv.input.switched("queue-only") {
		if len(args) != 1 || inv.input.has("through") || inv.input.has("using-exception") || slices.ContainsFunc(exceptionOptions, inv.input.has) {
			return inv.render(intentResult{Outcome: intentRefused, code: 2,
				Summary: "--queue-only takes one goal and no other option; nothing was done", next: inv.publicArgv("work", "land", args[0], "--queue-only"),
				nextReason: "queues the goal's landing"})
		}
		return runIntentQueueOnly(inv, args[0])
	}
	if inv.input.has("lineage") {
		return inv.render(intentResult{Outcome: intentRefused, code: 2,
			Summary: "--lineage only goes with --queue-only; nothing was done",
			next:    inv.typedArgvWith("--queue-only"), nextReason: "queues the landing as that session"})
	}
	if len(args) == 1 && (inv.input.has("exception") || inv.input.has("using-exception")) {
		if problem := inv.landExceptionInput(args[0]); problem != nil {
			return inv.render(*problem)
		}
	}
	if result := inv.selectRoot(); result != nil {
		return inv.render(*result)
	}
	if inv.input.has("exception") || inv.input.has("using-exception") {
		return inv.render(inv.landException(args[0]))
	}
	for _, other := range exceptionOptions {
		if inv.input.has(other) {
			return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: fmt.Sprintf("--%s only goes with an exceptional landing (--exception); nothing was done", other),
				next: inv.typedArgvLess(other), nextReason: "an ordinary landing"})
		}
	}
	return inv.render(inv.landGoal(args[0], inv.input.text("through")))
}

func (inv *intentInvocation) landingBatchRoot(targets []intentTarget) (string, bool, *intentResult) {
	owners := inv.delivery()
	root, configured, err := owners.batchRoot(inv.layout.InstallationRoot, owners.now())
	var laneRefusal *lane.Refusal
	if errors.As(err, &laneRefusal) {
		return "", configured, &intentResult{Targets: targets, Outcome: intentRefused, code: 1, Summary: laneRefusal.Error(), Decision: laneRefusal.Fix}
	}
	if err != nil {
		return "", configured, &intentResult{Targets: targets, Outcome: intentRefused, code: 1,
			Summary: "the landing checkout setting can't be read, so nothing was landed",
			next:    inv.publicArgv("settings", "check"), nextReason: "names what is wrong with landing.batch-root", Details: []string{err.Error()}}
	}
	return root, configured, nil
}

func (inv *intentInvocation) landJob(job string) intentResult {
	targets := []intentTarget{jobTarget(job)}
	record, err := inv.jobRecord(job)
	if err != nil {
		return intentResult{Targets: targets, Outcome: intentRefused, code: 2, Summary: err.Error() + "; nothing was landed",
			next: inv.publicArgv("work", "status", "--all"), nextReason: "lists the jobs with their references"}
	}
	goalID := recordText(record, "goalId")
	if goalID == "" {
		return intentResult{Targets: targets, Outcome: intentRefused, code: 1, Summary: fmt.Sprintf("job %s serves no goal, so it has nothing to land", job),
			Decision: "nothing to do; only a goal's work lands"}
	}
	targets = append(targets, intentTarget{Kind: "goal", ID: goalID})
	landingRoot, configured, refused := inv.landingBatchRoot(targets)
	if refused != nil {
		return *refused
	}
	if !configured {
		return intentResult{Targets: targets, Outcome: intentRefused, code: 1,
			Summary: fmt.Sprintf("job %s lands through a landing checkout, and none is set; nothing was landed", job),
			next:    inv.publicArgv("settings", "set", "landing.batch-root", "DIR"), nextReason: "DIR is a checkout used only for landing"}
	}
	request := batchowner.BatchJoinRequest{SeatRoot: inv.layout.InstallationRoot, LandingRoot: landingRoot, GoalID: goalID, ChainID: job}
	if _, _, member, err := inv.delivery().batchUnit(landingRoot, request, ""); err != nil || member {
		return inv.noteLanded(goalID, inv.joinBatch(targets, request, ""))
	}
	// The human's word on a chain is bound to the commit the chain publishes,
	// the head of its candidate branch; the batch carries it to its
	// publication gate.
	request.ChainHead = chainHead(record)
	if refused := inv.admitLanding(targets, goalID, request.ChainHead); refused != nil {
		return *refused
	}
	return inv.noteLanded(goalID, inv.joinBatch(targets, request, ""))
}

// chainHead is the commit a certified chain publishes: its job record names
// no commit, so it is the head of the candidate branch the record names
// (branch) in the worktree it names (workspaceRoot), or "" where that branch
// cannot be read, which no human word can be bound to.
func chainHead(record map[string]any) string {
	workspace, branch := recordText(record, "workspaceRoot"), recordText(record, "branch")
	if workspace == "" || branch == "" {
		return ""
	}
	read := landingPathGit(landpath.GitCall{Dir: workspace, Args: []string{"rev-parse", "--verify", "--quiet", "refs/heads/" + branch + "^{commit}"}})
	if read.Code != 0 {
		return ""
	}
	return strings.TrimSpace(string(read.Stdout))
}

// joinBatch reads the goal's existing batch membership first: a joined unit
// is reported where the batch owner has it, and only a goal with no live
// membership joins. The same land command is the continuation until the
// batch records the landing.
func (inv *intentInvocation) joinBatch(targets []intentTarget, request batchowner.BatchJoinRequest, branchTip string) intentResult {
	owners := inv.delivery()
	record, unit, member, err := owners.batchUnit(request.LandingRoot, request, branchTip)
	if err != nil {
		return intentResult{Targets: targets, Outcome: intentFailed, Summary: "the landing batches can't be read, so nothing was landed", Data: map[string]any{"route": "batch"},
			next: inv.sameCommand(), nextReason: "try again; --verbose shows the cause", Details: []string{err.Error()}}
	}
	joined := false
	if !member {
		request.At = owners.now()
		record, err = owners.batchJoin(request)
		if err != nil {
			return intentResult{Targets: targets, Outcome: intentRefused, code: 1, Summary: err.Error(), Data: map[string]any{"route": "batch"},
				next: inv.publicArgv("landing", "status"), nextReason: "shows the landing batches"}
		}
		joined, unit = true, batch.Unit{GoalID: request.GoalID, Chain: request.ChainID, State: batch.UnitJoined}
	}
	targets = append(targets, intentTarget{Kind: "batch", ID: record.BatchID})
	data := map[string]any{"route": "batch", "batchId": record.BatchID, "batchState": record.State, "unitState": unit.State, "joinedNow": joined}
	if record.Landing != nil {
		data["landing"] = record.Landing
	}
	switch unit.State {
	case batch.UnitLanded:
		// The batch owner recorded the landing; this call only reads it, so
		// the repeat is success that changes nothing (R-129-ui).
		landed := ""
		if unit.LandedCommit != "" {
			landed = " as " + shortCommit(unit.LandedCommit)
		}
		return intentResult{Targets: targets, Outcome: intentUnchanged, Data: data,
			Summary: fmt.Sprintf("goal %s already landed%s through batch %s; the goal stays open until done", request.GoalID, landed, record.BatchID)}
	case batch.UnitEjected, batch.UnitWithdrawn, batch.UnitWithdrawnBudget:
		return intentResult{Targets: targets, Outcome: intentRefused, code: 1, Data: data,
			Summary: fmt.Sprintf("goal %s left landing batch %s (%s) and did not land", request.GoalID, record.BatchID, unit.State),
			next:    inv.publicArgv("landing", "status"), nextReason: "shows why it left the batch"}
	}
	summary := fmt.Sprintf("goal %s is %s in landing batch %s; the batch owner proves and pushes it", request.GoalID, unit.State, record.BatchID)
	if joined {
		summary = fmt.Sprintf("goal %s joined landing batch %s; the batch owner proves and pushes it", request.GoalID, record.BatchID)
	}
	return intentResult{Targets: targets, Outcome: intentInProgress, Data: data, Summary: summary,
		next: inv.sameCommand(), nextReason: "reads the same batch membership until it records the landing; it never joins twice"}
}

// intentLanded is the retained typed result of one hand landing's push and
// of the merged branch's sweep after it.
type intentLanded struct {
	Landing, Endpoint, Branch, Subject string
	Swept                              bool
	// ReleaseSet is the goal's workspaces this landing ends, recorded
	// before the push and released once the merged branch is swept.
	ReleaseSet *diskstore.ReleaseSet `json:",omitempty"`
	// Staged marks a staged landing's record (work land --staged): its
	// Endpoint is the remote-tracking ref the commit was pushed to, and it
	// has no branch to sweep, so Swept says the push succeeded. The
	// exception route's record is of the same shape and carries it too, so
	// every engine's retry finishes it the same way.
	Staged bool `json:",omitempty"`
	// Exception names the recorded exception an exception route's record
	// belongs to (work land G --exception / --using-exception).
	Exception string `json:",omitempty"`
}

func (inv *intentInvocation) landGoal(goalID, through string) intentResult {
	return inv.noteLanded(goalID, inv.landGoalRoute(goalID, through))
}

func (inv *intentInvocation) landGoalRoute(goalID, through string) intentResult {
	targets := []intentTarget{{Kind: "goal", ID: goalID}}
	if !validIntentJobID(goalID) {
		return intentResult{Targets: targets, Outcome: intentRefused, code: 2, Summary: fmt.Sprintf("%q is not a goal id; nothing was landed", goalID),
			next: inv.publicArgv("goal", "list"), nextReason: "lists the goals"}
	}
	root := inv.layout.InstallationRoot
	owners := inv.delivery()
	base := filepath.Join(root, "artifacts", "agents", "landing-intent", goalID)
	if result := inv.resumeSweep(targets, goalID, base); result != nil {
		return *result
	}
	inv.finishReleaseSets(base)
	landingRoot, configured, refused := inv.landingBatchRoot(targets)
	if refused != nil {
		return *refused
	}
	state, err := owners.branchState(root, goalID)
	if err != nil {
		return intentResult{Targets: targets, Outcome: intentRefused, code: 1, Summary: "the goal branch can't be read, so nothing was landed",
			next: inv.sameCommand(), nextReason: "try again; --verbose shows the cause", Details: []string{err.Error()}}
	}
	if configured {
		// The batch may already hold, or have landed and swept, exactly this
		// selection at this branch tip; its record answers first, including
		// once the branch is gone.
		request := batchowner.BatchJoinRequest{SeatRoot: root, LandingRoot: landingRoot, GoalID: goalID, Through: through, Last: through == ""}
		if _, _, member, err := owners.batchUnit(landingRoot, request, state.BranchTip); err != nil || member {
			return inv.joinBatch(targets, request, state.BranchTip)
		}
	}
	if state.BranchTip == "" {
		if landed, ok := latestLanded(base); ok {
			return intentResult{Targets: targets, Outcome: intentUnchanged, Data: map[string]any{"route": "hand", "landing": landed},
				Summary: fmt.Sprintf("goal %s landed %s on %s and its branch is swept", goalID, landed.Landing, landed.Endpoint)}
		}
		return intentResult{Targets: targets, Outcome: intentRefused, code: 1, Summary: fmt.Sprintf("origin has no goal/%s to land", goalID),
			next: inv.publicArgv("status", goalID), nextReason: "shows the goal's work"}
	}
	subject, count, refusal := handLandingSubject(targets, goalID, through, state)
	if refusal != nil {
		return *refusal
	}
	if refused := inv.admitLanding(targets, goalID, state.BranchTip); refused != nil {
		return *refused
	}
	if !configured {
		return inv.landByHand(targets, goalID, through, subject, state, base, configured)
	}
	// With a landing lane configured, the lane is the one route. It takes
	// only units read by a critic root; a selection holding a unit read by a
	// reader record lands by hand only as the fix of an open red on main.
	unread := slices.IndexFunc(state.Sources[:count], func(source string) bool { return source != "critic-root" })
	if unread < 0 {
		return inv.joinBatch(targets, batchowner.BatchJoinRequest{SeatRoot: root, LandingRoot: landingRoot, GoalID: goalID, Through: through, Last: through == ""}, state.BranchTip)
	}
	entry, err := inv.redOnMainFixed(goalID)
	if err != nil {
		return intentResult{Targets: targets, Outcome: intentRefused, code: 1,
			Summary: fmt.Sprintf("whether goal %s fixes a red on main can't be told, so nothing was landed", goalID),
			next:    inv.publicArgv("goal", "list", "--fetch"), nextReason: "fetches and checks the goal ledger; then repeat this command",
			Details: []string{err.Error()}}
	}
	if entry == "" {
		commit := state.Status.Units[unread].Commit
		return intentResult{Targets: targets, Outcome: intentRefused, code: 1, Data: map[string]any{"route": "batch", "unit": commit, "source": state.Sources[unread]},
			Summary: fmt.Sprintf("unit %s has no critic's read, which the landing lane needs; nothing was landed", commit),
			next:    []string{"metasystem", "work", "review", "--commit", commit, "--goal", goalID}, nextReason: "reads that unit through a critic, after which work land joins the lane",
			Details: []string{"if this goal fixes a red on main, find the incident with metasystem incident list and claim it for the goal (metasystem incident claim E --goal " + goalID +
				"); only the fix of an open trunk red lands by hand, not a flake or a closed incident"}}
	}
	result := inv.landByHand(targets, goalID, through, subject, state, base, configured)
	if data, ok := result.Data.(map[string]any); ok {
		data["redOnMain"] = entry
	}
	return result
}

// redOnMainFixed names the open red-on-main entry the goal is the fix goal
// of, or "" when it fixes none, read from a freshly fetched ledger so an
// incident claimed moments ago on another seat counts. Only a trunk red holds
// landings, so only a trunk red opens the hand route beside a configured
// landing lane.
func (inv *intentInvocation) redOnMainFixed(goalID string) (string, error) {
	if read := inv.delivery().redOnMain; read != nil {
		return read(inv, goalID)
	}
	endpoint, err := inv.owners.dependencies.endpoint(inv.stateRoot)
	if err != nil {
		return "", err
	}
	now, err := inv.owners.commandNow(inv.stateRoot)
	if err != nil {
		return "", err
	}
	projection, err := goal.Project(endpoint, true, now)
	if err != nil {
		return "", err
	}
	for _, entry := range projection.Tree.TrunkRed {
		if entry.Closed == nil && entry.FixGoal == goalID && entry.EntryClass() == goal.TrunkRedClassTrunkRed {
			return entry.ID, nil
		}
	}
	return "", nil
}

// resumeSweep finishes a pushed hand landing whose merged branch was not yet
// swept, before anything else reads the goal branch.
func (inv *intentInvocation) resumeSweep(targets []intentTarget, goalID, base string) *intentResult {
	entries, _ := filepath.Glob(filepath.Join(base, "*", "landed.json"))
	for _, path := range entries {
		var landed intentLanded
		if encoded, err := os.ReadFile(path); err != nil || json.Unmarshal(encoded, &landed) != nil || landed.Landing == "" || landed.Swept {
			continue
		}
		data := map[string]any{"route": "hand", "landing": landed, "retained": filepath.Dir(path)}
		if err := inv.delivery().sweep(inv.layout.InstallationRoot, goalID, landed.Landing); err != nil {
			return &intentResult{Targets: targets, Outcome: intentPartial, code: 1, Data: data,
				Summary: fmt.Sprintf("landed %s on %s, and the merged goal branch is still not swept: %v", landed.Landing, landed.Endpoint, err),
				next:    inv.sameCommand(), nextReason: "retries the branch owner's sweep of the pushed landing"}
		}
		landed.Swept = true
		data["landing"] = landed
		if err := writeIntentInputs(filepath.Dir(path), map[string]string{path: mustJSON(landed)}); err != nil {
			data["recordError"] = err.Error()
		}
		if released, err := inv.runReleaseSet(path); err != nil {
			data["releaseError"] = err.Error()
		} else {
			landed = released
			data["landing"] = landed
		}
		return &intentResult{Targets: targets, Outcome: intentConfirmed, Data: data,
			Summary: fmt.Sprintf("landed %s on %s and swept the merged goal branch; goal %s stays open until done%s", landed.Landing, landed.Endpoint, goalID, releaseSummary(landed.ReleaseSet))}
	}
	return nil
}

func latestLanded(base string) (intentLanded, bool) {
	entries, _ := filepath.Glob(filepath.Join(base, "*", "landed.json"))
	var latest intentLanded
	var latestTime time.Time
	for _, path := range entries {
		info, err := os.Stat(path)
		var landed intentLanded
		if encoded, readErr := os.ReadFile(path); err == nil && readErr == nil && json.Unmarshal(encoded, &landed) == nil && landed.Swept && info.ModTime().After(latestTime) {
			latest, latestTime = landed, info.ModTime()
		}
	}
	return latest, latest.Landing != ""
}

func (inv *intentInvocation) landByHand(targets []intentTarget, goalID, through, subject string, state intentBranchState, base string, batchConfigured bool) intentResult {
	root := inv.layout.InstallationRoot
	owners := inv.delivery()
	dir := filepath.Join(base, shortCommit(subject)+"-"+shortCommit(state.EndpointTip))
	data := map[string]any{"route": "hand", "subject": subject, "endpointTip": state.EndpointTip, "retained": dir, "batchConfigured": batchConfigured}
	landedPath := filepath.Join(dir, "landed.json")
	var landed intentLanded
	if encoded, err := os.ReadFile(landedPath); err == nil && json.Unmarshal(encoded, &landed) == nil && landed.Landing != "" {
		data["landing"] = landed
		return intentResult{Targets: targets, Outcome: intentUnchanged, Data: data,
			Summary: fmt.Sprintf("goal %s already landed %s on %s", goalID, landed.Landing, landed.Endpoint)}
	}
	writeHandLandingCard(inv.stderr, root, goalID, board.StageLanding)
	selection := []string{"--last"}
	if through != "" {
		selection = []string{"--through", through}
	}
	prepared := filepath.Join(dir, "prepared")
	if _, err := os.Stat(prepared); err != nil {
		candidate, code, err := owners.landCandidate(append([]string{"--root", root, "--goal", goalID}, selection...))
		if err != nil {
			return intentResult{Targets: targets, Outcome: intentRefused, code: max(code, 1), Data: data, Summary: "the landing can't be put together: " + err.Error(),
				next: inv.sameCommand(), nextReason: "once that is settled"}
		}
		data["candidate"] = candidate.Result.Candidate
		receipt := filepath.Join(dir, "receipt-"+shortCommit(candidate.Result.Candidate)+".json")
		if !receiptProves(receipt, candidate.Result.Candidate) {
			if result := inv.prepareReceipt(targets, data, goalID, candidate.Result.Candidate, dir, receipt); result != nil {
				return *result
			}
		}
		data["receipt"] = receipt
		outcome, code, err := owners.landPrep(append([]string{"--root", root, "--goal", goalID, "--out", prepared, "--test-receipt", receipt}, selection...))
		if err != nil {
			return intentResult{Targets: targets, Outcome: intentRefused, code: max(code, 1), Data: data, Summary: "the landing couldn't be prepared: " + err.Error(),
				next: inv.sameCommand(), nextReason: "once that is settled"}
		}
		data["prepared"] = outcome.Result
		if outcome.Classification != "" {
			data["classification"] = outcome.Classification
			return intentResult{Targets: targets, Outcome: intentRefused, code: 1, Data: data,
				Summary: fmt.Sprintf("the landing checks of %s failed (%s); nothing was pushed", goalID, outcome.Classification),
				next:    inv.sameCommand(), nextReason: "after fixing the failing checks on the goal branch; the new tip is checked again"}
		}
	}
	// The proof took its time; the gate is read again against the fresh
	// ledger right before the publication (g1-s70 D2).
	if refused := inv.admitLanding(targets, goalID, state.BranchTip); refused != nil {
		refused.Data = data
		return *refused
	}
	selected, setErr := inv.recordReleaseSet(dir, landedPath, goalID, subject)
	if setErr != nil {
		data["releaseError"] = setErr.Error()
	}
	pushed, endpoint, code, err := owners.landPush([]string{"--root", root, "--goal", goalID, "--prepared", prepared})
	if pushed.Landing == "" {
		data["pushError"] = fmt.Sprint(err)
		return intentResult{Targets: targets, Outcome: intentPartial, code: max(code, 1), Data: data,
			Summary: fmt.Sprintf("the landing of %s is proved and prepared in %s but not pushed: %v", goalID, prepared, err),
			next:    inv.sameCommand(), nextReason: "pushes the prepared landing; its checks are reused"}
	}
	landed = intentLanded{Landing: pushed.Landing, Endpoint: endpoint, Branch: pushed.Branch, Subject: subject, Swept: err == nil, ReleaseSet: selected.ReleaseSet}
	data["landing"] = landed
	writeHandLandingCard(inv.stderr, root, goalID, board.StageLanded)
	if writeErr := writeIntentInputs(dir, map[string]string{landedPath: mustJSON(landed)}); writeErr != nil {
		data["recordError"] = writeErr.Error()
	}
	if released, releaseErr := inv.runReleaseSet(landedPath); releaseErr != nil {
		data["releaseError"] = releaseErr.Error()
	} else {
		landed = released
	}
	data["landing"] = landed
	if err != nil {
		return intentResult{Targets: targets, Outcome: intentPartial, code: max(code, 1), Data: data,
			Summary: fmt.Sprintf("landed %s on %s, but the merged goal branch was not swept: %v", pushed.Landing, endpoint, err),
			next:    inv.sameCommand(), nextReason: "retries the branch owner's sweep of the pushed landing"}
	}
	return intentResult{Targets: targets, Outcome: intentConfirmed, Data: data,
		Summary: fmt.Sprintf("landed %s on %s; goal %s stays open until done%s", pushed.Landing, endpoint, goalID, releaseSummary(landed.ReleaseSet))}
}

// writeHandLandingCard projects the hand route onto the goal's board card
// (D14, R24): landing when the route begins, with this landing process as
// owner, so an abandoned attempt reads as a dead owner and the goal's next
// real transition overwrites it; landed after the push. The card is the
// goal's live card, or this installation's own seat. A card that cannot be
// written is reported and the landing goes on.
func writeHandLandingCard(stderr io.Writer, root, goalID string, stage board.Stage) {
	home, err := board.Home()
	if err != nil {
		return
	}
	seat, found := board.Seat{}, false
	if live, ok := board.LiveCard(home, goalID); ok {
		seat, found = live.Seat, true
	} else if machine, resolveErr := goal.ResolveMachine(root); resolveErr == nil {
		seat, found = board.Seat{Machine: machine, Installation: realpath.Resolve(root)}, true
	}
	if !found {
		return
	}
	card := board.Card{Seat: seat, Goal: goalID, Stage: stage, Writer: board.Writer{Component: "work-land"}}
	if stage == board.StageLanding {
		card.Owner = board.Self()
	}
	if err := board.Write(card); err != nil {
		fmt.Fprintf(stderr, "work land %s: the board card was not written: %v\n", goalID, err)
	}
}

// receiptProves reports whether a retained receipt is a schema-3 receipt of
// exactly this landing candidate.
func receiptProves(path, candidate string) bool {
	encoded, err := os.ReadFile(path)
	var receipt landing.TestReceipt
	return err == nil && json.Unmarshal(encoded, &receipt) == nil && receipt.SchemaVersion == 3 && receipt.Tree == candidate
}

// handLandingSubject is the unit commit a landing ends at and the number of
// units through it, provided every one of them has a clean read.
func handLandingSubject(targets []intentTarget, goalID, through string, state intentBranchState) (string, int, *intentResult) {
	units := state.Status.Units
	unread := func(index int) *intentResult {
		return &intentResult{Targets: targets, Outcome: intentRefused, code: 1,
			Summary: fmt.Sprintf("unit %s of goal %s has no clean read, so it cannot land", units[index].Commit, goalID),
			next:    []string{"metasystem", "work", "review", "--commit", units[index].Commit, "--goal", goalID}, nextReason: "reads that unit"}
	}
	if len(units) == 0 {
		return "", 0, &intentResult{Targets: targets, Outcome: intentRefused, code: 1, Summary: fmt.Sprintf("goal/%s has no committed work to land", goalID),
			Decision: "nothing to do; build and review the goal's work first"}
	}
	if len(state.Sources) < state.Status.Prefix {
		return "", 0, &intentResult{Targets: targets, Outcome: intentFailed, Summary: "the goal branch's reviews can't be matched to its commits, so nothing was landed",
			next: []string{"metasystem", "status", goalID}, nextReason: "shows the goal's work and its reviews",
			Details: []string{"the branch reader returned no attestation source for a read unit"}}
	}
	if through == "" {
		if state.Status.Prefix != len(units) {
			return "", 0, unread(state.Status.Prefix)
		}
		return state.BranchTip, len(units), nil
	}
	for index, unit := range units {
		if unit.Commit == through {
			if index >= state.Status.Prefix {
				return "", 0, unread(state.Status.Prefix)
			}
			return unit.Commit, index + 1, nil
		}
	}
	return "", 0, &intentResult{Targets: targets, Outcome: intentRefused, code: 2, Summary: fmt.Sprintf("--through %s is not a work commit on goal/%s; nothing was landed", through, goalID),
		next: []string{"metasystem", "status", goalID}, nextReason: "lists the goal's commits; --through takes one in full"}
}

// prepareReceipt runs the landing proof on the subject tree through the
// landing test-receipt owner and retains its typed receipt.
func (inv *intentInvocation) prepareReceipt(targets []intentTarget, data map[string]any, goalID, subject, dir, receipt string) *intentResult {
	// The landing proof runs in this process (design 6.2): this process is
	// the caller its admission classifies, the parent the former child
	// classified.
	caller, installation := ownercall.CurrentProcess(), inv.layout.InstallationRoot
	args := []string{"--root", installation, "--tree", subject, "--mode", "auto", "--goal", goalID}
	ran := ownerCall(func(stdout, stderr io.Writer) int {
		return inv.ownerCalls().landingTestReceipt(caller, stdout, stderr, installation, args)
	})
	var parsed landing.TestReceipt
	encoded := bytes.TrimSpace(ran.stdout)
	if ran.err != nil || ran.code != 0 || json.Unmarshal(encoded, &parsed) != nil || parsed.SchemaVersion != 3 {
		data["exitCode"] = ran.code
		return &intentResult{Targets: targets, Outcome: intentRefused, code: max(ran.code, 1), Data: data,
			Summary: fmt.Sprintf("the landing checks of %s gave no usable result (exit %d); nothing was prepared", goalID, ran.code),
			text:    nonEmptyLines(string(ran.stderr)), next: inv.sameCommand(), nextReason: "tries again; a matching finished run is reused"}
	}
	if err := writeIntentInputs(dir, map[string]string{receipt: string(encoded) + "\n"}); err != nil {
		return &intentResult{Targets: targets, Outcome: intentFailed, Summary: "the landing checks' result can't be saved, so nothing was prepared", Data: data,
			next: inv.sameCommand(), nextReason: "try again; --verbose shows the cause", Details: []string{err.Error()}}
	}
	data["receipt"] = receipt
	return nil
}

func shortCommit(commit string) string {
	if len(commit) > 12 {
		return commit[:12]
	}
	return commit
}

func mustJSON(value any) string {
	encoded, _ := json.MarshalIndent(value, "", "  ")
	return string(encoded) + "\n"
}

// closeJobReview completes a finished job review with the author's
// dispositions: the join is checked and the whole close owner closes the
// critic chain, exactly as close does for that chain.
func (inv *intentInvocation) closeJobReview(job string, reviewed intentResult) intentResult {
	critic := ""
	for _, target := range reviewed.Targets {
		if target.Kind == "job" && target.ID != job {
			critic = target.ID
		}
	}
	if critic == "" {
		if data, ok := reviewed.Data.(map[string]any); ok {
			critic, _ = data["rootJob"].(string)
		}
	}
	if critic == "" || reviewed.Outcome == intentRefused || reviewed.Outcome == intentFailed {
		reviewed.Summary = "the review of job " + job + " has no finished critic to decide yet: " + reviewed.Summary
		if reviewed.Outcome == intentConfirmed {
			reviewed.Outcome, reviewed.code = intentInProgress, 0
		}
		return reviewed
	}
	closed := inv.closeChain(critic)
	closed.Targets = append([]intentTarget{jobTarget(job)}, closed.Targets...)
	if closed.Outcome == intentConfirmed || closed.Outcome == intentUnchanged {
		closed.Summary = fmt.Sprintf("the review of job %s is decided and its chain %s is closed", job, critic)
	}
	return closed
}

// recordWriterPreflight classifies this executing engine for the checkout
// and asks the record-writer authority owner, as the close owner's own
// guards will; it proves permission to start, not that the close completes.
func recordWriterPreflight(root, job string) (string, error) {
	caller, err := classifyVerbCaller(root, int64(os.Getpid()))
	if err != nil {
		return "authority-unestablished", err
	}
	if err := authority.Authorize("record-writer", map[string]any{"class": caller.Class, "holder": caller.Holder,
		"jobId": caller.JobId, "stewardJob": caller.StewardJob}, job); err != nil {
		return "record-writer-refused", err
	}
	return "", nil
}

// closeCriticJob completes an existing review chain named by its critic root
// with the author's decisions, through the same whole close owner that
// review G and review commit use: the join, record-writer authority, live
// processes, caps and accepted findings are the owner's checks. A closed
// chain is not an accepted design, a collected goal read or permission to
// land; it only ends that review.
func (inv *intentInvocation) closeCriticJob(job string) intentResult {
	if !inv.input.has("dispositions") {
		return intentResult{Targets: []intentTarget{jobTarget(job)}, Outcome: intentRefused, code: 2,
			Summary: fmt.Sprintf("job %s is a review; its findings are decided, not reviewed again; nothing was done", job),
			next:    inv.publicArgv("work", "review", dispatchJobPrefix+job, "--dispositions", "FILE"), nextReason: "FILE decides every finding"}
	}
	closed := inv.closeChain(job)
	if closed.Outcome == intentConfirmed || closed.Outcome == intentUnchanged {
		closed.Summary = fmt.Sprintf("review chain %s is closed with the author's decisions; this ends that review only: it accepts no design, collects no goal read and does not permit landing", job)
	}
	return closed
}

// canonicalReviewArgv is the public review of the selected subject, for a
// continuation: the goal's named work when a work item was selected, else
// the committed version itself. It carries no flag of the call that printed
// it, so a continuation never repeats a stale --dispositions or --retry.
func (inv *intentInvocation) canonicalReviewArgv(targets []intentTarget, goalID, unit string) []string {
	for _, target := range targets {
		if target.Kind == "work" && target.ID != "" {
			return inv.publicArgv(append(reviewGoalWords(goalID), "--work", target.ID)...)
		}
	}
	for _, target := range targets {
		if target.Kind == "unit" && target.ID != "" {
			return inv.publicArgv("work", "review", unitRunPrefix+target.ID)
		}
	}
	return inv.publicArgv("work", "review", "--commit", unit, "--goal", goalID)
}

// firstOr is the first value, or fallback when there is none.
func firstOr(values []string, fallback string) string {
	if len(values) > 0 {
		return values[0]
	}
	return fallback
}

// replaceWord is argv with every word that names ref, bare or qualified,
// replaced by with.
func replaceWord(argv []string, ref, with string) []string {
	out := make([]string, 0, len(argv))
	for _, word := range argv {
		if _, id := splitReference(word); word == ref || id == ref {
			word = with
		}
		out = append(out, word)
	}
	return out
}
