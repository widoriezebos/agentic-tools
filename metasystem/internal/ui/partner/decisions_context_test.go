package partner

import (
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
)

// The Decisions page's own block: the inbox rows the page named, and nothing
// this server composed for itself.

func TestTheDecisionsPageCarriesTheRowsItIsShowing(t *testing.T) {
	t.Parallel()
	composed := Compose(readingFacts(), Page{
		Section: "Decisions", Path: "/decisions", Tab: "rulings",
		Records: []string{
			"approval · g1-s40 · Approve the pane reads a document for execution",
			"ruling-review · R-3 · Review R-3, due 2026-09-19: adopt, revise or withdraw",
		},
	}, "Wido", composedAt)

	testutil.Expect(t, "the inbox is named as the page's own",
		strings.Contains(composed, "as the page is showing it (2)"), true)
	testutil.Expect(t, "the approval row",
		strings.Contains(composed, "approval · g1-s40 · Approve the pane reads a document for execution"), true)
	testutil.Expect(t, "the review row",
		strings.Contains(composed, "ruling-review · R-3 · Review R-3, due 2026-09-19"), true)
	testutil.Expect(t, "the open tab", strings.Contains(composed, "- Tab: rulings"), true)
	// The board's lane counts are not what this page shows, and a fall
	// through to them would describe a page nobody is looking at.
	testutil.Expect(t, "no lane counts", strings.Contains(composed, "Goals per lane"), false)
}

// A page that named no rows says so, rather than being described from the
// ledger it is not showing.
func TestADecisionsPageThatNamedNoRowsSaysSo(t *testing.T) {
	t.Parallel()
	composed := Compose(readingFacts(), Page{Section: "Decisions", Path: "/decisions"}, "Wido", composedAt)

	testutil.Expect(t, "it says the page named nothing",
		strings.Contains(composed, "The page did not say which rows its inbox is showing."), true)
	testutil.Expect(t, "no lane counts", strings.Contains(composed, "Goals per lane"), false)
}

// A ruling's own words never travel with a question: the register is a
// document of the checkout, and the Partner reads it there.
func TestARulingsWordsNeverTravelWithAQuestion(t *testing.T) {
	t.Parallel()
	composed := Compose(readingFacts(), Page{
		Section: "Decisions", Path: "/decisions", Tab: "rulings",
		Records: []string{"ruling-review · R-3 · Review R-3, due 2026-09-19: adopt, revise or withdraw"},
	}, "Wido", composedAt)

	testutil.Expect(t, "the register is named as where they are read",
		strings.Contains(composed, "memory/rulings.md, which the documents tool reads whole"), true)
}
