package landing

import (
	"strings"
	"testing"
)

type fixtureAttestationError struct{ code string }

func (e fixtureAttestationError) Error() string                  { return "fixture " + e.code }
func (e fixtureAttestationError) LandingAttestationCode() string { return e.code }

func TestObserveAttestedBranchDeclarationEnforcesBoundary(t *testing.T) {
	f := newRepositoryObservationFixture(t)
	f.base("scripts/agents/path-classes.txt", string(f.baseFiles["scripts/agents/path-classes.txt"])+"install:product.txt behavior\n")
	f.base("plans/goals/goal-a.md", string(observationHeldGoal("goal-a", "m1", "lineage")))
	c := f.comparison(observeTreeB, string(chainDiff(chainReplacement("product.txt", "before\n", "candidate\n"))), "product.txt")
	c.declare("product.txt", observationText("before\n"), observationText("candidate\n"))
	receipt := c.bindCommandReceipt("true")
	commit := strings.Repeat("a", 40)
	base := ObserveParams{RepoRoot: f.root, CandidateTree: c.candidate, Goal: "goal-a", Actor: "m1+lineage",
		Attested: commit, AttestedSnapshot: strings.Repeat("b", 40), AttestedBase: strings.Repeat("c", 40), TestReceipt: receipt}
	passing := AttestedUnit{Goal: "goal-a", Unit: "u1", Digest: strings.Repeat("d", 64), CriticRoot: "critic-a",
		Round: 1, GoalRevision: 1, ChangedPaths: []string{"product.txt"}}
	bind := func(value AttestedUnit, bindingError error) func(string, string, string, string, string, string) (AttestedUnit, error) {
		return func(boundCommit, snapshot, attestedBase, goal, beforeTree, afterTree string) (AttestedUnit, error) {
			t.Helper()
			if boundCommit != base.Attested || snapshot != base.AttestedSnapshot || attestedBase != base.AttestedBase ||
				goal != base.Goal || beforeTree != observeBaseTree || afterTree != c.candidate {
				t.Fatalf("BindAttested arguments = (%q, %q, %q, %q, %q, %q)",
					boundCommit, snapshot, attestedBase, goal, beforeTree, afterTree)
			}
			return value, bindingError
		}
	}
	base.BindAttested = bind(passing, nil)
	got := c.observe(base)
	if got.Mode != "observe" || got.Bar != BarAttested || got.VerdictTrailer != "pass bar=e" || got.GoalRevision != 1 ||
		!strings.Contains(got.Provenance, "attested="+commit+" goal=goal-a unit=u1 critic=critic-a/1") {
		t.Fatalf("passing declaration = %+v", got)
	}

	reader := base
	withoutCritic := passing
	withoutCritic.CriticRoot = ""
	reader.BindAttested = bind(withoutCritic, nil)
	if got := c.observe(reader); got.Code != "attested-not-critic" || got.Mode != "refuse" {
		t.Fatalf("reader source = %+v", got)
	}

	mismatch := base
	mismatch.BindAttested = bind(AttestedUnit{}, fixtureAttestationError{code: "change-mismatch"})
	if got := c.observe(mismatch); got.Code != "attested-change-mismatch" || got.Mode != "refuse" {
		t.Fatalf("changed bytes = %+v", got)
	}

	destructive := base
	withoutPlan := passing
	withoutPlan.Destructive = true
	destructive.BindAttested = bind(withoutPlan, nil)
	if got := c.observe(destructive); got.Code != "attested-not-design-bearing" {
		t.Fatalf("destructive declaration = %+v", got)
	}
	withPlan := withoutPlan
	withPlan.HasPlan = true
	destructive.BindAttested = bind(withPlan, nil)
	if got := c.observe(destructive); got.VerdictTrailer != "pass bar=e" {
		t.Fatalf("planned destructive declaration = %+v", got)
	}

	moved := base
	movedRevision := passing
	movedRevision.GoalRevision = 2
	moved.BindAttested = bind(movedRevision, nil)
	if got := c.observe(moved); got.Code != "goal-revision-moved" {
		t.Fatalf("moved goal = %+v", got)
	}

	conflict := base
	conflict.Chain = "critic-a"
	if got := c.observe(conflict); got.Code != "conflicting-declarations" || got.Mode != "refuse" {
		t.Fatalf("conflicting declaration = %+v", got)
	}

	bad := base
	bad.Attested = "short"
	if got := c.observe(bad); got.Code != "attested-malformed-id" || got.Mode != "refuse" {
		t.Fatalf("malformed declaration = %+v", got)
	}
}
