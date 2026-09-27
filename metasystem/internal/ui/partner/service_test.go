package partner_test

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/partner"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/partner/fakeacp"
)

// One turn is admitted, streams its beats, and lands in the transcript with a
// terminal outcome, which is what a reload reads.
func TestATurnIsAdmittedStreamedAndWrittenDown(t *testing.T) {
	t.Parallel()
	service, _ := serviceOn(t, fakeacp.Script{
		Activity: "Read plans/goals/backlog.md", Chunks: []string{"Two goals ", "are ready."},
	})
	events, stop := service.Subscribe()
	defer stop()

	turn, err := service.Submit(context.Background(), "Wido", "key-1", "which goals are ready?",
		partner.Page{Section: "Backlog", Path: "/backlog"})
	testutil.Require(t, "admitted", err, nil)
	testutil.Expect(t, "with a turn", turn != "", true)

	beats := collect(t, events, partner.EventDone)
	// The page the human was looking at is the first thing the answer was read
	// from, and it is a beat of its own before anything the Partner chose.
	testutil.Expect(t, "the page is looked at first", beats[0].Kind, partner.EventLook)
	testutil.Expect(t, "then what it is doing", beats[1].Kind, partner.EventDoing)
	testutil.Expect(t, "then the text", beats[2].Kind, partner.EventText)
	testutil.Expect(t, "the sequence starts at one", beats[0].Seq, 1)
	testutil.Expect(t, "and counts up", beats[1].Seq, 2)
	testutil.Expect(t, "every beat names its turn", beats[0].Turn, turn)

	snapshot, err := service.Snapshot("Wido", 100)
	testutil.Require(t, "read back", err, nil)
	testutil.Expect(t, "nothing is running", snapshot.Busy, false)
	testutil.Require(t, "two messages", len(snapshot.Messages), 2)
	testutil.Expect(t, "the human's question", snapshot.Messages[0].Text, "which goals are ready?")
	testutil.Expect(t, "with its page context", snapshot.Messages[0].Page.Section, "Backlog")
	testutil.Expect(t, "the answer", snapshot.Messages[1].Text, "Two goals are ready.")
	testutil.Expect(t, "its outcome", snapshot.Messages[1].Outcome, partner.OutcomeComplete)
	// A tool call is a thing it was doing, not a line the answer keeps: what it
	// read is the looked list, and saying it twice would be saying it twice.
	testutil.Expect(t, "the answer keeps no activity of its own", len(snapshot.Messages[1].Activity), 0)
}

// A capture is held to its bounds at this boundary, before the transcript
// keeps it.
//
// The human's notepad lives outside every checkout so that no seat reads it. A
// message, though, is written INSIDE the checkout's state root and kept — so a
// capture that arrived carrying forty private reminders and was written down
// whole would put the notepad in the one place it exists to stay out of, and
// the block's own bound, which is applied later over what is already stored,
// would not have stopped it. The page caps what it sends; this is the check
// that does not take the page's word for it.
func TestACaptureIsBoundedBeforeTheTranscriptKeepsIt(t *testing.T) {
	t.Parallel()
	service, _ := serviceOn(t, fakeacp.Script{Chunks: []string{"noted"}})
	events, stop := service.Subscribe()
	defer stop()

	many := make([]partner.Sticky, 0, 40)
	for count := 0; count < 40; count++ {
		many = append(many, partner.Sticky{Text: "one of forty"})
	}
	sent := partner.Page{Section: "Fleet", Path: "/fleet", Sheet: "Stickies",
		StickiesOpen: 40, Stickies: many}

	_, err := service.Submit(context.Background(), "Wido", "key-1", "what did I want to remember?", sent)
	testutil.Require(t, "admitted", err, nil)
	collect(t, events, partner.EventDone)

	snapshot, err := service.Snapshot("Wido", 100)
	testutil.Require(t, "read back", err, nil)
	testutil.Require(t, "two messages", len(snapshot.Messages), 2)
	kept := snapshot.Messages[0].Page
	testutil.Require(t, "the question kept its page", kept != nil, true)
	testutil.Expect(t, "how many stickies the transcript kept", len(kept.Stickies), 25)
	testutil.Expect(t, "that it says how many it cut", kept.StickiesCut, 15)
	testutil.Expect(t, "that the open count is untouched", kept.StickiesOpen, 40)
	// And the sheet composes the same bound, through the same method, so what a
	// human is shown before asking is what the question would carry.
	testutil.Expect(t, "what the sheet says is missing",
		strings.Contains(service.See(sent, composedAtService).Block,
			"15 more the page was showing are not in this block"), true)
}

