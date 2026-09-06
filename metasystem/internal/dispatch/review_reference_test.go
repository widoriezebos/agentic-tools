package dispatch

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	critiqueModel "github.com/widoriezebos/agentic-tools/metasystem/internal/critique"
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
		params.GoalID, params.GoalRevision, params.GoalTier, params.MachineID = "", 0, 0, ""
		params.Request.Role = role
		params.Request.SessionKey = session
		params.Reviews = reviews
		result, err := ClaimLaunch(params, claimDependenciesForTest(&now, identity.Verification{}))
		if err != nil || result.Outcome != ClaimWON {
			t.Fatalf("claim %s = %s, %v", job, result.Outcome, err)
		}
	}

	claim("critic-one", "code-critic", "work-r2", "fake:critic-one")
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

func TestStampClaimedDesignCriticWithoutReviewsIsNoOp(t *testing.T) {
	root := t.TempDir()
	writeJSONFile(t, filepath.Join(root, "artifacts", "agents", "jobs"), "design-critic.json", map[string]any{
		"jobId": "design-critic", "role": "design-critic", "round": 1, "parentJob": nil,
		"status": "pending", "dispatchMode": DispatchModeFresh, "reviews": nil,
	})
	if err := StampClaimedReviewReference(root, "design-critic"); err != nil {
		t.Fatalf("fresh design critic without reviews should not stamp a reference: %v", err)
	}
}

type designReferenceFixture struct {
	repo, evidence, work, critic, design string
}

func newDesignReferenceFixture(t *testing.T) designReferenceFixture {
	t.Helper()
	repo, evidence, work := closeReadyHazardChain(t, HazardDesignBearing)
	design := "metasystem/plans/closed-design.md"
	writeJSONFile(t, filepath.Join(repo, "artifacts", "agents", work, "rounds", "1"), "return.json", map[string]any{
		"diffBoundary": []any{design},
	})
	critic := "design-critic"
	writeHazardEvidenceJob(t, repo, critic, HazardDesignBearing, map[string]any{
		"role": "design-critic", "reviews": nil, "parentJob": nil,
		"sessionId": "design-critic-session", "runtime": "codex", "requestedModel": "gpt-5-6-sol",
		"reasoningEffort": "xhigh", "configurationObligations": requiredConfigurationByHazard[HazardDesignBearing],
		"endedAt": "2026-08-30T10:01:00Z", "design": design,
		"declaredOutputs":      []any{"metasystem/records/design-critique.md"},
		"findingRegisterRound": 1,
		"findingRegister": encodeFindingRegister([]registerFinding{{
			FindingID: "F-resolved", Critic: critic, RigorClass: critiqueModel.Bounded,
			FactsDigest: strings.Repeat("a", 64), Facts: registerFacts(), Artifact: design,
			Title: "resolved design finding", Status: "resolved", Resolution: "withdrawn",
			Evidence: "the final design resolves the finding", EvidenceDigest: strings.Repeat("b", 64), Multiplicity: 1,
		}}),
	})
	return designReferenceFixture{repo: repo, evidence: evidence, work: work, critic: critic, design: design}
}

func (f designReferenceFixture) criticRecord(t *testing.T) map[string]any {
	t.Helper()
	return readRecord(t, f.repo, f.critic)
}

func (f designReferenceFixture) writeCriticRecord(t *testing.T, record map[string]any) {
	t.Helper()
	if err := writeRecord(filepath.Join(f.repo, "artifacts", "agents", "jobs", f.critic+".json"), record); err != nil {
		t.Fatal(err)
	}
}

func (f designReferenceFixture) assertUnstamped(t *testing.T) {
	t.Helper()
	if got := asString(readRecord(t, f.repo, f.work)[independentCritiqueReferenceField]); got != "" {
		t.Fatalf("failed reconciliation stamped critique reference %q", got)
	}
	if got := asString(f.criticRecord(t)["reviews"]); got != "" {
		t.Fatalf("failed reconciliation stamped reviews %q", got)
	}
}

