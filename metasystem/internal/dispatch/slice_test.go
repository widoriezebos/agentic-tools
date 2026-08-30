package dispatch

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

func TestSliceCapRefusesWithoutAndAdmitsWithRecordedApproval(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.budget.slice-norm-hours=4\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "memory"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "memory", "rulings.md"), []byte("| ID | Ruling |\n| R-16 | oversized slice approved for goal-a |\n| R-17 | goal=goal-a capMin=300 goalRevision=7 |\n| R-18 | goal-b oversized slice approved capMin=300 goalRevision=2; goal-a is unaffected |\n| R-19 | slice approved for goal-a; see rev 2 of the design doc, backlog-cap300 applies |\n| R-20b | bootstrap core approved for l13 |\n| R-21-m2 | machine ruling approved for goal-a |\n| R-22 | goal=goal-b capMin=999 goalRevision=9 goal=goal-a capMin=300 goalRevision=7 |\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	refused, err := EvaluateSliceAdmission(root, 241, "", "goal-a", 7)
	if err != nil || !refused.Refused() || !strings.Contains(refused.Refusal, "bring the slice to Wido") ||
		!strings.Contains(refused.Refusal, "carve it down") {
		t.Fatalf("oversized unapproved slice was not refused with its remedy: %+v %v", refused, err)
	}
	for _, request := range []struct {
		cap      uint64
		revision uint64
	}{{cap: 241, revision: 7}, {cap: 999, revision: 999}} {
		vague, err := EvaluateSliceAdmission(root, request.cap, "R-16", "goal-a", request.revision)
		if err != nil || !vague.Refused() || vague.Reason != "REFUSED-APPROVAL-CAP-UNPROVEN" {
			t.Fatalf("vague R-16 admitted cap=%d revision=%d: %+v %v", request.cap, request.revision, vague, err)
		}
		if !strings.Contains(vague.Refusal, "goal=<id> capMin=<n> goalRevision=<r>") {
			t.Fatalf("vague approval refusal did not teach the required form: %q", vague.Refusal)
		}
	}
	for _, ref := range []string{"R-18", "R-19"} {
		prose, err := EvaluateSliceAdmission(root, 300, ref, "goal-a", 2)
		if err != nil || !prose.Refused() || prose.Reason != "REFUSED-APPROVAL-CAP-UNPROVEN" ||
			!strings.Contains(prose.Refusal, "goal=<id> capMin=<n> goalRevision=<r>") {
			t.Fatalf("prose-scraped approval %s admitted goal-a: %+v %v", ref, prose, err)
		}
	}
	approved, err := EvaluateSliceAdmission(root, 300, "R-17", "goal-a", 7)
	if err != nil || approved.Refused() || approved.ApprovalClaim == nil ||
		approved.ApprovalClaim.CapMinutes != 300 || approved.ApprovalClaim.GoalRevision != 7 {
		t.Fatalf("cap-bound approval did not admit its proven coordinates: %+v %v", approved, err)
	}
	above, err := EvaluateSliceAdmission(root, 301, "R-17", "goal-a", 7)
	if err != nil || !above.Refused() || above.Reason != "REFUSED-APPROVAL-CAP-EXCEEDED" {
		t.Fatalf("cap-bound approval admitted above its proven cap: %+v %v", above, err)
	}
	drifted, err := EvaluateSliceAdmission(root, 300, "R-17", "goal-a", 8)
	if err != nil || !drifted.Refused() || drifted.Reason != "REFUSED-APPROVAL-REVISION-MISMATCH" {
		t.Fatalf("revision drift reused an old approval: %+v %v", drifted, err)
	}
	multi, err := EvaluateSliceAdmission(root, 300, "R-22", "goal-a", 7)
	if err != nil || multi.Refused() || multi.ApprovalClaim == nil || multi.ApprovalClaim.CapMinutes != 300 {
		t.Fatalf("matching strict triple did not bind independently of another goal's triple: %+v %v", multi, err)
	}
	unrelated, err := EvaluateSliceAdmission(root, 241, "R-20b", "goal-a", 7)
	if err != nil || !unrelated.Refused() || unrelated.Reason != "REFUSED-APPROVAL-CAP-UNPROVEN" {
		t.Fatalf("unrelated R-20b approval was accepted: %+v %v", unrelated, err)
	}
	nonslice, err := EvaluateSliceAdmission(root, 241, "R-21-m2", "goal-a", 7)
	if err != nil || !nonslice.Refused() || nonslice.Reason != "REFUSED-APPROVAL-CAP-UNPROVEN" {
		t.Fatalf("non-slice ruling was accepted as slice approval: %+v %v", nonslice, err)
	}
	invalid, err := EvaluateSliceAdmission(root, 241, "R-404", "goal-a", 7)
	if err != nil || !invalid.Refused() || !strings.Contains(invalid.Refusal, "does not name") {
		t.Fatalf("unrecorded approval reference was accepted: %+v %v", invalid, err)
	}
}

