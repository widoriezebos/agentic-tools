package steward

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/behaviorsurface"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goalbudget"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/governance"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/run"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

type cadenceGoalFixture struct {
	root       string
	repository *cadenceRepository
	minted     int
}

func cadenceGoalBed(t *testing.T, file *goal.GoalFile) *cadenceGoalFixture {
	t.Helper()
	return &cadenceGoalFixture{root: t.TempDir(), repository: newCadenceRepository(t, file)}
}

func (f *cadenceGoalFixture) link(t *testing.T) func(string, validationWindowObservation, time.Time) error {
	t.Helper()
	return func(root string, observation validationWindowObservation, now time.Time) error {
		if root != f.root {
			t.Fatalf("cadence linker root=%q, want %q", root, f.root)
		}
		return linkCadenceFailureWithOwners(root, observation, now,
			func(got string) bool {
				if got != f.root {
					t.Fatalf("new-world root=%q", got)
				}
				tree, problems := goal.ParseTreeFiles(copyCadenceFiles(f.repository.snapshot(f.repository.accepted)))
				if len(problems) != 0 || tree.Root == nil || tree.Live["bounded"] == nil {
					t.Fatalf("accepted cadence bytes are not a migrated goal tree: %v", problems)
				}
				return true
			},
			func(got string) (goal.Endpoint, error) {
				if got != f.root {
					t.Fatalf("resolve endpoint root=%q", got)
				}
				return goal.Endpoint{Root: f.root, Remote: "local", Branch: goal.LocalLedgerBranch, Repository: f.repository}, nil
			},
			func() (string, error) {
				f.minted++
				if f.minted != 1 {
					t.Fatalf("cadence operation identity minted %d times", f.minted)
				}
				return cadenceTestULID, nil
			})
	}
}

func assertCadenceGoalLinked(t *testing.T, f *cadenceGoalFixture, runID, attemptID string, linkedAt time.Time) {
	t.Helper()
	files := f.repository.acceptedFiles()
	tree, problems := goal.ParseTreeFiles(files)
	if len(problems) != 0 {
		t.Fatalf("accepted cadence goal no longer parses: %v", problems)
	}
	file := tree.Live["bounded"]
	if file == nil || file.Revision != 3 || len(file.History) != 2 || f.minted != 1 {
		t.Fatalf("cadence link revision/history/identity: goal=%+v minted=%d", file, f.minted)
	}
	before, _ := goal.ParseTreeFiles(f.repository.seed)
	if goal.RenderHistoryLine(file.History[0]) != goal.RenderHistoryLine(before.Live["bounded"].History[0]) {
		t.Fatalf("cadence link rewrote its parent history: %+v", file.History)
	}
	if file.Claimed == nil || file.Claimed.Machine != "bed-m1" || file.Claimed.Lineage != "coordinator" {
		t.Fatalf("cadence link changed the claimed actor: %+v", file.Claimed)
	}
	last := file.History[1]
	if last.Opid != f.repository.opid || last.At != linkedAt.UTC().Format(time.RFC3339) ||
		last.Verb != "defer-findings" || last.Actor != "bed-m1+coordinator" ||
		len(last.Targets) != 1 || last.Targets[0] != "bounded" {
		t.Fatalf("cadence link history has the wrong actor or operation: %+v", last)
	}
	obligations := file.ReviewObligations
	if len(obligations) != 1 || obligations[0].Finding != "cadence-"+runID || obligations[0].Chain != attemptID ||
		obligations[0].State != "open" || obligations[0].Artifact != "retained cadence result "+runID ||
		obligations[0].Test != "focused repair for testing attempt "+attemptID {
		t.Fatalf("cadence did not retain the exact open correction obligation: %+v", obligations)
	}
	f.repository.verify()
}

