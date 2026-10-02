package channel

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

// F-1 defence: a record the ledger still refuses is replaced by a minimal
// skipped record, confirmed, and the queue moves on.
func TestUnrecordableUpdateIsSkippedNotBlocking(t *testing.T) {
	bed, p, q, now := pollLedgerBed(t)
	t.Parallel()
	cfg := pollBedConfig(bed, p, now)
	cfg.validateInbox = func(e goal.Endpoint, commit string) error {
		for _, in := range mustInboxAt(t, e, commit) {
			if in.MessageID == "5" && in.Outcome != "skipped" {
				return errors.New("refused by the fixture")
			}
		}
		return validateInboxCommit(e, commit)
	}
	code, _ := TOTPCode("JBSWY3DPEHPK3PXP", now)
	p.inbound = []Inbound{
		{Ref: MessageRef{ID: "5", ThreadID: "1"}, ThreadID: "1", UserID: "UWIDO", Text: "odd", SentAt: now, Ack: "6", UpdateID: 5},
		{Ref: MessageRef{ID: "7", ThreadID: "1"}, ThreadID: "1", UserID: "UWIDO", Text: "approved " + code, SentAt: now, Ack: "8", UpdateID: 7},
	}
	if _, err := bed.poll(context.Background(), cfg); err != nil {
		t.Fatal(err)
	}
	// The refused build is never published; the fake's cleanup must not
	// mistake it for a leaked commit.
	for id := range bed.built {
		delete(bed.built, id)
	}
	skipped := bed.inbox()["plans/channel/inbox/fleet/fake-5.json"]
	if skipped.Outcome != "skipped" || skipped.Text != "" || skipped.Step != nil || len(p.confirmed) < 2 || p.confirmed[0] != "6" {
		t.Fatalf("skipped=%+v confirmed=%v", skipped, p.confirmed)
	}
	if len(p.posts) != 1 || !strings.HasPrefix(p.posts[0], "not recorded: unrecordable.") {
		t.Fatalf("posts=%v", p.posts)
	}
	if got, _ := ReadQuestion(bed.root, q.ID); got.Answer == nil {
		t.Fatalf("the good reply behind it was not matched: %+v", got)
	}
}

func mustInboxAt(t *testing.T, e goal.Endpoint, commit string) map[string]*goal.ChannelInbound {
	t.Helper()
	records, err := goal.ReadChannelInbox(e, commit)
	if err != nil {
		t.Fatal(err)
	}
	return records
}
