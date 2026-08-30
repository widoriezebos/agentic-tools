package dispatch

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

func TestClaimLaunchDerivesReviewReferencesOnReviewedChainRoot(t *testing.T) {
	root := t.TempDir()
	jobs := filepath.Join(root, "artifacts", "agents", "jobs")
	writeJSONFile(t, jobs, "work.json", map[string]any{
		"jobId": "work", "role": "implementer", "round": 1, "parentJob": nil, "status": "completed",
	})
	writeJSONFile(t, jobs, "work-r2.json", map[string]any{
		"jobId": "work-r2", "role": "implementer", "round": 2, "parentJob": "work", "status": "completed",
	})
	now := time.Date(2026, 8, 30, 10, 0, 0, 0, time.UTC)
	claim := func(job, role, reviews, session string) {
		t.Helper()
		params := claimParamsForTest(root, job)
		params.GoalID, params.GoalRevision, params.MachineID = "", 0, ""
		params.Request.Role = role
		params.Request.SessionKey = session
		params.Reviews = reviews
		result, err := ClaimLaunch(params, claimDependenciesForTest(&now, identity.Verification{}))
		if err != nil || result.Outcome != ClaimWON {
			t.Fatalf("claim %s = %s, %v", job, result.Outcome, err)
		}
	}

	claim("critic-one", "code-critic", "work", "fake:critic-one")
	if got := readRecord(t, root, "work")[independentCritiqueReferenceField]; got != "critic-one" {
		t.Fatalf("first critique reference = %v", got)
	}
	claim("critic-two", "warden", "work-r2", "fake:critic-two")
	if got := readRecord(t, root, "work")[independentCritiqueReferenceField]; got != "critic-two" {
		t.Fatalf("newer critique did not replace the pointer: %v", got)
	}
	claim("proof-two", "verifier", "work-r2", "fake:proof-two")
	if got := readRecord(t, root, "work")[liveProofReferenceField]; got != "proof-two" {
		t.Fatalf("live-proof reference = %v", got)
	}
}

func TestReconcileReviewReferenceRequiresFoldedTerminalCoverage(t *testing.T) {
	root := t.TempDir()
	jobs := filepath.Join(root, "artifacts", "agents", "jobs")
	writeJSONFile(t, jobs, "work.json", map[string]any{
		"jobId": "work", "role": "implementer", "round": 1, "parentJob": nil,
		"status": "completed", "endedAt": "2026-08-30T10:00:00Z",
	})
	writeJSONFile(t, jobs, "work-r2.json", map[string]any{
		"jobId": "work-r2", "role": "implementer", "round": 2, "parentJob": "work",
		"status": "completed", "endedAt": "2026-08-30T10:01:00Z",
	})
	criticPath := filepath.Join(jobs, "critic.json")
	writeJSONFile(t, jobs, "critic.json", map[string]any{
		"jobId": "critic", "role": "code-critic", "round": 1, "parentJob": nil,
		"reviews": "work-r2", "status": "completed", "findingRegister": []any{}, "findingRegisterRound": 0,
	})
	if err := ReconcileReviewReference(root, "work", "critic"); err == nil || !strings.Contains(err.Error(), "has not been folded") {
		t.Fatalf("unfolded critique reconciliation = %v", err)
	}
	critic := readJSONFile(t, criticPath)
	critic["findingRegisterRound"] = 1
	writeRecord(criticPath, critic)
	if err := ReconcileReviewReference(root, "work", "critic"); err != nil {
		t.Fatalf("folded terminal critique reconciliation: %v", err)
	}
	if got := readRecord(t, root, "work")[independentCritiqueReferenceField]; got != "critic" {
		t.Fatalf("reconciled critique reference = %v", got)
	}
	critic["reviews"] = "work"
	writeRecord(criticPath, critic)
	if err := ReconcileReviewReference(root, "work", "critic"); err == nil || !strings.Contains(err.Error(), "instead of terminal work round") {
		t.Fatalf("stale critique reconciliation = %v", err)
	}
}
