package partner_test

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/partner"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/partner/fakeacp"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/project"
)

// The review sitting (g1-s65 slice A) as the service owns it: a purpose and a
// subject it admits, a conversation of its own for every sitting (D16), one
// live session that follows the conversation last asked, the handoff while a
// turn runs (S65-06), a fresh reviewer (D3), the walks (D6), the finding and the
// desk's display suggestion (D5, D8), and the room kept on the mark (D9).

const (
	reviewA = "metasystem/plans/reviews/review-of-g1-s64.md"
	reviewB = "metasystem/plans/reviews/review-of-g1-s63.md"
	design  = "metasystem/plans/designs/sessions.md"
)

// reviewing is a service over the fake runtime with a document reader that
// knows which records are reviews, and a count of the sessions it opened.
type reviewing struct {
	service  *partner.Service
	watched  *fakeacp.Servers
	sessions *atomic.Int32
	root     string
	// opened is called each time a session opens, before it is used.
	mu     sync.Mutex
	onOpen func()
}

func reviewService(t *testing.T, script fakeacp.Script) *reviewing {
	t.Helper()
	root := t.TempDir()
	opener, watched := fakeacp.OpenWatched(script)
	held := &reviewing{watched: watched, sessions: &atomic.Int32{}, root: root}
	counting := func(ctx context.Context) (partner.Endpoint, error) {
		held.sessions.Add(1)
		held.mu.Lock()
		call := held.onOpen
		held.mu.Unlock()
		if call != nil {
			call()
		}
		return opener(ctx)
	}
	host := partner.NewHostOn(partner.Runtime{Name: "fake", ReadOnly: "a fake server reads nothing"}, root, counting)
	t.Cleanup(host.Close)
	facts := partner.Facts{Document: func(id string) (project.Document, error) {
		kind := "design"
		if strings.Contains(id, "/reviews/") {
			kind = "review"
		}
		return project.Document{ID: id, Record: &project.Head{Kind: kind, Goals: []string{"g1-s64"}}}, nil
	}}
	held.service = partner.NewService(host.Runtime(), host,
		func(human string) (*partner.Conversation, error) { return partner.OpenConversation(root, human) },
		facts, func() time.Time { return time.Date(2026, 9, 28, 9, 0, 0, 0, time.UTC) })
	return held
}

func reviewOf(record string) partner.Subject {
	return partner.Subject{Kind: partner.SubjectRecord, ID: record, Title: "Review of g1-s64"}
}

func inTheRoom(record string) partner.Page {
	return partner.Page{Section: "Review", Path: "/review/" + record, Kind: "record", Subject: record}
}

func prompt(t *testing.T, watched *fakeacp.Servers, at int) string {
	t.Helper()
	prompts := watched.Prompts()
	if len(prompts) <= at {
		t.Fatalf("the runtime received %d prompts, and prompt %d was expected", len(prompts), at+1)
	}
	return prompts[at]
}

func lastOutcome(t *testing.T, service *partner.Service, sitting string) string {
	t.Helper()
	read, err := service.SnapshotIn("Wido", sitting, 100)
	if err != nil {
		t.Fatalf("reading the conversation of %q: %v", sitting, err)
	}
	if len(read.Messages) == 0 {
		return ""
	}
	return read.Messages[len(read.Messages)-1].Outcome
}

