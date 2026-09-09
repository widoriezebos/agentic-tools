package steward

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/behaviorsurface"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goalbudget"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/governance"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/run"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

func cadenceGoalBed(t *testing.T, file *goal.GoalFile) string {
	t.Helper()
	root := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		command := exec.Command("git", append([]string{"-C", root}, args...)...)
		command.Env = gittree.ScrubbedEnviron()
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, output)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	run("init", "-q", "-b", "main")
	run("config", "user.name", "cadence-fixture")
	run("config", "user.email", "cadence-fixture@example.invalid")
	run("config", "metasystem.goal.machine", "bed-m1")
	run("config", "goal.sync-remote", "local")
	rootRecord := &goal.RootRecord{Identity: "01ARZ3NDEKTSV4RRFFQ69G5FAV", FormatVersion: "1", SyncMode: goal.SyncLocal, Revision: 1}
	for relative, data := range map[string][]byte{
		"plans/goals/backlog.md": goal.RenderRoot(rootRecord),
		"plans/goals/bounded.md": goal.RenderFile(file),
	} {
		path := filepath.Join(root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
		run("add", relative)
	}
	run("commit", "-q", "-m", "cadence bed")
	run("update-ref", goal.AcceptedRef, "HEAD")
	return root
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
	attempt, _, err := proofrun.ReserveLocked(proofrun.AdmissionRequest{ControlRoot: root, ExecutionRoot: root,
		GoalID: "bounded", GoalRevision: 2, AccountingRevision: 2, ReservedMinutes: 10, Identity: identity,
		Launcher: launcher, ReservationOwner: owner, Now: now, AttemptID: "attempt-" + runID})
	if err != nil {
		t.Fatal(err)
	}
	zero := 0
	result := proofrun.TestResult{SchemaVersion: proofrun.TestResultSchemaVersion, AttemptID: attempt.AttemptID,
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
			InputManifest: []string{"source/**"}, ExecutionIdentity: digest, CWD: ".", ToolIdentities: map[string]string{},
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
	root := cadenceGoalBed(t, &goal.GoalFile{
		Id: "bounded", State: goal.StateClaimed, Intent: "Repair bounded regressions", Origin: goal.OriginMain,
		NextStep: "Apply the focused repair.", OpenedAt: "2026-08-23T00:00:00Z", Revision: 2,
		Claimed: &goal.ClaimRecord{Machine: "bed-m1", Lineage: "coordinator", At: "2026-08-23T01:00:00Z"},
		History: bedHistory("bounded", "claim"),
	})
	writeValidationRun(t, root, "cadence-red", run.StatusRed, "artifacts/cadence-red.log", 1)
	attemptID := writeValidationAttempt(t, root, "cadence-red", cadenceCatchGroups[0])
	now := time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)
	if !goal.NewWorld(root) {
		t.Fatal("cadence fixture lost its accepted migrated goal tree")
	}
	if err := observeDirectValidationWindow(root, now); err != nil {
		t.Fatal(err)
	}
	if err := linkCadenceFailure(root, validationWindowObservation{RunID: "cadence-red", AttemptID: attemptID,
		Missing: []string{cadenceCatchGroups[0]}}, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	tip := attentionGit(t, root, "rev-parse", goal.LocalLedgerBranch)
	files, err := goal.ReadCommitGoals(root, tip)
	if err != nil {
		t.Fatal(err)
	}
	tree, problems := goal.ParseTreeFiles(files)
	if len(problems) > 0 {
		t.Fatalf("linked cadence goal no longer parses: %v", problems)
	}
	obligations := tree.Live["bounded"].ReviewObligations
	if len(obligations) != 1 || obligations[0].Finding != "cadence-cadence-red" || obligations[0].Chain != attemptID || obligations[0].State != "open" {
		t.Fatalf("cadence red did not retain one linked correction obligation: %+v", obligations)
	}
}

func TestCadenceObservationRetriesFailedGoalLink(t *testing.T) {
	root := cadenceGoalBed(t, &goal.GoalFile{
		Id: "bounded", State: goal.StateClaimed, Intent: "Repair bounded regressions", Origin: goal.OriginMain,
		NextStep: "Apply the focused repair.", OpenedAt: "2026-08-23T00:00:00Z", Revision: 2,
		Claimed: &goal.ClaimRecord{Machine: "bed-m1", Lineage: "coordinator", At: "2026-08-23T01:00:00Z"},
		History: bedHistory("bounded", "claim"),
	})
	writeValidationRun(t, root, "cadence-retry", run.StatusRed, "artifacts/cadence-retry.log", 1)
	writeValidationAttempt(t, root, "cadence-retry", cadenceCatchGroups[0])
	now := time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)
	original := cadenceFailureLinker
	t.Cleanup(func() { cadenceFailureLinker = original })
	calls := 0
	cadenceFailureLinker = func(string, validationWindowObservation, time.Time) error {
		calls++
		return os.ErrPermission
	}
	if err := observeDirectValidationWindow(root, now); !os.IsPermission(err) {
		t.Fatalf("first goal-link fault was not exposed: %v", err)
	}
	state, err := loadValidationWindow(root)
	if err != nil || len(state.Observations) != 1 {
		t.Fatalf("first failure did not preserve its durable observation: %+v %v", state, err)
	}
	cadenceFailureLinker = original
	if err := observeDirectValidationWindow(root, now.Add(time.Minute)); err != nil {
		t.Fatalf("second pass did not recover the preserved goal link: %v", err)
	}
	if calls != 1 {
		t.Fatalf("fault injection count = %d, want one failed link", calls)
	}
	tip := attentionGit(t, root, "rev-parse", goal.LocalLedgerBranch)
	files, err := goal.ReadCommitGoals(root, tip)
	if err != nil {
		t.Fatal(err)
	}
	tree, problems := goal.ParseTreeFiles(files)
	if len(problems) > 0 || len(tree.Live["bounded"].ReviewObligations) != 1 {
		t.Fatalf("recovery did not create exactly one obligation: problems=%v goal=%+v", problems, tree.Live["bounded"])
	}
}
