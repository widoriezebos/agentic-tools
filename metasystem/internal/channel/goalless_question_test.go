package channel

import (
	"context"
	"os"
	"path/filepath"
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
// alone: its reply has its one inbox record on the ledger as every reply
// does (Decision 8), no goal is answered on the ledger (Decision 3), and the
// answer closes locally with no receipt posted (Decision 7).
func TestGoallessAnswerIsRecordedWithoutTheGoalLedger(t *testing.T) {
	t.Parallel()
	bed, p, _, now := pollLedgerBed(t)
	if err := os.Remove(questionPath(bed.root, "01J5X0000000000000000000Q0")); err != nil {
		t.Fatal(err)
	}
	q := Question{ID: "01J5X0000000000000000000L0", About: "lane", Kind: "other", Machine: "machine", Lineage: "landing-agent", OpenedAt: now,
		Facts: []string{"Return the branch?"}, Thread: &MessageRef{ID: "1", ThreadID: "1"}, State: "open"}
	if err := writeJSON(questionPath(bed.root, q.ID), q); err != nil {
		t.Fatal(err)
	}
	history := len(bed.canonicalGoal(t, "g").History)
	code, _ := TOTPCode("JBSWY3DPEHPK3PXP", now)
	p.inbound = []Inbound{{Ref: MessageRef{ID: "2", ThreadID: "1"}, ThreadID: "1", UserID: "UWIDO", Text: "return it " + code}}
	if _, err := bed.poll(context.Background(), pollBedConfig(bed, p, now)); err != nil {
		t.Fatal(err)
	}
	got, err := ReadQuestion(bed.root, q.ID)
	if err != nil || got.Answer == nil || got.Answer.Text != "return it" || got.Answer.Phase != "closed" || got.State != "closed" {
		t.Fatalf("goal-less answer = %+v %v", got, err)
	}
	if after := len(bed.canonicalGoal(t, "g").History); after != history {
		t.Fatalf("a goal-less answer wrote %d goal history lines", after-history)
	}
	if inbox := bed.inbox(); len(inbox) != 1 {
		t.Fatalf("the reply's inbox records = %+v; want exactly one", inbox)
	}
	if len(p.posts) != 0 {
		t.Fatalf("an answered goal-less question posts no receipt: %q", p.posts)
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

// A questions folder that can't be listed is named, not read as a folder
// with no questions; a missing one is no questions.
func TestWalkQuestionsNamesAFolderItCannotList(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		t.Skip("root lists any folder")
	}
	root := t.TempDir()
	if all, unreadable := WalkQuestions(root); len(all) != 0 || len(unreadable) != 0 {
		t.Fatalf("missing folder = %v %v; want no questions and nothing unreadable", all, unreadable)
	}
	dir := filepath.Join(channelRoot(root), "questions")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })

	all, unreadable := WalkQuestions(root)

	if len(all) != 0 || len(unreadable) != 1 || !strings.HasPrefix(unreadable[0], dir+": ") {
		t.Fatalf("unlistable folder = %v %v; want the folder named as unreadable", all, unreadable)
	}
}
