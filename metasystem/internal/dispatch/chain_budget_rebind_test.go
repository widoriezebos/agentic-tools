package dispatch

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	critiqueModel "github.com/widoriezebos/agentic-tools/metasystem/internal/critique"
)

// TestChainBudgetRebindReopensAnExhaustedCodeCritique (U1d read F-1): a
// code-critic chain spent its one round; a person raises the goal's
// review-round member to three. The review of the implementation chain
// (work revise j2:<review>, which follows up the implementer root) and the
// close of the critic chain rebind the critic root first, so the follow-up's
// exhaustion check admits the next round instead of reading the stale limit.
func TestChainBudgetRebindReopensAnExhaustedCodeCritique(t *testing.T) {
	bed := newGoalAdmissionBed(t, 2)
	repo := bed.root
	jobs := filepath.Join(repo, "artifacts", "agents", "jobs")
	open := func(critic string) any {
		return encodeFindingRegister([]registerFinding{{FindingID: "B-1", Critic: critic, RigorClass: critiqueModel.Bounded, FactsDigest: digestJSON(registerFacts()),
			Facts: registerFacts(), Artifact: "NEW metasystem/a file.go", Title: "bounded title", Status: "open", Evidence: "proof", EvidenceDigest: digestJSON("proof"), Multiplicity: 1}})
	}
	writeJSONFile(t, jobs, "impl.json", map[string]any{"jobId": "impl", "role": "implementer", "round": 1, "parentJob": nil, "status": "completed", "goalId": "bounded"})
	writeJSONFile(t, jobs, "critic.json", map[string]any{
		"jobId": "critic", "role": "code-critic", "round": 1, "parentJob": nil, "status": "completed", "goalId": "bounded", "reviews": "impl",
		reviewRoundLimitField: 1, criticRoundsConsumedField: 1, findingRegisterField: open("critic"), findingRegisterRoundField: 1,
	})
	// An unrelated critic of another implementation is never touched.
	writeJSONFile(t, jobs, "other-critic.json", map[string]any{
		"jobId": "other-critic", "role": "code-critic", "round": 1, "parentJob": nil, "status": "completed", "goalId": "bounded", "reviews": "other",
		reviewRoundLimitField: 1, criticRoundsConsumedField: 1, findingRegisterField: open("other-critic"), findingRegisterRoundField: 1,
	})
	message := filepath.Join(t.TempDir(), "message.md")
	if err := os.WriteFile(message, []byte("fold\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := CritiqueExhaustionAdvance(repo, "impl", "implementer", message, ""); err == nil {
		t.Fatal("the exhausted chain admitted a follow-up before the rebind; the fixture proves nothing")
	}

	outcomes, err := critiqueChainBudgetRebindWithReads(repo, "impl", bed.reads)
	if err != nil || len(outcomes) != 1 || outcomes["critic"] != "rebound" {
		t.Fatalf("rebind through the implementer root = %v, %v; want the one critic reviewing it rebound", outcomes, err)
	}
	for id, want := range map[string]int64{"critic": 3, "other-critic": 1} {
		got := readJSONFile(t, filepath.Join(jobs, id+".json"))
		if limit, _ := numInt(got[reviewRoundLimitField]); limit != want {
			t.Fatalf("%s limit = %v, want %d", id, got[reviewRoundLimitField], want)
		}
	}
	if outcome, err := CritiqueExhaustionAdvance(repo, "impl", "implementer", message, ""); err != nil || outcome != "none" {
		t.Fatalf("the follow-up after the raise = %q, %v; want it admitted", outcome, err)
	}

	// The close names the critic root itself; the same rebind admits it.
	writeJSONFile(t, jobs, "other-critic.json", map[string]any{
		"jobId": "other-critic", "role": "code-critic", "round": 1, "parentJob": nil, "status": "completed", "goalId": "bounded", "reviews": "other",
		reviewRoundLimitField: 1, criticRoundsConsumedField: 1, findingRegisterField: open("other-critic"), findingRegisterRoundField: 1,
	})
	if _, err := CritiqueExhaustionAdvance(repo, "other-critic", "code-critic", message, ""); err == nil {
		t.Fatal("the exhausted critic root admitted before its rebind")
	}
	if outcomes, err := critiqueChainBudgetRebindWithReads(repo, "other-critic", bed.reads); err != nil || outcomes["other-critic"] != "rebound" {
		t.Fatalf("rebind through the critic root = %v, %v", outcomes, err)
	}
	if outcome, err := CritiqueExhaustionAdvance(repo, "other-critic", "code-critic", message, ""); err != nil || outcome != "none" {
		t.Fatalf("the critic root after the raise = %q, %v", outcome, err)
	}

	// A goal that is no longer a live claimed goal keeps the stored limit.
	writeJSONFile(t, jobs, "gone-critic.json", map[string]any{
		"jobId": "gone-critic", "role": "code-critic", "round": 1, "parentJob": nil, "status": "completed", "goalId": "no-such-goal",
		reviewRoundLimitField: 1, criticRoundsConsumedField: 1,
	})
	if outcomes, err := critiqueChainBudgetRebindWithReads(repo, "gone-critic", bed.reads); err != nil || outcomes["gone-critic"] != "kept" {
		t.Fatalf("a critic of a goal that is not claimed = %v, %v; want its limit kept", outcomes, err)
	}
	if _, err := critiqueChainBudgetRebindWithReads(repo, "missing", bed.reads); err == nil || !strings.Contains(err.Error(), "unreadable") {
		t.Fatalf("an unreadable root = %v", err)
	}
}