// composedAtService is when the capture above is composed. It is a fixed
// instant: nothing here waits for a clock.
var composedAtService = time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC)

// The same key twice is the same turn once, so a retry after a lost answer
// never asks the Partner twice.
func TestTheSameKeyIsTheSameTurn(t *testing.T) {
	t.Parallel()
	service, _ := serviceOn(t, fakeacp.Script{Chunks: []string{"once"}})
	events, stop := service.Subscribe()
	defer stop()
	first, err := service.Submit(context.Background(), "Wido", "key-1", "hello", partner.Page{})
	testutil.Require(t, "admitted", err, nil)
	collect(t, events, partner.EventDone)
	again, err := service.Submit(context.Background(), "Wido", "key-1", "hello", partner.Page{})
	testutil.Require(t, "admitted again", err, nil)
	testutil.Expect(t, "the same turn", again, first)
	snapshot, _ := service.Snapshot("Wido", 100)
	testutil.Expect(t, "and nothing was asked twice", len(snapshot.Messages), 2)
}

// A second send while a turn runs is refused, and the page keeps its draft.
func TestASecondSendWhileBusyIsRefused(t *testing.T) {
	t.Parallel()
	service, _ := serviceOn(t, fakeacp.Script{
		Chunks: []string{"a", "b", "c", "d"}, Pause: 200 * time.Millisecond})
	events, stop := service.Subscribe()
	defer stop()
	_, err := service.Submit(context.Background(), "Wido", "key-1", "first", partner.Page{})
	testutil.Require(t, "admitted", err, nil)
	waitFor(t, events, partner.EventText)
	_, second := service.Submit(context.Background(), "Wido", "key-2", "second", partner.Page{})
	testutil.Expect(t, "refused as busy", errors.Is(second, partner.ErrBusy), true)
	// The first turn is let finish, so the transcript is written while this
	// test's directory still exists.
	collect(t, events, partner.EventDone, partner.EventStopped)
}

// A reload mid-answer reads what the turn has said so far, and that it is
// still running, so the page never has to guess.
func TestAReloadMidAnswerReadsThePartialTextAndTheBusyState(t *testing.T) {
	t.Parallel()
	service, _ := serviceOn(t, fakeacp.Script{
		Chunks: []string{"half ", "an ", "answer"}, Pause: 150 * time.Millisecond})
	events, stop := service.Subscribe()
	defer stop()
	turn, err := service.Submit(context.Background(), "Wido", "key-1", "tell me", partner.Page{})
	testutil.Require(t, "admitted", err, nil)
	waitFor(t, events, partner.EventText)
	snapshot, err := service.Snapshot("Wido", 100)
	testutil.Require(t, "read back", err, nil)
	testutil.Expect(t, "it is busy", snapshot.Busy, true)
	testutil.Expect(t, "on this turn", snapshot.Turn, turn)
	testutil.Expect(t, "with what it has said", strings.HasPrefix(snapshot.Partial, "half"), true)
	testutil.Expect(t, "and where to join", snapshot.PartialSeq >= 1, true)
	collect(t, events, partner.EventStopped, partner.EventDone)
}

// Stop ends the turn, keeps the partial text, and marks it stopped.
func TestStopKeepsThePartialTextAndMarksTheTurnStopped(t *testing.T) {
	t.Parallel()
	service, _ := serviceOn(t, fakeacp.Script{
		Chunks: []string{"one ", "two ", "three ", "four"}, Pause: 200 * time.Millisecond})
	events, stop := service.Subscribe()
	defer stop()
	turn, err := service.Submit(context.Background(), "Wido", "key-1", "a long one", partner.Page{})
	testutil.Require(t, "admitted", err, nil)
	waitFor(t, events, partner.EventText)
	testutil.Require(t, "stopped", service.Stop(context.Background(), turn), nil)
	last := collect(t, events, partner.EventStopped, partner.EventDone)
	testutil.Expect(t, "the stream says stopped", last[len(last)-1].Kind, partner.EventStopped)
	snapshot, _ := service.Snapshot("Wido", 100)
	testutil.Expect(t, "the transcript says stopped", snapshot.Messages[1].Outcome, partner.OutcomeStopped)
	testutil.Expect(t, "and keeps what was said", strings.HasPrefix(snapshot.Messages[1].Text, "one"), true)
}