func TestAReviewIsAPurposeAndItsRecordASubject(t *testing.T) {
	t.Parallel()
	held := reviewService(t, fakeacp.Script{Chunks: []string{"Asked: ..."}})
	events, stop := held.service.Subscribe()
	defer stop()

	testutil.Expect(t, "review is admitted", held.service.Admits(context.Background(), partner.PurposeReview), nil)
	err := held.service.Admits(context.Background(), "learning")
	testutil.Expect(t, "learning is not", err != nil && strings.Contains(err.Error(), "learning sittings are not in this build"), true)

	_, err = held.service.Sit(context.Background(), "Wido", reviewOf(design), partner.PurposeReview, inTheRoom(design))
	testutil.Expect(t, "a review of a design record", err.Error(),
		"a review sitting is about a review record, and "+design+" is a design record")
	_, err = held.service.Sit(context.Background(), "Wido", reviewOf(reviewA), partner.PurposeShapeDesign, inTheRoom(reviewA))
	testutil.Expect(t, "shaping a review record", err.Error(),
		"a sitting is about an intent or a design record, and "+reviewA+" is a review record")

	sitting, err := held.service.Sit(context.Background(), "Wido", reviewOf(reviewA), partner.PurposeReview, inTheRoom(reviewA))
	testutil.Require(t, "the review opened", err, nil)
	testutil.Expect(t, "on its record", sitting.Subject.ID, reviewA)
	drain(t, events)

	read, err := held.service.SnapshotIn("Wido", reviewA, 100)
	testutil.Require(t, "the review's conversation", err, nil)
	testutil.Require(t, "carries the mark", read.Sitting != nil, true)
	testutil.Expect(t, "for a review", read.Sitting.Purpose, partner.PurposeReview)
	testutil.Require(t, "the opening turn and its answer", len(read.Messages), 2)
	testutil.Expect(t, "asked by the interface", read.Messages[0].Interface, true)
	testutil.Expect(t, "the review's own brief", read.Messages[0].Text, partner.OpeningRequest(sitting))

	ordinary, err := held.service.Snapshot("Wido", 100)
	testutil.Require(t, "the human's own conversation", err, nil)
	testutil.Expect(t, "is untouched", len(ordinary.Messages), 0)
	testutil.Expect(t, "and carries no mark", ordinary.Sitting == nil, true)
}

// D16 and D3: a turn on another conversation closes the live session and opens
// that conversation's, fresh, with its own history replayed and nobody else's —
// proven by what the fake runtime received.
func TestTheLiveSessionFollowsTheConversationLastAsked(t *testing.T) {
	t.Parallel()
	held := reviewService(t, fakeacp.Script{Chunks: []string{"An answer."}})
	events, stop := held.service.Subscribe()
	defer stop()
	ctx := context.Background()

	_, err := held.service.Submit(ctx, "Wido", "k1", "which goals wait for me?", partner.Page{})
	testutil.Require(t, "the drawer's question", err, nil)
	drain(t, events)
	_, err = held.service.Sit(ctx, "Wido", reviewOf(reviewA), partner.PurposeReview, inTheRoom(reviewA))
	testutil.Require(t, "the review opened", err, nil)
	drain(t, events)
	testutil.Expect(t, "the review opened a session of its own", held.sessions.Load(), int32(2))
	opening := prompt(t, held.watched, 1)
	testutil.Expect(t, "the review's first prompt carries its brief", strings.Contains(opening, "Open this review"), true)
	testutil.Expect(t, "and none of the drawer's words", strings.Contains(opening, "which goals wait for me?"), false)

	_, err = held.service.SubmitIn(ctx, "Wido", reviewA, "k2", "what if the press dies here?", inTheRoom(reviewA))
	testutil.Require(t, "a question in the room", err, nil)
	drain(t, events)
	testutil.Expect(t, "the same session answers it", held.sessions.Load(), int32(2))
	testutil.Expect(t, "with nothing replayed", strings.Contains(prompt(t, held.watched, 2), "Conversation so far"), false)

	_, err = held.service.Submit(ctx, "Wido", "k3", "and after that?", partner.Page{})
	testutil.Require(t, "back in the drawer", err, nil)
	drain(t, events)
	testutil.Expect(t, "the drawer's conversation opens its own session", held.sessions.Load(), int32(3))
	back := prompt(t, held.watched, 3)
	testutil.Expect(t, "with its own history replayed", strings.Contains(back, "which goals wait for me?"), true)
	testutil.Expect(t, "and none of the room's", strings.Contains(back, "what if the press dies here?"), false)
	testutil.Expect(t, "nor the room's brief", strings.Contains(back, "Open this review"), false)

	_, err = held.service.SubmitIn(ctx, "Wido", reviewA, "k4", "and the reconcile?", inTheRoom(reviewA))
	testutil.Require(t, "back in the room", err, nil)
	drain(t, events)
	testutil.Expect(t, "the room's conversation opens its own session again", held.sessions.Load(), int32(4))
	room := prompt(t, held.watched, 4)
	testutil.Expect(t, "the room's history replayed", strings.Contains(room, "what if the press dies here?"), true)
	testutil.Expect(t, "and none of the drawer's", strings.Contains(room, "which goals wait for me?"), false)
	testutil.Expect(t, "nor the drawer's last question", strings.Contains(room, "and after that?"), false)
}

