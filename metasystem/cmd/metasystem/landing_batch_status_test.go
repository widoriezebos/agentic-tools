package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/validate"
)

func TestBatchStatusExposesReturnRevisionHeadroomOwnerLockSampleAndDeadline(t *testing.T) {
	originalOwner, originalLock, originalSample, originalNow := batchStatusOwner, batchStatusLock, batchStatusSample, batchStatusNow
	t.Cleanup(func() {
		batchStatusOwner, batchStatusLock, batchStatusSample, batchStatusNow = originalOwner, originalLock, originalSample, originalNow
	})
	batchStatusOwner = func(string) (int64, identity.Liveness, error) { return 41, identity.Alive, nil }
	configuredLock := filepath.Join(t.TempDir(), "configured-proof-lock")
	batchStatusLock = func(lockDir string) string {
		if lockDir != configuredLock {
			t.Fatalf("status lock directory=%q, want %q", lockDir, configuredLock)
		}
		return "landing-batch-owner 41"
	}
	batchStatusSample = func(string) proofrun.LoadSample { return proofrun.LoadSample{OverlapKnown: true, OverlappingHost: 1} }
	now := time.Date(2030, 1, 1, 0, 30, 0, 0, time.UTC)
	batchStatusNow = func() time.Time { return now }
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "plans", "goals"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("dispatch.cap-min=20\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	budget, err := goal.NewBudget("4h", 1, 100, 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	file := &goal.GoalFile{Id: "goal-a", State: goal.StateClaimed, Intent: "status", Origin: goal.OriginHuman,
		OpenedAt: "2030-01-01T00:00:00Z", Revision: 2, Budget: &budget,
		Claimed: &goal.ClaimRecord{Machine: "landing", Lineage: "owner", At: "2030-01-01T00:00:00Z", Revision: 2,
			AccountingRevision: 2, EpisodeAt: "2030-01-01T00:00:00Z", EpisodeRevision: 2},
		History: []goal.HistoryLine{{At: "2030-01-01T00:00:00Z", Opid: "01J5X0000000000000000000BY-human-1a2b3c4d", Verb: "open", Actor: "human:wido", Keep: -1},
			{At: "2030-01-01T00:00:00Z", Opid: "01J5X0000000000000000000B1-landing-1a2b3c4d", Verb: "claim", Actor: "landing+owner", Keep: -1}}}
	if err := os.WriteFile(filepath.Join(root, "plans", "goals", "goal-a.md"), goal.RenderFile(file), 0o644); err != nil {
		t.Fatal(err)
	}
	settings, err := config.NewBatchLanding(root, 45*time.Minute, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	record := batch.Record{BatchID: "batch", State: batch.StateLanding, Proof: &batch.Proof{Status: "green"}, Seal: map[string]batch.Claim{"goal-a": {Revision: 2, AccountingRevision: 2}}, Units: []batch.Unit{{
		GoalID: "goal-a", Chain: "chain-a", State: batch.UnitReturnPending, Outcome: batch.UnitLanded,
		CommitIDs: []string{"source-a", "source-b"}, LastUnit: "10b",
		Claim: batch.Claim{Epoch: 3, Revision: 8, AccountingRevision: 5},
	}}}
	record.PrefixTrees = []string{"prefix-a"}
	record.Landing = &batch.LandingProgress{BranchTip: "branch-tip"}
	record.History = []batch.HistoryEntry{{At: "2030-01-01T00:00:00Z", Verb: "join"}}
	view := batchRecordStatus(record, settings, configuredLock)
	if view.Owner != "41" || view.OwnerLiveness == "" || !strings.Contains(view.Lock, "41") || view.ProofStatus != "green" ||
		len(view.Headroom) != 1 || view.Headroom[0].Status != string(dispatchcore.BudgetKnown) || view.Headroom[0].AttemptsLeft != 1 ||
		view.Headroom[0].ReservedMinutesLeft != 100 || view.Headroom[0].HasDiagnosticHeadroom || view.Deadline != "2030-01-01T00:45:00Z" ||
		view.Branch != "landing/batch" || view.BranchTip != "branch-tip" || len(view.Units) != 1 || view.Units[0].State != batch.UnitReturnPending ||
		view.Units[0].Revision != 8 || view.Units[0].AccountingRevision != 5 || view.Units[0].LastUnit != "10b" || view.Units[0].PrefixTree != "prefix-a" || !view.Sample.OverlapKnown {
		t.Fatalf("status=%+v", view)
	}
}

func TestBatchJoinAuthorComesFromApproverConfiguration(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("goal.human.wido=Wido Example <wido@example.com>\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	file := &goal.GoalFile{Approved: &goal.ApprovalRecord{By: "human:Wido"}}
	approver, name, email, err := productionBatchAuthor(root, file)
	if err != nil || approver != "Wido" || name != "Wido Example" || email != "wido@example.com" {
		t.Fatalf("identity=%q %q <%s> error=%v", approver, name, email, err)
	}
	if _, _, _, err := productionBatchAuthor(root, &goal.GoalFile{Approved: &goal.ApprovalRecord{By: "human:Absent"}}); err == nil || !strings.Contains(err.Error(), "BATCH_JOIN_AUTHOR_UNBOUND") {
		t.Fatalf("unbound approver error=%v", err)
	}
}

func TestLandingReceiptForwardsBothExpectedRevisions(t *testing.T) {
	root := t.TempDir()
	for _, args := range [][]string{{"init", "-q", "-b", "main", root}, {"-C", root, "config", "user.name", "Fixture"}, {"-C", root, "config", "user.email", "fixture@example.com"}} {
		if output, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, output)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "file"), []byte("one\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"-C", root, "add", "file"}, {"-C", root, "commit", "-qm", "base"}} {
		if err := exec.Command("git", args...).Run(); err != nil {
			t.Fatal(err)
		}
	}
	original := landingReceiptTestRun
	t.Cleanup(func() { landingReceiptTestRun = original })
	var forwarded []string
	landingReceiptTestRun = func(args []string) int {
		forwarded = append([]string(nil), args...)
		return proofrun.ExitAdmissionRefused
	}
	treeOutput, err := exec.Command("git", "-C", root, "rev-parse", "HEAD^{tree}").Output()
	if err != nil {
		t.Fatal(err)
	}
	code := runLandingTestReceipt([]string{"--root", root, "--tree", strings.TrimSpace(string(treeOutput)), "--mode", "auto", "--goal", "goal-a", "--expected-goal-revision", "7", "--expected-accounting-revision", "5"})
	joined := strings.Join(forwarded, " ")
	if code != proofrun.ExitAdmissionRefused || !strings.Contains(joined, "--expected-goal-revision 7 --expected-accounting-revision 5") {
		t.Fatalf("code=%d forwarded=%v", code, forwarded)
	}
}

func TestDiagnosticNoReuseForcesFreshRunsAndDeliveryRefuses(t *testing.T) {
	root := t.TempDir()
	diagnostic, _, status := parseTestingSelection("test run", []string{
		"--root", root, "--purpose", "diagnostic", "--groups", "red-group", "--no-reuse",
	}, true)
	if status != 0 || !diagnostic.NoReuse || diagnostic.Purpose != "diagnostic" {
		t.Fatalf("diagnostic request=%+v status=%d", diagnostic, status)
	}
	if _, _, status := parseTestingSelection("test run", []string{
		"--root", root, "--purpose", "delivery", "--no-reuse",
	}, true); status != 2 {
		t.Fatalf("delivery --no-reuse status=%d, want usage refusal", status)
	}
	originalExecute := batchDiagnosticExecute
	t.Cleanup(func() { batchDiagnosticExecute = originalExecute })
	var recorded []string
	batchDiagnosticExecute = func(_ string, args []string, dir string, _ []string) ([]byte, int, error) {
		recorded = append([]string(nil), args...)
		resultAt := slices.Index(args, "--result")
		if resultAt < 0 || resultAt+1 >= len(args) {
			t.Fatalf("production diagnostic omitted --result: %v", args)
		}
		if err := os.MkdirAll(filepath.Dir(args[resultAt+1]), 0o755); err != nil {
			t.Fatal(err)
		}
		data, err := json.Marshal(proofrun.TestResult{AttemptID: "fresh-diagnostic"})
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(args[resultAt+1], append(data, '\n'), 0o600); err != nil {
			t.Fatal(err)
		}
		if dir != root {
			t.Fatalf("diagnostic dir=%q want %q", dir, root)
		}
		return nil, 0, nil
	}
	result, err := launchBatchDiagnostic(root, "01j5x00000000000000000ba12", batch.DiagnosticRequest{GoalID: "goal-k", Tree: "base-tree", Groups: []string{"F"},
		Claim: batch.Claim{Revision: 7, AccountingRevision: 5}})
	if err != nil || result.AttemptID != "fresh-diagnostic" {
		t.Fatalf("production diagnostic result=%+v err=%v", result, err)
	}
	joined := strings.Join(recorded, " ")
	for _, want := range []string{"--purpose diagnostic", "--no-reuse", "--mode canary", "--groups F", "--tree base-tree", "--goal goal-k",
		"--expected-goal-revision 7", "--expected-accounting-revision 5"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("diagnostic argv %q does not contain %q", joined, want)
		}
	}
	// The owner's clearing path passes the claim beside the request; its revisions must reach the run.
	if _, err := clearingDiagnostic(root)("01j5x00000000000000000ba12", batch.DiagnosticRequest{GoalID: "goal-k", Tree: "new-base", Groups: []string{"F"}},
		batch.Claim{Revision: 11, AccountingRevision: 13}); err != nil {
		t.Fatal(err)
	}
	joined = strings.Join(recorded, " ")
	for _, want := range []string{"--tree new-base", "--expected-goal-revision 11", "--expected-accounting-revision 13"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("clearing diagnostic argv %q does not contain %q", joined, want)
		}
	}

	controlRoot, now := proofExtensionGoalFixture(t)
	amendSyncedGoalFixture(t, controlRoot, "diagnostic no-reuse fixture", func(file *goal.GoalFile) {
		file.Budget.AttemptLimit = 4
		file.Approved.Digest = goal.ApprovalDigest(file.Intent, file.Tier, *file.Budget, file.Risk)
	})
	announceProofFixtureHolder(t, controlRoot)
	t.Setenv("METASYSTEM_GOAL_NOW", now.Format(time.RFC3339))
	admission := proofLaunchAdmission{ControlRoot: controlRoot, ExecutionRoot: controlRoot, ConfPath: filepath.Join(controlRoot, "metasystem.conf"),
		GoalID: "standing-validation", CapMin: "1", ScopeClass: "selected", CommandClass: "testing",
		IdentityInputs: []string{"diagnostic-no-reuse"}, ExecuteAfresh: true}
	retained, decision, _, err := admitProofLaunch(admission)
	if err != nil || decision.Disposition != proofrun.DispositionExecuted || retained.AttemptID == "" {
		t.Fatalf("seed admission=%+v attempt=%+v err=%v", decision, retained, err)
	}
	if _, err := proofrun.FinalizeAttempt(controlRoot, retained.AttemptID, proofrun.TerminalSuccess, 0, "retained green", nil, now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	admission.ExecuteAfresh = false
	if _, reused, _, err := admitProofLaunch(admission); err != nil || reused.Disposition != proofrun.DispositionReusableSuccess {
		t.Fatalf("same diagnostic without --no-reuse did not reuse: decision=%+v err=%v", reused, err)
	}
	fresh, decision, _, err := admitTestingRun(diagnostic, admission)
	if err != nil || decision.Disposition != proofrun.DispositionExecuted || fresh.AttemptID == "" || fresh.AttemptID == retained.AttemptID {
		t.Fatalf("--no-reuse did not reserve a fresh native attempt: fresh=%+v decision=%+v err=%v", fresh, decision, err)
	}
	if _, err := proofrun.FinalizeAttempt(controlRoot, fresh.AttemptID, proofrun.TerminalFailed, 1, "fixture cleanup", nil, now.Add(3*time.Second)); err != nil {
		t.Fatal(err)
	}
}

