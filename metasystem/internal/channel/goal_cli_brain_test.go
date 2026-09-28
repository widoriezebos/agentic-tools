package channel

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/brain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

// TestGoalCLIBrainVerifiedChannelAnswerApprovesThroughTheBrain ports the last
// block of goal-cli-fixtures.sh brain-human-word-refuses: in a checkout
// declared the brain, a budget question asked on the channel and answered by
// the configured human with a valid code approves the goal. The brain fence
// refuses a human's word carried by a seat, never the verified answer.
func TestGoalCLIBrainVerifiedChannelAnswerApprovesThroughTheBrain(t *testing.T) {
	t.Parallel()
	bed, provider, _, now := pollLedgerBed(t)
	root := bed.root
	record := brain.Record{Schema: brain.Schema, Ledger: "01ARZ3NDEKTSV4RRFFQ69G5FAV", Machine: "machine", DeclaredBy: "Wido", DeclaredAt: "2026-09-03T00:00:00Z"}
	data, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(brain.Path(root)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(brain.Path(root), append(data, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	if state := brain.Read(root, goal.ExistingLedgerIdentityAtEndpoint(bed.endpoint)); state.State != brain.Declared {
		t.Fatalf("the bed is not declared the brain: %+v", state)
	}

	box, err := goal.NewBudget("1h", 1, 60, 1, 3)
	if err != nil {
		t.Fatal(err)
	}
	question, err := Ask(AskRequest{
		RepoRoot: root, Goal: "g", Kind: "budget-above-norm", Machine: "machine",
		Facts: []string{"Approve the brain channel fixture."}, Options: []Option{{Label: "approve", Consequence: "continue"}},
		Recommendation: "approve", Wants: "goal=g minutes=60 reviewRounds=3 goalRevision=1",
		Budget: &box, Provider: provider, Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if question.Thread == nil || question.Wants == "" {
		t.Fatalf("the ask recorded no thread or answer token: %+v", question)
	}
	code, err := TOTPCode("JBSWY3DPEHPK3PXP", now)
	if err != nil {
		t.Fatal(err)
	}
	thread := question.Thread.ThreadID
	if thread == "" {
		thread = question.Thread.ID
	}
	provider.inbound = []Inbound{{Ref: MessageRef{ID: "answer", ThreadID: thread}, ThreadID: thread, UserID: "UWIDO",
		Text: question.Wants + " " + code, SentAt: now}}
	if _, err := bed.poll(context.Background(), pollBedConfig(bed, provider, now)); err != nil {
		t.Fatal(err)
	}

	approved := projectGoal(t, bed, "g")
	var row *goal.HistoryLine
	for index := range approved.History {
		if approved.History[index].Verb == "approve" {
			row = &approved.History[index]
		}
	}
	if row == nil || row.Actor != "human:UWIDO" || row.AuthorityOutcome != goal.AuthorityOutcomeVerifiedChannelAnswer || approved.Approved == nil {
		t.Fatalf("verified channel answer did not approve through the brain checkout: row=%+v goal=%+v posts=%v", row, approved, provider.posts)
	}
	if state := brain.Read(root, goal.ExistingLedgerIdentityAtEndpoint(bed.endpoint)); state.State != brain.Declared {
		t.Fatalf("the answer changed the declaration: %+v", state)
	}
}