// S65-06: a turn asked on B while A's turn runs is not refused as busy. The
// service stops A through its own Stop, A's answer is written down as stopped
// on A's transcript, and only then is B's session opened.
func TestATurnOnAnotherConversationStopsTheRunningOneFirst(t *testing.T) {
	t.Parallel()
	held := reviewService(t, fakeacp.Script{
		Chunks: []string{"Asked: the lock ", "covers publish ", "and reconcile."},
		// Only the review's opening pauses, and it pauses for longer than
		// any test runs: it ends when it is stopped and in no other way.
		Pause: time.Hour, PauseOnly: "Open this review",
	})
	events, stop := held.service.Subscribe()
	defer stop()
	ctx := context.Background()

	_, err := held.service.Sit(ctx, "Wido", reviewOf(reviewA), partner.PurposeReview, inTheRoom(reviewA))
	testutil.Require(t, "the review opened", err, nil)
	waitFor(t, events, partner.EventText)
	seenAtOpen := make(chan string, 1)
	held.mu.Lock()
	held.onOpen = func() { seenAtOpen <- lastOutcome(t, held.service, reviewA) }
	held.mu.Unlock()

	_, err = held.service.Submit(ctx, "Wido", "k1", "the board?", partner.Page{})
	testutil.Require(t, "the drawer's question is admitted, not refused as busy", err, nil)

	testutil.Expect(t, "A was written down as stopped before B's session opened", <-seenAtOpen, partner.OutcomeStopped)
	testutil.Expect(t, "A's transcript ends with the stopped answer", lastOutcome(t, held.service, reviewA),
		partner.OutcomeStopped)
	drain(t, events)
	drain(t, events)
	testutil.Expect(t, "B's own turn completed", lastOutcome(t, held.service, ""), partner.OutcomeComplete)
	testutil.Expect(t, "and B's prompt carries none of A's words",
		strings.Contains(prompt(t, held.watched, 1), "Open this review"), false)
}

// S65-06: where A's turn does not settle within the wait, the switch is refused
// in words and nothing of B starts.
func TestAnUnsettledTurnRefusesTheSwitchInWords(t *testing.T) {
	t.Parallel()
	hold := make(chan struct{})
	prompted := make(chan string, 8)
	held := reviewService(t, fakeacp.Script{Chunks: []string{"An answer."}, Hold: hold, Prompted: prompted})
	partner.SettleWithin(held.service, 0)
	events, stop := held.service.Subscribe()
	defer stop()
	ctx := context.Background()

	_, err := held.service.Sit(ctx, "Wido", reviewOf(reviewA), partner.PurposeReview, inTheRoom(reviewA))
	testutil.Require(t, "the review opened", err, nil)
	// A's answer is in flight at the runtime, which will not settle it.
	<-prompted

	_, err = held.service.Submit(ctx, "Wido", "k1", "the board?", partner.Page{})
	testutil.Expect(t, "the switch is refused in words", err, partner.ErrUnsettled)
	testutil.Expect(t, "the words", err.Error(), "the previous room's answer has not settled; try again in a moment")
	ordinary, readErr := held.service.Snapshot("Wido", 100)
	testutil.Require(t, "the drawer's conversation", readErr, nil)
	testutil.Expect(t, "nothing of B was written", len(ordinary.Messages), 0)
	testutil.Expect(t, "no session of B was opened", held.sessions.Load(), int32(1))

	close(hold)
	drain(t, events)
	testutil.Expect(t, "A settles as stopped when it can", lastOutcome(t, held.service, reviewA), partner.OutcomeStopped)
	_, err = held.service.Submit(ctx, "Wido", "k1", "the board?", partner.Page{})
	testutil.Require(t, "the switch is admitted once A has settled", err, nil)
	drain(t, events)
}

