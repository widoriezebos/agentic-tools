package partner_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/partner"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/partner/fakeacp"
)

// The live line's clock has one origin, the server's (g1-s74 D4): the instant
// the service admitted the turn, which the question's answer and the snapshot
// both carry while the turn runs, and which the snapshot drops when it ends.
// The clock here moves a second on every reading, so a snapshot that stamped
// its own reading rather than the admit instant would say a later time.
func TestTheRunningTurnCarriesTheInstantItWasAdmitted(t *testing.T) {
	t.Parallel()
	hold := make(chan struct{})
	service := serviceTicking(t, fakeacp.Script{Chunks: []string{"done"}, Hold: hold})
	events, stop := service.Subscribe()
	defer stop()

	turn, err := service.Submit(context.Background(), "Wido", "key-1", "how long?", partner.Page{})
	testutil.Require(t, "admitted", err, nil)
	started := service.StartedAt(turn)
	testutil.Expect(t, "the admit instant", started, "2026-09-23T12:00:00Z")

	running, err := service.Snapshot("Wido", 100)
	testutil.Require(t, "read while running", err, nil)
	testutil.Expect(t, "running", running.Busy, true)
	testutil.Expect(t, "the snapshot carries the same instant", running.StartedAt, started)

	close(hold)
	collect(t, events, partner.EventDone)
	idle, err := service.Snapshot("Wido", 100)
	testutil.Require(t, "read when idle", err, nil)
	testutil.Expect(t, "idle carries no start", idle.StartedAt, "")
	testutil.Expect(t, "and a turn that ended has none", service.StartedAt(turn), "")
}

// serviceTicking is serviceOn with a clock that moves one second on every
// reading.
func serviceTicking(t *testing.T, script fakeacp.Script) *partner.Service {
	t.Helper()
	root := t.TempDir()
	host := partner.NewHostOn(partner.Runtime{Name: "fake"}, root, fakeacp.Open(script))
	t.Cleanup(host.Close)
	var mu sync.Mutex
	at := time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC)
	return partner.NewService(host.Runtime(), host,
		func(human string) (*partner.Conversation, error) {
			return partner.OpenConversation(root, human)
		},
		partner.Facts{}, func() time.Time {
			mu.Lock()
			defer mu.Unlock()
			now := at
			at = at.Add(time.Second)
			return now
		})
}
