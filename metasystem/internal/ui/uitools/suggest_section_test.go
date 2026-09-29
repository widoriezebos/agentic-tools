package uitools

import (
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
)

// Fold by section (g1-s66 D3): suggest gains a target of a document and a
// heading. The Partner drafts that section anew; the frame names the document
// and the heading, and the words follow whole. A section target is one target:
// a field and a section in one call are refused, and so is half of either.

func TestASectionSuggestionAnswersInItsOwnFrame(t *testing.T) {
	t.Parallel()
	text := "## 4. Decisions\n\n- D1. The design verbs as browser acts, with the budget."
	result := fixture(t).Answer(OpSuggest, Args{"document": "plans/designs/g1-s66.md", "section": "4. Decisions", "text": text})
	testutil.Expect(t, "it is not a failure", result.Failed(), false)
	testutil.Expect(t, "the frame names the document and the heading, then the words whole", result.Text(),
		PreparedSectionLine+"\n"+SectionSuggestionHeader+"plans/designs/g1-s66.md"+SuggestionJoin+"4. Decisions\n"+SuggestionSeparator+"\n"+text+"\n")
}

func TestASectionSuggestionIsRefusedInWords(t *testing.T) {
	t.Parallel()
	for _, refused := range []struct {
		args  Args
		words string
	}{
		{Args{"document": "plans/designs/g1-s66.md", "text": "x"}, "a section is named by its heading"},
		{Args{"section": "4. Decisions", "text": "x"}, "a section is named with the document it is in"},
		{Args{"document": "plans/designs/g1-s66.md", "section": "4. Decisions", "editor": "Edit goal", "field": "Intent", "text": "x"},
			"one suggestion is for a field of a sheet or for a section of a document, not both"},
		{Args{"document": "plans/designs/g1-s66.md", "section": "4. Decisions", "text": " "}, "the section's whole new text"},
		{Args{"document": "plans/designs/g1-s66.md", "section": "4. Decisions", "text": strings.Repeat("x", maxSection+1)},
			"a section suggestion carries at most"},
	} {
		result := fixture(t).Answer(OpSuggest, refused.args)
		testutil.Expect(t, refused.words+" is a failure", result.Failed(), true)
		testutil.Expect(t, "refused as "+refused.words, strings.Contains(result.Problem, refused.words), true)
	}
}
