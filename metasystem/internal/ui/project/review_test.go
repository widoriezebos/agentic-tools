package project

import (
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
)

// A review is a record of its own (g1-s65 D1): kind review, in a home beside
// the designs, with a head naming the goal, what was reviewed and the evidence
// a design's own Evidence line names, and the six sections the room writes.
func TestCreateReviewWritesTheHeadAndTheSections(t *testing.T) {
	t.Parallel()
	roots := selfHostedFixture(t)
	seed(t, roots)
	plant(t, roots.StateRoot, "plans/designs/evidenced.md",
		"# Evidenced\n\n- Kind: design\n- Id: design-evidenced\n- Status: draft\n- Goals: ledger-sync\n"+
			"- Evidence: ~/evidence/ledger-sync/\n\n## Outcome\n")

	written, err := CreateReview(roots, NewReview{Goal: "ledger-sync", Reviewed: "84f8acfbf0000000000000000000000000000000"}, wroteAt)

	testutil.Require(t, "create the review", err, nil)
	testutil.Expect(t, "the path", written.Path, "metasystem/plans/reviews/review-of-ledger-sync.md")
	testutil.Expect(t, "the kind", written.Record.Kind, "review")
	on := fileAt(t, roots, written.Path)
	for _, line := range []string{
		"# Review of ledger-sync\n",
		"- Kind: review\n",
		"- Status: draft\n",
		"- Goals: ledger-sync\n",
		"- Reviewed: 84f8acfbf0000000000000000000000000000000\n",
		"- Evidence: ~/evidence/ledger-sync/\n",
		"\n## Facts\n", "\n## Findings\n", "\n## Decisions\n", "\n## Open questions\n",
		"\n## Drawings\n", "\n## Outcome\n",
	} {
		testutil.Expect(t, "the page carries "+strings.TrimSpace(line), strings.Contains(on, line), true)
	}
	pane, err := ReadPane(roots, readAt)
	testutil.Require(t, "read the pane", err, nil)
	testutil.Expect(t, "the project refuses nothing", pane.Problems, []Problem{})
}

// A second review of the same goal is a second record beside the first, never
// a write over it, and a goal no design gives evidence for carries no Evidence
// line at all.
func TestCreateReviewNeverWritesOverAnEarlierOne(t *testing.T) {
	t.Parallel()
	roots := selfHostedFixture(t)
	seed(t, roots)

	first, err := CreateReview(roots, NewReview{Goal: "ledger-sync", Reviewed: NoneFound}, wroteAt)
	testutil.Require(t, "the first review", err, nil)
	second, err := CreateReview(roots, NewReview{Goal: "ledger-sync", Reviewed: NoneFound}, wroteAt)
	testutil.Require(t, "the second review", err, nil)

	testutil.Expect(t, "two records", second.Path, "metasystem/plans/reviews/review-of-ledger-sync-2.md")
	testutil.Expect(t, "the first path", first.Path, "metasystem/plans/reviews/review-of-ledger-sync.md")
	on := fileAt(t, roots, first.Path)
	testutil.Expect(t, "the none-found line", strings.Contains(on, "- Reviewed: none found; write them here\n"), true)
	testutil.Expect(t, "no evidence line", strings.Contains(on, "- Evidence:"), false)
}

// A review names a goal the ledger carries; one it does not is refused before
// anything is written, and the ordinary creator does not make a review at all:
// the server creates it at Start.
func TestCreateReviewRefusals(t *testing.T) {
	t.Parallel()
	roots := selfHostedFixture(t)
	seed(t, roots)

	_, err := CreateReview(roots, NewReview{Goal: "no-such-goal", Reviewed: NoneFound}, wroteAt)
	testutil.Expect(t, "an unknown goal", refusalOf(t, err).Message, `the goal "no-such-goal" is not in the ledger`)
	_, err = CreateReview(roots, NewReview{Goal: " ", Reviewed: NoneFound}, wroteAt)
	testutil.Expect(t, "no goal", refusalOf(t, err).Message, "a review names the goal it reviews")
	_, err = CreateRecord(roots, NewRecord{Kind: "review", Title: "A review by hand"}, wroteAt)
	testutil.Expect(t, "not by the ordinary creator", refusalOf(t, err).Message,
		`the kind "review" is not one of intent, doctrine, decision, design`)
}
