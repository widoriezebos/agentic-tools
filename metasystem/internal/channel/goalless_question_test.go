package channel

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

// A question about the lane or the machine names no goal: it is an ordinary
// question record with about set and the asker's lineage, and no goal
// ledger act is recorded for it (this root has no ledger at all, so any
// ledger step would fail the ask).
func TestGoallessAskRecordsTheQuestionAndNoLedgerAct(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	p := &testProvider{}
	q, existing, err := AskOrFind(AskRequest{RepoRoot: root, About: "lane", Kind: "other", Machine: "m1", Lineage: "landing-agent",
		Facts: []string{"Merge or return the conflicting branch?", "push refused: HEAD does not contain origin's main"}, Provider: p})
	if err != nil || existing {
		t.Fatalf("goal-less ask: existing=%t err=%v", existing, err)
	}
	stored, err := ReadQuestion(root, q.ID)
	if err != nil || stored.Goal != "" || stored.About != "lane" || stored.Machine != "m1" || stored.Lineage != "landing-agent" || stored.State != "open" || stored.Thread == nil {
		t.Fatalf("stored goal-less question = %+v %v", stored, err)
	}
	if len(p.posts) != 1 || !strings.HasPrefix(p.posts[0], "the landing lane") || strings.Contains(p.posts[0], "goal ") {
		t.Fatalf("posted text names no goal and says what it is about: %q", p.posts)
	}
	again, existing, err := AskOrFind(AskRequest{RepoRoot: root, About: "lane", Kind: "other", Machine: "m1", Lineage: "landing-agent",
		Facts: []string{"Merge or return the conflicting branch?", "push refused: HEAD does not contain origin's main"}, Provider: p})
	if err != nil || !existing || again.ID != q.ID || len(p.posts) != 1 {
		t.Fatalf("the same goal-less question asked again: existing=%t id=%s posts=%d err=%v", existing, again.ID, len(p.posts), err)
	}
	machine, _, err := AskOrFind(AskRequest{RepoRoot: root, About: "machine", Kind: "other", Machine: "m1", Lineage: "seat",
		Facts: []string{"Merge or return the conflicting branch?", "push refused: HEAD does not contain origin's main"}})
	if err != nil || machine.ID == q.ID || machine.About != "machine" {
		t.Fatalf("the same facts about the machine are another question: %+v %v", machine, err)
	}
}

func TestGoallessAskRefusesWhatItCannotRecord(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for _, test := range []struct {
		name    string
		request AskRequest
		want    string
	}{
		{"an authority kind", AskRequest{About: "lane", Kind: "stop", Wants: "resume x"}, "about"},
		{"an unknown subject", AskRequest{About: "seat", Kind: "other"}, "the lane or the machine"},
		{"both a goal and a subject", AskRequest{Goal: "g", About: "lane", Kind: "other"}, "about"},
		{"neither a goal nor a subject", AskRequest{Kind: "other"}, "goal"},
	} {
		request := test.request
		request.RepoRoot, request.Machine, request.Facts = root, "m1", []string{"fact"}
		if _, _, err := AskOrFind(request); err == nil || !strings.Contains(err.Error(), test.want) {
			t.Fatalf("%s: err=%v; want a refusal naming %q", test.name, err, test.want)
		}
	}
	if questions, _ := WalkQuestions(root); len(questions) != 0 {
		t.Fatalf("a refused ask wrote %d question records", len(questions))
	}
}

// The answer to a goal-less question is recorded on the question record
// alone: no goal ledger act, straight to recorded, then receipted and
// closed as any answer is.
func TestGoallessAnswerIsRecordedWithoutTheGoalLedger(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	now := time.Date(2026, 10, 2, 9, 0, 0, 0, time.UTC)
	q := Question{ID: "01J5X0000000000000000000L0", About: "lane", Kind: "other", Machine: "machine", Lineage: "landing-agent", OpenedAt: now,
		Facts: []string{"Return the branch?"}, Thread: &MessageRef{ID: "1", ThreadID: "1"}, State: "open"}
	if err := writeJSON(questionPath(root, q.ID), q); err != nil {
		t.Fatal(err)
	}
	code, _ := TOTPCode("JBSWY3DPEHPK3PXP", now)
	p := &testProvider{cursor: "done", inbound: []Inbound{{Ref: MessageRef{ID: "2", ThreadID: "1"}, ThreadID: "1", UserID: "UWIDO", Text: "return it " + code}}}
	noLedger := func(string) (goal.Endpoint, error) {
		return goal.Endpoint{}, errors.New("a goal-less answer reached the goal ledger")
	}
	config := PollConfig{RepoRoot: root, Destination: "fleet", ProviderName: "fake", HumanUserID: "UWIDO", TOTPSecret: "JBSWY3DPEHPK3PXP", Machine: "machine", Lineage: "lineage", Provider: p, Now: now}
	if _, err := pollWithEndpoint(context.Background(), config, noLedger); err != nil {
		t.Fatal(err)
	}
	got, err := ReadQuestion(root, q.ID)
	if err != nil || got.Answer == nil || got.Answer.Text != "return it" || got.Answer.Phase != "closed" || got.State != "closed" {
		t.Fatalf("goal-less answer = %+v %v", got, err)
	}
	if last := p.posts[len(p.posts)-1]; !strings.Contains(last, "the landing lane") || strings.Contains(last, "ledger operation") {
		t.Fatalf("the receipt names what was answered and no ledger act: %q", last)
	}
}

func TestWalkQuestionsReadsEveryRecordAndOpenQuestionsTheOpenOnes(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	now := time.Date(2026, 10, 2, 9, 0, 0, 0, time.UTC)
	for _, q := range []Question{
		{ID: "01J5X0000000000000000000W0", Goal: "g", Kind: "other", Machine: "m1", Lineage: "seat", OpenedAt: now, Facts: []string{"a"}, State: "open"},
		{ID: "01J5X0000000000000000000W1", About: "lane", Kind: "other", Machine: "m1", Lineage: "landing-agent", OpenedAt: now.Add(time.Minute), Facts: []string{"b"}, State: "answered", Answer: &Answer{Text: "x", At: now}},
		{ID: "01J5X0000000000000000000W2", About: "machine", Kind: "other", Machine: "m1", OpenedAt: now.Add(2 * time.Minute), Facts: []string{"c"}, State: "closed"},
	} {
		if err := writeJSON(questionPath(root, q.ID), q); err != nil {
			t.Fatal(err)
		}
	}
	all, unreadable := WalkQuestions(root)
	if len(all) != 3 || len(unreadable) != 0 {
		t.Fatalf("every record: %d %v", len(all), unreadable)
	}
	open := GoalOpenQuestions(root)
	if len(open) != 1 || open[0] != (goal.OpenQuestion{ID: "01J5X0000000000000000000W0", Goal: "g", Machine: "m1", Lineage: "seat"}) {
		t.Fatalf("open questions for the goal judgment = %+v", open)
	}
}