// A runtime that cannot start refuses the send, with its own words and the
// install line, and writes nothing down: the question stays in the composer.
func TestARuntimeThatCannotStartRefusesTheSend(t *testing.T) {
	t.Parallel()
	service, _ := serviceOn(t, fakeacp.Script{InitError: "Authentication required"})
	_, err := service.Submit(context.Background(), "Wido", "key-1", "hello", partner.Page{})
	testutil.Require(t, "refused", err != nil, true)
	var start *partner.StartError
	testutil.Require(t, "as a start refusal", errors.As(err, &start), true)
	snapshot, _ := service.Snapshot("Wido", 100)
	testutil.Expect(t, "nothing was written down", len(snapshot.Messages), 0)
}

// After a process loss the next turn opens a fresh session, gives it the last
// messages, and says in an activity line how many it gave.
func TestAFreshSessionIsGivenTheHistoryAndSaysHowMuch(t *testing.T) {
	t.Parallel()
	service, host := serviceOn(t, fakeacp.Script{Chunks: []string{"yes"}})
	events, stop := service.Subscribe()
	defer stop()
	_, err := service.Submit(context.Background(), "Wido", "key-1", "first", partner.Page{Section: "Backlog"})
	testutil.Require(t, "admitted", err, nil)
	collect(t, events, partner.EventDone)

	// The process dies: the next turn must open a fresh session.
	host.Close()
	_, err = service.Submit(context.Background(), "Wido", "key-2", "and now?", partner.Page{Section: "Backlog"})
	testutil.Require(t, "admitted again", err, nil)
	beats := collect(t, events, partner.EventDone)
	testutil.Expect(t, "the page is looked at first", beats[0].Kind, partner.EventLook)
	testutil.Expect(t, "then the activity line", beats[1].Kind, partner.EventActivity)
	testutil.Expect(t, "it says a fresh session was opened",
		strings.Contains(beats[1].Text, "a fresh one was opened and given the last 2 messages"), true)
}

// An empty question is not a turn.
func TestAnEmptyQuestionIsRefused(t *testing.T) {
	t.Parallel()
	service, _ := serviceOn(t, fakeacp.Script{})
	_, err := service.Submit(context.Background(), "Wido", "key-1", "   ", partner.Page{})
	testutil.Require(t, "refused", err != nil, true)
}

// Two humans on one seat keep two transcripts, which is what the file name
// says they do.
func TestTwoHumansKeepTwoConversations(t *testing.T) {
	t.Parallel()
	service, _ := serviceOn(t, fakeacp.Script{Chunks: []string{"ok"}})
	events, stop := service.Subscribe()
	defer stop()
	_, err := service.Submit(context.Background(), "Wido", "key-1", "mine", partner.Page{})
	testutil.Require(t, "admitted", err, nil)
	collect(t, events, partner.EventDone)
	_, err = service.Submit(context.Background(), "Ada", "key-2", "and mine", partner.Page{})
	testutil.Require(t, "admitted for the other", err, nil)
	collect(t, events, partner.EventDone)

	wido, _ := service.Snapshot("Wido", 100)
	ada, _ := service.Snapshot("Ada", 100)
	testutil.Expect(t, "one each", len(wido.Messages), 2)
	testutil.Expect(t, "and one each again", len(ada.Messages), 2)
	testutil.Expect(t, "the first human's question", wido.Messages[0].Text, "mine")
	testutil.Expect(t, "the second human's question", ada.Messages[0].Text, "and mine")
}

// A human who already has a transcript takes the unnamed seat's messages after
// their own, and the seat is left with none.
//
// The route test in internal/ui/httpd proves the first sign-in on a seat that
// has never been named; this is the other half of the same move, and it is
// proved on the files rather than in memory, because what a later run reads is
// the file (Astra A-03).
func TestTheSeatsMessagesMoveToTheHumanAndLeaveTheSeatEmpty(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	host := partner.NewHostOn(
		partner.Runtime{Name: "fake", ReadOnly: "a fake server reads nothing"},
		root, fakeacp.Open(fakeacp.Script{Chunks: []string{"noted"}}))
	t.Cleanup(host.Close)
	service := partner.NewService(host.Runtime(), host,
		func(human string) (*partner.Conversation, error) { return partner.OpenConversation(root, human) },
		partner.Facts{}, func() time.Time { return time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC) })
	ask := func(human, key, question string) {
		t.Helper()
		events, stop := service.Subscribe()
		defer stop()
		_, err := service.Submit(context.Background(), human, key, question,
			partner.Page{Section: "Backlog", Path: "/backlog"})
		testutil.Require(t, "the turn on "+key+" is admitted", err, nil)
		collect(t, events, partner.EventDone)
	}
	ask("Wido", "key-1", "which goals are ready?")
	ask("seat", "key-2", "and what about this one?")

	testutil.Require(t, "the seat's conversation is handed over", service.Adopt("seat", "Wido"), nil)

	mine, err := partner.OpenConversation(root, "Wido")
	testutil.Require(t, "Wido's transcript reads back from the file", err, nil)
	kept := mine.Messages(0)
	testutil.Require(t, "four messages", len(kept), 4)
	testutil.Expect(t, "their own question first", kept[0].Text, "which goals are ready?")
	testutil.Expect(t, "and the seat's after it", kept[2].Text, "and what about this one?")
	seat, err := partner.OpenConversation(root, "seat")
	testutil.Require(t, "the seat's transcript reads back too", err, nil)
	testutil.Expect(t, "with nothing left in it", len(seat.Messages(0)), 0)
}

