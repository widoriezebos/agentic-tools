package uitools

import (
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
)

// The one operation that reads nothing, and the whole of what it decides.
//
// It cannot judge whether the field is one the human handed over — this process
// has no capture — so what it can be held to is its arguments and the form it
// answers in. Both are here, because the form is what the interface reads a
// suggestion out of and a form that drifted would be a card that stopped
// appearing.

// The result is the fixed form: the one line the model reads, the header naming
// the editor and the field, the separator, and then the text.
func TestAPreparedSuggestionAnswersInTheFixedForm(t *testing.T) {
	t.Parallel()
	result := fixture(t).Answer(OpSuggest, Args{
		"editor": "Edit goal",
		"field":  "Intent",
		"text":   "Every refund lands within a day, with nobody touching the queue.",
	})
	testutil.Expect(t, "it is not a failure", result.Failed(), false)
	lines := strings.Split(strings.TrimRight(result.Text(), "\n"), "\n")
	testutil.Require(t, "four lines", len(lines), 4)
	testutil.Expect(t, "the model is told what it prepared, and who decides", lines[0],
		"prepared as a suggestion for Intent; the human decides whether it is offered and used")
	testutil.Expect(t, "the header names the editor and the field", lines[1],
		SuggestionHeader+"Edit goal"+SuggestionJoin+"Intent")
	testutil.Expect(t, "the framing ends at the separator", lines[2], SuggestionSeparator)
	testutil.Expect(t, "and the text is the whole of the rest", lines[3],
		"Every refund lands within a day, with nobody touching the queue.")
}

// It read nothing, so it is stamped with nothing: a source would name a reading
// that never happened and a count would count rows there were none of.
func TestAPreparedSuggestionIsStampedWithNoReading(t *testing.T) {
	t.Parallel()
	result := fixture(t).Answer(OpSuggest, Args{"editor": "New goal", "field": "Next step", "text": "Take it to an end state."})
	testutil.Expect(t, "no source", result.Source, "")
	testutil.Expect(t, "no rows supplied", result.Supplied, 0)
	testutil.Expect(t, "no cursor", result.Cursor, "")
	testutil.Expect(t, "and the text says neither", strings.Contains(result.Text(), "Supplied:"), false)
}

// The text is opaque to the end. A suggestion may itself hold a line that reads
// like this server's own framing — a Partner improving an intent may quote one —
// and what it carries is the human's field, not a second header.
func TestAPreparedSuggestionCarriesItsTextWhole(t *testing.T) {
	t.Parallel()
	said := "Source: not a reading at all.\n" + SuggestionSeparator + "\nand a second line."
	result := fixture(t).Answer(OpSuggest, Args{"editor": "Edit goal", "field": "Intent", "text": said})
	at := strings.Index(result.Text(), SuggestionSeparator+"\n")
	testutil.Require(t, "the framing is there", at >= 0, true)
	testutil.Expect(t, "and everything after it is the text as it was written",
		strings.TrimRight(result.Text()[at+len(SuggestionSeparator)+1:], "\n"), said)
}

// The two names and the text are the whole of what it can hold a call to, so
// each refusal says which one it was and stays a failure the model is told
// about rather than a suggestion nobody offered.
func TestASuggestionIsRefusedOnItsArgumentsInWords(t *testing.T) {
	t.Parallel()
	for _, refused := range []struct {
		what string
		args Args
		says string
	}{
		{
			what: "no editor",
			args: Args{"field": "Intent", "text": "Something."},
			says: "this tool needs the name of the editor",
		},
		{
			what: "no field",
			args: Args{"editor": "Edit goal", "text": "Something."},
			says: "this tool needs the name of the field",
		},
		{
			what: "no text",
			args: Args{"editor": "Edit goal", "field": "Intent", "text": "   "},
			says: "this tool needs the text to offer for Intent",
		},
		{
			what: "text past the bound",
			args: Args{"editor": "Edit goal", "field": "Intent", "text": strings.Repeat("é", maxSuggestion+1)},
			says: "a suggestion carries at most 4000 characters and this one is 4001",
		},
	} {
		result := fixture(t).Answer(OpSuggest, refused.args)
		testutil.Expect(t, refused.what+" is a failure", result.Failed(), true)
		testutil.Expect(t, refused.what+" says which", strings.Contains(result.Problem, refused.says), true)
		testutil.Expect(t, refused.what+" is told to the model in its own words",
			strings.Contains(result.Text(), "this call was refused — "+refused.says), true)
		testutil.Expect(t, refused.what+" prepares no suggestion",
			strings.Contains(result.Text(), SuggestionHeader), false)
	}
}

// Four thousand characters is four thousand characters and not bytes: a field
// written in a language whose letters take two of them is not a shorter field.
func TestTheBoundIsCharactersAndNotBytes(t *testing.T) {
	t.Parallel()
	result := fixture(t).Answer(OpSuggest, Args{
		"editor": "Edit goal", "field": "Intent", "text": strings.Repeat("é", maxSuggestion),
	})
	testutil.Expect(t, "the bound itself is admitted", result.Failed(), false)
}

// This server publishes it beside the read tools, takes the three arguments and
// requires them, and the permission rule that admits calls to this server
// admits it — which is one list, named once.
func TestTheSuggestToolIsPublishedAndAdmittedLikeTheRest(t *testing.T) {
	t.Parallel()
	testutil.Expect(t, "the server answers it", Names(OpSuggest), true)
	var published Tool
	for _, tool := range Catalogue() {
		if tool.Name == OpSuggest {
			published = tool
		}
	}
	testutil.Require(t, "it is in the catalogue", published.Name, OpSuggest)
	properties, _ := published.InputSchema["properties"].(map[string]any)
	for _, named := range []string{"editor", "field", "text"} {
		_, there := properties[named]
		testutil.Expect(t, "it takes "+named, there, true)
	}
	required, _ := published.InputSchema["required"].([]string)
	testutil.Expect(t, "and needs all three", required, []string{"editor", "field", "text"})
	testutil.Expect(t, "its description says the human decides",
		strings.Contains(published.Description, "the human sees a card and decides"), true)
}