func TestPrefixReceiptAllowsIdentityReuseAndBindsRevisions(t *testing.T) {
	args := batchPrefixReceiptArgs("/landing", "goal-a", "tree-a", "result.json", []string{"same", "different"}, batch.Claim{Revision: 7, AccountingRevision: 5})
	joined := strings.Join(args, " ")
	if strings.Contains(joined, "--no-reuse") || !strings.Contains(joined, "--purpose delivery --groups same,different") ||
		!strings.Contains(joined, "--expected-goal-revision 7 --expected-accounting-revision 5") {
		t.Fatalf("prefix receipt argv=%v", args)
	}
}

func TestPrefixReceiptAcceptsSufficientReusableExit(t *testing.T) {
	const batchID = "01j5x00000000000000000ba22"
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "artifacts", "agents", "proof-runs", "batch"), 0o755); err != nil {
		t.Fatal(err)
	}
	fake := filepath.Join(root, "fake-metasystem")
	script := `#!/usr/bin/env bash
set -euo pipefail
result=
while (( $# )); do
  if [[ "$1" == --result ]]; then result=$2; shift 2; else shift; fi
done
printf '%s\n' '{"delivery":{"sufficient":true},"groups":[{"id":"same","status":"reused","reuseAttempt":"tip-attempt"}]}' >"$result"
exit 76
`
	if err := os.WriteFile(fake, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	original := batchPrefixReceiptExecutable
	t.Cleanup(func() { batchPrefixReceiptExecutable = original })
	batchPrefixReceiptExecutable = func() (string, error) { return fake, nil }
	claim := batch.Claim{Machine: "landing", Lineage: "owner", Epoch: 1, Revision: 7, AccountingRevision: 5}
	record := batch.Record{Schema: 1, BatchID: batchID, State: batch.StateLanding, BaseTree: "base", PrefixTrees: []string{"prefix", "tip"}, TipTree: "tip",
		Units: []batch.Unit{{GoalID: "goal-a", Chain: "chain-a", Claim: claim, State: batch.UnitJoined}, {GoalID: "goal-b", Chain: "chain-b", Claim: claim, State: batch.UnitJoined}},
		Proof: &batch.Proof{Status: "green", SelectedGroups: []string{"same"}},
	}
	store := batch.NewStore(root, nil)
	if err := store.Create(record); err != nil {
		t.Fatal(err)
	}
	err := batch.ComposePrefixReceipts(store, batchID, "owner", time.Unix(3, 0), batch.PrefixReceiptSeams{Execute: func(goalID, tree string, groups []string) (batch.PrefixRunResult, error) {
		return executeBatchPrefixReceipt(root, batchID, record, goalID, tree, groups)
	}})
	if err != nil {
		t.Fatal(err)
	}
	stored, err := store.Load(batchID)
	if err != nil {
		t.Fatal(err)
	}
	receipt := stored.Receipts["goal-a"]
	if receipt.AttemptID != "" || len(receipt.Executed) != 0 || receipt.Reused["same"] != "tip-attempt" {
		t.Fatalf("reusable prefix receipt=%+v", receipt)
	}
}