// A sitting the human opened before signing in moves with the messages.
//
// The mark is not on the messages: it is in the state file beside them, which
// is why a move that carried the transcript alone left it behind. A human who
// started a sitting from a record's page and then signed in would find their
// own conversation sitting on nothing — every deposit refused for want of a
// sitting — while the emptied seat went on claiming the record (Astra A-03).
func TestTheSeatsSittingMovesWithItsMessages(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	host := partner.NewHostOn(
		partner.Runtime{Name: "fake", ReadOnly: "a fake server reads nothing"},
		root, fakeacp.Open(fakeacp.Script{Chunks: []string{"here is what the record holds"}}))
	t.Cleanup(host.Close)
	service := partner.NewService(host.Runtime(), host,
		func(human string) (*partner.Conversation, error) { return partner.OpenConversation(root, human) },
		partner.Facts{}, func() time.Time { return time.Date(2026, 9, 27, 9, 0, 0, 0, time.UTC) })
	events, stop := service.Subscribe()
	defer stop()

	// The seat is who the human is until they sign in, and Start is on the
	// record's page before that as much as after it.
	_, err := service.Sit(context.Background(), "seat", subjectOf(), partner.PurposeShapeDesign, onTheRecord())
	testutil.Require(t, "the sitting opened on the seat", err, nil)
	drain(t, events)

	testutil.Require(t, "the seat's conversation is handed over", service.Adopt("seat", "Wido"), nil)

	read, err := service.Snapshot("Wido", 100)
	testutil.Require(t, "Wido's conversation reads back", err, nil)
	testutil.Require(t, "the sitting is theirs now", read.Sitting != nil, true)
	testutil.Expect(t, "on the record it was opened on", read.Sitting.Subject.ID, subjectOf().ID)
	left, err := service.Snapshot("seat", 100)
	testutil.Require(t, "the seat reads back too", err, nil)
	testutil.Expect(t, "and the seat sits on nothing", left.Sitting == nil, true)

	// And on the files, because what a later run of this seat reads is the file.
	mine, err := partner.OpenConversation(root, "Wido")
	testutil.Require(t, "Wido's state file reads back", err, nil)
	testutil.Expect(t, "the mark is in it", mine.Sitting() != nil, true)
	seat, err := partner.OpenConversation(root, "seat")
	testutil.Require(t, "the seat's state file reads back", err, nil)
	testutil.Expect(t, "and gone from the seat's", seat.Sitting() == nil, true)
}

