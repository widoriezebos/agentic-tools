package partner_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/partner"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/partner/fakeacp"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/uitools"
)

// Who decides whether the Partner's words are offered to the human at all.
//
// The tool server has no capture and cannot know what is on the human's screen;
// the host has no turn. This service holds the capture the turn was asked with,
// so it is the one place that can say whether the field the Partner wrote for is
// a field the human handed over — and when it is not, it says so in the
// conversation rather than dropping the words in silence.

const said = "Every refund lands within a day, with nobody touching the queue."

// serviceSuggesting is a Partner that answers in words and prepares one
// suggestion for the field named, in the tool's own fixed form.
func serviceSuggesting(t *testing.T, editor, field, text string) *partner.Service {
	t.Helper()
	root := t.TempDir()
	host := partner.NewHostOn(
		partner.Runtime{Name: "fake", ReadOnly: "a fake server reads nothing"},
		root, fakeacp.Open(fakeacp.Script{
			Reads: []fakeacp.Read{{
				Name:  "mcp__metasystem__suggest",
				Title: "suggest(" + editor + " · " + field + ")",
				Result: uitools.PreparedLine + "\n" +
					uitools.SuggestionHeader + editor + uitools.SuggestionJoin + field + "\n" +
					uitools.SuggestionSeparator + "\n" + text + "\n",
			}},
			Chunks: []string{"Here is a tighter one. ", "Use it if it says what you meant."},
		}))
	t.Cleanup(host.Close)
	return partner.NewService(host.Runtime(), host,
		func(human string) (*partner.Conversation, error) { return partner.OpenConversation(root, human) },
		partner.Facts{}, func() time.Time { return time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC) })
}

// editing is the capture a human asking from the edit sheet sends: the sheet
// they handed over, the opening it is, the fields as they stand, and which of
// them the sheet says may be written into.
func editing() partner.Page {
	return partner.Page{
		Section: "Backlog", Path: "/backlog", Sheet: "Edit goal",
		Draft: &partner.Draft{
			Sheet:   "Edit goal",
			Opening: "opening-7",
			Fields: []partner.DraftField{
				{Name: "Goal", Value: "ui-1"},
				{Name: "Intent", Value: "The board reads the ledger somehow."},
			},
			Writable: []string{"Intent", "Next step", "Labels"},
			// Where the caret was, which is what a request naming no field means.
			Writing: "Intent",
		},
	}
}

// A suggestion for a writable field of the opening the human handed over is
// offered: the stream carries it as it is admitted, stamped with that opening,
// and the answer keeps it.
func TestASuggestionForAHandedOverFieldIsOffered(t *testing.T) {
	t.Parallel()
	service := serviceSuggesting(t, "Edit goal", "Intent", said)
	events, stop := service.Subscribe()
	defer stop()
	_, err := service.Submit(context.Background(), "Wido", "key-1", "tighten this, one line",
		editing())
	testutil.Require(t, "admitted", err, nil)
	beats := drain(t, events)

	offered := []partner.Event{}
	for _, beat := range beats {
		if beat.Kind == partner.EventSuggestion {
			offered = append(offered, beat)
		}
	}
	testutil.Require(t, "one suggestion beat", len(offered), 1)
	testutil.Require(t, "carrying the suggestion", offered[0].Suggestion != nil, true)
	testutil.Expect(t, "stamped with the opening the human handed over", offered[0].Suggestion.Opening, "opening-7")
	testutil.Expect(t, "the editor", offered[0].Suggestion.Editor, "Edit goal")
	testutil.Expect(t, "the field", offered[0].Suggestion.Field, "Intent")
	testutil.Expect(t, "and the words whole", offered[0].Suggestion.Text, said)
	testutil.Expect(t, "marked as offered", offered[0].Suggestion.Offered, true)
	testutil.Expect(t, "with nothing to explain", offered[0].Suggestion.Reason, "")

	// It arrives before the answer ends, which is what puts the card under the
	// answer while the answer is still being written.
	at, ended := -1, -1
	for index, beat := range beats {
		if beat.Kind == partner.EventSuggestion && at < 0 {
			at = index
		}
		if beat.Kind == partner.EventDone {
			ended = index
		}
	}
	testutil.Expect(t, "before the turn ended", at < ended, true)

	read, err := service.Snapshot("Wido", 100)
	testutil.Require(t, "read back", err, nil)
	testutil.Require(t, "two messages", len(read.Messages), 2)
	answered := read.Messages[1]
	testutil.Require(t, "the answer keeps it", len(answered.Suggestions), 1)
	testutil.Expect(t, "whole", answered.Suggestions[0].Text, said)
	testutil.Expect(t, "with its opening", answered.Suggestions[0].Opening, "opening-7")
	testutil.Expect(t, "the answer marks it offered too", answered.Suggestions[0].Offered, true)
	testutil.Expect(t, "and nothing was refused", len(answered.Activity), 0)
	// The call is accounted for as a look as well: a suggestion never arrives
	// without the call that prepared it being listed.
	testutil.Require(t, "the page and the call", len(answered.Looked), 2)
	testutil.Expect(t, "the call is named", answered.Looked[1].What, "suggest(Edit goal · Intent)")
}

