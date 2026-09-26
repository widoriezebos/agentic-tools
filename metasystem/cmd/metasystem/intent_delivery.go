package main

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/authority"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/project"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/validate"
)

// The delivery commands review a subject, fold a review's accepted findings
// into a follow-up, close a finished chain and land a goal. Each one resolves
// the named subject to its recorded evidence and hands it to the existing
// owner: the delegate boundary for critic dispatch and follow-ups, the goal
// branch read for a unit commit, dispatch.sh for the whole chain close, and
// the batch join or the hand land-prep and land-push for landing. The owners
// keep their authority, proof and retry identity; these commands never judge
// a finding, certify a standalone result or conclude a goal.

func intentDeliveryCommands() []intentCommand {
	goalFlag := intentFlag{name: "goal", value: "G", usage: "the goal the subject serves (default: the one the record names)"}
	return []intentCommand{
		{
			name: "review", group: "work", primary: true, audience: "both", summary: "independently review a goal's built work, a design, a job or a commit",
			usage: []string{reviewGoalUsage, reviewSubmitUsage, reviewFindingUsage, reviewDesignUsage,
				reviewJobUsage, reviewCommitUsage, reviewChangesUsage, reviewDiffUsage, reviewExplicitGoalUsage},
			helpForms: reviewHelpForms(),
			details: []string{
				"review G records the goal's built result as the candidate and requests its independent examination: the one named with --work,",
				"or the only work item whose result is built. Repeat the same command to see the examination's progress and findings.",
				"An examination with no findings completes by itself: the review closes and its read is collected and published.",
				"Findings stop at the author's decision: the review writes a decisions file bound to this exact examination; decide",
				"each finding (accepted, refuted, out-of-scope or noted) and run review G --dispositions FILE. An accepted material",
				"finding needs a fix first: revise G --after N --brief FILE --dispositions FILE, then review the new attempt.",
				"An examination round that fails without findings is retried with review G --retry N (its round number): the same",
				"review chain examines the same subject once more under its round limit; repeating it rejoins that retry.",
				"review G --finding F --test NAME discharges a finding's obligation with the test that proves it resolved; the",
				"examination is inferred when the goal has exactly one, else named with --review R. The session holding G",
				"discharges in its own name; anyone else's discharge is a person's act.",
				"--changes or --patch PATCH submits work a person or agent wrote as the goal's work (named --work, default main): claims the goal,",
				"commits the change on the goal branch in the goal's worktree, publishes it and requests the same independent examination built work",
				"gets; decisions, closing, collection and landing continue exactly as for built work. --changes takes this checkout's tracked and",
				"untracked changes; system records and the named brief stay behind. From the goal's own worktree the edits are committed in place.",
				"Repeating the command rejoins the committed version. A correction names the version it replaces: --after COMMIT (from status).",
				"design FILE: an independent critique of a design document; the goal comes from the document or --goal.",
				"job: a completed implementer job is reviewed by the code-critic lane; its goal and subject come from the job record.",
				"review job J --dispositions FILE records your decisions on that review's findings and completes the review.",
				"commit SHA: reads one committed version of a goal's work; review commit SHA --goal G --dispositions FILE decides its findings, closes and collects.",
				"Repeat the same command to collect the result; it continues the existing review without starting another.",
				"Findings are for the author to disposition; a review never accepts its own findings.",
				"changes and diff PATCH ask independent readers for feedback on this checkout's current tracked and untracked changes, or on",
				"a supplied patch. It is diagnostic feedback only: no goal review, approval or landing follows from it, and the checkout is not",
				"changed. The result's REF drives show review REF, wait review REF and stop review REF; the same request rejoins its reads,",
				"and --retry N asks for one new attempt after attempt N failed or was stopped.",
				"review goal G reviews goal G even when its id is one of the subject words design, job, unit, commit, changes, diff or goal.",
			},
			flags: []intentFlag{
				goalFlag,
				{name: "work", value: "NAME", usage: "review G: the goal's named work to review"},
				{name: "changes", usage: "review G: submit this checkout's current changes as the goal's work"},
				{name: "patch", value: "PATCH", usage: "review G: submit this patch file as the goal's work"},
				{name: "dispositions", value: "FILE", usage: "review G: the author's decisions, in the bound file the review wrote"},
				{name: "retry", value: "N", usage: "review G or design: examine the subject once more after examination N failed without findings"},
				{name: "after", value: "N", usage: "review design: the examination the decisions answer; with --changes or --patch: the version (commit) being corrected"},
				{name: "finding", value: "F", usage: "review G: discharge this finding's obligation with --test (or the fixture proof)"},
				{name: "test", value: "NAME", usage: "with --finding: the test that proves the finding resolved"},
				{name: "review", aliases: []string{"chain"}, value: "R", advanced: true, usage: "with --finding: the examination, when the goal has more than one"},
				{name: "implementation-chain", value: "J", advanced: true, usage: "with --finding, a fixture obligation: the implementation chain carrying the fix"},
				{name: "artifact", value: "PATH", advanced: true, usage: "with --finding, a fixture obligation: the changed artifact"},
				{name: "result", value: "RUN", advanced: true, usage: "with --finding, a fixture obligation: the retained governed test result"},
				{name: "critic", value: "ROOT", advanced: true, usage: "with --finding, a fixture obligation: the clean code-critic root"},
				intentByFlag, intentLineageFlag, intentFixtureFlag,
				{name: "brief", value: "FILE", advanced: true, usage: "commit review: the accepted implementation brief frozen into the read"},
				{name: "tool-calls", value: "N", usage: "design and job review: the reader's maximum tool calls, stated in its brief"},
				{name: "model", value: "MODEL", usage: "unit and commit review: the critic model, subject to roster authorization; kept with the read"},
				{name: "effort", value: "VALUE", hidden: true, usage: "refused: every review's reasoning effort is set by its hazard class's configuration obligations"},
			},
			maxArgs: 2,
			examples: []string{"metasystem review verbs-match-intent", "metasystem review verbs-match-intent --work discovery", "metasystem review design plans/designs/intent.md --tool-calls 60", "metasystem review job impl-01 --tool-calls 40",
				"metasystem review commit 3f2a9c1 --goal verbs-match-intent"},
			run: runIntentReview,
		},
		{
			name: "revise", group: "work", audience: "agent", summary: "correct a goal's work with a brief: one new attempt, reviewed again",
			usage: []string{"metasystem revise G [--work NAME] [--after N] --brief FILE [--dispositions FILE]", "metasystem revise job R --dispositions FILE --brief FILE"},
			details: []string{
				"Every attempt of a work item has a number N, whether it passed or failed; --after N names the attempt being corrected.",
				"The request (work, N and the brief's exact bytes) is kept before anything starts, so repeating it, even after a lost",
				"response or a failure, reaches the same new attempt and never spends another. Without --after an identical earlier",
				"request is rejoined first; otherwise the newest attempt is corrected. A request against an older attempt is refused.",
				"When the new attempt fails, the same brief with --after of that attempt's number deliberately makes one more attempt.",
				"Without --work: the only work item that failed, else the only one that finished. The result must be reviewed again.",
				"--dispositions is the decisions file review G wrote for an examination of attempt R (R may be before N). Its findings",
				"and decisions are frozen with the request and stay the builder's input across failed corrections; a file answering an",
				"examination that a later attempt's completed examination superseded is refused.",
				"revise job R continues a finished job review's implementer chain with the author's decisions on every finding.",
			},
			flags: []intentFlag{
				{name: "work", value: "NAME", usage: "the goal's named work to correct"},
				{name: "after", value: "N", usage: "the attempt being corrected (default: rejoin the same request, else the newest attempt)"},
				intentBriefFlag,
				{name: "dispositions", value: "FILE", usage: "the bound decisions file of the reviewed examination"},
			},
			maxArgs:  2,
			examples: []string{"metasystem revise verbs-match-intent --brief fix.md", "metasystem revise job crit-01 --dispositions r1-dispositions.md --brief r1-fix.md", "metasystem revise verbs-match-intent --work discovery --after 2 --brief fix.md"},
			run:      runIntentRevise,
		},
		{
			name: "fold", group: "work", compatibility: true, replacedBy: "metasystem revise G --brief FILE, or metasystem revise job R --dispositions FILE --brief FILE", audience: "agent", summary: "fold a review's dispositioned findings into a follow-up round",
			usage: []string{"metasystem fold review R --dispositions FILE --brief FILE", "metasystem fold unit U --brief FILE"},
			details: []string{
				"review: every finding of review chain R must have exactly one disposition row; the join is checked before anything starts.",
				"The follow-up continues the reviewed implementer chain with the author's brief. Dispositions are the author's decisions, never generated.",
				"unit: the unit run's own follow-up. The unit runner reuses the run's plan and proof, builds the follow-up and reads it again, and ends awaiting judgement; a run may not go past the round limit it started with.",
				"After a unit follow-up, metasystem review unit RUN commits the new round as a replacement of the unit's earlier commit and requests a fresh committed review; the earlier read does not carry over.",
			},
			flags: []intentFlag{
				{name: "dispositions", value: "FILE", usage: "the Markdown dispositions table (Finding id | Disposition | Reasoning and evidence | Amendment)"},
				{name: "brief", value: "FILE", usage: "the follow-up brief"},
			},
			maxArgs: 2,
			examples: []string{"metasystem fold review crit-01 --dispositions plans/r1-dispositions.md --brief plans/r1-fix.md",
				"metasystem fold unit 20260925T101500Z-abc123 --brief follow-up.md"},
			run: runIntentFold,
		},
		{
			name: "close", group: "work", compatibility: true, replacedBy: "metasystem review job J --dispositions FILE, review commit SHA --goal G --dispositions FILE, review design FILE --dispositions FILE or review G --dispositions FILE", audience: "agent", summary: "close a finished job chain after its findings are dispositioned",
			usage: []string{"metasystem close J [--dispositions FILE] [--reconcile-evidence JOB]"},
			details: []string{
				"J is the chain's root job. A review chain needs --dispositions; the join is checked and its out-of-scope rows are recorded first.",
				"The whole close then runs: chain lock, evidence mirrors, register close, close check and the closed stamp.",
				"An already closed chain is reported unchanged; a refusal names what the close owner found open.",
			},
			flags: []intentFlag{
				{name: "dispositions", value: "FILE", usage: "the Markdown dispositions table for a review chain"},
				{name: "reconcile-evidence", value: "JOB", advanced: true, usage: "a review evidence job reconciled into the chain before closing"},
			},
			maxArgs:  1,
			examples: []string{"metasystem close crit-01 --dispositions plans/r1-dispositions.md"},
			run:      runIntentClose,
		},
		{
			name: "land", group: "work", primary: true, audience: "both", summary: "land a goal's reviewed work",
			usage: []string{"metasystem land G [--through COMMIT]", "metasystem land G --queue-only", "metasystem land job J",
				"metasystem land G --exception CODE --reason TEXT --by NAME [--expires 2h] [--replace-exception ID [--transfer]] [--upgrade-goals]",
				"metasystem land G --using-exception ID"},
			details: []string{
				"--queue-only marks the held goal built and waiting to land, and nothing else: no proof runs, no read is collected and nothing is pushed.",
				"--exception is a person's explicit act, never implied by land G: the goal's whole landing candidate is computed, the",
				"exception past exactly one refusal code (or one group:NAME) is recorded with the enrolled person's proof, and the carried",
				"landing delivers it from this checkout's main. Repeating the request rejoins the recorded exception; a changed candidate",
				"needs --replace-exception. --using-exception ID lands under an exception already recorded, locally or through the channel.",
				"The claim leaves the one-claim quota and its elapsed fence until it lands; each machine has one landing slot.",
				"With landing.batch-root configured, the goal branch (or the certified chain) joins the landing batch, which proves and pushes it.",
				"Without it, the read-clean goal branch is proved on its landing candidate, prepared and pushed by hand.",
				"Missing reads, proof or approval refuse with the missing input; no other route is tried instead.",
				"A repeat reuses the retained receipt and prepared landing; a moved endpoint starts from a new proof. The goal is not concluded: that stays done G.",
			},
			flags: []intentFlag{
				{name: "through", value: "COMMIT", usage: "land a human-approved prefix ending at this unit commit"},
				{name: "queue-only", usage: "only mark the held goal waiting to land, for a later land G"},
				{name: "exception", value: "CODE", advanced: true, usage: "a person's exception: the one refusal code or group:NAME this landing is carried past"},
				{name: "reason", value: "TEXT", advanced: true, usage: "with --exception: why"},
				{name: "by", value: "NAME", advanced: true, usage: "with --exception: the person deciding, at the enrolled terminal"},
				{name: "expires", value: "DURATION", advanced: true, usage: "with --exception: how long it stays usable (default 2h, at most 4h)"},
				{name: "replace-exception", value: "ID", advanced: true, usage: "with --exception: the unused exception this one replaces"},
				{name: "transfer", advanced: true, usage: "with --replace-exception: take over an exception recorded on another seat"},
				{name: "upgrade-goals", advanced: true, usage: "with --exception: raise the goal ledger's format in the same act"},
				{name: "using-exception", value: "ID", advanced: true, usage: "land under this already recorded exception (local or answered through the channel)"},
				intentLineageFlag,
			},
			maxArgs:  2,
			examples: []string{"metasystem land verbs-match-intent", "metasystem land verbs-match-intent --queue-only", "metasystem land verbs-match-intent --through 3f2a9c1", "metasystem land job impl-01"},
			run:      runIntentLand,
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
	executable   func() (string, error)
	branchRead   func([]string) (branch.BranchReadResult, int, error)
	branchState  func(root, goalID string) (intentBranchState, error)
	landPrep     func([]string) (goalBranchLandPrepOutcome, int, error)
	landPush     func([]string) (branch.PreparedLanding, string, int, error)
	// landCandidate composes the pending landing and returns the candidate
	// tree its receipt must prove.
	landCandidate func([]string) (goalBranchLandPrepOutcome, int, error)
	sweep         func(root, goalID, landing string) error
	// batchUnit finds the batch member a land request names; branchTip is the
	// live goal branch, or empty once the branch is gone.
	batchUnit    func(landingRoot string, request batchJoinRequest, branchTip string) (batch.Record, batch.Unit, bool, error)
	publishRead  func(root, goalID, unit string) (branch.PublishReadResult, error)
	batchRoot    func(root string, now time.Time) (string, bool, error)
	batchJoin    func(batchJoinRequest) (batch.Record, error)
	now          func() time.Time
	foldUnitHook func(inv *intentInvocation, unitRun string) int
}

// intentBranchState is the goal branch as the landing owners read it.
type intentBranchState struct {
	EndpointTip, BranchTip string
	Status                 branch.Status
	// Sources is each read unit's attestation source kind, in branch order.
	Sources []string
}

func defaultIntentDeliveryOwners() *intentDeliveryOwners {
	return &intentDeliveryOwners{
		recordWriter: recordWriterPreflight,
		process:      runIntentOwnerProcess,
		executable:   os.Executable,
		branchRead: func(args []string) (branch.BranchReadResult, int, error) {
			binary, err := os.Executable()
			if err != nil {
				return branch.BranchReadResult{}, 1, err
			}
			return goalBranchReadRun(args, goalBranchReadDependencies{Binary: binary})
		},
		branchState: productionIntentBranchState,
		landPrep: func(args []string) (goalBranchLandPrepOutcome, int, error) {
			return goalBranchLandPrepRun(args, goalBranchLandPrepDependencies{Prepare: branch.PrepareLanding,
				LoadContract: func(root string) (testpolicy.Contract, error) {
					_, contract, _, err := loadPhysicalTestingContract(root)
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
		batchJoin: func(request batchJoinRequest) (batch.Record, error) {
			return executeBatchJoin(request, batchJoinDependenciesForCommand())
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
	endpoint, err := goalBranchEndpoint(root)
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
func productionIntentBatchUnit(landingRoot string, request batchJoinRequest, branchTip string) (batch.Record, batch.Unit, bool, error) {
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
func intentBatchMember(unit batch.Unit, request batchJoinRequest, branchTip string) bool {
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

// productionIntentBatchRoot reports whether landing.batch-root is set and,
// when it is, the checkout the existing resolver admits.
func productionIntentBatchRoot(root string, now time.Time) (string, bool, error) {
	confPath := filepath.Join(root, "metasystem.conf")
	raw, _, err := config.Get(config.GetParams{Key: config.BatchRootKey, ConfPath: confPath, Default: "", DefaultSet: true})
	if err != nil {
		return "", false, err
	}
	if strings.TrimSpace(raw) == "" {
		return "", false, nil
	}
	settings, err := config.ResolveBatchLanding(confPath, root, func() time.Time { return now })
	if err != nil {
		return "", true, err
	}
	return settings.Root, true, nil
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
		return nil, fmt.Errorf("job %s has no readable record: %v", id, err)
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

func (inv *intentInvocation) sameCommand() []string {
	return append([]string{"metasystem", inv.command.name}, inv.raw...)
}

// ---- review

func runIntentReview(inv *intentInvocation) int {
	manual := inv.input.has("changes") || inv.input.has("patch")
	switch args := inv.input.args; {
	case manual && len(args) == 1 && !slices.Contains(reviewSubjectWords, args[0]):
		return runIntentReviewManual(inv, args[0])
	case manual && len(args) == 2 && args[0] == "goal":
		return runIntentReviewManual(inv, args[1])
	case manual:
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "--changes and --patch submit a goal's work: review G --changes|--patch PATCH --brief FILE; nothing was done"})
	case len(args) == 1 && !slices.Contains(reviewSubjectWords, args[0]):
		return runIntentReviewGoal(inv, args[0])
	case len(args) == 2 && args[0] == "goal":
		return runIntentReviewGoal(inv, args[1])
	case len(args) == 1 && args[0] == "changes":
		return runIntentReviewDiagnostic(inv, "")
	case len(args) == 2 && args[0] == "diff":
		return runIntentReviewDiagnostic(inv, args[1])
	}
	if inv.input.has("work") {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "--work names a goal's work: review G --work NAME; nothing was done"})
	}
	for _, number := range []string{"retry", "after"} {
		if value, err := strconv.Atoi(inv.input.text(number)); inv.input.has(number) && (err != nil || value < 1) {
			return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: fmt.Sprintf("--%s takes an examination number such as 1; nothing was done", number)})
		}
	}
	if len(inv.input.args) != 2 {
		return inv.render(intentResult{Outcome: intentRefused, code: 2,
			Summary:    "review names what to review: G, goal G, design FILE, job J, unit RUN, commit SHA, changes or diff PATCH",
			nextReason: "for example: metasystem review design plans/designs/intent.md"})
	}
	if result := inv.selectRoot(); result != nil {
		return inv.render(*result)
	}
	kind, subject := inv.input.args[0], inv.input.args[1]
	if inv.input.has("effort") {
		return inv.render(intentResult{Outcome: intentRefused, code: 2,
			Summary:  "no review owner accepts a reasoning-effort override: dispatch sets it from the hazard class's configuration obligations",
			Decision: "omit --effort; the roster and the destructive-reach class decide the critic's effort"})
	}
	if inv.input.has("model") && kind != "commit" && kind != "unit" {
		return inv.render(intentResult{Outcome: intentRefused, code: 2,
			Summary:  fmt.Sprintf("review %s takes no --model: the delegate accepts a critic model override only for a unit or commit read's code critic", kind),
			Decision: "omit --model, or review the built unit with metasystem review unit RUN --model MODEL"})
	}
	switch kind {
	case "design":
		return inv.render(inv.reviewDesign(subject))
	case "job":
		// The reference status work prints selects its store; review reads a
		// dispatch job, always by its raw id.
		job, problem := inv.resolveJob(subject, "review")
		qualified := strings.HasPrefix(subject, launchJobPrefix) || strings.HasPrefix(subject, dispatchJobPrefix)
		switch {
		case problem != nil && qualified:
			return inv.render(*problem)
		case problem != nil:
			if data, _ := problem.Data.(map[string]any); data["candidates"] != nil {
				return inv.render(*problem)
			}
		case job.kind == "launch":
			return inv.render(intentResult{Outcome: intentRefused, code: 2, Targets: []intentTarget{{Kind: "job", ID: jobReference(job)}},
				Summary: fmt.Sprintf("%s is a launch; review job reads a dispatch job (j2:ID); nothing was done", jobReference(job))})
		default:
			subject = job.id
		}
		if record, err := inv.jobRecord(subject); err == nil && criticRole(recordText(record, "role")) {
			return inv.render(inv.closeCriticJob(subject))
		}
		result := inv.reviewJob(subject)
		if inv.input.has("dispositions") {
			return inv.render(inv.closeJobReview(subject, result))
		}
		return inv.render(result)
	case "commit":
		return inv.render(inv.reviewCommit(subject))
	case "unit":
		return inv.render(inv.reviewUnit(subject))
	}
	return inv.render(intentResult{Outcome: intentRefused, code: 2,
		Summary:    fmt.Sprintf("review has no subject kind %q", kind),
		nextReason: "the kinds are design FILE, job J and commit SHA"})
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
		return record, data, fmt.Errorf("%s is not a design document of a goal; metasystem design G writes one, or name the goal's existing design", path)
	}
	return record, data, nil
}

// reviewBriefFacts are the parts of a review brief that come from recorded
// state: the goal's approved review rounds and the caller's reader budget.
func (inv *intentInvocation) reviewBriefFacts(targets []intentTarget, goalID string) (int64, int, *intentResult) {
	calls := 0
	if _, err := fmt.Sscanf(inv.input.text("tool-calls"), "%d", &calls); err != nil || calls < 1 {
		return 0, 0, &intentResult{Targets: targets, Outcome: intentRefused, code: 2,
			Summary:  "a review brief states the reader's tool-call budget, and none is configured",
			Decision: "name it with --tool-calls N"}
	}
	projection, _, failed := inv.projection()
	if failed != nil {
		failed.Targets = targets
		return 0, 0, failed
	}
	file := projection.Tree.Live[goalID]
	if file == nil || file.Approved == nil || file.Budget == nil || file.Budget.ReviewRoundLimit < 1 {
		return 0, 0, &intentResult{Targets: targets, Outcome: intentRefused, code: 1,
			Summary:  fmt.Sprintf("goal %s has no approved review-round budget for this review", goalID),
			Decision: fmt.Sprintf("approve goal %s with a budget first: metasystem approve %s", goalID, goalID)}
	}
	return file.Budget.ReviewRoundLimit, calls, nil
}

// reviewBrief renders scripts/agents/templates/review-brief.md for one
// subject. Every value is recorded state or the caller's explicit input.
const designCritiqueRounds = 2

func reviewBrief(mode, chain, goalID string, rounds int64, calls int, threat, scope, copyPath, contents, findings string, checklist []string) string {
	lines := []string{
		"Working Mode: " + mode,
		"Orchestrator Identity: metasystem review",
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
		return intentResult{Targets: target, Outcome: intentRefused, code: 1, Summary: "the project's design homes are unreadable: " + err.Error()}
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
			Summary: fmt.Sprintf("%s is not in one of this project's design homes (%s)", file, strings.Join(homes, ", "))}
	}
	if !strings.HasPrefix(gitRel, "metasystem/") && !dispatchcore.RepositoryDesignPath(gitRel) {
		return intentResult{Targets: target, Outcome: intentRefused, code: 2,
			Summary: fmt.Sprintf("design %s is outside the paths the design-critic subject admits (metasystem/ or plans/designs/)", gitRel)}
	}
	record, data, err := readIntentDesignRecord(resolved)
	if err != nil {
		return intentResult{Targets: target, Outcome: intentRefused, code: 2, Summary: err.Error()}
	}
	goalID := inv.input.text("goal")
	if goalID == "" {
		if len(record.Goals) != 1 {
			return intentResult{Targets: target, Outcome: intentRefused, code: 2,
				Summary:  fmt.Sprintf("design %s names %d goals; the review serves one", record.ID, len(record.Goals)),
				Decision: "name the goal with --goal G"}
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
	if err := writeIntentInputs(dir, map[string]string{brief: briefText, outputs: gitRel + "\n"}); err != nil {
		return intentResult{Targets: target, Outcome: intentFailed, Summary: err.Error()}
	}
	designPath := filepath.Join(git, filepath.FromSlash(gitRel))
	plan := designReviewPlan{targets: target, goalID: goalID, recordID: record.ID, design: designPath, subject: hex.EncodeToString(digest[:]), brief: brief}
	if decided := inv.reviewDesignChain(plan); decided != nil {
		return *decided
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
		return intentResult{Targets: target, Outcome: intentRefused, code: 2, Summary: err.Error()}
	}
	if role := recordText(record, "role"); role != "implementer" {
		return intentResult{Targets: target, Outcome: intentRefused, code: 2,
			Summary: fmt.Sprintf("job %s is a %s job; review job names an implementer job", job, role)}
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
			Summary: fmt.Sprintf("job %s records no goal", job), Decision: "name the goal with --goal G"}
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
		return intentResult{Targets: target, Outcome: intentFailed, Summary: err.Error()}
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

// delegate runs `metasystem delegate` and reads its typed outcome.
func (inv *intentInvocation) delegate(targets []intentTarget, args []string) (delegateOutcome, *intentResult) {
	owners := inv.delivery()
	binary, err := owners.executable()
	if err != nil {
		return delegateOutcome{}, &intentResult{Targets: targets, Outcome: intentFailed, Summary: err.Error()}
	}
	ran := owners.process(intentProcess{argv: append([]string{binary, "delegate"}, args...), dir: inv.layout.InstallationRoot})
	var outcome delegateOutcome
	if ran.err != nil || json.Unmarshal(bytes.TrimSpace(ran.stdout), &outcome) != nil || outcome.Outcome == "" {
		detail := "the delegate boundary returned no typed outcome"
		if ran.err != nil {
			detail = ran.err.Error()
		}
		return outcome, &intentResult{Targets: targets, Outcome: intentInProgress,
			Summary:    "the dispatch outcome is unknown: " + detail,
			Data:       map[string]any{"exitCode": ran.code},
			next:       inv.sameCommand(),
			nextReason: "the same request is recovered under its recorded dispatch identity; it is not dispatched again"}
	}
	switch {
	case outcome.JobID != "" && !strings.HasPrefix(outcome.Outcome, "REFUSED"):
		return outcome, nil
	case outcome.Outcome == "RECONCILING" || outcome.Outcome == "IN-PROGRESS" || outcome.Outcome == "BOUND":
		return outcome, &intentResult{Targets: targets, Outcome: intentInProgress,
			Summary: "the dispatch is " + strings.ToLower(outcome.Outcome) + " and names no job yet", Data: map[string]any{"delegate": outcome},
			next: inv.sameCommand(), nextReason: "collects the same dispatch once it names its job"}
	}
	return outcome, &intentResult{Targets: targets, Outcome: intentRefused, code: max(ran.code, 1),
		Summary: strings.TrimSpace(outcome.Outcome + ": " + outcome.Detail), Data: map[string]any{"delegate": outcome}}
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
		return intentResult{Targets: targets, Outcome: intentFailed, Summary: fmt.Sprintf("review job %s ended %s (%s)", job, status, recordText(record, "reason")), Data: data}
	}
	path := inv.returnPath(root, recordRound(record))
	findings, verdict, err := readIntentFindings(path)
	if err != nil {
		return intentResult{Targets: targets, Outcome: intentFailed, Summary: fmt.Sprintf("review job %s completed without a readable return: %v", job, err), Data: data}
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
			result.text = []string{"correct: " + shellCommand(inv.publicArgv("revise", "job", job, "--dispositions", "FILE", "--brief", "FILE")),
				"close: " + shellCommand(inv.publicArgv("review", "job", reviewed, "--dispositions", "FILE"))}
		} else {
			result.text = []string{"close: " + shellCommand(inv.publicArgv("close", job, "--dispositions", "FILE"))}
		}
	} else if len(targets) > 0 && targets[0].Kind == "job" {
		result.next, result.nextReason = inv.publicArgv("review", "job", targets[0].ID, "--dispositions", "FILE"), "close the review with the author's (empty) decisions"
	}
	return result
}

func (inv *intentInvocation) reviewCommit(unit string) intentResult {
	goalID := inv.input.text("goal")
	targets := []intentTarget{{Kind: "commit", ID: unit}}
	if goalID == "" {
		return intentResult{Targets: targets, Outcome: intentRefused, code: 2,
			Summary: "a commit review reads a unit on one goal branch", Decision: "name the goal with --goal G"}
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
		return &intentResult{Targets: targets, Outcome: intentFailed, Summary: err.Error()}
	}
	if _, closed, err := dispatchcore.ReadClosure(record); err != nil {
		return &intentResult{Targets: targets, Outcome: intentFailed, Summary: fmt.Sprintf("critic %s has a malformed closure: %v", rootJob, err)}
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
				Summary:  fmt.Sprintf("examination round %d of critic %s ended without findings to decide", recordRound(round), rootJob),
				Decision: "examine the subject once more in the same chain, once the old round is proven stopped: " + shellCommand(again)}
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
		Summary: fmt.Sprintf("critic %s has finished reading unit %s, but its chain is not closed", rootJob, unit),
		Decision: fmt.Sprintf("the author decides every finding of %s in a dispositions file, then runs %s",
			rootJob, shellCommand(append(slices.DeleteFunc(inv.sameCommand(), func(word string) bool { return word == "--json" }), "--dispositions", "FILE")))}
}

// ---- fold

func runIntentFold(inv *intentInvocation) int {
	if len(inv.input.args) != 2 || (inv.input.args[0] != "review" && inv.input.args[0] != "unit") {
		return inv.render(intentResult{Outcome: intentRefused, code: 2,
			Summary:    "fold names review R or unit U",
			nextReason: "for example: metasystem fold review crit-01 --dispositions FILE --brief FILE"})
	}
	if inv.input.args[0] == "unit" {
		hook := inv.delivery().foldUnitHook
		if hook == nil {
			hook = runIntentFoldUnit
		}
		return hook(inv, inv.input.args[1])
	}
	if result := inv.selectRoot(); result != nil {
		return inv.render(*result)
	}
	return inv.render(inv.foldReview(inv.input.args[1]))
}

func (inv *intentInvocation) foldReview(review string) intentResult {
	targets := []intentTarget{{Kind: "review", ID: review}}
	if !inv.input.has("dispositions") || !inv.input.has("brief") {
		return intentResult{Targets: targets, Outcome: intentRefused, code: 2,
			Summary: "fold review needs the author's --dispositions FILE and the follow-up --brief FILE"}
	}
	round, result := inv.finishedReview(targets, review)
	if result != nil {
		return *result
	}
	returnPath := inv.returnPath(review, recordRound(round))
	if violations := validate.CritiqueClosed(returnPath, inv.flagPath("dispositions")); len(violations) > 0 {
		return joinRefusal(targets, review, violations)
	}
	root, _ := inv.jobRecord(review)
	subject := recordText(root, "reviews")
	if recordText(root, "role") == "design-critic" || subject == "" {
		return intentResult{Targets: targets, Outcome: intentRefused, code: 2,
			Summary:  fmt.Sprintf("review %s reviewed a design, which has no implementer chain to follow up", review),
			Decision: "the design's author revises the design with these dispositions, then runs metasystem review design FILE again"}
	}
	if strings.HasPrefix(subject, "commit:") {
		return intentResult{Targets: targets, Outcome: intentRefused, code: 2,
			Summary:  fmt.Sprintf("review %s read a goal-branch commit; a unit fix is a new unit commit on the branch", review),
			Decision: "commit the fix on the goal branch and read it with metasystem review commit SHA --goal G"}
	}
	implementer, err := inv.jobRecord(subject)
	if err != nil {
		return intentResult{Targets: targets, Outcome: intentRefused, code: 1, Summary: err.Error()}
	}
	subjectRoot := subject
	if parent := recordText(implementer, "parentJob"); parent != "" {
		subjectRoot = parent
	}
	targets = append(targets, jobTarget(subjectRoot))
	message, err := inv.composeFoldMessage(review, recordRound(round), subject, subjectRoot, returnPath)
	if err != nil {
		return intentResult{Targets: targets, Outcome: intentFailed, Summary: err.Error()}
	}
	outcome, refused := inv.delegate(targets, []string{"--follow-up", subjectRoot, "--brief", message})
	if refused != nil {
		return *refused
	}
	return intentResult{Targets: append(targets, jobTarget(outcome.JobID)), Outcome: intentInProgress,
		Summary: fmt.Sprintf("follow-up %s of chain %s is %s for review %s", outcome.JobID, subjectRoot, strings.ToLower(outcome.Headline), review),
		Data:    map[string]any{"delegate": outcome, "review": review, "reviewRound": recordRound(round), "chain": subjectRoot, "message": message},
		next:    []string{"metasystem", "review", "job", outcome.JobID}, nextReason: "reviews the follow-up once it completes"}
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
		return nil, &intentResult{Targets: targets, Outcome: intentRefused, code: 2, Summary: err.Error()}
	}
	if !criticRole(recordText(root, "role")) {
		return nil, &intentResult{Targets: targets, Outcome: intentRefused, code: 2,
			Summary: fmt.Sprintf("job %s is a %s job, not a review chain", review, recordText(root, "role"))}
	}
	if parent := recordText(root, "parentJob"); parent != "" {
		return nil, &intentResult{Targets: targets, Outcome: intentRefused, code: 2,
			Summary: fmt.Sprintf("job %s is a round of review %s; name the review's root", review, parent)}
	}
	round, err := inv.newestRound(review)
	if err != nil {
		return nil, &intentResult{Targets: targets, Outcome: intentRefused, code: 1, Summary: err.Error()}
	}
	if status := recordText(round, "status"); status != "completed" {
		outcome := intentRefused
		if !dispatchcore.TerminalStatus(status) {
			outcome = intentInProgress
		}
		return nil, &intentResult{Targets: targets, Outcome: outcome,
			Summary: fmt.Sprintf("review %s round %d is %s; dispositions answer a completed round", review, recordRound(round), status)}
	}
	return round, nil
}

func joinRefusal(targets []intentTarget, review string, violations []string) intentResult {
	return intentResult{Targets: targets, Outcome: intentRefused, code: 1,
		Summary:  fmt.Sprintf("the dispositions do not join review %s's findings (%d problems)", review, len(violations)),
		text:     violations,
		Data:     map[string]any{"violations": violations},
		Decision: "the author gives every finding exactly one disposition row"}
}

// ---- close

func runIntentClose(inv *intentInvocation) int {
	if len(inv.input.args) != 1 {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "close names one chain's root job: metasystem close J"})
	}
	if result := inv.selectRoot(); result != nil {
		return inv.render(*result)
	}
	return inv.render(inv.closeChain(inv.input.args[0]))
}

func (inv *intentInvocation) closeChain(job string) intentResult {
	targets := []intentTarget{jobTarget(job)}
	root, err := inv.jobRecord(job)
	if err != nil {
		return intentResult{Targets: targets, Outcome: intentRefused, code: 2, Summary: err.Error()}
	}
	if parent := recordText(root, "parentJob"); parent != "" {
		return intentResult{Targets: targets, Outcome: intentRefused, code: 2,
			Summary: fmt.Sprintf("job %s is a round of chain %s", job, parent), next: []string{"metasystem", "close", parent}, nextReason: "close names the chain's root"}
	}
	if closed, _ := root["chainClosed"].(bool); closed {
		return intentResult{Targets: targets, Outcome: intentUnchanged, Summary: fmt.Sprintf("chain %s is already closed", job), Data: map[string]any{"chainClosed": true}}
	}
	if evidence := inv.input.text("reconcile-evidence"); evidence != "" && !validIntentJobID(evidence) {
		return intentResult{Targets: targets, Outcome: intentRefused, code: 2, Summary: fmt.Sprintf("%q is not a review evidence job id", evidence)}
	}
	writer := inv.delivery().recordWriter
	if writer == nil {
		writer = recordWriterPreflight
	}
	if cause, err := writer(inv.layout.InstallationRoot, job); err != nil {
		result := intentResult{Targets: targets, Outcome: intentRefused, code: 1, Data: map[string]any{"cause": cause},
			Summary: fmt.Sprintf("the close of %s was not started: %v", job, err)}
		if cause == "record-writer-refused" {
			result.Decision = "the close writes the review's records, which only a person, the checkout's lease holder or the chain's own job may do; one of them runs the same command"
		} else {
			result.Decision = "the caller's authority could not be established; nothing was written; run the same command where the checkout's authority can be read"
		}
		return result
	}
	if criticRole(recordText(root, "role")) {
		if !inv.input.has("dispositions") {
			return intentResult{Targets: targets, Outcome: intentRefused, code: 2,
				Summary: fmt.Sprintf("review chain %s closes against its author's dispositions", job), Decision: "supply --dispositions FILE"}
		}
		round, result := inv.finishedReview(targets, job)
		if result != nil {
			return *result
		}
		if violations := validate.CritiqueClosedWithRegister(inv.returnPath(job, recordRound(round)), inv.flagPath("dispositions"),
			inv.layout.InstallationRoot, job); len(violations) > 0 {
			return joinRefusal(targets, job, violations)
		}
	} else if inv.input.has("dispositions") {
		return intentResult{Targets: targets, Outcome: intentRefused, code: 2,
			Summary: fmt.Sprintf("chain %s is a %s chain with no findings of its own; dispositions close its review chain", job, recordText(root, "role"))}
	} else if newest, err := inv.newestRound(job); err == nil && !dispatchcore.TerminalStatus(recordText(newest, "status")) {
		return intentResult{Targets: targets, Outcome: intentInProgress,
			Summary: fmt.Sprintf("chain %s still has round %s %s", job, recordText(newest, "jobId"), recordText(newest, "status"))}
	}
	argv := []string{filepath.Join(inv.layout.InstallationRoot, "scripts", "agents", "dispatch.sh"), "close", "--job", job}
	if evidence := inv.input.text("reconcile-evidence"); evidence != "" {
		argv = append(argv, "--reconcile-evidence", evidence)
	}
	ran := inv.delivery().process(intentProcess{argv: argv, dir: inv.layout.InstallationRoot})
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
			Summary: fmt.Sprintf("the close was refused before it started: %s %s", fenced.Outcome, fenced.Detail)}
	}
	summary := fmt.Sprintf("the close owner stopped (exit %d) and chain %s is not closed", ran.code, job)
	if ran.err != nil {
		summary = fmt.Sprintf("the close owner could not run: %v; chain %s is not closed", ran.err, job)
	}
	data["ownerMessage"] = nonEmptyLines(string(ran.stderr))
	return intentResult{Targets: targets, Outcome: intentRefused, code: max(ran.code, 1), Data: data, Summary: summary,
		text:     nonEmptyLines(string(ran.stderr)),
		Decision: "resolve what the close owner names above, then run the same close again"}
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
	args := inv.input.args
	if inv.input.switched("queue-only") {
		if len(args) != 1 || inv.input.has("through") || inv.input.has("using-exception") || slices.ContainsFunc(exceptionOptions, inv.input.has) {
			return inv.render(intentResult{Outcome: intentRefused, code: 2,
				Summary: "--queue-only takes one goal and no other choice; nothing was done", Decision: "metasystem land G --queue-only"})
		}
		return runIntentQueueOnly(inv, args[0])
	}
	if inv.input.has("lineage") {
		return inv.render(intentResult{Outcome: intentRefused, code: 2,
			Summary: "--lineage names the session that queues a landing; it is taken only with --queue-only; nothing was done"})
	}
	if len(args) == 1 && (inv.input.has("exception") || inv.input.has("using-exception")) {
		if problem := inv.landExceptionInput(args[0]); problem != nil {
			return inv.render(*problem)
		}
	}
	if len(args) == 0 || len(args) > 2 || (len(args) == 2 && args[0] != "job") {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "land names a goal, or job J",
			nextReason: "for example: metasystem land verbs-match-intent"})
	}
	if len(args) == 2 && inv.input.has("through") {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "--through selects goal-branch units; land job J lands its whole certified chain"})
	}
	if result := inv.selectRoot(); result != nil {
		return inv.render(*result)
	}
	if len(args) == 2 {
		for _, other := range append([]string{"using-exception"}, exceptionOptions...) {
			if inv.input.has(other) {
				return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "an exception lands a goal, not a job chain; nothing was done"})
			}
		}
		return inv.render(inv.landJob(args[1]))
	}
	if inv.input.has("exception") || inv.input.has("using-exception") {
		return inv.render(inv.landException(args[0]))
	}
	for _, other := range exceptionOptions {
		if inv.input.has(other) {
			return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: fmt.Sprintf("--%s belongs to an exceptional landing with --exception CODE; nothing was done", other)})
		}
	}
	return inv.render(inv.landGoal(args[0], inv.input.text("through")))
}