// A first sign-in while the Partner is answering lands that answer in the
// human's conversation, and not in the seat the move has just emptied.
//
// It is the sequence the sign-in sheet itself invites: the human asks
// something, the Partner starts answering, and they sign in while it does.
// Adopt moves what is written down, and the running turn's answer is not
// written down yet — so the turn has to move with it, or the words and the
// actions they propose arrive in a transcript nobody reads again (Astra A-03).
func TestASignInDuringAnAnswerLandsItInTheHumansConversation(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	release := make(chan struct{})
	opens := fakeacp.Open(fakeacp.Script{Chunks: []string{"half an answer"}})
	host := partner.NewHostOn(
		partner.Runtime{Name: "fake", ReadOnly: "a fake server reads nothing"}, root,
		func(ctx context.Context) (partner.Endpoint, error) {
			endpoint, err := opens(ctx)
			if err != nil {
				return endpoint, err
			}
			// The turn is held after the chunk the page has just been shown, so
			// the sign-in below happens inside the answer rather than after it.
			endpoint.Reader = holdAfter(endpoint.Reader, "agent_message_chunk", release)
			return endpoint, nil
		})
	t.Cleanup(host.Close)
	service := partner.NewService(host.Runtime(), host,
		func(human string) (*partner.Conversation, error) { return partner.OpenConversation(root, human) },
		partner.Facts{}, func() time.Time { return time.Date(2026, 9, 27, 9, 0, 0, 0, time.UTC) })
	events, stop := service.Subscribe()
	defer stop()

	turn, err := service.Submit(context.Background(), "seat", "key-1", "what is this goal?", partner.Page{})
	testutil.Require(t, "the turn is admitted before anybody is named", err, nil)
	waitFor(t, events, partner.EventText)

	testutil.Require(t, "the seat's conversation is handed over mid-answer",
		service.Adopt("seat", "Wido"), nil)

	// The page is the human's own now, and the turn it is watching is the one
	// still arriving.
	mid, err := service.Snapshot("Wido", 100)
	testutil.Require(t, "Wido's conversation reads back", err, nil)
	testutil.Expect(t, "the turn is running for them", mid.Busy, true)
	testutil.Expect(t, "it is the turn that was asked", mid.Turn, turn)
	testutil.Expect(t, "with what has been said so far", mid.Partial, "half an answer")

	close(release)
	collect(t, events, partner.EventDone)

	kept, err := service.Snapshot("Wido", 100)
	testutil.Require(t, "Wido's conversation reads back again", err, nil)
	testutil.Require(t, "the question and the answer", len(kept.Messages), 2)
	testutil.Expect(t, "the question is theirs", kept.Messages[0].Text, "what is this goal?")
	testutil.Expect(t, "and so is the answer", kept.Messages[1].Text, "half an answer")
	// On the files too: what a later run reads is the file, and everything the
	// answer carries — the actions it proposed above all — rides on that line.
	seat, err := partner.OpenConversation(root, "seat")
	testutil.Require(t, "the seat's transcript reads back", err, nil)
	testutil.Expect(t, "with nothing left on the seat", len(seat.Messages(0)), 0)
}

// holdAfter is the fake runtime's stream, stopped after the line carrying what
// is named until the test lets it go.
//
// It is how a turn is held in the middle of its answer without a clock: the
// chunk reaches the page, and the frame that would settle the turn waits on a
// channel. A pause between chunks would be a race dressed up as a duration.
func holdAfter(reader io.Reader, after string, release <-chan struct{}) io.Reader {
	return &heldStream{lines: bufio.NewReader(reader), after: after, release: release}
}

type heldStream struct {
	lines   *bufio.Reader
	after   string
	release <-chan struct{}
	holding bool
	rest    []byte
}

// Read answers one line at a time, waiting before the line that follows the
// one that tripped it. A reader that handed back more than a line at a time
// would hand back the settling frame with the chunk.
func (h *heldStream) Read(into []byte) (int, error) {
	if len(h.rest) == 0 {
		if h.holding {
			<-h.release
			h.holding = false
		}
		line, err := h.lines.ReadBytes('\n')
		if len(line) == 0 {
			return 0, err
		}
		h.holding = bytes.Contains(line, []byte(h.after))
		h.rest = line
	}
	written := copy(into, h.rest)
	h.rest = h.rest[written:]
	return written, nil
}

func serviceOn(t *testing.T, script fakeacp.Script) (*partner.Service, *partner.Host) {
	t.Helper()
	root := t.TempDir()
	host := partner.NewHostOn(
		partner.Runtime{Name: "fake", ReadOnly: "a fake server reads nothing"},
		root, fakeacp.Open(script))
	t.Cleanup(host.Close)
	service := partner.NewService(host.Runtime(), host,
		func(human string) (*partner.Conversation, error) {
			return partner.OpenConversation(root, human)
		},
		partner.Facts{}, func() time.Time { return time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC) })
	return service, host
}

// collect reads beats until one of the terminal kinds arrives, and answers
// everything it read.
func collect(t *testing.T, events <-chan partner.Event, terminal ...string) []partner.Event {
	t.Helper()
	var read []partner.Event
	for event := range events {
		read = append(read, event)
		for _, kind := range terminal {
			if event.Kind == kind {
				return read
			}
		}
	}
	t.Fatalf("the turn never ended; read %d beats", len(read))
	return read
}

// waitFor reads beats until one of the given kind arrives.
func waitFor(t *testing.T, events <-chan partner.Event, kind string) {
	t.Helper()
	for event := range events {
		if event.Kind == kind {
			return
		}
	}
	t.Fatalf("no %s beat arrived", kind)
}
