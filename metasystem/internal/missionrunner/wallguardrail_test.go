package missionrunner

// Guardrail custody (the app-guardrail program's first slice): the
// contract names the files that ARE the app's net, and the wall refuses
// any ordinary implementation authorization touching them — the net is
// never changed by the work it judges. Only an authorization issued
// down the warden-reviewed lane, a fact inside the digest-authenticated
// record, may carry a guardrail change.

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/mission"
)

// An ordinary authorization touching a guardrail-classed file refuses;
// the same change through the warden's lane consumes normally.
func TestWallRefusesGuardrailChangeOutsideTheWardenLane(t *testing.T) {
	bed := newWallPolicyBed(t)
	writeText(t, filepath.Join(bed.root, "goldens", "case.txt"), "golden\n")
	pre, reviewed := "pre", "reviewed"
	writeText(t, filepath.Join(bed.root, "goldens", "case.txt"), "weakened\n")
	guardrails, gviol := mission.ParseGuardrails(mission.ContractGuardrailSubject, "goldens/", protectedArtifactPath)
	if gviol != "" {
		t.Fatal(gviol)
	}

	patch := wallPolicyPatch("golden reviewed")
	plain := bed.authorization(pre, reviewed, []string{"goldens/case.txt"}, patch, true, nil)
	certified := []map[string]any{{"jobId": "job-w", "verdict": "accepted", "authorizationDigest": plain}}
	inspection, err := bed.inspectWithGuardrails(pre, wallState(), certified, map[string]bool{}, guardrails, "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(inspection.Violation, "outside the warden's lane") {
		t.Fatalf("an ordinary authorization must never carry a guardrail change: %q", inspection.Violation)
	}

	laned := bed.authorization(pre, reviewed, []string{"goldens/case.txt"}, patch, true, func(record map[string]any) {
		record["guardrailLane"] = true
	})
	certified = []map[string]any{{"jobId": "job-w", "verdict": "accepted", "authorizationDigest": laned}}
	bed.facts.expectApply(pre, patch, reviewed)
	bed.facts.expectEntries(reviewed, []string{"goldens/case.txt"}, map[string]gittree.Entry{"goldens/case.txt": wallPolicyFile})
	bed.facts.expectEntries(reviewed, []string{"goldens/case.txt"}, map[string]gittree.Entry{"goldens/case.txt": wallPolicyFile})
	bed.facts.expectSnapshot(reviewed, reviewed)
	inspection, err = bed.inspectWithGuardrails(pre, wallState(), certified, map[string]bool{}, guardrails, "")
	if err != nil {
		t.Fatal(err)
	}
	if inspection.Violation != "" {
		t.Fatalf("the warden's lane must consume: %q", inspection.Violation)
	}
	if len(inspection.OrderedDigests) != 1 {
		t.Fatalf("consumption: %v", inspection.OrderedDigests)
	}
}

// The lane fact is inside the authenticated record: stamping it onto
// the bytes after issuance breaks the digest and refuses as tamper.
func TestWallGuardrailLaneCannotBeForgedAfterIssuance(t *testing.T) {
	bed := newWallPolicyBed(t)
	writeText(t, filepath.Join(bed.root, "goldens", "case.txt"), "golden\n")
	pre, reviewed := "pre", "reviewed"
	writeText(t, filepath.Join(bed.root, "goldens", "case.txt"), "weakened\n")
	guardrails, _ := mission.ParseGuardrails(mission.ContractGuardrailSubject, "goldens/", protectedArtifactPath)

	digest := bed.authorization(pre, reviewed, []string{"goldens/case.txt"}, wallPolicyPatch("golden forged"), true, nil)
	patchRecord(t, bed.root, "demo", digest, map[string]any{"guardrailLane": true})
	certified := []map[string]any{{"jobId": "job-w", "verdict": "accepted", "authorizationDigest": digest}}
	inspection, err := bed.inspectWithGuardrails(pre, wallState(), certified, map[string]bool{}, guardrails, "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(inspection.Violation, "do not match their digest") {
		t.Fatalf("a post-issuance lane stamp must refuse as tamper: %q", inspection.Violation)
	}
}

// Exact files and directory prefixes both cover; unrelated paths do not.
func TestGuardrailClassCoverage(t *testing.T) {
	class, violation := mission.ParseGuardrails(mission.ContractGuardrailSubject, "specs/gate.sh, goldens/, budgets.json", protectedArtifactPath)
	if violation != "" {
		t.Fatal(violation)
	}
	for path, want := range map[string]bool{
		"specs/gate.sh":      true,
		"goldens/case.txt":   true,
		"goldens/deep/a.bin": true,
		"budgets.json":       true,
		"specs/other.sh":     false,
		"goldensx/case.txt":  false,
		"main.go":            false,
	} {
		if class.Covers(path) != want {
			t.Fatalf("covers(%q) = %v, want %v", path, !want, want)
		}
	}
	if class.Empty() {
		t.Fatal("a declared class is not empty")
	}
	empty, _ := mission.ParseGuardrails(mission.ContractGuardrailSubject, "", protectedArtifactPath)
	if !empty.Empty() || empty.Covers("anything") {
		t.Fatal("an empty declaration covers nothing")
	}
}

// The parser refuses what the host-artifact grammar refuses — plus
// nothing may be both a guardrail and a protected path.
func TestParseGuardrailsRefusals(t *testing.T) {
	for name, value := range map[string]string{
		"empty path":  "goldens/,,specs.sh",
		"absolute":    "/etc/goldens",
		"traversal":   "goldens/../secrets",
		"glob":        "goldens/*.txt",
		"dot segment": "goldens/./",
		"doubled":     "goldens//deep",
		"lone dot":    ".",
		"protected":   "scripts/agents/gate.sh",
	} {
		if _, violation := mission.ParseGuardrails(mission.ContractGuardrailSubject, value, protectedArtifactPath); violation == "" {
			t.Fatalf("%s must refuse", name)
		}
	}
}

// A contract declaring a guardrail as a host artifact contradicts
// itself: the first is a free host write, the second must never be one.
func TestWallRefusesGuardrailHostArtifactContradiction(t *testing.T) {
	declared, dviol := parseHostArtifacts("goldens/case.txt, notes.md")
	if dviol != "" {
		t.Fatal(dviol)
	}
	guardrails, gviol := mission.ParseGuardrails(mission.ContractGuardrailSubject, "goldens/", protectedArtifactPath)
	if gviol != "" {
		t.Fatal(gviol)
	}
	if v := mission.GuardrailContradiction(declared, guardrails); !strings.Contains(v, "goldens/case.txt") {
		t.Fatalf("the contradiction must refuse by name: %q", v)
	}
	disjoint, _ := mission.ParseGuardrails(mission.ContractGuardrailSubject, "specs/", protectedArtifactPath)
	if v := mission.GuardrailContradiction(declared, disjoint); v != "" {
		t.Fatalf("disjoint declarations must compose: %q", v)
	}
}
