package dispatch

import (
	"path/filepath"
	"strings"
	"testing"

	critiqueModel "github.com/widoriezebos/agentic-tools/metasystem/internal/critique"
)

// TestOutOfScopeDemotedFinding: a material finding the fold demoted (its
// artifact lies outside the reviewed subject) never enters the register, so
// the author's out-of-scope decision for it has nothing to resolve and is not
// an absent identifier. An identifier neither in the register nor demoted is
// still refused, and nothing is written.
func TestOutOfScopeDemotedFinding(t *testing.T) {
	repo := t.TempDir()
	path := filepath.Join(repo, "artifacts", "agents", "jobs")
	kept := registerFinding{FindingID: "F-2", Critic: "critic", RigorClass: critiqueModel.Unproven, FactsDigest: digestJSON(registerFacts()), Facts: registerFacts(),
		Artifact: "metasystem/in.go", Title: "in subject", Status: "accepted-risk", Resolution: "accepted-risk", DecisionOpID: "op-1", Evidence: "proof", EvidenceDigest: digestJSON("proof"), Multiplicity: 1}
	root := map[string]any{"jobId": "critic", "role": "code-critic", findingRegisterRoundField: 1,
		findingRegisterField: encodeFindingRegister([]registerFinding{kept}),
		"demotions":          []any{map[string]any{"round": 1, "findingId": "F-1", "artifact": "metasystem/out.go", "reason": "artifact is outside the reviewed subject set"}}}
	writeJSONFile(t, path, "critic.json", root)
	before := readJSONFile(t, filepath.Join(path, "critic.json"))
	if err := CritiqueRegisterResolveOutOfScope(repo, "critic", []string{"F-1"}); err != nil {
		t.Fatalf("a demoted finding decided out-of-scope: %v", err)
	}
	if err := CritiqueRegisterResolveOutOfScope(repo, "critic", []string{"F-1", "F-9"}); err == nil ||
		!strings.Contains(err.Error(), "finding identifiers are absent: F-9") || strings.Contains(err.Error(), "F-1") {
		t.Fatalf("an unknown identifier beside a demoted one = %v", err)
	}
	after := readJSONFile(t, filepath.Join(path, "critic.json"))
	if string(canonicalJSON(before)) != string(canonicalJSON(after)) {
		t.Fatal("a demoted or refused out-of-scope decision changed the register")
	}
}

// TestOutOfScopeAcceptedRiskFinding: a severe finding a person accepted as
// risk is resolved by that record, so an out-of-scope row for it is not
// refused for its severity and leaves it accepted-risk. A severe open finding
// is still refused.
func TestOutOfScopeAcceptedRiskFinding(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	path := filepath.Join(repo, "artifacts", "agents", "jobs")
	accepted := registerFinding{FindingID: "S-1", Critic: "critic", RigorClass: critiqueModel.Severe, FactsDigest: digestJSON(registerFacts()), Facts: registerFacts(),
		Artifact: "metasystem/in.go", Title: "accepted", Status: "accepted-risk", Resolution: "accepted-risk", DecisionOpID: "op-1", Evidence: "evidence", EvidenceDigest: digestJSON("evidence"), Multiplicity: 1}
	open := accepted
	open.FindingID, open.Status, open.Resolution, open.DecisionOpID = "S-2", "open", "", ""
	writeJSONFile(t, path, "critic.json", map[string]any{"jobId": "critic", "role": "code-critic", findingRegisterRoundField: 1,
		findingRegisterField: encodeFindingRegister([]registerFinding{accepted, open})})
	before := readJSONFile(t, filepath.Join(path, "critic.json"))
	if err := CritiqueRegisterResolveOutOfScope(repo, "critic", []string{"S-1"}); err != nil {
		t.Fatalf("an accepted risk decided out-of-scope = %v", err)
	}
	if after := readJSONFile(t, filepath.Join(path, "critic.json")); string(canonicalJSON(before)) != string(canonicalJSON(after)) {
		t.Fatal("an out-of-scope row changed a finding a person accepted as risk")
	}
	if ids, err := CritiqueAcceptedRiskFindingIDs(repo, "critic"); err != nil || strings.Join(ids, ",") != "S-1" {
		t.Fatalf("accepted risks = %v, %v", ids, err)
	}
	if err := CritiqueRegisterResolveOutOfScope(repo, "critic", []string{"S-1", "S-2"}); err == nil ||
		!strings.Contains(err.Error(), "finding S-2 is severe and cannot be resolved out-of-scope") {
		t.Fatalf("a severe open finding decided out-of-scope = %v", err)
	}
	if after := readJSONFile(t, filepath.Join(path, "critic.json")); string(canonicalJSON(before)) != string(canonicalJSON(after)) {
		t.Fatal("a refused out-of-scope decision changed the register")
	}
}