// A field the human never handed over is not offered, and the refusal is carried
// as the suggestion it was, with its reason, so the drawer can show it as a card
// where the human reads the answer.
//
// It used to be an activity line, and an activity line was how the one live
// sitting went wrong: the drawer does not show them, so the human saw nothing
// and was told a card had been put on their sheet (g1-s52 §1, D3).
func TestASuggestionForAFieldNobodyHandedOverIsNotOffered(t *testing.T) {
	t.Parallel()
	for _, probe := range []struct {
		what   string
		editor string
		field  string
		reason string
		page   func() partner.Page
	}{
		{"a field the sheet did not register", "Edit goal", "Tier",
			"Tier is not open for proposals", editing},
		{"a field handed over only as context", "Edit goal", "Goal",
			"Goal is not open for proposals", editing},
		{"another editor's field", "New goal", "Intent",
			"Intent is not open for proposals", editing},
		{"a sheet nobody handed over at all", "Edit goal", "Intent",
			"the draft was left out; press Ask about this to hand it over again",
			func() partner.Page {
				return partner.Page{Section: "Backlog", Path: "/backlog"}
			}},
		{"a sheet handed over with no opening", "Edit goal", "Intent",
			"Intent is not open for proposals", func() partner.Page {
				return partner.Page{Section: "Backlog", Path: "/backlog", Draft: &partner.Draft{
					Sheet: "Edit goal", Writable: []string{"Intent"},
				}}
			}},
	} {
		service := serviceSuggesting(t, probe.editor, probe.field, said)
		events, stop := service.Subscribe()
		_, err := service.Submit(context.Background(), "Wido", "key-1", "suggest a better wording", probe.page())
		testutil.Require(t, "admitted "+probe.what, err, nil)
		beats := drain(t, events)
		stop()
		refused := []partner.Suggestion{}
		for _, beat := range beats {
			if beat.Kind == partner.EventSuggestion && beat.Suggestion != nil {
				refused = append(refused, *beat.Suggestion)
			}
		}
		// The stream carries it, so the card appears under the answer as the call
		// completes, exactly as an admitted one does.
		testutil.Require(t, probe.what+" reaches the stream", len(refused), 1)
		testutil.Expect(t, probe.what+" is not offered", refused[0].Offered, false)
		testutil.Expect(t, probe.what+" says why", refused[0].Reason, probe.reason)
		// And it is never stamped with an opening: nothing may write it anywhere.
		testutil.Expect(t, probe.what+" belongs to no opening", refused[0].Opening, "")

		read, err := service.Snapshot("Wido", 100)
		testutil.Require(t, "read back "+probe.what, err, nil)
		answered := read.Messages[len(read.Messages)-1]
		testutil.Require(t, probe.what+" is kept on the answer", len(answered.Suggestions), 1)
		testutil.Expect(t, probe.what+" as not offered", answered.Suggestions[0].Offered, false)
		testutil.Expect(t, probe.what+" with its reason", answered.Suggestions[0].Reason, probe.reason)
		testutil.Expect(t, probe.what+" and the words it would have offered",
			answered.Suggestions[0].Text, said)
		testutil.Expect(t, probe.what+" records no activity line", len(answered.Activity), 0)
	}
}

// An empty field is admitted like any other. "Suggest the next step" on a next
// step nobody has written yet is the case the whole registration exists for: the
// writable names travel whether or not there is anything in them.
func TestASuggestionForAnEmptyHandedOverFieldIsOffered(t *testing.T) {
	t.Parallel()
	service := serviceSuggesting(t, "Edit goal", "Next step", "Take the worker to a working end state.")
	events, stop := service.Subscribe()
	defer stop()
	page := editing()
	// The sheet's next step is empty, so the captured fields do not carry it at
	// all; only the writable list says it is there.
	testutil.Require(t, "the field carries no value", len(page.Draft.Fields), 2)
	_, err := service.Submit(context.Background(), "Wido", "key-1", "suggest the next step", page)
	testutil.Require(t, "admitted", err, nil)
	drain(t, events)

	read, err := service.Snapshot("Wido", 100)
	testutil.Require(t, "read back", err, nil)
	answered := read.Messages[1]
	testutil.Require(t, "it is offered", len(answered.Suggestions), 1)
	testutil.Expect(t, "for the empty field", answered.Suggestions[0].Field, "Next step")
	testutil.Expect(t, "nothing was refused", len(answered.Activity), 0)
}

// The words the human asked for are still an offer and nothing else: the field
// is not written to here, the draft the question carried is kept exactly as the
// human had it, and no act is recorded.
func TestOfferingASuggestionWritesNothing(t *testing.T) {
	t.Parallel()
	service := serviceSuggesting(t, "Edit goal", "Intent", said)
	events, stop := service.Subscribe()
	defer stop()
	_, err := service.Submit(context.Background(), "Wido", "key-1", "tighten this", editing())
	testutil.Require(t, "admitted", err, nil)
	drain(t, events)

	read, err := service.Snapshot("Wido", 100)
	testutil.Require(t, "read back", err, nil)
	asked := read.Messages[0]
	testutil.Require(t, "the question kept the draft", asked.Page != nil && asked.Page.Draft != nil, true)
	testutil.Expect(t, "as the human had it", asked.Page.Draft.Fields[1].Value,
		"The board reads the ledger somehow.")
	testutil.Expect(t, "the suggestion is not in it",
		strings.Contains(asked.Page.Draft.Fields[1].Value, "Every refund"), false)
	testutil.Expect(t, "and the sheet's own opening travelled with it", asked.Page.Draft.Opening, "opening-7")
	testutil.Expect(t, "with the field the caret was in", asked.Page.Draft.Writing, "Intent")
}
