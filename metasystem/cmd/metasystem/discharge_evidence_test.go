package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

func seedDischargeObligation(t *testing.T, fixture *obligationCommandFixture, obligation goal.ReviewObligation) {
	t.Helper()
	const path = "plans/goals/standing-validation.md"
	opid := goal.Opid("01ARZ3NDEKTSV4RRFFQ69G5FAC", "mac-cli", "m1")
	parent, err := fixture.repo.Capture(opid)
	if err != nil {
		t.Fatal(err)
	}
	file, problems := goal.ParseFile(fixture.repo.commit(parent).files[path])
	if file == nil || len(problems) != 0 {
		t.Fatalf("seed goal: file=%+v problems=%v", file, problems)
	}
	file.ReviewObligations = append(file.ReviewObligations, obligation)
	rendered := goal.RenderFile(file)
	commit, err := fixture.repo.Build(opid, parent, []goal.Change{{Path: path, Content: rendered}}, "seed review obligation")
	if err != nil {
		t.Fatal(err)
	}
	if outcome, err := fixture.repo.Publish(parent, commit); err != nil || outcome != goal.CASLanded {
		t.Fatalf("seed publication: outcome=%s err=%v", outcome, err)
	}
	if err := fixture.repo.AcceptedCAS(parent, commit); err != nil {
		t.Fatal(err)
	}
	if err := fixture.repo.Release(opid); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fixture.root(), filepath.FromSlash(path)), rendered, 0o644); err != nil {
		t.Fatal(err)
	}
}

func dischargeFixtureRequest(fixture *obligationCommandFixture) func(string, string, string, string) (goal.VerbRequest, error) {
	return func(verb, root, by, lineage string) (goal.VerbRequest, error) {
		return syncReqWithProofAtWithDependencies(verb, root, by, lineage, nil, fixture.commandNow, fixture.dependencies())
	}
}

func TestDischargeReviewObligationCLIWiring(t *testing.T) {
	fixture, commit := newObligationCommandFixture(t), strings.Repeat("a", 40)
	seedDischargeObligation(t, fixture, goal.ReviewObligation{Finding: "carried:" + commit, Chain: goal.HumanCarriedChain, Artifact: "commit:" + commit, Test: "pending", State: "open"})
	root := fixture.root()
	watchWriteJSON(t, root, "artifacts/agents/jobs/critic-root.json", map[string]any{"jobId": "critic-root", "role": "code-critic", "reviews": "commit:" + commit, "status": "completed", "round": 1, "chainClosed": true, "findingRegister": []any{}, "findingRegisterRound": 1})
	watchWriteJSON(t, root, "artifacts/agents/critic-root/rounds/1/return.json", map[string]any{"verdictMaterialCount": 0})
	stdout, code := captureStdout(t, func() int {
		return runGoalDischargeReviewObligationWithOwners([]string{"--root", root, "--id", "standing-validation", "--finding", "carried:" + commit, "--chain", goal.HumanCarriedChain, "--by", "Wido", "--lineage", "m1", "--test", "critic-root"}, dischargeFixtureRequest(fixture), goal.DischargeReviewObligation)
	})
	if code != 0 || !strings.Contains(stdout, `"outcome":"confirmed"`) {
		t.Fatalf("human discharge = %d %q", code, stdout)
	}
	file, _ := fixture.acceptedGoal()
	if len(file.ReviewObligations) != 1 || file.ReviewObligations[0].State != "discharged" || file.ReviewObligations[0].Test != "critic-root" {
		t.Fatalf("human obligation was not published: %+v", file.ReviewObligations)
	}
	if fixture.repo.builds != 2 || fixture.repo.publications != 2 || fixture.repo.advances != 2 || fixture.repo.captures != 3 || fixture.repo.releases != 3 || fixture.repo.canonical != fixture.repo.accepted {
		t.Fatalf("human publication transactions: %+v", fixture.repo)
	}
	fixture.facts.expect(1, 1, 0, 1, 0, 0, "repository top", "ledger identity", "guard", "endpoint", "machine", "clock")

	fixture = newObligationCommandFixture(t)
	seedDischargeObligation(t, fixture, goal.ReviewObligation{Finding: "F-1", Chain: "design-critic", Artifact: "a.go", Test: "prove: group:a", Fixture: "group:section/a", State: "open"})
	root = fixture.root()
	_, acceptedBefore := fixture.acceptedGoal()
	stdout, code = captureStdout(t, func() int {
		return runGoalDischargeReviewObligationWithOwners([]string{"--root", root, "--id", "standing-validation", "--finding", "F-1", "--chain", "design-critic", "--by", "Wido", "--lineage", "m1", "--test", "bare", "--implementation-chain", "implementation-chain", "--artifact", "a.go", "--result", "run-passed"}, dischargeFixtureRequest(fixture), goal.DischargeReviewObligation)
	})
	if code != 1 || !strings.Contains(stdout, "requires --critic") {
		t.Fatalf("missing critic = %d %q", code, stdout)
	}
	_, acceptedAfter := fixture.acceptedGoal()
	if string(acceptedAfter) != string(acceptedBefore) || fixture.repo.builds != 1 || fixture.repo.publications != 1 || fixture.repo.advances != 1 || fixture.repo.captures != 2 || fixture.repo.releases != 2 || fixture.repo.canonical != fixture.repo.accepted {
		t.Fatalf("refused mutation changed accepted goal or transaction counts: %+v", fixture.repo)
	}
	fixture.facts.expect(1, 1, 0, 1, 0, 0, "repository top", "ledger identity", "guard", "endpoint", "machine", "clock")
	var captured goal.DischargeEvidence
	code = runGoalDischargeReviewObligationWithOwners([]string{"--root", root, "--id", "standing-validation", "--finding", "F-1", "--chain", "design-critic", "--by", "Wido", "--lineage", "m1", "--implementation-chain", "implementation-chain", "--artifact", "a.go", "--result", "run-passed", "--critic", "critic-root"}, dischargeFixtureRequest(fixture), func(_ goal.VerbRequest, _, _, _, _, _ string, supplied ...goal.DischargeEvidence) (goal.PublishResult, error) {
		captured = supplied[0]
		return goal.PublishResult{Outcome: goal.OutcomeConfirmed}, nil
	})
	if code != 0 || captured != (goal.DischargeEvidence{Root: root, ImplementationChain: "implementation-chain", Artifact: "a.go", ResultRunID: "run-passed", CriticRoot: "critic-root"}) {
		t.Fatalf("fixture evidence wiring = %+v %d", captured, code)
	}
	fixture.facts.expect(2, 2, 0, 2, 0, 0, "repository top", "ledger identity", "guard", "endpoint", "machine", "clock", "repository top", "ledger identity", "guard", "endpoint", "machine", "clock")
}
