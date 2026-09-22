package project

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
)

// The summary rule, on bodies rather than on files: what counts as the first
// prose paragraph, what is stripped out of it, and where it is cut.

func TestSummaryIsTheFirstProseParagraph(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		body string
		want string
	}{
		{
			name: "the paragraph the body opens with",
			body: "The pane reads the homes.\nIt writes nothing.\n\nA second paragraph.",
			want: "The pane reads the homes. It writes nothing.",
		},
		{
			name: "the first paragraph under a heading",
			body: "## Vision\n\nEngineering after the shift.",
			want: "Engineering after the shift.",
		},
		{
			name: "past a list, a table, a rule, a quote and a fence",
			body: "- a list item\n\n| a | b |\n| --- | --- |\n| 1 | 2 |\n\n---\n\n> a quotation\n\n```go\nfunc main() {}\n```\n\nThe prose at last.",
			want: "The prose at last.",
		},
		{
			name: "emphasis, code and links reduced to their words",
			body: "The **first** _paragraph_, with a [link](./to.md), `code`, ~~a cut~~ and an ![image](p.png) alt.",
			want: "The first paragraph, with a link, code, a cut and an image alt.",
		},
		{
			name: "an underscore inside a word is part of the word",
			body: "The metasystem_ui package is named for what it serves.",
			want: "The metasystem_ui package is named for what it serves.",
		},
		{
			name: "an escaped marker is the character it escaped",
			body: "A literal \\*asterisk\\* survives.",
			want: "A literal *asterisk* survives.",
		},
		{
			name: "a body with no prose at all",
			body: "## Chapters\n\n- one\n- two\n",
			want: "",
		},
		{
			name: "an empty body",
			body: "",
			want: "",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			testutil.Expect(t, "the summary", summaryOf(splitLines(tc.body)), tc.want)
		})
	}
}

// A summary is a line beside a title, so it is kept to its measure and cut
// where a sentence ends rather than in the middle of one.
func TestSummaryIsClippedAtASentenceEnd(t *testing.T) {
	t.Parallel()

	sentence := strings.Repeat("The pane reads what the records declare. ", 20)

	clipped := summaryOf(splitLines(sentence))

	testutil.Expect(t, "it ends a sentence", strings.HasSuffix(clipped, "declare."), true)
	testutil.Expect(t, "it is within the measure", len([]rune(clipped)) <= summaryLimit, true)
	testutil.Expect(t, "and it carries whole sentences", strings.Count(clipped, "declare."), 7)
}

// A first sentence longer than the measure has no sentence end to cut at, so
// the last whole word is kept and an ellipsis says that there is more.
func TestSummaryWithNoSentenceEndKeepsWholeWords(t *testing.T) {
	t.Parallel()

	long := strings.TrimSpace(strings.Repeat("words ", 200))

	clipped := summaryOf(splitLines(long))

	testutil.Expect(t, "it says there is more", strings.HasSuffix(clipped, "…"), true)
	testutil.Expect(t, "no word is cut through", strings.HasSuffix(clipped, "words…"), true)
	testutil.Expect(t, "it is within the measure", len([]rune(clipped)) <= summaryLimit+1, true)
}

// A bound document is not a record, and its summary is read from the file the
// chapter binds, after the title that opens it.
func TestSummaryOfABoundDocument(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	plant(t, directory, "paper/01-the-shift.md", "# 1. The Shift\n\nSoftware is no longer written by hand.\n")

	testutil.Expect(t, "the first paragraph after the title",
		summaryOfFile(filepath.Join(directory, "paper", "01-the-shift.md")),
		"Software is no longer written by hand.")
	testutil.Expect(t, "a file that is not there has no summary",
		summaryOfFile(filepath.Join(directory, "paper", "absent.md")), "")
}
