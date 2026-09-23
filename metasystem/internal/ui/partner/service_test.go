package partner_test

import (
	"context"
	"errors"
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
	testutil.Expect(t, "the activity line", beats[0].Kind, partner.EventActivity)
	testutil.Expect(t, "then the text", beats[1].Kind, partner.EventText)
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
	testutil.Expect(t, "and what it did", snapshot.Messages[1].Activity, []string{"Read plans/goals/backlog.md"})
}

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
	testutil.Expect(t, "the first beat is the activity line", beats[0].Kind, partner.EventActivity)
	testutil.Expect(t, "it says a fresh session was opened",
		strings.Contains(beats[0].Text, "a fresh one was opened and given the last 2 messages"), true)
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
	deadline := time.NewTimer(20 * time.Second)
	defer deadline.Stop()
	for {
		select {
		case event := <-events:
			read = append(read, event)
			for _, kind := range terminal {
				if event.Kind == kind {
					return read
				}
			}
		case <-deadline.C:
			t.Fatalf("the turn never ended; read %d beats", len(read))
			return read
		}
	}
}

// waitFor reads beats until one of the given kind arrives.
func waitFor(t *testing.T, events <-chan partner.Event, kind string) {
	t.Helper()
	deadline := time.NewTimer(20 * time.Second)
	defer deadline.Stop()
	for {
		select {
		case event := <-events:
			if event.Kind == kind {
				return
			}
		case <-deadline.C:
			t.Fatalf("no %s beat arrived", kind)
			return
		}
	}
}