func (inv *intentInvocation) landingBatchRoot(targets []intentTarget) (string, bool, *intentResult) {
	owners := inv.delivery()
	root, configured, err := owners.batchRoot(inv.layout.InstallationRoot, owners.now())
	if err != nil {
		return "", configured, &intentResult{Targets: targets, Outcome: intentRefused, code: 1,
			Summary: "the landing batch policy is unreadable: " + err.Error(), Decision: "correct landing.batch-root in metasystem.conf"}
	}
	return root, configured, nil
}

func (inv *intentInvocation) landJob(job string) intentResult {
	targets := []intentTarget{jobTarget(job)}
	record, err := inv.jobRecord(job)
	if err != nil {
		return intentResult{Targets: targets, Outcome: intentRefused, code: 2, Summary: err.Error()}
	}
	goalID := recordText(record, "goalId")
	if goalID == "" {
		return intentResult{Targets: targets, Outcome: intentRefused, code: 1, Summary: fmt.Sprintf("chain %s serves no goal, so it has nothing to land", job)}
	}
	targets = append(targets, intentTarget{Kind: "goal", ID: goalID})
	landingRoot, configured, refused := inv.landingBatchRoot(targets)
	if refused != nil {
		return *refused
	}
	if !configured {
		return intentResult{Targets: targets, Outcome: intentRefused, code: 1,
			Summary:  fmt.Sprintf("certified chain %s lands only through the landing batch, and landing.batch-root is not set", job),
			Decision: "set landing.batch-root to a dedicated landing checkout"}
	}
	return inv.joinBatch(targets, batchJoinRequest{SeatRoot: inv.layout.InstallationRoot, LandingRoot: landingRoot, GoalID: goalID, ChainID: job}, "")
}