func TestPrefixReceiptPersistsRedResultFromExitOne(t *testing.T) {
	const batchID = "01j5x00000000000000000ba23"
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "artifacts", "agents", "proof-runs", "batch"), 0o755); err != nil {
		t.Fatal(err)
	}
	fake := filepath.Join(root, "fake-metasystem")
	script := `#!/usr/bin/env bash
set -euo pipefail
result=
while (( $# )); do
  if [[ "$1" == --result ]]; then result=$2; shift 2; else shift; fi
done
printf '%s\n' '{"attemptId":"prefix-attempt","groups":[{"id":"red-group","status":"failed","nativeLaunched":true,"logPath":"red.log","logDigest":"sha256:red","inputManifest":["source/**"]}]}' >"$result"
exit 1
`
	if err := os.WriteFile(fake, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	original := batchPrefixReceiptExecutable
	t.Cleanup(func() { batchPrefixReceiptExecutable = original })
	batchPrefixReceiptExecutable = func() (string, error) { return fake, nil }
	claim := batch.Claim{Machine: "landing", Lineage: "owner", Epoch: 1, Revision: 7, AccountingRevision: 5}
	record := batch.Record{Schema: 1, BatchID: batchID, State: batch.StateLanding, BaseTree: "base", PrefixTrees: []string{"prefix", "tip"}, TipTree: "tip",
		Units: []batch.Unit{{GoalID: "goal-a", Chain: "chain-a", Claim: claim, State: batch.UnitJoined}, {GoalID: "goal-b", Chain: "chain-b", Claim: claim, State: batch.UnitJoined}},
		Proof: &batch.Proof{Status: "green", SelectedGroups: []string{"red-group"}},
	}
	store := batch.NewStore(root, nil)
	if err := store.Create(record); err != nil {
		t.Fatal(err)
	}
	err := batch.ComposePrefixReceipts(store, batchID, "owner", time.Unix(3, 0), batch.PrefixReceiptSeams{Execute: func(goalID, tree string, groups []string) (batch.PrefixRunResult, error) {
		return executeBatchPrefixReceipt(root, batchID, record, goalID, tree, groups)
	}})
	var prefixRed *batch.PrefixRedError
	if !errors.As(err, &prefixRed) {
		t.Fatalf("prefix red error=%T %v", err, err)
	}
	stored, loadErr := store.Load(batchID)
	if loadErr != nil {
		t.Fatal(loadErr)
	}
	if stored.State != batch.StateDiagnosing || stored.Proof.Status != "prefix-red" || stored.Proof.PrefixGoal != "goal-a" ||
		len(stored.Proof.RedGroups) != 1 || stored.Proof.RedGroups[0].ID != "red-group" || stored.Units[0].State != batch.UnitJoined {
		t.Fatalf("prefix red record=%+v", stored)
	}
}

