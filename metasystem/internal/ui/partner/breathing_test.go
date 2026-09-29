package partner_test

import (
	"context"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/partner"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/partner/fakeacp"
)

// A completed call retires its line (g1-s74 D1): the host says, as a doing
// beat, what is still in flight — the earliest started of the calls that
// remain, or nothing — so a read followed by a minute of thought shows
// thinking rather than the finished read.
func TestACompletedCallRetiresItsDoingLine(t *testing.T) {
	t.Parallel()
	alone := hostOn(t, fakeacp.Script{
		Reads:  []fakeacp.Read{{Title: "Read plans/goals/backlog.md", Result: "the backlog"}},
		Chunks: []string{"read it"},
	})
	testutil.Expect(t, "with no call left, the line is emptied", doingOf(t, alone, "alone"),
		[]string{"Read plans/goals/backlog.md", ""})

	beside := hostOn(t, fakeacp.Script{
		Activity: "Search the ledger",
		Reads:    []fakeacp.Read{{Title: "Read plans/goals/backlog.md", Result: "the backlog"}},
		Chunks:   []string{"still searching"},
	})
	testutil.Expect(t, "with a call left, the line names it", doingOf(t, beside, "beside"),
		[]string{"Search the ledger", "Read plans/goals/backlog.md", "Search the ledger"})
}

// A call an earlier turn left running is not what this turn is doing: a
// stopped answer's calls may never complete, and the next answer's line must
// not name them.
func TestACallAnEarlierTurnLeftRunningIsNotNamed(t *testing.T) {
	t.Parallel()
	host := hostOn(t, fakeacp.Script{
		Activity: "Search the ledger", ActivityWhen: "first",
		Reads:  []fakeacp.Read{{Title: "Read plans/goals/backlog.md", Result: "the backlog", When: "second"}},
		Chunks: []string{"answered"},
	})
	_, err := host.Ready(context.Background())
	testutil.Require(t, "ready", err, nil)
	runTurn(t, host, "first")
	doing := []string{}
	_, err = host.Prompt(context.Background(), "second", func(update partner.Update) {
		if update.Kind == partner.UpdateDoing {
			doing = append(doing, update.Text)
		}
	})
	testutil.Require(t, "prompted", err, nil)
	testutil.Expect(t, "the second turn names only its own calls", doing,
		[]string{"Read plans/goals/backlog.md", ""})
}

// The snapshot's doing follows the retirement, so a reload after a finished
// read shows thinking too. The answer pauses for an hour after its read, which
// is a turn standing in thought; the stop wakes it.
func TestTheSnapshotsDoingFollowsACompletedCall(t *testing.T) {
	t.Parallel()
	service, _ := serviceOn(t, fakeacp.Script{
		Reads:  []fakeacp.Read{{Title: "Read plans/goals/backlog.md", Result: "the backlog"}},
		Chunks: []string{"never", "reached"}, Pause: time.Hour,
	})
	events, stop := service.Subscribe()
	defer stop()
	turn, err := service.Submit(context.Background(), "Wido", "key-1", "what is ready?", partner.Page{})
	testutil.Require(t, "admitted", err, nil)
	// The line is retired before the call becomes a look, so by the time the
	// look is published the turn's own state already says it.
	for event := range events {
		if event.Kind == partner.EventLook && event.Look != nil && !event.Look.Page {
			break
		}
	}
	snapshot, err := service.Snapshot("Wido", 100)
	testutil.Require(t, "read", err, nil)
	testutil.Expect(t, "still running", snapshot.Busy, true)
	testutil.Expect(t, "and doing nothing named", snapshot.Doing, "")
	testutil.Require(t, "stopped", service.Stop(context.Background(), turn), nil)
	collect(t, events, partner.EventStopped)
}

// doingOf runs one turn and answers every doing beat it said, in order.
func doingOf(t *testing.T, host *partner.Host, which string) []string {
	t.Helper()
	_, err := host.Ready(context.Background())
	testutil.Require(t, which+" ready", err, nil)
	doing := []string{}
	_, err = host.Prompt(context.Background(), "go", func(update partner.Update) {
		if update.Kind == partner.UpdateDoing {
			doing = append(doing, update.Text)
		}
	})
	testutil.Require(t, which+" prompted", err, nil)
	return doing
}
