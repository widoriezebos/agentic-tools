package channel

import (
	"bytes"
	"os"
	"testing"
	"time"
)

func TestRetireLetsTheDecisionAskAgain(t *testing.T) {
	t.Parallel()
	r := contractRequest(t.TempDir())
	r.Goal, r.About, r.Machine = "", "machine", "ui"
	r.Decision = &Decision{Kind: "steward-hold", Subject: "held-goal", State: "held"}
	q, _, err := AskOrFind(r)
	if err != nil {
		t.Fatal(err)
	}
	at := r.Now.Add(time.Minute)
	if err := Retire(r.RepoRoot, q.ID, "the hold cleared", at); err != nil {
		t.Fatal(err)
	}
	q, err = ReadQuestion(r.RepoRoot, q.ID)
	if err != nil || q.State != "closed" || !q.ClosedAt.Equal(at) || !q.RetiredAt.Equal(at) || q.ClosedBecause != "the hold cleared" {
		t.Fatalf("retire: %+v %v", q, err)
	}
	before, err := os.ReadFile(questionPath(r.RepoRoot, q.ID))
	if err != nil {
		t.Fatal(err)
	}
	before = append(before, '\n')
	if err := os.WriteFile(questionPath(r.RepoRoot, q.ID), before, 0600); err != nil {
		t.Fatal(err)
	}
	if err := Retire(r.RepoRoot, q.ID, "another explanation", at.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(questionPath(r.RepoRoot, q.ID))
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("a repeated retirement rewrote the question")
	}
	r.Now = at.Add(2 * time.Minute)
	again, found, err := AskOrFind(r)
	if err != nil || found || again.ID == q.ID {
		t.Fatalf("same decision after retirement: %+v found=%t %v", again, found, err)
	}
	if subject := QuestionSubject(again); subject != "held goal" {
		t.Fatalf("confirmation subject = %q", subject)
	}
}
