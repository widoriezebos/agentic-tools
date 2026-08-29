package dispatch

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

func TestSliceCapRefusesWithoutAndAdmitsWithRecordedApproval(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.budget.slice-norm-hours=4\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "memory"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "memory", "rulings.md"), []byte("| ID | Ruling |\n| R-16 | oversized slice approved |\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	refused, err := EvaluateSliceAdmission(root, 241, "")
	if err != nil || !refused.Refused() || !strings.Contains(refused.Refusal, "bring the slice to Wido") ||
		!strings.Contains(refused.Refusal, "carve it down") {
		t.Fatalf("oversized unapproved slice was not refused with its remedy: %+v %v", refused, err)
	}
	approved, err := EvaluateSliceAdmission(root, 241, "R-16")
	if err != nil || approved.Refused() {
		t.Fatalf("recorded approval did not admit the oversized slice: %+v %v", approved, err)
	}
	invalid, err := EvaluateSliceAdmission(root, 241, "R-404")
	if err != nil || !invalid.Refused() || !strings.Contains(invalid.Refusal, "does not name") {
		t.Fatalf("unrecorded approval reference was accepted: %+v %v", invalid, err)
	}
}

func TestApprovedRefIsStoredAndImmutableOnTheReservation(t *testing.T) {
	root := t.TempDir()
	stage := t.TempDir()
	capFile := writeJSON(t, filepath.Join(stage, "cap.json"), map[string]any{
		"capMin": 300, "capDeadline": "2026-08-29T16:00:00Z",
		"source": map[string]any{"rule": "fixture", "origin": "fixture", "truncatedBy": nil},
	})
	setup := filepath.Join(stage, "setup.json")
	if err := BuildSetup(setup, "long-slice", "implementer", "", "main-1", "5", "", 0, capFile, "", "R-16"); err != nil {
		t.Fatal(err)
	}
	if JobRecordOf(readJSONFile(t, setup)).ApprovedRef() != "R-16" {
		t.Fatal("the reservation husk lost its approval reference")
	}
	if err := RecordCreate(root, "long-slice", setup); err != nil {
		t.Fatal(err)
	}
	patch := writeJSON(t, filepath.Join(stage, "patch.json"), map[string]any{"approvedRef": "R-18"})
	if _, err := RecordCAS(root, "long-slice", "pending-setup", "pending-setup", patch); err == nil ||
		!strings.Contains(err.Error(), "immutable identity") {
		t.Fatalf("approvedRef was mutable after reservation: %v", err)
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
		Targets: []string{"bounded"}, Keep: -1,
	})
	if err := os.WriteFile(path, goal.RenderFile(file), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"add", "plans/goals/bounded.md"}, {"commit", "-q", "-m", "human approval"}, {"update-ref", goal.AcceptedRef, "HEAD"}} {
		if output, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, output)
		}
	}
	verdict, err := EvaluateSliceAdmission(root, 241, humanRef)
	if err != nil || verdict.Refused() {
		t.Fatalf("human goal-history approval did not admit the slice: %+v %v", verdict, err)
	}
}
