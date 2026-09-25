package dispatch

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

func TestSTR4R1RaiseTransactionDispatchSnapshots(t *testing.T) {
	bed := newGoalAdmissionBed(t, 2)
	root := bed.root
	now := time.Date(2026, 8, 28, 9, 30, 0, 0, time.UTC)
	low := goal.RiskRecord{Severity: 1, Novelty: 1, Exposure: 1, Accumulation: 1, Basis: "landed precedent"}
	bed.commitRisk(t, &low, 1)
	before, err := bed.binding("bounded", now)
	if err != nil || before.Tier != 1 || before.GateWidth != "area" {
		t.Fatalf("pre-raise binding = %+v, %v", before, err)
	}
	capFile := writeJSONFile(t, t.TempDir(), "cap.json", map[string]any{
		"capMin": 5, "capDeadline": "2026-08-28T10:00:00Z",
		"source": map[string]any{"rule": "fixture", "origin": "fixture", "truncatedBy": nil},
	})
	oldSetup := filepath.Join(t.TempDir(), "old.json")
	if err := buildSetupWithGoalReads(root, oldSetup, "old-root", "implementer", "", "main", "7", "bounded", before.Revision, before.Tier, capFile, "bed-m1", "", bed.reads); err != nil {
		t.Fatal(err)
	}
	raised := goal.RiskRecord{Severity: 1, Novelty: 1, Exposure: 1, Accumulation: 2, Basis: "accumulation discovered"}
	bed.commitRisk(t, &raised, 2)
	after, err := bed.binding("bounded", now)
	if err != nil || after.Tier != 2 || after.GateWidth != "full" {
		t.Fatalf("post-raise binding = %+v, %v", after, err)
	}
	oldRecord := readJSONFile(t, oldSetup)
	if tier, _ := numInt(oldRecord["goalTier"]); tier != 1 || oldRecord["gateWidth"] != "area" {
		t.Fatalf("already-dispatched root changed after raise: %+v", oldRecord)
	}
	newSetup := filepath.Join(t.TempDir(), "new.json")
	if err := buildSetupWithGoalReads(root, newSetup, "new-root", "implementer", "", "main", "7", "bounded", after.Revision, after.Tier, capFile, "bed-m1", "", bed.reads); err != nil {
		t.Fatal(err)
	}
	newRecord := readJSONFile(t, newSetup)
	if tier, _ := numInt(newRecord["goalTier"]); tier != 2 || newRecord["gateWidth"] != "full" {
		t.Fatalf("next dispatch did not read raised tier and width: %+v", newRecord)
	}
}

func TestRiskGateAdmissionMarksThenEnforces(t *testing.T) {
	bed := newGoalAdmissionBed(t, 2)
	now := time.Date(2026, 8, 28, 9, 30, 0, 0, time.UTC)
	bed.commitRisk(t, nil, 3)
	unanswered, err := bed.revisionAdmission("bounded", 2, 5, now, HazardMechanical)
	if err != nil || !unanswered.Refused() || unanswered.PolicyRefusal != "RISK_UNANSWERED goal=bounded tier=3 next: goal edit --risk" {
		t.Fatalf("unanswered-risk admission = %+v, %v", unanswered, err)
	}
	risk := goal.RiskRecord{Severity: 3, Novelty: 3, Exposure: 1, Accumulation: 1, Basis: "The fixture answers every risk question."}
	bed.commitRisk(t, &risk, 3)
	answered, err := bed.revisionAdmission("bounded", 2, 5, now, HazardMechanical)
	if err != nil || answered.Refused() {
		t.Fatalf("answered-risk admission = %+v, %v", answered, err)
	}
}

func TestSTR4R1EvidenceGrammar(t *testing.T) {
	root := t.TempDir()
	jobs := filepath.Join(root, "artifacts", "agents", "jobs")
	if err := os.MkdirAll(jobs, 0o755); err != nil {
		t.Fatal(err)
	}
	job := []byte(`{"jobId":"risk-root","goalId":"risk-goal","findingRegister":[{"findingId":"F-1"}]}`)
	if err := os.WriteFile(filepath.Join(jobs, "risk-root.json"), job, 0o644); err != nil {
		t.Fatal(err)
	}
	for _, evidence := range []string{"root:risk-root", "finding:risk-root/F-1", "refusal:BUDGET_REFUSED"} {
		if err := ValidateMisclassificationEvidence(root, "risk-goal", evidence); err != nil {
			t.Errorf("existing %s refused: %v", evidence, err)
		}
	}
	for _, evidence := range []string{"root:missing", "finding:risk-root/F-2", "refusal:NOT_A_CODE"} {
		if err := ValidateMisclassificationEvidence(root, "risk-goal", evidence); err == nil {
			t.Errorf("missing or unlisted %s was admitted", evidence)
		}
	}
	if err := ValidateMisclassificationEvidence(root, "risk-goal", "refusal:NOT_A_CODE"); err == nil || !strings.Contains(err.Error(), strings.Join(AdmissionRefusalCodes, ", ")) {
		t.Fatalf("unlisted-code refusal does not name the complete list: %v", err)
	}
}
