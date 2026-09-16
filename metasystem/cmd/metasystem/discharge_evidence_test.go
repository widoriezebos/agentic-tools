package main

import (
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"strings"
	"testing"
)

func TestDischargeReviewObligationCLIWiring(t *testing.T) {
	root, commit := syncedClaimedGoalFixture(t), strings.Repeat("a", 40)
	amendSyncedGoalFixture(t, root, "human obligation", func(file *goal.GoalFile) {
		file.ReviewObligations = append(file.ReviewObligations, goal.ReviewObligation{Finding: "carried:" + commit, Chain: goal.HumanCarriedChain, Artifact: "commit:" + commit, Test: "pending", State: "open"})
	})
	watchWriteJSON(t, root, "artifacts/agents/jobs/critic-root.json", map[string]any{"jobId": "critic-root", "role": "code-critic", "reviews": "commit:" + commit, "status": "completed", "round": 1, "chainClosed": true, "findingRegister": []any{}, "findingRegisterRound": 1})
	watchWriteJSON(t, root, "artifacts/agents/critic-root/rounds/1/return.json", map[string]any{"verdictMaterialCount": 0})
	if code := runGoalDischargeReviewObligation([]string{"--root", root, "--id", "standing-validation", "--finding", "carried:" + commit, "--chain", goal.HumanCarriedChain, "--by", "Wido", "--lineage", "m1", "--test", "critic-root"}); code != 0 {
		t.Fatalf("human discharge = %d", code)
	}
	root = syncedClaimedGoalFixture(t)
	amendSyncedGoalFixture(t, root, "fixture obligation", func(file *goal.GoalFile) {
		file.ReviewObligations = append(file.ReviewObligations, goal.ReviewObligation{Finding: "F-1", Chain: "design-critic", Artifact: "a.go", Test: "prove: group:a", Fixture: "group:section/a", State: "open"})
	})
	stdout, code := captureStdout(t, func() int {
		return runGoalDischargeReviewObligation([]string{"--root", root, "--id", "standing-validation", "--finding", "F-1", "--chain", "design-critic", "--by", "Wido", "--lineage", "m1", "--test", "bare", "--implementation-chain", "implementation-chain", "--artifact", "a.go", "--result", "run-passed"})
	})
	if code != 1 || !strings.Contains(stdout, "requires --critic") {
		t.Fatalf("missing critic = %d %q", code, stdout)
	}
	prior := dischargeReviewObligation
	defer func() { dischargeReviewObligation = prior }()
	var captured goal.DischargeEvidence
	dischargeReviewObligation = func(_ goal.VerbRequest, _, _, _, _, _ string, supplied ...goal.DischargeEvidence) (goal.PublishResult, error) {
		captured = supplied[0]
		return goal.PublishResult{Outcome: goal.OutcomeConfirmed}, nil
	}
	code = runGoalDischargeReviewObligation([]string{"--root", root, "--id", "standing-validation", "--finding", "F-1", "--chain", "design-critic", "--by", "Wido", "--lineage", "m1", "--implementation-chain", "implementation-chain", "--artifact", "a.go", "--result", "run-passed", "--critic", "critic-root"})
	if code != 0 || captured != (goal.DischargeEvidence{Root: root, ImplementationChain: "implementation-chain", Artifact: "a.go", ResultRunID: "run-passed", CriticRoot: "critic-root"}) {
		t.Fatalf("fixture evidence wiring = %+v %d", captured, code)
	}
}
