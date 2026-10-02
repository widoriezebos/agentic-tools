package channel

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

// Decision 8: receive with no offset, commit every update to the ledger
// inbox, and Confirm its Ack only once the commit is durable.
func TestPollCommitsEachUpdateToTheInboxBeforeConfirm(t *testing.T) {
	bed, p, q, now := pollLedgerBed(t)
	code, _ := TOTPCode("JBSWY3DPEHPK3PXP", now)
	p.inbound = []Inbound{
		{Ref: MessageRef{ID: "7"}, UserID: "stranger", Text: "hello", SentAt: now, Ack: "8", UpdateID: 7},
		{Ref: MessageRef{ID: "9", ThreadID: "1"}, ThreadID: "1", UserID: "UWIDO", Text: "approved " + code, SentAt: now, Ack: "10", UpdateID: 9},
	}
	var seen []int
	p.onConfirm = func(Cursor) { seen = append(seen, len(bed.inbox())) }
	if _, err := bed.poll(context.Background(), pollBedConfig(bed, p, now)); err != nil {
		t.Fatal(err)
	}
	if p.after != "" {
		t.Fatalf("receive sent the saved offset %q", p.after)
	}
	// Each update is confirmed after its own commit, and the batch cursor
	// last, once every item is committed.
	if fmt.Sprint(p.confirmed) != "[8 10 done]" || fmt.Sprint(seen) != "[1 2 2]" {
		t.Fatalf("confirmed %v with inbox sizes %v", p.confirmed, seen)
	}
	inbox := bed.inbox()
	stranger, answer := inbox["plans/channel/inbox/fleet/fake-7.json"], inbox["plans/channel/inbox/fleet/fake-9.json"]
	if stranger.Outcome != "wrong-user" || stranger.Step != nil || stranger.ReplyTo != nil || stranger.Question != "unmatched" || stranger.ReceivedBy != "machine" {
		t.Fatalf("stranger record: %+v", stranger)
	}
	if answer.Outcome != "verified" || answer.Step == nil || answer.Text != "approved" || answer.ReplyTo == nil || *answer.ReplyTo != "1" || answer.UpdateID != "9" {
		t.Fatalf("answer record: %+v", answer)
	}
	if got, _ := ReadQuestion(bed.root, q.ID); got.State != "closed" || got.Answer == nil || got.Answer.Text != "approved" {
		t.Fatalf("question: %+v", got)
	}
}

func TestPollKilledAfterCommitConfirmsNothing(t *testing.T) {
	bed, p, _, now := pollLedgerBed(t)
	p.inbound = []Inbound{{Ref: MessageRef{ID: "7"}, UserID: "stranger", Text: "hello", SentAt: now, Ack: "8", UpdateID: 7}}
	cfg := pollBedConfig(bed, p, now)
	cfg.FailurePoint = func(point string) error {
		if point == "inbox-committed" {
			return errors.New("killed")
		}
		return nil
	}
	if _, err := bed.poll(context.Background(), cfg); err == nil {
		t.Fatal("the kill after the commit did not fire")
	}
	if len(bed.inbox()) != 1 || p.confirms != 0 {
		t.Fatalf("inbox=%v confirms=%d", bed.inbox(), p.confirms)
	}
	cfg.FailurePoint = nil
	if _, err := bed.poll(context.Background(), cfg); err != nil {
		t.Fatal(err)
	}
	if len(bed.inbox()) != 1 || fmt.Sprint(p.confirmed) != "[8 done]" {
		t.Fatalf("redelivery: inbox=%v confirmed=%v", bed.inbox(), p.confirmed)
	}
	if len(p.posts) != 1 {
		t.Fatalf("the lost redelivery posted a second notice: %v", p.posts)
	}
}

