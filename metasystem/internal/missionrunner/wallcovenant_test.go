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
	bed := newWallPolicyBed(t)
	pre, reviewed := "pre", "reviewed"
	writeText(t, filepath.Join(bed.root, "covenant.json"), covenantBody("score", ">=3", "check-1"))
	emptyNet, gviol := mission.ParseGuardrails(mission.ContractGuardrailSubject, "", protectedArtifactPath)
	if gviol != "" {
		t.Fatal(gviol)
	}
	plain := bed.authorization(pre, reviewed, []string{"covenant.json"}, wallPolicyPatch("covenant ordinary"), true, nil)
	certified := []map[string]any{{"jobId": "job-c", "verdict": "accepted", "authorizationDigest": plain}}
	inspection, err := bed.inspectWithGuardrails(pre, wallState(), certified, map[string]bool{}, emptyNet, "")
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
	bed := newWallPolicyBed(t)
	inspection, err := bed.inspectWithGuardrails("pre", wallState(), nil,
		map[string]bool{"covenant.json": true}, emptyNet, contradiction)
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
	bed := newWallPolicyBed(t)
	old := covenantBody("score", ">=3", "check-1")
	writeText(t, filepath.Join(bed.root, "covenant.json"), old)
	pre := "pre"
	emptyNet, gviol := mission.ParseGuardrails(mission.ContractGuardrailSubject, "", protectedArtifactPath)
	if gviol != "" {
		t.Fatal(gviol)
	}

	// A requirement-row change rides the lane.
	improved := covenantBody("score", ">=3", "check-1-improved")
	writeText(t, filepath.Join(bed.root, "covenant.json"), improved)
	reviewed := "requirement-reviewed"
	patch := wallPolicyPatch("requirement reviewed")
	laned := bed.authorization(pre, reviewed, []string{"covenant.json"}, patch, true, func(record map[string]any) {
		record["guardrailLane"] = true
	})
	certified := []map[string]any{{"jobId": "job-c", "verdict": "accepted", "authorizationDigest": laned}}
	bed.facts.expectFileAt(pre, []byte(old), true, nil)
	bed.facts.expectFileAt(reviewed, []byte(improved), true, nil)
	bed.facts.expectApply(pre, patch, reviewed)
	bed.facts.expectEntries(reviewed, []string{"covenant.json"}, map[string]gittree.Entry{"covenant.json": wallPolicyFile})
	bed.facts.expectEntries(reviewed, []string{"covenant.json"}, map[string]gittree.Entry{"covenant.json": wallPolicyFile})
	bed.facts.expectSnapshot(reviewed, reviewed)
	inspection, err := bed.inspectWithGuardrails(pre, wallState(), certified, map[string]bool{}, emptyNet, "")
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
	weak := covenantBody("score", ">=1", "check-1")
	writeText(t, filepath.Join(bed.root, "covenant.json"), weak)
	weakened := "battery-reviewed"
	writeText(t, filepath.Join(bed.root, "covenant.json"), old)
	lanedWeaker := bed.authorization(pre, weakened, []string{"covenant.json"}, wallPolicyPatch("battery reviewed"), true, func(record map[string]any) {
		record["guardrailLane"] = true
	})
	certified = []map[string]any{{"jobId": "job-c", "verdict": "accepted", "authorizationDigest": lanedWeaker}}
	bed.facts.expectFileAt(pre, []byte(old), true, nil)
	bed.facts.expectFileAt(weakened, []byte(weak), true, nil)
	inspection, err = bed.inspectWithGuardrails(pre, wallState(), certified, map[string]bool{}, emptyNet, "")
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
	bed := newWallPolicyBed(t)
	pre, reviewed := "pre", "reviewed"
	born := covenantBody("score", ">=3", "check-1")
	writeText(t, filepath.Join(bed.root, "covenant.json"), born)
	emptyNet, gviol := mission.ParseGuardrails(mission.ContractGuardrailSubject, "", protectedArtifactPath)
	if gviol != "" {
		t.Fatal(gviol)
	}
	laned := bed.authorization(pre, reviewed, []string{"covenant.json"}, wallPolicyPatch("covenant born"), true, func(record map[string]any) {
		record["guardrailLane"] = true
	})
	certified := []map[string]any{{"jobId": "job-c", "verdict": "accepted", "authorizationDigest": laned}}
	bed.facts.expectFileAt(pre, nil, false, nil)
	bed.facts.expectFileAt(reviewed, []byte(born), true, nil)
	inspection, err := bed.inspectWithGuardrails(pre, wallState(), certified, map[string]bool{}, emptyNet, "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(inspection.Violation, "inception or retrofit") {
		t.Fatalf("a covenant born inside a mission must escalate: %q", inspection.Violation)
	}
}

// A reviewed tree that deletes the covenant escalates; a reviewed
// tree that cannot answer at all is a governance violation too, never a silent runner error
// that skips the human-tier park.
func TestReviewedTreeDeletionAndUnreadableEscalate(t *testing.T) {
	bed := newWallPolicyBed(t)
	old := []byte(covenantBody("score", ">=3", "check-1"))
	writeText(t, filepath.Join(bed.root, "covenant.json"), string(old))
	pre := "pre"

	bed.facts.expectFileAt(pre, old, true, nil)
	bed.facts.expectFileAt(pre, old, true, nil)
	violation, err := bed.governance(pre, pre, "digest-same")
	if err != nil || violation != "" {
		t.Fatalf("an unchanged covenant must pass the deep check: %q %v", violation, err)
	}

	// Deletion: an empty tree as the reviewed tree removes the covenant.
	empty := "empty"
	bed.facts.expectFileAt(pre, old, true, nil)
	bed.facts.expectFileAt(empty, nil, false, nil)
	violation, err = bed.governance(pre, empty, "digest-del")
	if err != nil || !strings.Contains(violation, "deletes") {
		t.Fatalf("a reviewed tree deleting the covenant must escalate: %q %v", violation, err)
	}

	// The repository answers that the reviewed tree is unreadable, so
	// governance carries the failure to the human path.
	bogus := "0000000000000000000000000000000000000000"
	bed.facts.expectFileAt(pre, old, true, nil)
	bed.facts.expectFileAt(bogus, nil, false, fmt.Errorf("reviewed tree %s is unreadable", bogus))
	violation, err = bed.governance(pre, bogus, "digest-bogus")
	if err != nil || !strings.Contains(violation, "human tier resolves") {
		t.Fatalf("an unanswerable reviewed tree must escalate, not error: %q %v", violation, err)
	}
}
