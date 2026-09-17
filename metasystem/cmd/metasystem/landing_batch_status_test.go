package main

import (
	"encoding/json"
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
	batchStatusLock = func(string) string { return "landing-batch-owner 41" }
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
		Claim: batch.Claim{Epoch: 3, Revision: 8, AccountingRevision: 5},
	}}}
	record.History = []batch.HistoryEntry{{At: "2030-01-01T00:00:00Z", Verb: "join"}}
	view := batchRecordStatus(record, settings)
	if view.Owner != "41" || view.OwnerLiveness == "" || !strings.Contains(view.Lock, "41") || view.ProofStatus != "green" ||
		len(view.Headroom) != 1 || view.Headroom[0].Status != string(dispatchcore.BudgetKnown) || view.Headroom[0].AttemptsLeft != 1 ||
		view.Headroom[0].ReservedMinutesLeft != 100 || view.Headroom[0].HasDiagnosticHeadroom || view.Deadline != "2030-01-01T00:45:00Z" ||
		len(view.Units) != 1 || view.Units[0].State != batch.UnitReturnPending || view.Units[0].Revision != 8 || view.Units[0].AccountingRevision != 5 || !view.Sample.OverlapKnown {
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