func TestApprovedRefIsStoredAndImmutableOnTheReservation(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.budget.slice-norm-hours=4\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "memory"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "memory", "rulings.md"), []byte("| ID | Ruling |\n| R-16 | oversized slice approved goal=goal-a capMin=300 goalRevision=2 |\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	params := claimParamsForTest(root, "long-slice")
	params.ApprovedRef = "R-16"
	params.GoalRevision = 2
	params.Request.CapMinutes = 300
	params.DefaultCapMinutes = 300
	now := time.Date(2026, 8, 30, 10, 0, 0, 0, time.UTC)
	result, err := ClaimLaunch(params, claimDependenciesForTest(&now, identity.Verification{}))
	if err != nil || result.Outcome != ClaimWON {
		t.Fatalf("approved reservation was not published: %+v %v", result, err)
	}
	reservation := readRecord(t, root, "long-slice")
	if JobRecordOf(reservation).ApprovedRef() != "R-16" {
		t.Fatal("the reservation husk lost its approval reference")
	}
	claim, ok := reservation["sliceApprovalClaim"].(map[string]any)
	if !ok || claim["goalId"] != "goal-a" || claim["approvedRef"] != "R-16" ||
		!looseEqual(claim["capMin"], 300) || !looseEqual(claim["goalRevision"], 2) {
		t.Fatalf("reservation lost its complete approval claim: %v", claim)
	}
	patch := writeJSON(t, filepath.Join(t.TempDir(), "patch.json"), map[string]any{"sliceApprovalClaim": map[string]any{
		"approvedRef": "R-16", "goalId": "goal-a", "goalRevision": 2, "capMin": 999,
	}})
	if _, err := RecordCAS(root, "long-slice", "pending-setup", "pending-setup", patch); err == nil ||
		!strings.Contains(err.Error(), "immutable identity") {
		t.Fatalf("proven approval coordinates were mutable after reservation: %v", err)
	}
}

func TestSliceAdmissionAcceptsHumanGoalHistoryOperation(t *testing.T) {
	root := revisionBindingBed(t, 2)
	path := filepath.Join(root, "plans", "goals", "bounded.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	file, problems := goal.ParseFile(data)
	if len(problems) != 0 {
		t.Fatalf("fixture goal did not parse: %v", problems)
	}
	humanRef := "01ARZ3NDEKTSV4RRFFQ69G5FAZ-bed-m1-00000004"
	file.Revision++
	file.History = append(file.History, goal.HistoryLine{
		At: "2026-08-29T10:00:00Z", Opid: humanRef, Verb: "edit", Actor: "human:wido",
		Targets: []string{"bounded"}, Keep: -1, Reason: "goal=bounded capMin=300 goalRevision=2",
	})
	if err := os.WriteFile(path, goal.RenderFile(file), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"add", "plans/goals/bounded.md"}, {"commit", "-q", "-m", "human approval"}, {"update-ref", goal.AcceptedRef, "HEAD"}} {
		if output, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, output)
		}
	}
	verdict, err := EvaluateSliceAdmission(root, 241, humanRef, "bounded", 2)
	if err != nil || verdict.Refused() || verdict.ApprovalClaim == nil ||
		verdict.ApprovalClaim.CapMinutes != 300 || verdict.ApprovalClaim.GoalRevision != 2 {
		t.Fatalf("human goal-history approval did not admit the slice: %+v %v", verdict, err)
	}
}