func TestReconcileDesignCriticClosesDesignBearingChain(t *testing.T) {
	f := newDesignReferenceFixture(t)
	if err := ReconcileReviewReference(f.repo, f.work, f.critic); err != nil {
		t.Fatalf("reconcile closed design critique: %v", err)
	}
	if got := readRecord(t, f.repo, f.work)[independentCritiqueReferenceField]; got != f.critic {
		t.Fatalf("independent critique reference = %v", got)
	}
	if got := f.criticRecord(t)["reviews"]; got != f.work {
		t.Fatalf("design-critic reviews = %v", got)
	}
	refreshHazardMirror(t, f.repo, f.evidence, f.work)
	if err := CloseCheck(f.repo, f.work); err != nil {
		t.Fatalf("design-bearing chain did not close after reconciliation: %v", err)
	}
}

func TestReconcileDesignCriticRefusesOpenRegisterFinding(t *testing.T) {
	f := newDesignReferenceFixture(t)
	record := f.criticRecord(t)
	record["findingRegister"] = encodeFindingRegister([]registerFinding{{
		FindingID: "F-open", Critic: f.critic, RigorClass: critiqueModel.Bounded,
		FactsDigest: strings.Repeat("a", 64), Facts: registerFacts(), Artifact: f.design,
		Title: "open design finding", Status: "open",
		Evidence: "the design still has an open finding", EvidenceDigest: strings.Repeat("b", 64), Multiplicity: 1,
	}})
	f.writeCriticRecord(t, record)
	err := ReconcileReviewReference(f.repo, f.work, f.critic)
	if err == nil || !strings.Contains(err.Error(), "open or disputed finding identifiers: F-open") {
		t.Fatalf("open finding reconciliation = %v", err)
	}
	f.assertUnstamped(t)
}

func TestReconcileDesignCriticRefusesMismatchedDesignBoundary(t *testing.T) {
	f := newDesignReferenceFixture(t)
	other := "metasystem/plans/other-design.md"
	writeJSONFile(t, filepath.Join(f.repo, "artifacts", "agents", f.work, "rounds", "1"), "return.json", map[string]any{
		"diffBoundary": []any{other},
	})
	err := ReconcileReviewReference(f.repo, f.work, f.critic)
	if err == nil || !strings.Contains(err.Error(), f.design) || !strings.Contains(err.Error(), other) {
		t.Fatalf("mismatched design reconciliation = %v", err)
	}
	f.assertUnstamped(t)
}

func TestReconcileDesignCriticRefusesCritiqueBeforeFinalWork(t *testing.T) {
	f := newDesignReferenceFixture(t)
	record := f.criticRecord(t)
	record["endedAt"] = "2026-08-30T09:59:00Z"
	f.writeCriticRecord(t, record)
	err := ReconcileReviewReference(f.repo, f.work, f.critic)
	if err == nil || !strings.Contains(err.Error(), "2026-08-30T09:59:00Z") ||
		!strings.Contains(err.Error(), "2026-08-30T10:00:00Z") || !strings.Contains(err.Error(), "unexamined") {
		t.Fatalf("early design critique reconciliation = %v", err)
	}
	f.assertUnstamped(t)
}

func TestReconcileLegacyDesignCriticAcceptsExplicitPairing(t *testing.T) {
	f := newDesignReferenceFixture(t)
	record := f.criticRecord(t)
	delete(record, "design")
	f.writeCriticRecord(t, record)
	if err := ReconcileReviewReference(f.repo, f.work, f.critic); err != nil {
		t.Fatalf("legacy explicit pairing: %v", err)
	}
	if got := readRecord(t, f.repo, f.work)[independentCritiqueReferenceField]; got != f.critic {
		t.Fatalf("legacy independent critique reference = %v", got)
	}
	if got := f.criticRecord(t)["reviews"]; got != f.work {
		t.Fatalf("legacy design-critic reviews = %v", got)
	}
}