func TestPrefixReceiptClassifiesRevisionMove(t *testing.T) {
	root := t.TempDir()
	fake := filepath.Join(root, "fake-metasystem")
	script := fmt.Sprintf("#!/usr/bin/env bash\nprintf '%%s\\n' 'GOAL_REVISION_MOVED: goal-a changed after seal' >&2\nexit %d\n", proofrun.ExitAdmissionRefused)
	if err := os.WriteFile(fake, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	original := batchPrefixReceiptExecutable
	t.Cleanup(func() { batchPrefixReceiptExecutable = original })
	batchPrefixReceiptExecutable = func() (string, error) { return fake, nil }
	record := batch.Record{Units: []batch.Unit{{GoalID: "goal-a", Claim: batch.Claim{Revision: 7, AccountingRevision: 5}}}}
	_, err := executeBatchPrefixReceipt(root, "batch", record, "goal-a", "tree", []string{"same"})
	var revision *batch.PrefixRevisionRefusal
	if !errors.As(err, &revision) || !strings.Contains(revision.Error(), "GOAL_REVISION_MOVED") {
		t.Fatalf("revision refusal=%T %v", err, err)
	}
}

func TestBatchDiagnosisForwardsStoredPrefixEvidence(t *testing.T) {
	const batchID = "01j5x00000000000000000ba20"
	root := t.TempDir()
	claim := batch.Claim{Machine: "seat", Lineage: "seat-lineage", Epoch: 1, Revision: 2, AccountingRevision: 1}
	record := batch.Record{Schema: 1, BatchID: batchID, State: batch.StateDiagnosing, BaseTree: "base", TipTree: "tip",
		Units: []batch.Unit{{GoalID: "goal-a", Chain: "chain-a", Claim: claim, State: batch.UnitJoined}},
		Proof: &batch.Proof{Status: "prefix-red", PrefixGoal: "goal-a", RedGroups: []batch.RedGroup{{ID: "red", InputManifest: []string{"a.go"}}}},
	}
	store := batch.NewStore(root, nil)
	if err := store.Create(record); err != nil {
		t.Fatal(err)
	}
	originalDiagnosis, originalOwner := batchDiagnosisSeams, productionTrunkRedLedgerOwner
	t.Cleanup(func() { batchDiagnosisSeams, productionTrunkRedLedgerOwner = originalDiagnosis, originalOwner })
	productionTrunkRedLedgerOwner = func(string) (batch.LedgerOwner, error) { return batch.UnboundLedgerOwner{}, nil }
	batchDiagnosisSeams.commitForTree = func(string, string, string) (string, error) { return "base-commit", nil }
	var groups []batch.RedGroup
	var prefixGoal string
	batchDiagnosisSeams.diagnose = func(_ batch.Store, _, _ string, gotGroups []batch.RedGroup, gotGoal string, _ time.Time, _ batch.RedSeams) error {
		groups, prefixGoal = gotGroups, gotGoal
		return nil
	}
	if err := executeBatchDiagnosis(root, batchID, "owner", time.Unix(2, 0)); err != nil {
		t.Fatal(err)
	}
	if len(groups) != 1 || groups[0].ID != "red" || prefixGoal != "goal-a" {
		t.Fatalf("diagnosis groups=%+v prefixGoal=%q", groups, prefixGoal)
	}
}

func TestBatchLandTrunkMovedRebasesOrReopens(t *testing.T) {
	for _, test := range []struct {
		name, movedPath, manifest, childFailure string
		reopen                                  bool
	}{
		{name: "disjoint", movedPath: "plans/goals/ledger.md", manifest: "source/**"},
		{name: "selected-input", movedPath: "source/input.go", manifest: "source/**", reopen: true},
		{name: "selected-input-conflict", movedPath: "unit.txt", manifest: "unit.txt", reopen: true},
		{name: "held-refusal", movedPath: "plans/goals/ledger.md", manifest: "source/**", childFailure: "held", reopen: true},
		{name: "verify-refusal", movedPath: "plans/goals/ledger.md", manifest: "source/**", childFailure: "verify", reopen: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			root, peer, origin, baseCommit, baseTree, tip := movedBatchGitFixture(t)
			if err := os.WriteFile(filepath.Join(peer, filepath.FromSlash(test.movedPath)), []byte("moved\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			runBatchFixtureGit(t, peer, "add", "--", test.movedPath)
			runBatchFixtureGit(t, peer, "commit", "-qm", "move trunk")
			runBatchFixtureGit(t, peer, "push", "-q", "origin", "main")
			movedCommit := strings.TrimSpace(runBatchFixtureGit(t, peer, "rev-parse", "HEAD"))
			movedTree := strings.TrimSpace(runBatchFixtureGit(t, peer, "rev-parse", "HEAD^{tree}"))
			candidateTree := strings.TrimSpace(runBatchFixtureGit(t, root, "rev-parse", tip+"^{tree}"))
			patch := runBatchFixtureGit(t, root, "diff", "--binary", baseCommit, tip)
			chainDir := filepath.Join(root, "artifacts", "agents", "landing-batches", "chains", "chain-a")
			if err := os.MkdirAll(chainDir, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(chainDir, "diff.patch"), []byte(patch), 0o644); err != nil {
				t.Fatal(err)
			}
			claim := batch.Claim{Machine: "landing", Lineage: "owner", Epoch: 1, Revision: 1, AccountingRevision: 1}
			record := batch.Record{Schema: 1, BatchID: "01j5x00000000000000000ba21", State: batch.StateLanding, BaseTree: baseTree,
				PrefixTrees: []string{candidateTree}, TipTree: candidateTree,
				Units:   []batch.Unit{{GoalID: "goal-a", Chain: "chain-a", Claim: claim, State: batch.UnitJoined}},
				Proof:   &batch.Proof{Status: "green", AttemptID: "tip-proof", SelectedGroups: []string{"selected"}, InputManifests: map[string][]string{"selected": {test.manifest}}},
				Landing: &batch.LandingProgress{Base: baseTree, BranchTip: tip, Commits: map[string]string{}},
				History: []batch.HistoryEntry{{At: time.Unix(1, 0).UTC().Format(time.RFC3339Nano), To: batch.StateLanding, Actor: "owner"}},
			}
			store := batch.NewStore(root, nil)
			if err := store.Create(record); err != nil {
				t.Fatal(err)
			}
			originalChild := batchChildRunner
			originalPush := batchMovedEndpointPush
			t.Cleanup(func() { batchChildRunner, batchMovedEndpointPush = originalChild, originalPush })
			var children [][]string
			pushes, refusedPushes := 0, 0
			batchChildRunner = func(_ string, _ string, args ...string) error {
				children = append(children, append([]string(nil), args...))
				if test.childFailure == "held" && len(args) > 1 && args[0] == "landing" && args[1] == "held" {
					return errors.New("held refusal")
				}
				if test.childFailure == "verify" && len(args) > 1 && args[0] == "test" && args[1] == "verify" {
					return errors.New("verify refusal")
				}
				return nil
			}
			batchMovedEndpointPush = func(root, id, base, tip string) error {
				pushes++
				return originalPush(root, id, base, tip)
			}
			seams := batchLandSeams(root, record.BatchID, record, baseCommit, "owner")
			seams.Prepare = func(string) error { return nil }
			seams.Apply = func(batch.Unit) error { return nil }
			seams.AppendReceipt = func(batch.Unit, batch.PrefixReceipt) error { return nil }
			seams.Commit = func(batch.Unit, batch.PrefixReceipt) (string, error) { return tip, nil }
			seams.Held = func(string, string) error { return nil }
			seams.Push = func(string, string) error {
				refusedPushes++
				return batch.LandLandingBranch(root, record.BatchID, baseCommit, tip)
			}
			if err := batch.LandSeries(store, record.BatchID, "owner", time.Unix(2, 0), seams); err != nil {
				t.Fatal(err)
			}
			landed, err := store.Load(record.BatchID)
			if err != nil {
				t.Fatal(err)
			}
			mainTip := strings.TrimSpace(runBatchFixtureGit(t, origin, "rev-parse", "refs/heads/main"))
			branchRef := "refs/heads/landing/01j5x00000000000000000ba21"
			if test.reopen {
				wantChildren := 0
				if test.childFailure == "held" {
					wantChildren = 1
				} else if test.childFailure == "verify" {
					wantChildren = 2
				}
				if landed.State != batch.StateOpen && landed.State != batch.StateDissolved || landed.BaseTree != movedTree || landed.Landing != nil || mainTip != movedCommit || len(children) != wantChildren || pushes != 0 || refusedPushes != 1 {
					t.Fatalf("input move record=%+v main=%s children=%v pushes=%d refused=%d", landed, mainTip, children, pushes, refusedPushes)
				}
				if test.name == "selected-input-conflict" && (landed.State != batch.StateDissolved || landed.Units[0].State != batch.UnitReturnPending || landed.Units[0].Outcome != batch.UnitEjected) {
					t.Fatalf("conflicting unit was not ejected before dissolve: %+v", landed)
				}
				if landed.State == batch.StateOpen {
					if err := finishBatchLanding(root, store, record.BatchID, "owner", time.Unix(2, 0)); err != nil {
						t.Fatalf("open input-move path entered P6 recovery: %v", err)
					}
				}
			} else {
				if landed.State != batch.StateLanding || landed.Landing == nil || !landed.Landing.PushComplete || mainTip != landed.Landing.PushedTip || len(children) != 2 || children[0][0] != "landing" || children[1][0] != "test" || children[1][1] != "verify" || pushes != 1 || refusedPushes != 1 {
					t.Fatalf("disjoint record=%+v main=%s children=%v pushes=%d refused=%d", landed, mainTip, children, pushes, refusedPushes)
				}
				if err := exec.Command("git", "--git-dir", origin, "merge-base", "--is-ancestor", movedCommit, landed.Landing.PushedTip).Run(); err != nil {
					t.Fatalf("rebased tip does not descend from moved origin: %v", err)
				}
			}
			if err := exec.Command("git", "--git-dir", origin, "show-ref", "--verify", "--quiet", branchRef).Run(); err == nil {
				t.Fatalf("candidate branch %s survived recovery", branchRef)
			}
		})
	}
}

func TestBatchLandProductionSeamsBoundRecoveryAndAbandon(t *testing.T) {
	root, _, _, baseCommit, baseTree, tip := movedBatchGitFixture(t)
	candidateTree := strings.TrimSpace(runBatchFixtureGit(t, root, "rev-parse", tip+"^{tree}"))
	patch := runBatchFixtureGit(t, root, "diff", "--binary", baseCommit, tip)
	chainDir := filepath.Join(root, "artifacts", "agents", "landing-batches", "chains", "chain-a")
	if err := os.MkdirAll(chainDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(chainDir, "diff.patch"), []byte(patch), 0o644); err != nil {
		t.Fatal(err)
	}
	record := batch.Record{Schema: 1, BatchID: "01j5x00000000000000000ba21", State: batch.StateLanding, BaseTree: baseTree,
		PrefixTrees: []string{candidateTree}, TipTree: candidateTree,
		Units: []batch.Unit{{GoalID: "goal-a", Chain: "chain-a", State: batch.UnitJoined,
			Claim: batch.Claim{Machine: "landing", Lineage: "owner", Epoch: 1, Revision: 1, AccountingRevision: 1}}},
		Receipts: map[string]batch.PrefixReceipt{"goal-a": {GoalID: "goal-a", Tree: candidateTree}},
		Proof:    &batch.Proof{Status: "green", AttemptID: "tip-proof"},
		Landing:  &batch.LandingProgress{Base: baseTree, BranchTip: tip, Commits: map[string]string{"goal-a": tip}, HeldChecked: true},
		History:  []batch.HistoryEntry{{At: time.Unix(1, 0).UTC().Format(time.RFC3339Nano), To: batch.StateLanding, Actor: "owner"}},
	}
	store := batch.NewStore(root, nil)
	if err := store.Create(record); err != nil {
		t.Fatal(err)
	}
	originalFetch, originalTree, originalAbandon, originalRecover := batchLandFetchOrigin, batchLandOriginTree, batchLandAbandon, batchLandRecoverPush
	t.Cleanup(func() {
		batchLandFetchOrigin, batchLandOriginTree, batchLandAbandon, batchLandRecoverPush = originalFetch, originalTree, originalAbandon, originalRecover
	})
	origins := []string{tip, tip, baseCommit, tip}
	originReads := 0
	batchLandFetchOrigin = func(string) (string, string, error) {
		origin := origins[min(originReads, len(origins)-1)]
		originReads++
		return origin, "ignored-tree", nil
	}
	treeReads := 0
	batchLandOriginTree = func(string, string) (string, error) {
		treeReads++
		return baseTree, nil
	}
	abandons := 0
	batchLandAbandon = func(_, _ string, _, _ string) error {
		abandons++
		return nil
	}
	recoveries := 0
	batchLandRecoverPush = func(_ string, _ string, _ batch.Record, _ string, origin, _ string, _ string) (batch.PushRecovery, error) {
		recoveries++
		return batch.PushRecovery{Origin: origin}, errors.New("recovery transport unavailable")
	}
	seams := batchLandSeams(root, record.BatchID, record, baseCommit, "owner")
	seams.Prepare = func(string) error { return nil }
	seams.Apply = func(batch.Unit) error { return nil }
	seams.AppendReceipt = func(batch.Unit, batch.PrefixReceipt) error { return nil }
	seams.Commit = func(batch.Unit, batch.PrefixReceipt) (string, error) { return tip, nil }
	seams.Held = func(string, string) error { return nil }
	seams.PublishBranch = func(string, string) error { return nil }
	seams.Push = func(string, string) error {
		return &batch.EndpointPushError{Cause: errors.New("stale info"), StaleLease: true, RemoteRejected: true}
	}
	seams.SeriesOnOrigin = func(string, string) (bool, error) { return false, nil }
	seams.Cleanup = nil
	for tick := 0; tick < 3; tick++ {
		_ = batch.LandSeries(store, record.BatchID, "owner", time.Unix(int64(2+tick), 0), seams)
	}
	landed, err := store.Load(record.BatchID)
	if err != nil {
		t.Fatal(err)
	}
	if seams.LeaseBase != baseCommit || recoveries != 3 || treeReads != 1 || abandons != 1 || landed.State != batch.StateOpen || landed.Landing != nil {
		t.Fatalf("lease=%q recoveries=%d treeReads=%d abandons=%d record=%+v", seams.LeaseBase, recoveries, treeReads, abandons, landed)
	}
}

func TestBatchRebasedVerifyUsesIdentityComposition(t *testing.T) {
	args := batchRebasedVerifyArgs("/landing", "goal-a", "tree-a", []string{"one", "two"})
	if got := strings.Join(args, " "); got != "test verify --root /landing --goal goal-a --tree tree-a --mode auto --purpose delivery --groups one,two" {
		t.Fatalf("rebased verify argv=%v", args)
	}
}

func TestBatchProofInputsMovedIncludesEnginePaths(t *testing.T) {
	record := batch.Record{Proof: &batch.Proof{SelectedGroups: []string{"docs"}, InputManifests: map[string][]string{"docs": {"metasystem/docs/**"}}}}
	if !batchProofInputsMoved(record, []string{"metasystem/internal/other/x.go"}, "metasystem") {
		t.Fatal("an engine path outside the selected manifest did not require a new proof")
	}
}

func TestBatchMovedPushRecoveryDoesNotRetryUnchangedOrigin(t *testing.T) {
	root, _, origin, baseCommit, baseTree, tip := movedBatchGitFixture(t)
	originalPush := batchMovedEndpointPush
	t.Cleanup(func() { batchMovedEndpointPush = originalPush })
	pushes := 0
	batchMovedEndpointPush = func(string, string, string, string) error {
		pushes++
		return nil
	}
	recovery, err := recoverMovedBatchPush(root, "01j5x00000000000000000ba21", batch.Record{}, baseCommit, baseCommit, baseTree, tip)
	if err != nil {
		t.Fatal(err)
	}
	if recovery.Pushed || recovery.Reopen || pushes != 0 {
		t.Fatalf("unchanged origin recovery=%+v pushes=%d", recovery, pushes)
	}
	mainTip := strings.TrimSpace(runBatchFixtureGit(t, origin, "rev-parse", "refs/heads/main"))
	if mainTip != baseCommit {
		t.Fatalf("unchanged origin moved main from %s to %s", baseCommit, mainTip)
	}
}

func movedBatchGitFixture(t *testing.T) (root, peer, origin, baseCommit, baseTree, tip string) {
	t.Helper()
	base := t.TempDir()
	origin, root, peer = filepath.Join(base, "origin.git"), filepath.Join(base, "landing"), filepath.Join(base, "peer")
	if output, err := exec.Command("git", "init", "-q", "--bare", origin).CombinedOutput(); err != nil {
		t.Fatalf("init origin: %v: %s", err, output)
	}
	if output, err := exec.Command("git", "init", "-q", "-b", "main", root).CombinedOutput(); err != nil {
		t.Fatalf("init landing: %v: %s", err, output)
	}
	runBatchFixtureGit(t, root, "config", "user.name", "Fixture")
	runBatchFixtureGit(t, root, "config", "user.email", "fixture@example.com")
	for path, contents := range map[string]string{"source/input.go": "base\n", "plans/goals/ledger.md": "base\n", "unit.txt": "base\n"} {
		full := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	runBatchFixtureGit(t, root, "add", ".")
	runBatchFixtureGit(t, root, "commit", "-qm", "base")
	runBatchFixtureGit(t, root, "remote", "add", "origin", origin)
	runBatchFixtureGit(t, root, "push", "-q", "-u", "origin", "main")
	baseCommit = strings.TrimSpace(runBatchFixtureGit(t, root, "rev-parse", "HEAD"))
	baseTree = strings.TrimSpace(runBatchFixtureGit(t, root, "rev-parse", "HEAD^{tree}"))
	if output, err := exec.Command("git", "clone", "-q", origin, peer).CombinedOutput(); err != nil {
		t.Fatalf("clone peer: %v: %s", err, output)
	}
	runBatchFixtureGit(t, peer, "config", "user.name", "Peer")
	runBatchFixtureGit(t, peer, "config", "user.email", "peer@example.com")
	if err := batch.PrepareLandingBranch(root, "01j5x00000000000000000ba21", baseCommit); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "unit.txt"), []byte("candidate\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runBatchFixtureGit(t, root, "add", "unit.txt")
	runBatchFixtureGit(t, root, "commit", "-qm", "candidate")
	tip = strings.TrimSpace(runBatchFixtureGit(t, root, "rev-parse", "HEAD"))
	if err := batch.PublishLandingBranch(root, "01j5x00000000000000000ba21", "", tip); err != nil {
		t.Fatal(err)
	}
	return root, peer, origin, baseCommit, baseTree, tip
}

func runBatchFixtureGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	commandArgs := append([]string{"-C", root}, args...)
	if strings.HasSuffix(root, ".git") {
		commandArgs = append([]string{"--git-dir", root}, args...)
	}
	output, err := exec.Command("git", commandArgs...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", commandArgs, err, output)
	}
	return string(output)
}

func TestBatchMovedEffectsInventoryIsComplete(t *testing.T) {
	page, err := os.ReadFile(filepath.Join("..", "..", "plans", "units-land-in-batches-under-one-proof-brief-b.md"))
	if err != nil {
		t.Fatal(err)
	}
	report := validate.CheckMovedEffects(page, func(path string) bool {
		_, err := os.Stat(filepath.Join("..", "..", "..", filepath.FromSlash(path)))
		return err == nil
	})
	if report.Inventory != "present" || len(report.Problems) != 0 || len(report.Rows) != 14 {
		t.Fatalf("moved effects inventory=%s problems=%v", report.Inventory, report.Problems)
	}
	text := string(page)
	start, end := strings.Index(text, "| Effect | Old writer |"), strings.Index(text, "The origin/main validator")
	if start < 0 || end <= start {
		t.Fatal("authoritative moved-effects table is absent")
	}
	authoritative := text[start:end]
	for _, effect := range []string{"Claim transfer", "Claim return", "Queue and stale-entry cleanup", "Proof lock", "Re-arm", "Proof reservation", "Receipt row", "Commit", "Push", "Fast-forward", "Goal Next edits", "Trunk-red record", "Worktree cleanup", "Reporting"} {
		if count := strings.Count(authoritative, "| "+effect+" |"); count != 1 {
			t.Fatalf("authoritative moved effect %q occurs %d times, want one row", effect, count)
		}
	}
}