// joinBatch reads the goal's existing batch membership first: a joined unit
// is reported where the batch owner has it, and only a goal with no live
// membership joins. The same land command is the continuation until the
// batch records the landing.
func (inv *intentInvocation) joinBatch(targets []intentTarget, request batchJoinRequest, branchTip string) intentResult {
	owners := inv.delivery()
	record, unit, member, err := owners.batchUnit(request.LandingRoot, request, branchTip)
	if err != nil {
		return intentResult{Targets: targets, Outcome: intentFailed, Summary: "the landing batches are unreadable: " + err.Error(), Data: map[string]any{"route": "batch"}}
	}
	joined := false
	if !member {
		request.At = owners.now()
		record, err = owners.batchJoin(request)
		if err != nil {
			return intentResult{Targets: targets, Outcome: intentRefused, code: 1, Summary: err.Error(), Data: map[string]any{"route": "batch"}}
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
		return intentResult{Targets: targets, Outcome: intentConfirmed, Data: data,
			Summary: fmt.Sprintf("goal %s landed through batch %s; the goal stays open until done", request.GoalID, record.BatchID)}
	case batch.UnitEjected, batch.UnitWithdrawn, batch.UnitWithdrawnBudget:
		return intentResult{Targets: targets, Outcome: intentRefused, code: 1, Data: data,
			Summary: fmt.Sprintf("goal %s left batch %s as %s", request.GoalID, record.BatchID, unit.State)}
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
}

func (inv *intentInvocation) landGoal(goalID, through string) intentResult {
	targets := []intentTarget{{Kind: "goal", ID: goalID}}
	if !validIntentJobID(goalID) {
		return intentResult{Targets: targets, Outcome: intentRefused, code: 2, Summary: fmt.Sprintf("%q is not a goal id", goalID)}
	}
	root := inv.layout.InstallationRoot
	owners := inv.delivery()
	base := filepath.Join(root, "artifacts", "agents", "landing-intent", goalID)
	if result := inv.resumeSweep(targets, goalID, base); result != nil {
		return *result
	}
	landingRoot, configured, refused := inv.landingBatchRoot(targets)
	if refused != nil {
		return *refused
	}
	state, err := owners.branchState(root, goalID)
	if err != nil {
		return intentResult{Targets: targets, Outcome: intentRefused, code: 1, Summary: "the goal branch is unreadable: " + err.Error()}
	}
	if configured {
		// The batch may already hold, or have landed and swept, exactly this
		// selection at this branch tip; its record answers first, including
		// once the branch is gone.
		request := batchJoinRequest{SeatRoot: root, LandingRoot: landingRoot, GoalID: goalID, Through: through, Last: through == ""}
		if _, _, member, err := owners.batchUnit(landingRoot, request, state.BranchTip); err != nil || member {
			return inv.joinBatch(targets, request, state.BranchTip)
		}
	}
	if state.BranchTip == "" {
		if landed, ok := latestLanded(base); ok {
			return intentResult{Targets: targets, Outcome: intentUnchanged, Data: map[string]any{"route": "hand", "landing": landed},
				Summary: fmt.Sprintf("goal %s landed %s on %s and its branch is swept", goalID, landed.Landing, landed.Endpoint)}
		}
		return intentResult{Targets: targets, Outcome: intentRefused, code: 1, Summary: fmt.Sprintf("origin has no goal/%s to land", goalID)}
	}
	subject, count, refusal := handLandingSubject(targets, goalID, through, state)
	if refusal != nil {
		return *refusal
	}
	kinds := map[string]bool{}
	for _, source := range state.Sources[:count] {
		kinds[source] = true
	}
	if configured && len(kinds) == 1 && kinds["critic-root"] {
		return inv.joinBatch(targets, batchJoinRequest{SeatRoot: root, LandingRoot: landingRoot, GoalID: goalID, Through: through, Last: through == ""}, state.BranchTip)
	}
	return inv.landByHand(targets, goalID, through, subject, state, base, configured)
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
		return &intentResult{Targets: targets, Outcome: intentConfirmed, Data: data,
			Summary: fmt.Sprintf("landed %s on %s and swept the merged goal branch; goal %s stays open until done", landed.Landing, landed.Endpoint, goalID)}
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
	selection := []string{"--last"}
	if through != "" {
		selection = []string{"--through", through}
	}
	prepared := filepath.Join(dir, "prepared")
	if _, err := os.Stat(prepared); err != nil {
		candidate, code, err := owners.landCandidate(append([]string{"--root", root, "--goal", goalID}, selection...))
		if err != nil {
			return intentResult{Targets: targets, Outcome: intentRefused, code: max(code, 1), Data: data, Summary: "the landing candidate cannot be composed: " + err.Error()}
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
			return intentResult{Targets: targets, Outcome: intentRefused, code: max(code, 1), Data: data, Summary: "land-prep refused: " + err.Error()}
		}
		data["prepared"] = outcome.Result
		if outcome.Classification != "" {
			data["classification"] = outcome.Classification
			return intentResult{Targets: targets, Outcome: intentRefused, code: 1, Data: data,
				Summary:  fmt.Sprintf("the landing proof of %s is red (%s); nothing was pushed", goalID, outcome.Classification),
				Decision: "fix the failing groups on the goal branch; a new branch tip gets a new proof"}
		}
	}
	pushed, endpoint, code, err := owners.landPush([]string{"--root", root, "--goal", goalID, "--prepared", prepared})
	if pushed.Landing == "" {
		data["pushError"] = fmt.Sprint(err)
		return intentResult{Targets: targets, Outcome: intentPartial, code: max(code, 1), Data: data,
			Summary: fmt.Sprintf("the landing of %s is proved and prepared in %s but not pushed: %v", goalID, prepared, err),
			next:    inv.sameCommand(), nextReason: "pushes the retained prepared landing; its proof is reused"}
	}
	landed = intentLanded{Landing: pushed.Landing, Endpoint: endpoint, Branch: pushed.Branch, Subject: subject, Swept: err == nil}
	data["landing"] = landed
	if writeErr := writeIntentInputs(dir, map[string]string{landedPath: mustJSON(landed)}); writeErr != nil {
		data["recordError"] = writeErr.Error()
	}
	if err != nil {
		return intentResult{Targets: targets, Outcome: intentPartial, code: max(code, 1), Data: data,
			Summary: fmt.Sprintf("landed %s on %s, but the merged goal branch was not swept: %v", pushed.Landing, endpoint, err),
			next:    inv.sameCommand(), nextReason: "retries the branch owner's sweep of the pushed landing"}
	}
	return intentResult{Targets: targets, Outcome: intentConfirmed, Data: data,
		Summary: fmt.Sprintf("landed %s on %s; goal %s stays open until done", pushed.Landing, endpoint, goalID)}
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
			next:    []string{"metasystem", "review", "commit", units[index].Commit, "--goal", goalID}, nextReason: "reads that unit"}
	}
	if len(units) == 0 {
		return "", 0, &intentResult{Targets: targets, Outcome: intentRefused, code: 1, Summary: fmt.Sprintf("goal/%s has no units to land", goalID)}
	}
	if len(state.Sources) < state.Status.Prefix {
		return "", 0, &intentResult{Targets: targets, Outcome: intentFailed, Summary: "the branch reader returned no attestation source for a read unit"}
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
	return "", 0, &intentResult{Targets: targets, Outcome: intentRefused, code: 2, Summary: fmt.Sprintf("%s is not a unit commit on goal/%s; --through takes the full commit", through, goalID)}
}

// prepareReceipt runs the landing proof on the subject tree through the
// landing test-receipt owner and retains its typed receipt.
func (inv *intentInvocation) prepareReceipt(targets []intentTarget, data map[string]any, goalID, subject, dir, receipt string) *intentResult {
	owners := inv.delivery()
	binary, err := owners.executable()
	if err != nil {
		return &intentResult{Targets: targets, Outcome: intentFailed, Summary: err.Error(), Data: data}
	}
	ran := owners.process(intentProcess{dir: inv.layout.InstallationRoot, argv: []string{binary, "landing", "test-receipt",
		"--root", inv.layout.InstallationRoot, "--tree", subject, "--mode", "auto", "--goal", goalID}})
	var parsed landing.TestReceipt
	encoded := bytes.TrimSpace(ran.stdout)
	if ran.err != nil || ran.code != 0 || json.Unmarshal(encoded, &parsed) != nil || parsed.SchemaVersion != 3 {
		data["exitCode"] = ran.code
		return &intentResult{Targets: targets, Outcome: intentRefused, code: max(ran.code, 1), Data: data,
			Summary: fmt.Sprintf("the landing proof of %s produced no schema-3 receipt (exit %d); nothing was prepared", goalID, ran.code),
			text:    nonEmptyLines(string(ran.stderr)), next: inv.sameCommand(), nextReason: "the proof owner reuses a matching completed proof"}
	}
	if err := writeIntentInputs(dir, map[string]string{receipt: string(encoded) + "\n"}); err != nil {
		return &intentResult{Targets: targets, Outcome: intentFailed, Summary: err.Error(), Data: data}
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
			Summary:  fmt.Sprintf("job %s is a review chain; its findings are decided, not reviewed again; nothing was done", job),
			Decision: "decide every finding in a dispositions file, then run " + shellCommand(inv.publicArgv("review", "job", dispatchJobPrefix+job, "--dispositions", "FILE"))}
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
			return inv.publicArgv("review", "unit", target.ID)
		}
	}
	return inv.publicArgv("review", "commit", unit, "--goal", goalID)
}