func writeValidationRun(t *testing.T, root, id, status, log string, seq int64) {
	t.Helper()
	ended := "2026-08-30T11:00:00Z"
	observed := uint64(1)
	weightGeneration := uint64(3)
	record := run.Record{SchemaVersion: 1, RunId: id, Kind: "suite", Display: "weight-triggered direct validation",
		Custody: run.CustodyWrapped, Generation: 1, LaunchNonce: strings.Repeat("a", 32), Log: log,
		StartedAt: "2026-08-30T10:59:00Z", GoalId: "bounded", StaleAfterMin: 30, WindDownMin: 10,
		Evidence: run.Evidence{Mode: run.EvidenceSidecar}, Status: status, TerminalSeq: &seq, EndedAt: &ended,
		Governed: &run.GovernedAttempt{GoalRevision: 2, ObligationRevision: 3, WeightGeneration: &weightGeneration,
			Recurrence: governance.StandingSharedProcess, ExecutionCostMinutes: 1, ObservedCostMinutes: &observed,
			AttemptOrdinal: uint64(seq), Budget: goalbudget.Budget{ElapsedLimit: "1h", AttemptLimit: 4,
				ReservedJobMinutesLimit: 10, ActiveJobLimit: 1}, BudgetStartedAt: "2026-08-30T10:00:00Z",
			ExpectedAssumptions: governance.ObligationAssumptions{Recurrence: governance.StandingSharedProcess,
				Platform: "fixture/os", ToolchainIdentity: "fixture-go", SurfaceDigest: "fixture-digest",
				MaxActiveJobs: 1, TimingEnvelopeSeconds: 60, ObservationSource: "run-terminal-record"},
			AdmissionDecision: governance.ConsequenceDecision{Apply: true}, Observation: &run.AssumptionObservation{
				ObservedAt: ended, AssumptionState: run.AssumptionMatch}, Breaker: run.BreakerClosed}}
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	path := run.RecordPath(root, id)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeValidationAttempt(t *testing.T, root, runID, omitted string) string {
	t.Helper()
	digest := strings.Repeat("a", 64)
	now := time.Date(2026, 8, 30, 10, 59, 0, 0, time.UTC)
	identity := proofrun.BuildProofIdentityForContext(proofrun.ExecutionContext{ManifestDigest: digest, Configuration: digest,
		Platform: "fixture/os", Toolchain: digest}, "selected", "testing", nil, behaviorsurface.SupportedVersion)
	launcher, err := proofrun.CurrentProcessIdentity(nil)
	if err != nil {
		t.Fatal(err)
	}
	owner := &proofrun.ReservationOwner{ControlRoot: root, RunID: runID, RunGeneration: 1,
		LaunchNonce: strings.Repeat("a", 32), GoalRevision: 2, ObligationRevision: 3, AttemptOrdinal: 3,
		Deadline: now.Add(10 * time.Minute).Format(time.RFC3339Nano)}
	attempt, disposition, err := proofrun.ReserveLocked(proofrun.WithTestHostLoadSampler(proofrun.AdmissionRequest{ControlRoot: root, ExecutionRoot: root,
		GoalID: "bounded", GoalRevision: 2, AccountingRevision: 2,
		CandidateGoalID: "bounded", CandidateRevision: 2, CandidateTree: digest[:40], ReservedMinutes: 10, Identity: identity,
		Launcher: launcher, ReservationOwner: owner, Now: now, AttemptID: "attempt-" + runID}, "0"))
	if err != nil {
		t.Fatal(err)
	}
	if disposition.Disposition == proofrun.DispositionAdmissionRefused {
		t.Fatalf("validation proof fixture was admission-refused: %+v", disposition)
	}
	zero, admissionMaximum := 0, 0
	result := proofrun.TestResult{SchemaVersion: proofrun.TestResultSchemaVersion, AttemptID: attempt.AttemptID,
		WorkerPolicyVersion: proofrun.TestWorkerPolicyVersion, Workers: 1, AdmissionMaximum: &admissionMaximum,
		Purpose: testpolicy.PurposeCadence, RequestedMode: testpolicy.ModeDeep, RequiredMode: testpolicy.ModeDeep,
		ExecutedMode: testpolicy.ModeDeep, ProjectRoot: root, BaseCommit: digest[:40], CandidateTree: digest[:40],
		PolicyBaseCommit: digest[:40], ContractDigest: digest, BaseContractDigest: digest, PolicyEngineDigest: digest,
		BehaviorPolicyDigest: digest, PlanDigest: digest, LaunchCounts: proofrun.LaunchCounts{CountsComplete: true},
		Cost: proofrun.TestCost{DeclaredTargetMS: 1}}
	for _, id := range cadenceCatchGroups {
		if id == omitted {
			continue
		}
		result.RequiredGroups = append(result.RequiredGroups, id)
		result.SelectedGroups = append(result.SelectedGroups, id)
		result.Groups = append(result.Groups, proofrun.GroupResult{ID: id, Kind: "integration", InputDigest: digest,
			IdentityVersion: proofrun.GroupExecutionIdentityVersion,
			InputManifest:   []string{"source/**"}, ExecutionIdentity: digest, CWD: ".", ToolIdentities: map[string]string{},
			ReportDigests: map[string]string{}, Status: "passed", NativeLaunched: true, NativeExitStatus: &zero, CollectionComplete: true})
	}
	result.RecomputeDelivery()
	if _, err := proofrun.FinalizeAttemptWithTestResultLocked(root, attempt.AttemptID, proofrun.TerminalSuccess, 0,
		"fixture", nil, &result, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	return attempt.AttemptID
}

func TestValidationWindowObservesOnlyResultBoundGovernedRuns(t *testing.T) {
	root := t.TempDir()
	writeValidationRun(t, root, "aborted-proof", run.StatusLaunchFailed, "artifacts/missing-aborted.log", 1)
	writeValidationRun(t, root, "unclassifiable-proof", run.StatusGreen, "artifacts/missing-green.log", 2)
	writeValidationRun(t, root, "measured-proof", run.StatusGreen, "artifacts/direct.log", 3)
	attemptID := writeValidationAttempt(t, root, "measured-proof", cadenceCatchGroups[0])
	if err := observeDirectValidationWindow(root, time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	state, err := loadValidationWindow(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Observations) != 1 || state.Observations[0].RunID != "measured-proof" || state.Observations[0].AttemptID != attemptID ||
		strings.Join(state.Observations[0].Missing, ",") != cadenceCatchGroups[0] || len(state.Observations[0].NonGreen) != 0 {
		t.Fatalf("result-bound observer advanced on aborted/unclassifiable evidence or lost the group diff: %+v", state)
	}
}

func TestCadenceRedLinksOneExistingGoalReviewObligation(t *testing.T) {
	fixture := cadenceGoalBed(t, &goal.GoalFile{
		Id: "bounded", State: goal.StateClaimed, Intent: "Repair bounded regressions", Origin: goal.OriginMain,
		NextStep: "Apply the focused repair.", OpenedAt: "2026-08-23T00:00:00Z", Revision: 2,
		Claimed: &goal.ClaimRecord{Machine: "bed-m1", Lineage: "coordinator", At: "2026-08-23T01:00:00Z"},
		History: bedHistory("bounded", "claim"),
	})
	writeValidationRun(t, fixture.root, "cadence-red", run.StatusRed, "artifacts/cadence-red.log", 1)
	attemptID := writeValidationAttempt(t, fixture.root, "cadence-red", cadenceCatchGroups[0])
	now := time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)
	link := fixture.link(t)
	if err := observeDirectValidationWindowWithLinker(fixture.root, now, link); err != nil {
		t.Fatal(err)
	}
	if err := link(fixture.root, validationWindowObservation{RunID: "cadence-red", AttemptID: attemptID,
		Missing: []string{cadenceCatchGroups[0]}}, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	assertCadenceGoalLinked(t, fixture, "cadence-red", attemptID, now)
}

func TestCadenceObservationRetriesFailedGoalLink(t *testing.T) {
	fixture := cadenceGoalBed(t, &goal.GoalFile{
		Id: "bounded", State: goal.StateClaimed, Intent: "Repair bounded regressions", Origin: goal.OriginMain,
		NextStep: "Apply the focused repair.", OpenedAt: "2026-08-23T00:00:00Z", Revision: 2,
		Claimed: &goal.ClaimRecord{Machine: "bed-m1", Lineage: "coordinator", At: "2026-08-23T01:00:00Z"},
		History: bedHistory("bounded", "claim"),
	})
	writeValidationRun(t, fixture.root, "cadence-retry", run.StatusRed, "artifacts/cadence-retry.log", 1)
	attemptID := writeValidationAttempt(t, fixture.root, "cadence-retry", cadenceCatchGroups[0])
	now := time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)
	calls := 0
	failedLink := func(root string, observation validationWindowObservation, at time.Time) error {
		calls++
		state, err := loadValidationWindow(root)
		if err != nil || len(state.Observations) != 1 || state.Observations[0].RunID != observation.RunID ||
			observation.RunID != "cadence-retry" || observation.AttemptID != attemptID ||
			at != now || len(observation.Missing) != 1 || observation.Missing[0] != cadenceCatchGroups[0] {
			t.Fatalf("cadence link ran before its exact observation was saved: state=%+v observation=%+v err=%v", state, observation, err)
		}
		return os.ErrPermission
	}
	if err := observeDirectValidationWindowWithLinker(fixture.root, now, failedLink); !os.IsPermission(err) {
		t.Fatalf("first goal-link fault was not exposed: %v", err)
	}
	state, err := loadValidationWindow(fixture.root)
	if err != nil || len(state.Observations) != 1 || state.Observations[0].RunID != "cadence-retry" ||
		state.Observations[0].AttemptID != attemptID || len(state.Observations[0].Missing) != 1 {
		t.Fatalf("first failure did not preserve its durable observation: %+v %v", state, err)
	}
	link := fixture.link(t)
	if err := observeDirectValidationWindowWithLinker(fixture.root, now.Add(time.Minute), link); err != nil {
		t.Fatalf("second pass did not recover the preserved goal link: %v", err)
	}
	if calls != 1 {
		t.Fatalf("fault injection count = %d, want one failed link", calls)
	}
	if err := link(fixture.root, state.Observations[0], now.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	assertCadenceGoalLinked(t, fixture, "cadence-retry", attemptID, now.Add(time.Minute))
}
