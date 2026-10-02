package channel

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

// Wido 2026-10-02: channel.human.answer-code=off verifies an answer by the
// sender's user id alone, with the same authority as a code-verified one.
func answerCodeOff(t *testing.T, root string) {
	t.Helper()
	f, err := os.OpenFile(filepath.Join(root, "metasystem.conf"), os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.WriteString("channel.human.answer-code=off\n"); err != nil {
		t.Fatal(err)
	}
	if err = f.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestAnswerCodeOffRecordsTheHumansAnswerWithoutACode(t *testing.T) {
	t.Parallel()
	bed, p, q, now := pollLedgerBed(t)
	answerCodeOff(t, bed.root)
	q.Thread = nil
	if err := writeJSON(questionPath(bed.root, q.ID), q); err != nil {
		t.Fatal(err)
	}
	cfg := pollBedConfig(bed, p, now)
	cfg.TOTPSecret = ""
	if _, err := bed.poll(context.Background(), cfg); err != nil {
		t.Fatal(err)
	}
	if len(p.posts) != 1 || strings.Contains(p.posts[0], "code") || !strings.Contains(p.posts[0], "Reply in this thread with your answer") {
		t.Fatalf("question post: %q", p.posts)
	}
	p.inbound = []Inbound{{Ref: MessageRef{ID: "2", ThreadID: "1"}, ThreadID: "1", UserID: "UWIDO", Text: "approved", SentAt: now}}
	if _, err := bed.poll(context.Background(), cfg); err != nil {
		t.Fatal(err)
	}
	got, _ := ReadQuestion(bed.root, q.ID)
	if got.State != "closed" || got.Answer == nil || got.Answer.Text != "approved" {
		t.Fatalf("question=%+v", got)
	}
	if in := bed.inbox()["plans/channel/inbox/fleet/fake-2.json"]; in.Outcome != "verified" || in.Step == nil {
		t.Fatalf("record=%+v", in)
	}
	g := projectGoal(t, bed, "g")
	last := g.History[len(g.History)-1]
	if last.Verb != "answer" || last.AuthorityOutcome != goal.AuthorityOutcomeAuthenticatedChannelWord || last.ChannelUser != "UWIDO" || last.ChannelStep <= 0 {
		t.Fatalf("answer row=%+v", last)
	}
}

func TestAnswerCodeOffRejectsAnotherSenderWithoutCodeWording(t *testing.T) {
	t.Parallel()
	bed, p, q, now := pollLedgerBed(t)
	answerCodeOff(t, bed.root)
	cfg := pollBedConfig(bed, p, now)
	cfg.TOTPSecret = ""
	p.inbound = []Inbound{{Ref: MessageRef{ID: "2", ThreadID: "1"}, ThreadID: "1", UserID: "stranger", Text: "approved", SentAt: now}}
	if _, err := bed.poll(context.Background(), cfg); err != nil {
		t.Fatal(err)
	}
	if got, _ := ReadQuestion(bed.root, q.ID); got.State != "open" || got.Answer != nil {
		t.Fatalf("question=%+v", got)
	}
	if len(p.posts) != 1 || p.posts[0] != "not recorded: wrong user. Reply to the question above with your answer" {
		t.Fatalf("posts=%q", p.posts)
	}
}

// With the code off, an answer that binds an act carries the same proven
// channel authority as a code-verified one.
func TestAnswerCodeOffBudgetAnswerApprovesAsProven(t *testing.T) {
	t.Parallel()
	bed, p, _, now := pollLedgerBed(t)
	answerCodeOff(t, bed.root)
	box, _ := goal.NewBudget("2h", 5, 600, 1, 0)
	q := Question{ID: "01J5X0000000000000000000B1", Goal: "g", Kind: "budget-above-norm", Machine: "machine", OpenedAt: now, Facts: []string{"raise the box"}, Wants: "yes", Budget: &box, Thread: &MessageRef{ID: "10", ThreadID: "10"}, State: "open"}
	if err := writeJSON(questionPath(bed.root, q.ID), q); err != nil {
		t.Fatal(err)
	}
	cfg := pollBedConfig(bed, p, now)
	cfg.TOTPSecret = ""
	p.inbound = []Inbound{{Ref: MessageRef{ID: "11", ThreadID: "10"}, ThreadID: "10", UserID: "UWIDO", Text: "yes", SentAt: now}}
	if _, err := bed.poll(context.Background(), cfg); err != nil {
		t.Fatal(err)
	}
	g := projectGoal(t, bed, "g")
	if g.Budget == nil || *g.Budget != box || g.Approved == nil || g.Approved.Authority != goal.ApprovalAuthorityChannel || g.NormApproval == nil || g.NormApproval.ApprovedRef != q.ID {
		t.Fatalf("the uncoded budget answer did not approve with channel authority: %+v", g)
	}
	for _, h := range g.History {
		if h.Verb == "approve" && h.AuthorityOutcome != goal.AuthorityOutcomeVerifiedChannelAnswer {
			t.Fatalf("approve row authority=%q", h.AuthorityOutcome)
		}
	}
}

// R2-2: with the code off, six-digit numbers in an answer are the human's
// words and stay intact.
func TestAnswerCodeOffKeepsSixDigitNumbers(t *testing.T) {
	t.Parallel()
	bed, p, q, now := pollLedgerBed(t)
	answerCodeOff(t, bed.root)
	cfg := pollBedConfig(bed, p, now)
	cfg.TOTPSecret = ""
	p.inbound = []Inbound{{Ref: MessageRef{ID: "2", ThreadID: "1"}, ThreadID: "1", UserID: "UWIDO", Text: "limit 100000 rows", SentAt: now}}
	if _, err := bed.poll(context.Background(), cfg); err != nil {
		t.Fatal(err)
	}
	if got, _ := ReadQuestion(bed.root, q.ID); got.Answer == nil || got.Answer.Text != "limit 100000 rows" {
		t.Fatalf("question=%+v", got)
	}
}