// Sol SOL-A-06: Stop answers whether the turn settled. A runtime that holds its
// answer past the settle wait is ErrUnsettled, in the design's words, so Step
// out can stay in the room; once it settles, Stop is no refusal.
func TestStopAnswersWhetherTheTurnSettled(t *testing.T) {
	t.Parallel()
	hold := make(chan struct{})
	prompted := make(chan string, 8)
	held := reviewService(t, fakeacp.Script{Chunks: []string{"An answer."}, Hold: hold, Prompted: prompted})
	partner.SettleWithin(held.service, 0)
	events, stop := held.service.Subscribe()
	defer stop()
	ctx := context.Background()

	_, err := held.service.Sit(ctx, "Wido", reviewOf(reviewA), partner.PurposeReview, inTheRoom(reviewA))
	testutil.Require(t, "the review opened", err, nil)
	<-prompted
	read, err := held.service.SnapshotIn("Wido", reviewA, 100)
	testutil.Require(t, "the room's conversation", err, nil)
	testutil.Require(t, "an answer is running", read.Turn != "", true)

	err = held.service.Stop(ctx, read.Turn)
	testutil.Expect(t, "an unsettled stop says so", err, partner.ErrUnsettled)

	close(hold)
	drain(t, events)
	testutil.Expect(t, "it settles as stopped when it can", lastOutcome(t, held.service, reviewA), partner.OutcomeStopped)
	testutil.Expect(t, "a stop of a settled turn is no refusal", held.service.Stop(ctx, read.Turn), nil)
}

// D3: the review's session is fresh, and it stays so on recovery. Neither the
// ordinary conversation's history nor the proposals block composed from its
// transcript reaches a review's prompt.
func TestAReviewSessionCarriesNothingOfTheOrdinaryConversation(t *testing.T) {
	t.Parallel()
	held := reviewService(t, fakeacp.Script{Chunks: []string{"An answer."}})
	ordinary, err := partner.OpenConversation(held.root, "Wido")
	testutil.Require(t, "the human's own conversation", err, nil)
	at := "2026-09-28T08:00:00Z"
	testutil.Require(t, "a question", ordinary.Append(partner.Message{ID: "q", Turn: "t0", Role: partner.RoleHuman,
		Text: "the ordinary question", At: at}), nil)
	testutil.Require(t, "an answer that proposed an act", ordinary.Append(partner.Message{ID: "a", Turn: "t0",
		Role: partner.RolePartner, Outcome: partner.OutcomeComplete, Text: "the ordinary answer", At: at,
		Proposals: []partner.Proposal{{Index: 0, Verb: "park-goal", Goal: "g1-s44", Offered: true, Title: "Pause g1-s44",
			State: partner.ProposalWaiting}}}), nil)
	events, stop := held.service.Subscribe()
	defer stop()
	ctx := context.Background()

	_, err = held.service.Sit(ctx, "Wido", reviewOf(reviewA), partner.PurposeReview, inTheRoom(reviewA))
	testutil.Require(t, "the review opened", err, nil)
	drain(t, events)
	opening := prompt(t, held.watched, 0)
	testutil.Expect(t, "the boot context", strings.Contains(opening, "Project Partner"), true)
	testutil.Expect(t, "the review brief", strings.Contains(opening, "Open this review"), true)
	testutil.Expect(t, "the review record", strings.Contains(opening, reviewA), true)
	testutil.Expect(t, "no earlier turn", strings.Contains(opening, "the ordinary question"), false)
	testutil.Expect(t, "no proposals block", strings.Contains(opening, "What happened to the actions you proposed"), false)

	held.service.Close()
	_, err = held.service.SubmitIn(ctx, "Wido", reviewA, "k1", "and the reconcile?", inTheRoom(reviewA))
	testutil.Require(t, "a question after the process was lost", err, nil)
	drain(t, events)
	recovered := prompt(t, held.watched, 1)
	testutil.Expect(t, "the review's own history is replayed", strings.Contains(recovered, "Conversation so far"), true)
	testutil.Expect(t, "still no earlier turn of the drawer's", strings.Contains(recovered, "the ordinary question"), false)
	testutil.Expect(t, "still no proposals block", strings.Contains(recovered, "What happened to the actions you proposed"), false)
}

