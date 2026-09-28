package partner_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/partner"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/partner/fakeacp"
)

// The room for every sitting (g1-s67) as the service owns it: the walks keyed by
// purpose (D3), the desk's display suggestion admitted in every sitting (D4),
// and one rule for which conversation a room's address names (D6).

func designOf(record string) partner.Subject {
	return partner.Subject{Kind: partner.SubjectRecord, ID: record, Title: "Sessions"}
}

// D4: the Partner puts on the desk what it explains in every sitting, a shaping
// one included; outside a sitting there is no room, and the activity says so.
func TestADeskItemReachesEverySittingsRoom(t *testing.T) {
	t.Parallel()
	script := fakeacp.Script{
		Reads: []fakeacp.Read{
			{Name: "mcp__metasystem__present", Title: "present(source)",
				Result: "prepared for the desk\nDesk: source\nPath: internal/owner.go\nFrom: 41\nTo: 88\n"},
		},
		Chunks: []string{"Today: ..."},
	}
	held := reviewService(t, script)
	events, stop := held.service.Subscribe()
	defer stop()
	ctx := context.Background()

	_, err := held.service.Sit(ctx, "Wido", designOf(design), partner.PurposeShapeDesign, inTheRoom(design))
	testutil.Require(t, "a design sitting opened", err, nil)
	var present *partner.Present
	for _, beat := range drain(t, events) {
		if beat.Present != nil {
			present = beat.Present
			if beat.Conversation != design {
				t.Fatalf("the desk item named %q, not the room", beat.Conversation)
			}
		}
		if beat.Kind == partner.EventActivity && strings.Contains(beat.Text, "desk") {
			t.Fatalf("a shaping room has a desk, and the activity said %q", beat.Text)
		}
	}
	testutil.Require(t, "a desk item arrived", present != nil, true)
	testutil.Expect(t, "the file at its range", *present,
		partner.Present{Kind: "source", Path: "internal/owner.go", From: 41, To: 88})

	_, err = held.service.Submit(ctx, "Wido", "k1", "where is the lock taken?", partner.Page{Section: "Backlog"})
	testutil.Require(t, "a question in the ordinary conversation", err, nil)
	presented, activity := false, ""
	for _, beat := range drain(t, events) {
		presented = presented || beat.Present != nil
		if beat.Kind == partner.EventActivity && strings.Contains(beat.Text, "desk") {
			activity = beat.Text
		}
	}
	testutil.Expect(t, "no desk item outside a sitting", presented, false)
	testutil.Expect(t, "said in the activity instead", activity,
		"The Partner offered something for a desk, and only a sitting's room has one.")
}

