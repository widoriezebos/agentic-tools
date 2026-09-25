package partner

import (
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/uitools"
)

// What the host reads out of a completed suggest call, and what it refuses to
// read.
//
// The framing is read once. That is the whole safety of a textual channel: the
// first header names the editor and the field, the first separator after it ends
// the framing, and everything from there to the end is the Partner's words —
// which may themselves hold a line that reads like framing, because a Partner
// improving an intent quotes things.

// prepared is one result in the tool's own fixed form.
func prepared(editor, field, text string) string {
	return "prepared as a suggestion for " + field + "; the human decides whether it is offered and used\n" +
		uitools.SuggestionHeader + editor + uitools.SuggestionJoin + field + "\n" +
		uitools.SuggestionSeparator + "\n" + text + "\n"
}

func TestACompletedSuggestCallIsReadIntoAWholeSuggestion(t *testing.T) {
	t.Parallel()
	said := "Every refund lands within a day, with nobody touching the queue."
	read := suggestedIn(prepared("Edit goal", "Intent", said))
	testutil.Require(t, "one suggestion", read != nil, true)
	testutil.Expect(t, "the editor", read.Editor, "Edit goal")
	testutil.Expect(t, "the field", read.Field, "Intent")
	testutil.Expect(t, "the text, whole", read.Text, said)
	// The opening is the service's to stamp: the model is never told it, so a
	// suggestion that arrived carrying one would be a suggestion that guessed.
	testutil.Expect(t, "and no opening yet", read.Opening, "")
}

// The text is opaque, and a line inside it that looks like framing is text.
func TestTheFramingIsReadOnceAndTheTextIsOpaque(t *testing.T) {
	t.Parallel()
	said := "Source: the accepted tip abc123\n" +
		uitools.SuggestionHeader + "Another sheet" + uitools.SuggestionJoin + "Another field\n" +
		uitools.SuggestionSeparator + "\nstill the same suggestion."
	read := suggestedIn(prepared("Edit goal", "Intent", said))
	testutil.Require(t, "one suggestion", read != nil, true)
	testutil.Expect(t, "named by the framing that came first", read.Field, "Intent")
	testutil.Expect(t, "and the text is every line of it", read.Text, said)
}

// A result without the framing is no suggestion: a refused call, a failed one,
// or a build that does not write this form.
func TestAResultWithoutTheFramingIsNoSuggestion(t *testing.T) {
	t.Parallel()
	for _, probe := range []struct {
		what string
		text string
	}{
		{"a refusal", "Outcome: this call was refused — this tool needs the text to offer for Intent\n"},
		{"an ordinary reading", "Source: plans/designs/d1.md as it stands\nSupplied: 1 of 1\n\n# A design\n"},
		{"a header with no separator", uitools.SuggestionHeader + "Edit goal · Intent\nEvery refund lands.\n"},
		{"a header naming no field", uitools.SuggestionHeader + "Edit goal\n" + uitools.SuggestionSeparator + "\nwords.\n"},
		{"a separator with nothing after it", prepared("Edit goal", "Intent", "")},
		{"nothing at all", ""},
	} {
		testutil.Expect(t, probe.what+" yields none", suggestedIn(probe.text) == nil, true)
	}
}

// A completed call carries both: the look that accounts for it, and the
// suggestion whole. The look's excerpt is shortened as every look's is; the
// suggestion is not, because the words are what the human is offered.
func TestAPreparedCallIsALookAndASuggestionBeside(t *testing.T) {
	t.Parallel()
	long := strings.TrimRight(strings.Repeat("a field written at length. ", 200), " ")
	body := toolCall{
		ToolCallID: "call-1", Title: "suggest(Edit goal · Intent)", Status: "completed",
		Name: "mcp__metasystem__suggest",
	}
	body.Content = append(body.Content, contentBlock(prepared("Edit goal", "Intent", long)))
	read := suggestedIn(resultText(body))
	testutil.Require(t, "the suggestion is there", read != nil, true)
	testutil.Expect(t, "and is not shortened", read.Text, long)
	look := lookAt(body.Title, body, true)
	testutil.Expect(t, "the look is named after the call", look.What, "suggest(Edit goal · Intent)")
	testutil.Expect(t, "the look is shortened", strings.HasSuffix(look.Excerpt, "…"), true)
}