// D6: the five walks are fixed requests the interface submits with its own
// provenance, and a walk outside a review, or one the room does not have, is
// refused.
func TestTheWalksAreFixedRequestsWithTheInterfacesProvenance(t *testing.T) {
	t.Parallel()
	held := reviewService(t, fakeacp.Script{Chunks: []string{"Built: ..."}})
	events, stop := held.service.Subscribe()
	defer stop()
	ctx := context.Background()
	sitting, err := held.service.Sit(ctx, "Wido", reviewOf(reviewA), partner.PurposeReview, inTheRoom(reviewA))
	testutil.Require(t, "the review opened", err, nil)
	drain(t, events)

	testutil.Expect(t, "the five", partner.Walks[partner.PurposeReview], []string{"asked", "built", "examined", "proven", "behaves"})
	_, err = held.service.Walk(ctx, "Wido", reviewA, "built", inTheRoom(reviewA))
	testutil.Require(t, "the Built walk", err, nil)
	drain(t, events)
	read, err := held.service.SnapshotIn("Wido", reviewA, 100)
	testutil.Require(t, "read back", err, nil)
	asked := read.Messages[len(read.Messages)-2]
	testutil.Expect(t, "the interface's own question", asked.Interface, true)
	testutil.Expect(t, "its fixed words", asked.Text, partner.WalkRequest("built", sitting))
	testutil.Expect(t, "which names the part", strings.Contains(asked.Text, "Built"), true)

	_, err = held.service.Walk(ctx, "Wido", reviewA, "gossip", inTheRoom(reviewA))
	testutil.Expect(t, "a walk the room has not", err.Error(),
		"a walk of this sitting is one of asked, built, examined, proven, behaves; gossip is none of them")
	_, err = held.service.Walk(ctx, "Wido", reviewB, "built", inTheRoom(reviewB))
	testutil.Expect(t, "a walk where no sitting stands", err.Error(), "no sitting is open on "+reviewB+", so there is nothing to walk through")
}

// D9: the room's working state is written through the sitting and read back
// with it; the Sittings list names every standing sitting of this human.
func TestTheRoomIsKeptAndEveryStandingSittingIsListed(t *testing.T) {
	t.Parallel()
	held := reviewService(t, fakeacp.Script{Chunks: []string{"An answer."}})
	events, stop := held.service.Subscribe()
	defer stop()
	ctx := context.Background()
	for _, record := range []string{reviewA, reviewB} {
		_, err := held.service.Sit(ctx, "Wido", reviewOf(record), partner.PurposeReview, inTheRoom(record))
		testutil.Require(t, "the review of "+record, err, nil)
		drain(t, events)
	}

	room := partner.Room{Desk: []byte(`{"items":[],"current":-1}`), Face: "board",
		Drafts: []byte(`{"deposit:t#0":{"text":"half a finding"}}`)}
	testutil.Require(t, "keep A's room", held.service.KeepRoom("Wido", reviewA, room), nil)
	read, err := held.service.SnapshotIn("Wido", reviewA, 1)
	testutil.Require(t, "read A", err, nil)
	testutil.Expect(t, "A's face", read.Sitting.Room.Face, "board")
	testutil.Expect(t, "A's drafts", string(read.Sitting.Room.Drafts), string(room.Drafts))
	other, err := held.service.SnapshotIn("Wido", reviewB, 1)
	testutil.Require(t, "read B", err, nil)
	testutil.Expect(t, "B keeps none of A's", other.Sitting.Room == nil, true)
	testutil.Expect(t, "no room where nothing stands", held.service.KeepRoom("Wido", design, room).Error(),
		"no sitting is open on this conversation, so there is no room to keep")

	standing, err := held.service.Standing("Wido")
	testutil.Require(t, "the standing sittings", err, nil)
	named := []string{}
	for _, one := range standing {
		named = append(named, one.Subject.ID)
	}
	testutil.Expect(t, "both reviews, by record", strings.Join(named, " "), reviewB+" "+reviewA)
}

