package missionrunner

// Covenant self-custody (the adopted H2 ruling): covenant.json is a
// member of every guardrail class by construction, a host may never
// declare it as an artifact, and even the warden's lane may not move
// the covenant's identity or battery — those changes escalate to the
// human tier through the wall's governance violation, which the
// recovery ladder's mechanical rung never touches.

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/mission"
)

func covenantBody(metric, threshold, firstRequirementProof string) string {
	return fmt.Sprintf(`{
  "schemaVersion": 1,
  "identity": {"name": "app", "entryPoint": "./run", "sourcePaths": ["src/"]},
  "requirements": [{"id": "1", "ref": "spec 1", "proof": %q}],
  "battery": {"command": "bash gate.sh", "metric": %q, "direction": "max", "threshold": %q},
  "budgets": [],
  "guards": [],
  "guardrails": ["gate.sh"]
}`, firstRequirementProof, metric, threshold)
}

// An UNDECLARED net still custodies the covenant: an ordinary
// authorization touching covenant.json refuses even when the contract
// declares no guardrails at all — membership is construction, not
// declaration.
func TestWallCustodiesTheCovenantByConstruction(t *testing.T) {
	root := wallRepo(t)
	writeText(t, filepath.Join(root, "covenant.json"), covenantBody("score", ">=3", "check-1"))
	pre := snapshotTree(t, root)
	writeText(t, filepath.Join(root, "covenant.json"), covenantBody("score", ">=1", "check-1"))
	reviewed := snapshotTree(t, root)
	emptyNet, gviol := mission.ParseGuardrails(mission.ContractGuardrailSubject, "", protectedArtifactPath)
	if gviol != "" {
		t.Fatal(gviol)
	}
	plain := wallAuthorization(t, root, "demo", pre, reviewed, nil)
	certified := []map[string]any{{"jobId": "job-c", "verdict": "accepted", "authorizationDigest": plain}}
	inspection, err := inspectWall(root, "demo", pre, wallState(), certified, map[string]bool{}, emptyNet, "", legacySnapshot(root, "demo"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(inspection.Violation, "outside the warden's lane") {
		t.Fatalf("the covenant must be custodied under an empty net: %q", inspection.Violation)
	}
	if inspection.UndeclaredOnly {
		t.Fatal("a custody violation must stay off the mechanical rung")
	}
}

// A host may never declare the covenant as its own artifact: the
// contradiction fires from the same construction.
func TestHostArtifactDeclaringCovenantContradicts(t *testing.T) {
	emptyNet, gviol := mission.ParseGuardrails(mission.ContractGuardrailSubject, "", protectedArtifactPath)
	if gviol != "" {
		t.Fatal(gviol)
	}
	contradiction := mission.GuardrailContradiction(map[string]bool{"covenant.json": true}, emptyNet)
	if !strings.Contains(contradiction, "never a free host write") {
		t.Fatalf("declaring the covenant as a host artifact must contradict: %q", contradiction)
	}
	// The same contradiction through the wall's own declaration path.
	root := wallRepo(t)
	pre := snapshotTree(t, root)
	inspection, err := inspectWall(root, "demo", pre, wallState(), nil,
		map[string]bool{"covenant.json": true}, emptyNet, contradiction, legacySnapshot(root, "demo"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(inspection.Violation, "never a free host write") {
		t.Fatalf("the wall must carry the contradiction as its violation: %q", inspection.Violation)
	}
}

// The warden may move a requirement row; the battery escalates to the
// human tier even down the warden's lane.
func TestWardenLaneCovenantGovernance(t *testing.T) {
	root := wallRepo(t)
	writeText(t, filepath.Join(root, "covenant.json"), covenantBody("score", ">=3", "check-1"))
	pre := snapshotTree(t, root)
	emptyNet, gviol := mission.ParseGuardrails(mission.ContractGuardrailSubject, "", protectedArtifactPath)
	if gviol != "" {
		t.Fatal(gviol)
	}

	// A requirement-row change rides the lane.
	writeText(t, filepath.Join(root, "covenant.json"), covenantBody("score", ">=3", "check-1-improved"))
	reviewed := snapshotTree(t, root)
	laned := wallAuthorization(t, root, "demo", pre, reviewed, func(record map[string]any) {
		record["guardrailLane"] = true
	})
	certified := []map[string]any{{"jobId": "job-c", "verdict": "accepted", "authorizationDigest": laned}}
	inspection, err := inspectWall(root, "demo", pre, wallState(), certified, map[string]bool{}, emptyNet, "", legacySnapshot(root, "demo"))
	if err != nil {
		t.Fatal(err)
	}
	if inspection.Violation != "" {
		t.Fatalf("a requirement change must ride the warden's lane: %q", inspection.Violation)
	}

	// A battery change escalates even down the lane — and the check
	// judges the authorization's REVIEWED tree, not the workspace: the
	// workspace deliberately reverts to the old covenant before the
	// inspection, and the escalation still fires from the reviewed
	// bytes alone.
	writeText(t, filepath.Join(root, "covenant.json"), covenantBody("score", ">=1", "check-1"))
	weakened := snapshotTree(t, root)
	writeText(t, filepath.Join(root, "covenant.json"), covenantBody("score", ">=3", "check-1"))
	lanedWeaker := wallAuthorization(t, root, "demo", pre, weakened, func(record map[string]any) {
		record["guardrailLane"] = true
	})
	certified = []map[string]any{{"jobId": "job-c", "verdict": "accepted", "authorizationDigest": lanedWeaker}}
	inspection, err = inspectWall(root, "demo", pre, wallState(), certified, map[string]bool{}, emptyNet, "", legacySnapshot(root, "demo"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(inspection.Violation, "human tier") {
		t.Fatalf("a battery change must escalate to the human tier: %q", inspection.Violation)
	}
	if inspection.UndeclaredOnly {
		t.Fatal("a governance violation must stay off the mechanical rung")
	}
}

// A covenant born inside a mission escalates whole: covenants arrive
// by inception or retrofit, never by a turn.
func TestCovenantBornInsideAMissionEscalates(t *testing.T) {
	root := wallRepo(t)
	pre := snapshotTree(t, root)
	writeText(t, filepath.Join(root, "covenant.json"), covenantBody("score", ">=3", "check-1"))
	reviewed := snapshotTree(t, root)
	emptyNet, gviol := mission.ParseGuardrails(mission.ContractGuardrailSubject, "", protectedArtifactPath)
	if gviol != "" {
		t.Fatal(gviol)
	}
	laned := wallAuthorization(t, root, "demo", pre, reviewed, func(record map[string]any) {
		record["guardrailLane"] = true
	})
	certified := []map[string]any{{"jobId": "job-c", "verdict": "accepted", "authorizationDigest": laned}}
	inspection, err := inspectWall(root, "demo", pre, wallState(), certified, map[string]bool{}, emptyNet, "", legacySnapshot(root, "demo"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(inspection.Violation, "inception or retrofit") {
		t.Fatalf("a covenant born inside a mission must escalate: %q", inspection.Violation)
	}
}

// A reviewed tree that deletes the covenant escalates; a reviewed
// tree that cannot answer at all (a bogus tree id — git ran and
// refused) is a governance violation too, never a silent runner error
// that skips the human-tier park.
func TestReviewedTreeDeletionAndUnreadableEscalate(t *testing.T) {
	root := wallRepo(t)
	writeText(t, filepath.Join(root, "covenant.json"), covenantBody("score", ">=3", "check-1"))
	pre := snapshotTree(t, root)
	workspace := gittree.Workspace{Dir: root}

	violation, err := covenantGovernanceViolation(workspace, pre, pre, "digest-same")
	if err != nil || violation != "" {
		t.Fatalf("an unchanged covenant must pass the deep check: %q %v", violation, err)
	}

	// Deletion: an empty tree as the reviewed tree removes the covenant.
	empty := snapshotTree(t, wallRepo(t))
	violation, err = covenantGovernanceViolation(workspace, pre, empty, "digest-del")
	if err != nil || !strings.Contains(violation, "deletes") {
		t.Fatalf("a reviewed tree deleting the covenant must escalate: %q %v", violation, err)
	}

	// A bogus reviewed tree: git answers, and the answer is a
	// governance violation on the human path.
	violation, err = covenantGovernanceViolation(workspace, pre, "0000000000000000000000000000000000000000", "digest-bogus")
	if err != nil || !strings.Contains(violation, "human tier resolves") {
		t.Fatalf("an unanswerable reviewed tree must escalate, not error: %q %v", violation, err)
	}
}