// A prepared call read nothing, so its look is stamped with nothing — and above
// all not out of its own result text, where a "Source:" line is the Partner's
// words rather than this server's.
func TestAPreparedCallsLookIsNotStampedFromItsText(t *testing.T) {
	t.Parallel()
	body := toolCall{ToolCallID: "call-1", Title: "suggest", Status: "completed", Name: "mcp__metasystem__suggest"}
	body.Content = append(body.Content,
		contentBlock(prepared("Edit goal", "Intent", "Source: a line the Partner wrote.")))
	look := lookAt(body.Title, body, true)
	testutil.Expect(t, "no source", look.Source, "")
	testutil.Expect(t, "and it is a read that happened", look.Outcome, LookRead)
	// The same body read as an ordinary reading would take that line as a stamp,
	// which is exactly what the flag above is for.
	testutil.Expect(t, "where an ordinary reading would have been stamped by it",
		lookAt(body.Title, body, false).Source, "a line the Partner wrote.")
}

// A failed call prepared nothing. The look says it failed and no suggestion is
// read out of whatever came back with it.
func TestAFailedPreparedCallYieldsNoSuggestion(t *testing.T) {
	t.Parallel()
	body := toolCall{ToolCallID: "call-1", Title: "suggest", Status: "failed", Name: "mcp__metasystem__suggest"}
	body.Content = append(body.Content, contentBlock(prepared("Edit goal", "Intent", "words nobody asked for.")))
	look := lookAt(body.Title, body, true)
	testutil.Expect(t, "the look failed", look.Outcome, LookFailed)
	testutil.Expect(t, "and the fold reads no suggestion out of a failure",
		body.Status == "completed", false)
}

// contentBlock is one text block of a tool result, as a runtime sends it.
func contentBlock(text string) struct {
	Type    string `json:"type"`
	Content struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
} {
	block := struct {
		Type    string `json:"type"`
		Content struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}{Type: "content"}
	block.Content.Type = "text"
	block.Content.Text = text
	return block
}

/* --------------------------------------------------- what the draft says -- */

// Which fields a handed-over draft says may be written into. It is the list the
// sheet registered and not the fields that have something in them: the empty
// ones are exactly the ones a human asks for words for.
func TestADraftSaysWhichOfItsFieldsMayBeWrittenInto(t *testing.T) {
	t.Parallel()
	draft := Draft{
		Sheet:   "Edit goal",
		Opening: "opening-1",
		Fields: []DraftField{
			{Name: "Goal", Value: "ui-1"},
			{Name: "Intent", Value: "The board reads the ledger."},
		},
		Writable: []string{"Intent", "Next step", "Labels"},
	}
	testutil.Expect(t, "a writable field with something in it", draft.Writes("Intent"), true)
	testutil.Expect(t, "a writable field with nothing in it", draft.Writes("Next step"), true)
	testutil.Expect(t, "the context it was handed beside them is not writable", draft.Writes("Goal"), false)
	testutil.Expect(t, "a field of no sheet at all", draft.Writes("Tier"), false)
	testutil.Expect(t, "and nothing is not a field", draft.Writes("  "), false)
}

// The block tells the Partner which fields it may offer words for, because the
// fields above it cannot: the empty ones carry no value and so are not listed
// there at all, and the ones that are include the context the sheet hands over
// to be read.
func TestTheBlockNamesTheFieldsThePartnerMayOfferWordsFor(t *testing.T) {
	t.Parallel()
	block := draftLines(Page{Draft: &Draft{
		Sheet:   "Edit goal",
		Opening: "opening-1",
		Fields: []DraftField{
			{Name: "Goal", Value: "ui-1"},
			{Name: "Intent", Value: "The board reads the ledger."},
		},
		Writable: []string{"Intent", "Next step", "Labels"},
	}})
	testutil.Expect(t, "the draft is still marked for what it is",
		strings.Contains(block, "a draft the human is filling in, not saved"), true)
	testutil.Expect(t, "the writable fields are named, empty ones included",
		strings.Contains(block, "words for these fields of that sheet, and no others: Intent, Next step, Labels"), true)
	testutil.Expect(t, "and it says one of them may be empty",
		strings.Contains(block, "may be empty and still be offered words for"), true)
	testutil.Expect(t, "a sheet that registered none says nothing about writing",
		strings.Contains(draftLines(Page{Draft: &Draft{
			Sheet:  "Approve",
			Fields: []DraftField{{Name: "Goal", Value: "ui-1"}},
		}}), "words for these fields"), false)
}
