package landing

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type fixtureAttestationError struct{ code string }

func (e fixtureAttestationError) Error() string                  { return "fixture " + e.code }
func (e fixtureAttestationError) LandingAttestationCode() string { return e.code }

func TestObserveAttestedBranchDeclarationEnforcesBoundary(t *testing.T) {
	f := newObserveFixture(t)
	manifest, err := os.ReadFile(filepath.Join(f.root, "scripts/agents/path-classes.txt"))
	if err != nil {
		t.Fatal(err)
	}
	f.write("scripts/agents/path-classes.txt", string(manifest)+"install:product.txt behavior\n")
	f.writeHeldGoal("goal-a", "m1", "lineage")
	f.git("add", ".")
	f.git("commit", "-qm", "held goal")
	f.write("product.txt", "candidate\n")
	f.git("add", "product.txt")
	candidate := f.tree()
	if _, err := CreateTestReceipt(f.root, candidate, "true", io.Discard, io.Discard); err != nil {
		t.Fatal(err)
	}
	receipt := TestReceiptPath(f.root, candidate)
	commit := strings.Repeat("a", 40)
	base := ObserveParams{RepoRoot: f.root, CandidateTree: candidate, Goal: "goal-a", Actor: "m1+lineage",
		Attested: commit, AttestedSnapshot: strings.Repeat("b", 40), AttestedBase: strings.Repeat("c", 40), TestReceipt: receipt}
	passing := AttestedUnit{Goal: "goal-a", Unit: "u1", Digest: strings.Repeat("d", 64), CriticRoot: "critic-a",
		Round: 1, GoalRevision: 1, ChangedPaths: []string{"product.txt"}}
	base.BindAttested = func(string, string, string, string, string, string) (AttestedUnit, error) { return passing, nil }
	got := Observe(base)
	if got.Mode != "observe" || got.Bar != BarAttested || got.VerdictTrailer != "pass bar=e" || got.GoalRevision != 1 ||
		!strings.Contains(got.Provenance, "attested="+commit+" goal=goal-a unit=u1 critic=critic-a/1") {
		t.Fatalf("passing declaration = %+v", got)
	}

	reader := base
	reader.BindAttested = func(string, string, string, string, string, string) (AttestedUnit, error) {
		value := passing
		value.CriticRoot = ""
		return value, nil
	}
	if got := Observe(reader); got.Code != "attested-not-critic" || got.Mode != "refuse" {
		t.Fatalf("reader source = %+v", got)
	}

	mismatch := base
	mismatch.BindAttested = func(string, string, string, string, string, string) (AttestedUnit, error) {
		return AttestedUnit{}, fixtureAttestationError{code: "change-mismatch"}
	}
	if got := Observe(mismatch); got.Code != "attested-change-mismatch" || got.Mode != "refuse" {
		t.Fatalf("changed bytes = %+v", got)
	}

	destructive := base
	destructive.BindAttested = func(string, string, string, string, string, string) (AttestedUnit, error) {
		value := passing
		value.Destructive = true
		return value, nil
	}
	if got := Observe(destructive); got.Code != "attested-not-design-bearing" {
		t.Fatalf("destructive declaration = %+v", got)
	}
	destructive.BindAttested = func(string, string, string, string, string, string) (AttestedUnit, error) {
		value := passing
		value.Destructive, value.HasPlan = true, true
		return value, nil
	}
	if got := Observe(destructive); got.VerdictTrailer != "pass bar=e" {
		t.Fatalf("planned destructive declaration = %+v", got)
	}

	moved := base
	moved.BindAttested = func(string, string, string, string, string, string) (AttestedUnit, error) {
		value := passing
		value.GoalRevision = 2
		return value, nil
	}
	if got := Observe(moved); got.Code != "goal-revision-moved" {
		t.Fatalf("moved goal = %+v", got)
	}

	conflict := base
	conflict.Chain = "critic-a"
	if got := Observe(conflict); got.Code != "conflicting-declarations" || got.Mode != "refuse" {
		t.Fatalf("conflicting declaration = %+v", got)
	}

	bad := base
	bad.Attested = "short"
	if got := Observe(bad); got.Code != "attested-malformed-id" || got.Mode != "refuse" {
		t.Fatalf("malformed declaration = %+v", got)
	}
}