// A rejected reply gets one notice in the question's thread and the
// question stays open; no rejection is saved on the question.
func TestRejectedReplyIsNoticedOnceInTheQuestionThread(t *testing.T) {
	bed, p, q, now := pollLedgerBed(t)
	p.inbound = []Inbound{{Ref: MessageRef{ID: "5", ThreadID: "1"}, ThreadID: "1", UserID: "UWIDO", Text: "approved 000000", SentAt: now, Ack: "6", UpdateID: 5}}
	cfg := pollBedConfig(bed, p, now)
	for i := 0; i < 2; i++ {
		if _, err := bed.poll(context.Background(), cfg); err != nil {
			t.Fatal(err)
		}
	}
	if len(p.posts) != 1 || p.posts[0] != "not recorded: bad code. Reply to the question above with your answer and your code" ||
		p.postThreads[0] == nil || p.postThreads[0].ID != "1" {
		t.Fatalf("posts=%v threads=%+v", p.posts, p.postThreads)
	}
	got, _ := ReadQuestion(bed.root, q.ID)
	if got.State != "open" || got.Answer != nil || len(got.Rejected) != 0 {
		t.Fatalf("question: %+v", got)
	}
	if in := bed.inbox()["plans/channel/inbox/fleet/fake-5.json"]; in.Outcome != "bad-code" || in.Text != "approved" {
		t.Fatalf("record: %+v", in)
	}
}

// Decision 7: the answer receipt is not posted, and the question still
// closes locally.
func TestAnsweredQuestionClosesWithoutAReceiptPost(t *testing.T) {
	bed, p, q, now := pollLedgerBed(t)
	code, _ := TOTPCode("JBSWY3DPEHPK3PXP", now)
	p.inbound = []Inbound{{Ref: MessageRef{ID: "2", ThreadID: "1"}, ThreadID: "1", UserID: "UWIDO", Text: "approved " + code, SentAt: now}}
	if _, err := bed.poll(context.Background(), pollBedConfig(bed, p, now)); err != nil {
		t.Fatal(err)
	}
	got, _ := ReadQuestion(bed.root, q.ID)
	if got.State != "closed" || got.Answer == nil || got.Answer.Phase != "closed" || len(p.posts) != 0 {
		t.Fatalf("question=%+v posts=%v", got, p.posts)
	}
}

// An unthreaded verified reply binds by the question's token.
func TestUnthreadedReplyMatchesByToken(t *testing.T) {
	bed, p, q, now := pollLedgerBed(t)
	q.Kind, q.Wants = "stop", "stop g"
	if err := writeJSON(questionPath(bed.root, q.ID), q); err != nil {
		t.Fatal(err)
	}
	code, _ := TOTPCode("JBSWY3DPEHPK3PXP", now)
	p.inbound = []Inbound{{Ref: MessageRef{ID: "3"}, UserID: "UWIDO", Text: "stop g " + code, SentAt: now.Add(time.Second)}}
	if _, err := bed.poll(context.Background(), pollBedConfig(bed, p, now.Add(time.Minute))); err != nil {
		t.Fatal(err)
	}
	if got, _ := ReadQuestion(bed.root, q.ID); got.Answer == nil || got.Answer.Text != "stop g" {
		t.Fatalf("question: %+v", got)
	}
}

// An installation without the human's id or secret cannot check a reply, so
// it receives nothing and confirms nothing.
func TestUnconfiguredInstallationReceivesNothing(t *testing.T) {
	bed, p, _, now := pollLedgerBed(t)
	p.inbound = []Inbound{{Ref: MessageRef{ID: "2", ThreadID: "1"}, ThreadID: "1", UserID: "UWIDO", Text: "approved 123456", SentAt: now, Ack: "3"}}
	cfg := pollBedConfig(bed, p, now)
	cfg.TOTPSecret = ""
	if _, err := bed.poll(context.Background(), cfg); err != nil {
		t.Fatal(err)
	}
	if p.receives != 0 || p.confirms != 0 || len(bed.inbox()) != 0 || len(p.posts) != 0 {
		t.Fatalf("receives=%d confirms=%d inbox=%v posts=%v", p.receives, p.confirms, bed.inbox(), p.posts)
	}
}
