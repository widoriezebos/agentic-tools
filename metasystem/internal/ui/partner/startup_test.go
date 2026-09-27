package partner_test

// A sign-in while the runtime is still starting.
//
// Submit captures the conversation it is about to write into, and only then
// waits for the runtime to be ready — a process spawn, an initialize, a
// session/new. The turn was bound to that conversation AFTER the wait, so for
// the whole of it there was nothing for a sign-in to find: Adopt saw an empty
// seat with no running turn, decided there was nothing to adopt, and startup
// then bound the turn to the seat the human had just left. The question, the
// answer and every action that answer proposed stayed in the seat's own
// transcript, and the human's page showed an empty conversation (Astra A-03,
// second confirmation read).
//
// So the turn is reserved before the wait, and released again if the runtime
// refuses to start. The startup below is gated on a channel, so the sign-in
// lands inside that window by construction rather than by timing.

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/partner"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/partner/fakeacp"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/uitools"
)

// startupGate is a service whose runtime cannot start until the test lets it:
// `starting` closes when the first startup is under way, and it returns when
// `release` is closed. The opener's own error, where one is given, is what
// Ready answers with instead of opening at all.
type startupGate struct {
	service  *partner.Service
	starting chan struct{}
	release  chan struct{}
}

func startupGated(t *testing.T, refuse error, reads ...fakeacp.Read) *startupGate {
	t.Helper()
	root := t.TempDir()
	gate := &startupGate{starting: make(chan struct{}), release: make(chan struct{})}
	opener := fakeacp.Open(fakeacp.Script{
		Reads:  reads,
		Chunks: []string{"I have proposed them. ", "Read them and apply the ones you want."},
	})
	var once sync.Once
	host := partner.NewHostOn(
		partner.Runtime{Name: "fake", ReadOnly: "a fake server reads nothing"}, root,
		func(ctx context.Context) (partner.Endpoint, error) {
			once.Do(func() {
				close(gate.starting)
				<-gate.release
			})
			if refuse != nil {
				return partner.Endpoint{}, refuse
			}
			return opener(ctx)
		})
	t.Cleanup(host.Close)
	gate.service = partner.NewService(host.Runtime(), host,
		func(human string) (*partner.Conversation, error) { return partner.OpenConversation(root, human) },
		partner.Facts{Observe: proposingLedger},
		func() time.Time { return time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC) })
	return gate
}

func TestASignInWhileTheRuntimeIsStartingLandsTheWholeTurn(t *testing.T) {
	t.Parallel()
	gate := startupGated(t, nil, proposed(uitools.ProposePark, "fleet-presence",
		[]string{uitools.ProposalBecause + "superseded by the seat inventory (g1-s42)"},
		"the inventory answers what this one was for"))
	events, stop := gate.service.Subscribe()
	defer stop()

	admitted := make(chan error, 1)
	go func() {
		_, err := gate.service.Submit(context.Background(), "seat", "key-1",
			"put the presence goal away", onDecisions())
		admitted <- err
	}()

	// The turn is inside the startup wait: the seat's transcript holds nothing
	// yet, because the question has not been written down.
	<-gate.starting
	testutil.Require(t, "the sign-in", gate.service.Adopt("seat", "Wido"), nil)
	close(gate.release)

	testutil.Require(t, "the turn is admitted", <-admitted, nil)
	drain(t, events)

	mine, err := gate.service.Snapshot("Wido", 100)
	testutil.Require(t, "the human's conversation reads back", err, nil)
	testutil.Require(t, "it holds the question and the answer", len(mine.Messages), 2)
	testutil.Expect(t, "the question is theirs", mine.Messages[0].Text, "put the presence goal away")
	testutil.Expect(t, "and so is the answer",
		mine.Messages[1].Text, "I have proposed them. Read them and apply the ones you want.")
	testutil.Require(t, "the answer's proposals are theirs", len(mine.Messages[1].Proposals), 1)
	testutil.Expect(t, "the action it proposed", mine.Messages[1].Proposals[0].Goal, "fleet-presence")
	testutil.Expect(t, "offered to them", mine.Messages[1].Proposals[0].Offered, true)

	seat, err := gate.service.Snapshot("seat", 100)
	testutil.Require(t, "the seat's conversation reads back", err, nil)
	testutil.Expect(t, "and the seat kept nothing", len(seat.Messages), 0)
}

// A runtime that refuses to start releases the reservation: the send is refused
// with the runtime's own words, nothing is written down, and the next send is
// not told the Partner is busy.
func TestAStartupThatRefusesReleasesTheReservedTurn(t *testing.T) {
	t.Parallel()
	refused := errors.New("this runtime is not installed")
	gate := startupGated(t, refused)

	admitted := make(chan error, 1)
	go func() {
		_, err := gate.service.Submit(context.Background(), "Wido", "key-1", "hello", onDecisions())
		admitted <- err
	}()
	<-gate.starting
	close(gate.release)

	err := <-admitted
	if err == nil || !strings.Contains(err.Error(), refused.Error()) {
		t.Fatalf("the send answered %v, want the runtime's own words", err)
	}
	read, err := gate.service.Snapshot("Wido", 100)
	testutil.Require(t, "the conversation reads back", err, nil)
	testutil.Expect(t, "nothing was written down", len(read.Messages), 0)
	testutil.Expect(t, "and no turn is running", read.Busy, false)

	// The reservation is gone, so the next send is admitted rather than refused
	// as busy — which is what a stranded one would have answered forever.
	if _, again := gate.service.Submit(context.Background(), "Wido", "key-2", "hello again",
		onDecisions()); errors.Is(again, partner.ErrBusy) {
		t.Fatal("the refused startup left its reservation behind: the next send was refused as busy")
	}
}
