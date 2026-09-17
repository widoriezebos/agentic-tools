package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/validate"
)

func TestBatchStatusExposesReturnRevisionHeadroomOwnerLockSampleAndDeadline(t *testing.T) {
	originalOwner, originalLock, originalSample := batchStatusOwner, batchStatusLock, batchStatusSample
	t.Cleanup(func() {
		batchStatusOwner, batchStatusLock, batchStatusSample = originalOwner, originalLock, originalSample
	})
	batchStatusOwner = func(string) (int64, identity.Liveness, error) { return 41, identity.Alive, nil }
	batchStatusLock = func() string { return "landing-batch-owner 41" }
	batchStatusSample = func(string) proofrun.LoadSample { return proofrun.LoadSample{OverlapKnown: true, OverlappingHost: 1} }
	settings, err := config.NewBatchLanding(t.TempDir(), 45*time.Minute, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	record := batch.Record{BatchID: "batch", State: batch.StateLanding, Proof: &batch.Proof{Status: "green"}, Units: []batch.Unit{{
		GoalID: "goal-a", Chain: "chain-a", State: batch.UnitReturnPending, Outcome: batch.UnitLanded,
		Claim: batch.Claim{Epoch: 3, Revision: 8, AccountingRevision: 5},
	}}}
	record.History = []batch.HistoryEntry{{At: "2030-01-01T00:00:00Z", Verb: "join"}}
	view := batchRecordStatus(record, settings)
	if view.Owner != "41" || view.OwnerLiveness == "" || !strings.Contains(view.Lock, "41") || view.Headroom != "green" || view.Deadline != "2030-01-01T00:45:00Z" ||
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