// D6, with its two cautions: a shaping sitting marked on
// the human's ordinary conversation before D16, with its words there and no
// record-keyed files, is its room's conversation. Its answers carry the room's
// record as their key, a card records against it, End ends it, and afterwards the
// ordinary conversation is ordinary again and a Start on the same record opens a
// record-keyed sitting — the fallback was never kept under the record's key.
func TestASittingMarkedOnTheOrdinaryConversationIsItsRoomsConversation(t *testing.T) {
	t.Parallel()
	script := fakeacp.Script{
		Reads: []fakeacp.Read{
			{Name: "mcp__metasystem__deposit", Title: "deposit(fact)",
				Result: "prepared\nDeposit: fact\nAnchor: internal/owner.go:41-88\n" +
					"--- the deposit follows, whole and to the end ---\nthe lock is taken in begin\n"},
		},
		Chunks: []string{"The lock ..."},
	}
	held := reviewService(t, script)
	at := time.Date(2026, 9, 20, 9, 0, 0, 0, time.UTC)
	before, err := partner.OpenConversation(held.root, "Wido")
	testutil.Require(t, "the ordinary conversation opened", err, nil)
	testutil.Require(t, "marked before D16", before.Sit(partner.Sitting{Subject: designOf(design),
		Purpose: partner.PurposeShapeDesign, StartedAt: at.Format(time.RFC3339)}, at), nil)
	testutil.Require(t, "its question", before.Append(partner.Message{ID: "m1", Turn: "t1", Role: partner.RoleHuman,
		Text: "where is the lock taken?", At: at.Format(time.RFC3339)}), nil)
	testutil.Require(t, "its answer", before.Append(partner.Message{ID: "m2", Turn: "t1", Role: partner.RolePartner,
		Text: "In begin.", At: at.Format(time.RFC3339)}), nil)
	events, stop := held.service.Subscribe()
	defer stop()
	ctx := context.Background()
	service := held.service

	room, err := service.SnapshotIn("Wido", design, 100)
	testutil.Require(t, "the room read", err, nil)
	testutil.Expect(t, "keyed by the room's record", room.Conversation, design)
	testutil.Require(t, "the sitting stands in it", room.Sitting != nil, true)
	testutil.Expect(t, "on its record", room.Sitting.Subject.ID, design)
	testutil.Expect(t, "with the words said before", len(room.Messages), 2)
	testutil.Expect(t, "the first of them", room.Messages[0].Text, "where is the lock taken?")
	standing, err := service.Standing("Wido")
	testutil.Require(t, "the standing sittings", err, nil)
	testutil.Expect(t, "listed once", len(standing), 1)

	_, err = service.SubmitIn(ctx, "Wido", design, "k1", "and the fact of it?", inTheRoom(design))
	testutil.Require(t, "a question in the room", err, nil)
	var fact *partner.Deposit
	for at, beat := range drain(t, events) {
		testutil.Expect(t, fmt.Sprintf("beat %d names the room", at), beat.Conversation, design)
		if beat.Deposit != nil {
			fact = beat.Deposit
		}
	}
	testutil.Require(t, "the fact arrived", fact != nil, true)
	testutil.Expect(t, "offered", fact.Offered, true)
	testutil.Expect(t, "against that sitting's record", fact.Subject.ID, design)
	testutil.Require(t, "the room kept", service.KeepRoom("Wido", design, partner.Room{Face: "board", Seq: 1}), nil)
	ordinary, err := service.SnapshotIn("Wido", "", 100)
	testutil.Require(t, "the ordinary conversation read", err, nil)
	testutil.Expect(t, "nothing moved or copied: the words are the ordinary one's", len(ordinary.Messages), 4)
	testutil.Expect(t, "and the room is on its mark", ordinary.Sitting.Room.Face, "board")
	sittings, _ := filepath.Glob(filepath.Join(held.root, "sittings", "*", "*"))
	testutil.Expect(t, "no record-keyed file was written", len(sittings), 0)

	_, err = service.ClosingIn(ctx, "Wido", design, "", inTheRoom(design))
	testutil.Require(t, "End drafts the Outcome without a verdict", err, nil)
	drain(t, events)
	testutil.Require(t, "End ends it", service.RiseIn("Wido", design), nil)
	after, err := service.SittingIn("Wido", "")
	testutil.Require(t, "the ordinary sitting read", err, nil)
	testutil.Expect(t, "the ordinary conversation is ordinary again", after == nil, true)
	emptied, err := service.SnapshotIn("Wido", design, 100)
	testutil.Require(t, "the room read again", err, nil)
	testutil.Expect(t, "no sitting stands at the address", emptied.Sitting == nil, true)
	testutil.Expect(t, "and its own conversation is empty", len(emptied.Messages), 0)

	_, err = service.Sit(ctx, "Wido", designOf(design), partner.PurposeShapeDesign, inTheRoom(design))
	testutil.Require(t, "a Start on the same record", err, nil)
	drain(t, events)
	fresh, err := service.SnapshotIn("Wido", design, 100)
	testutil.Require(t, "the new sitting read", err, nil)
	testutil.Require(t, "it stands", fresh.Sitting != nil, true)
	testutil.Expect(t, "its opening and answer, on a conversation of its own", len(fresh.Messages), 2)
	still, err := service.SittingIn("Wido", "")
	testutil.Require(t, "the ordinary read", err, nil)
	testutil.Expect(t, "the ordinary one stays unmarked", still == nil, true)
	if _, err := os.Stat(filepath.Join(held.root, "sittings")); err != nil {
		t.Fatalf("the Start wrote a record-keyed conversation: %v", err)
	}
}