// D8 and D5 at the service: a finding is offered only to a review, and says why
// in words elsewhere; a desk item reaches every sitting's room (g1-s67 D4).
func TestAFindingIsAReviewsOwnAndADeskItemEverySittings(t *testing.T) {
	t.Parallel()
	script := fakeacp.Script{
		Reads: []fakeacp.Read{
			{Name: "mcp__metasystem__deposit", Title: "deposit(finding)",
				Result: "prepared\nDeposit: finding\nAnchor: internal/owner.go:60\n" +
					"Consequence: the lock stays held\n--- the deposit follows, whole and to the end ---\n" +
					"nothing covers a press that dies here\n"},
			{Name: "mcp__metasystem__present", Title: "present(source)",
				Result: "prepared for the desk\nDesk: source\nPath: internal/owner.go\nFrom: 41\nTo: 88\n"},
		},
		Chunks: []string{"Asked: ..."},
	}
	held := reviewService(t, script)
	events, stop := held.service.Subscribe()
	defer stop()
	ctx := context.Background()

	_, err := held.service.Sit(ctx, "Wido", reviewOf(reviewA), partner.PurposeReview, inTheRoom(reviewA))
	testutil.Require(t, "the review opened", err, nil)
	beats := drain(t, events)
	var finding *partner.Deposit
	var present *partner.Present
	for at, beat := range beats {
		testutil.Expect(t, fmt.Sprintf("beat %d names its conversation", at), beat.Conversation, reviewA)
		if beat.Deposit != nil {
			finding = beat.Deposit
		}
		if beat.Present != nil {
			present = beat.Present
		}
	}
	testutil.Require(t, "a finding arrived", finding != nil, true)
	testutil.Expect(t, "offered", finding.Offered, true)
	testutil.Expect(t, "with its anchor", finding.Anchor, "internal/owner.go:60")
	testutil.Expect(t, "and its consequence", finding.Consequence, "the lock stays held")
	testutil.Require(t, "a desk item arrived", present != nil, true)
	testutil.Expect(t, "the file at its range", *present,
		partner.Present{Kind: "source", Path: "internal/owner.go", From: 41, To: 88})

	_, err = held.service.Sit(ctx, "Wido", reviewOf(design), partner.PurposeShapeDesign, inTheRoom(design))
	testutil.Require(t, "a design sitting opened", err, nil)
	beats = drain(t, events)
	var refused *partner.Deposit
	presented := false
	activity := ""
	for _, beat := range beats {
		if beat.Deposit != nil {
			refused = beat.Deposit
		}
		presented = presented || beat.Present != nil
		if beat.Kind == partner.EventActivity {
			activity = beat.Text
		}
	}
	testutil.Require(t, "the finding arrived there too", refused != nil, true)
	testutil.Expect(t, "not offered", refused.Offered, false)
	testutil.Expect(t, "saying why", refused.NotOffered,
		"a finding is offered in a review sitting, and this sitting shapes a record; say it as a fact or an open question instead")
	// g1-s67 D4: every sitting has a room with a desk, a shaping one included.
	testutil.Expect(t, "a desk item in a shaping room too", presented, true)
	testutil.Expect(t, "and no activity saying otherwise", activity, "")
}
