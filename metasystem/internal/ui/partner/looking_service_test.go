package partner_test

import (
	"context"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/partner"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/partner/fakeacp"
)

// What an answer was read from is a list with outcomes, not a count. The page
// the human was looking at is the first entry; a read that failed is listed
// and never counted; a read that left rows behind is partial.
func TestWhatAnAnswerWasReadFromIsAListWithOutcomes(t *testing.T) {
	t.Parallel()
	service, _ := serviceWithReads(t)
	events, stop := service.Subscribe()
	defer stop()

	_, err := service.Submit(context.Background(), "Wido", "key-1", "what does the design leave out?",
		partner.Page{Section: "Backlog", Path: "/backlog", Label: "Backlog · board"})
	testutil.Require(t, "admitted", err, nil)
	drain(t, events)

	read, err := service.Snapshot("Wido", 100)
	testutil.Require(t, "read back", err, nil)
	testutil.Require(t, "two messages", len(read.Messages), 2)
	looked := read.Messages[1].Looked
	testutil.Require(t, "four entries", len(looked), 4)

	testutil.Expect(t, "the page is first", strings.HasPrefix(looked[0].What, "The page you were looking at"), true)
	testutil.Expect(t, "and is a read", looked[0].Outcome, partner.LookRead)

	testutil.Expect(t, "then the document", looked[1].What, "document(plans/designs/g1-s26.md)")
	testutil.Expect(t, "from the file as it stands", looked[1].Source, "plans/designs/g1-s26.md as it stands, revision r7")
	testutil.Expect(t, "read whole", looked[1].Outcome, partner.LookRead)
	testutil.Expect(t, "with the excerpt behind it", strings.Contains(looked[1].Excerpt, "What it leaves out"), true)

	testutil.Expect(t, "a read that left rows behind is partial", looked[2].Outcome, partner.LookPartial)
	testutil.Expect(t, "and names its own source",
		strings.HasPrefix(looked[2].Source, "the accepted tip abc123"), true)

	testutil.Expect(t, "a read that did not happen is a failure", looked[3].Outcome, partner.LookFailed)
	testutil.Expect(t, "listed with what failed",
		strings.Contains(looked[3].Excerpt, "no goal ghost"), true)

	counted := 0
	for _, look := range looked {
		if look.Counted() {
			counted++
		}
	}
	testutil.Expect(t, "and the count leaves the failure out", counted, 3)
}

// A look reaches the page as it happens, so a human watching the drawer sees
// the list grow rather than waiting for the answer.
func TestALookIsABeatOfItsOwn(t *testing.T) {
	t.Parallel()
	service, _ := serviceWithReads(t)
	events, stop := service.Subscribe()
	defer stop()
	_, err := service.Submit(context.Background(), "Wido", "key-1", "read something",
		partner.Page{Section: "Backlog", Path: "/backlog"})
	testutil.Require(t, "admitted", err, nil)
	beats := drain(t, events)
	looks := 0
	for at, beat := range beats {
		if beat.Kind != partner.EventLook {
			continue
		}
		looks++
		testutil.Expect(t, "look beat carries one "+strconv.Itoa(at), beat.Look != nil, true)
	}
	testutil.Expect(t, "four of them", looks, 4)
}

// End to end over the wire: the tool server is handed to the session, a
// permission request for one of its operations is admitted, and a request to
// write is refused exactly as before.
func TestTheToolServerIsHandedOverAndItsCallsAdmitted(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	opener, handed := fakeacp.OpenWatched(fakeacp.Script{
		AskFor:         "mcp__metasystem__board",
		Permission:     "Write plans/goals/waiting.md",
		PermissionKind: "edit",
		Chunks:         []string{"two goals are ready."},
	})
	runtime := partner.Runtime{
		Name: "fake", ReadOnly: "a fake server reads nothing",
		Tools: &partner.ToolServer{
			Name: "metasystem", Command: "/bin/metasystem",
			Args: []string{"ui", "tools", "--root", root},
		},
	}
	host := partner.NewHostOn(runtime, root, opener)
	t.Cleanup(host.Close)
	service := partner.NewService(runtime, host,
		func(human string) (*partner.Conversation, error) { return partner.OpenConversation(root, human) },
		partner.Facts{}, func() time.Time { return time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC) })

	events, stop := service.Subscribe()
	defer stop()
	_, err := service.Submit(context.Background(), "Wido", "key-1", "which goals are ready?",
		partner.Page{Section: "Backlog", Path: "/backlog"})
	testutil.Require(t, "admitted", err, nil)
	drain(t, events)

	testutil.Expect(t, "the session was handed the tool server", handed.Named(), []string{"metasystem"})

	read, err := service.Snapshot("Wido", 100)
	testutil.Require(t, "read back", err, nil)
	testutil.Require(t, "two messages", len(read.Messages), 2)
	activity := read.Messages[1].Activity
	testutil.Require(t, "two decisions", len(activity), 2)
	testutil.Expect(t, "the tool call is admitted by name", activity[0],
		"Allowed a read through the interface's own tools: board")
	testutil.Expect(t, "and the write is still refused",
		strings.HasPrefix(activity[1], "Refused: the Partner reads this checkout and does nothing else"), true)
	testutil.Expect(t, "naming what was asked for",
		strings.Contains(activity[1], "Write plans/goals/waiting.md (edit)"), true)
}

/* ---------------------------------------------------------------- driving -- */

// serviceWithReads is a Partner that reads three things on the way to an
// answer: a document whole, a board that left rows behind, and a goal that is
// not there. The results are the tool server's own shape, because that is what
// the host reads the source and the outcome out of.
func serviceWithReads(t *testing.T) (*partner.Service, *partner.Host) {
	t.Helper()
	root := t.TempDir()
	host := partner.NewHostOn(
		partner.Runtime{Name: "fake", ReadOnly: "a fake server reads nothing"},
		root, fakeacp.Open(fakeacp.Script{
			Reads: []fakeacp.Read{
				{
					Title: "document(plans/designs/g1-s26.md)",
					Result: "Source: plans/designs/g1-s26.md as it stands, revision r7\n" +
						"Supplied: 1 of 1\n\n## What it leaves out\n\nThe register of suggested questions.\n",
				},
				{
					Title: "board()",
					Result: "Source: the accepted tip abc123, observed 2026-09-23T11:29:55Z\n" +
						"Supplied: 25 of 163\n" +
						"More remains: call this tool again with cursor \"25\".\n\n- todo: g1-s30 · Do the next thing.\n",
				},
				{
					Title:  "goal(ghost)",
					Failed: true,
					Result: "Source: the accepted tip abc123, observed 2026-09-23T11:29:55Z\n" +
						"Outcome: this read failed — the accepted tip carries no goal ghost\n",
				},
			},
			Chunks: []string{"It leaves out ", "the register."},
		}))
	t.Cleanup(host.Close)
	service := partner.NewService(host.Runtime(), host,
		func(human string) (*partner.Conversation, error) {
			return partner.OpenConversation(root, human)
		},
		partner.Facts{}, func() time.Time { return time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC) })
	return service, host
}

// drain reads every beat of one turn, up to its terminal one.
func drain(t *testing.T, events <-chan partner.Event) []partner.Event {
	t.Helper()
	return collect(t, events, partner.EventDone, partner.EventError, partner.EventStopped)
}
