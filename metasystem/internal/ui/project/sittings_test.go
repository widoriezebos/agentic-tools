package project

import (
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
)

// Project → Sittings: which records this project has sat on (g1-s55 D3).
//
// The one thing this has to get right is what it does NOT list. Astra's F2: the
// record creator writes an "Outcome" heading into every design and an "Open
// questions" heading into every intent the moment it is created, so a list that
// asked about headings would answer "every design and every intent" — which is
// the opposite of a list of the records somebody has sat on. It asks about the
// marks the entries carry instead, which the browser writes when a human presses
// Record it.

// entry is one line of a pile as the browser writes it: the date, the name, the
// words, and the mark of the deposit it was recorded from.
func entry(when, who, said, mark string) string {
	line := "- " + when + " · " + who + " · " + said
	if mark != "" {
		line += " [d:" + mark + "]"
	}
	return line
}

// A record is listed because its entries say a sitting recorded them, with the
// counts of its piles and the date of its last entry.
func TestASittingIsKnownByTheMarksItsEntriesCarry(t *testing.T) {
	t.Parallel()
	roots := selfHostedFixture(t)
	state := seed(t, roots)
	plant(t, roots.Checkout, state+"plans/designs/sessions.md",
		record("Session limits", "design", "design-sessions", "draft", "ledger-sync")+
			"\n## Facts\n\n"+
			entry("2026-09-24", "Wido", "the mobile client renews differently", "deposit:t1#0")+"\n"+
			"  - Anchor: internal/session/session.go:212\n\n"+
			"## Decisions\n\n"+
			entry("2026-09-26", "Wido", "the limit counts from last activity", "deposit:t1#1")+"\n"+
			"  - Reason: a page nobody has touched is not in use\n\n"+
			"## Open questions\n\n"+
			entry("2026-09-25", "Wido", "what the twelve hours protected", "deposit:t2#0")+"\n"+
			"  - Consequence: the new limit is chosen without knowing\n"+
			// A line a human wrote by hand is counted as the entry it is and
			// carries no mark, because showing a human the line they typed is
			// better than hiding it.
			"- a question somebody typed straight into the record\n")

	pane, err := ReadPane(roots, readAt)
	testutil.Require(t, "the pane was read", err, nil)
	testutil.Require(t, "one record was sat on", len(pane.Sittings), 1)
	row := pane.Sittings[0]
	testutil.Expect(t, "named by its id", row.Record.ID, "design-sessions")
	testutil.Expect(t, "and by its path", row.Record.Path, "metasystem/plans/designs/sessions.md")
	testutil.Expect(t, "with its kind", row.Record.Kind, "design")
	testutil.Expect(t, "and its title", row.Record.Title, "Session limits")
	testutil.Expect(t, "the piles as they stand", row.Counts,
		PileCounts{Facts: 1, Proposals: 0, Decisions: 1, Questions: 2})
	testutil.Expect(t, "the last entry recorded", row.LastAt, "2026-09-26")
	testutil.Expect(t, "and no sitting stands on it", row.Standing, false)
}

// A design with the headings its creator gave it and no marks in them is not a
// sitting: nobody has recorded anything into it. The seeded project is full of
// them, and the one above is the only row.
func TestARecordWithTheHeadingsAndNoMarksIsNotASitting(t *testing.T) {
	t.Parallel()
	roots := selfHostedFixture(t)
	state := seed(t, roots)
	// The seed's own design carries an Outcome heading, a table and a list under
	// it, which is what an ordinary design looks like.
	plant(t, roots.Checkout, state+"plans/designs/empty-piles.md",
		record("A design nobody has sat on", "design", "design-empty", "draft", "ledger-sync")+
			"\n## Facts\n\n## Proposals\n\n## Decisions\n\n## Open questions\n\n"+
			"- something a human wrote here by hand\n\n## Outcome\n\nNothing yet.\n")

	pane, err := ReadPane(roots, readAt)
	testutil.Require(t, "the pane was read", err, nil)
	testutil.Expect(t, "nothing was sat on", len(pane.Sittings), 0)
}

// An Outcome written by End carries the same mark, so a sitting whose only
// recorded thing is its outcome is a sitting all the same.
func TestAnOutcomeWrittenByEndIsSittingMaterial(t *testing.T) {
	t.Parallel()
	roots := selfHostedFixture(t)
	state := seed(t, roots)
	plant(t, roots.Checkout, state+"plans/designs/closed.md",
		record("A sitting that only closed", "design", "design-closed", "draft", "ledger-sync")+
			"\n## Outcome\n\nThe limit counts from last activity.\n\n"+
			"- Recorded from the sitting · 2026-09-26 · Wido [d:deposit:t9#0]\n")

	pane, err := ReadPane(roots, readAt)
	testutil.Require(t, "the pane was read", err, nil)
	testutil.Require(t, "it is listed", len(pane.Sittings), 1)
	testutil.Expect(t, "by its path", pane.Sittings[0].Record.Path, "metasystem/plans/designs/closed.md")
	testutil.Expect(t, "with no piles", pane.Sittings[0].Counts, PileCounts{})
	// And dated from the foot of that Outcome, which is the one thing recorded
	// into this record: a sitting whose close is all it has is dated by the close,
	// rather than listing with no date and sorting as the oldest row there is.
	testutil.Expect(t, "dated by its close", pane.Sittings[0].LastAt, "2026-09-26")
}

// A sub-heading inside the Outcome does not end it. The Outcome is prose, the
// close writes it with the sub-headings the Partner drafted, and the line that
// says the close was recorded is below them — so a reader that ended the section
// at the first of them would say this record was never sat on, and would date it
// from nothing.
func TestASubHeadingInsideTheOutcomeDoesNotEndIt(t *testing.T) {
	t.Parallel()
	counts, last, marked := satOn([]string{
		"## Outcome",
		"The limit counts from last activity.",
		"",
		"### Constraints",
		"",
		"The mobile client renews on its own clock.",
		"",
		"- Recorded from the sitting · 2026-09-26 · Wido [d:deposit:t9#0]",
		"",
		"## Scope",
		"- 2026-09-27 · Wido · not a pile at all [d:deposit:t9#1]",
	})
	testutil.Expect(t, "it is sitting material", marked, true)
	testutil.Expect(t, "dated from the foot under the sub-heading", last, "2026-09-26")
	// The Outcome holds no pile, and the section after it at its own level is not
	// the Outcome: a heading at the Outcome's level or higher still ends it.
	testutil.Expect(t, "and no pile was counted", counts, PileCounts{})
}

// The Outcome's date is the one its foot carries and nothing else: a human's own
// prose under the heading dates nothing, even where it carries a mark.
func TestOnlyTheOutcomesFootDatesIt(t *testing.T) {
	t.Parallel()
	_, last, marked := satOn([]string{
		"## Outcome",
		"- what somebody typed here themselves · with a dot in it [d:deposit:t9#0]",
	})
	testutil.Expect(t, "the mark is still a mark", marked, true)
	testutil.Expect(t, "but nothing is dated from prose", last, "")
}

// Newest entry first, and a tie is settled by the path so two reads are one
// order.
func TestSittingsAreNewestEntryFirst(t *testing.T) {
	t.Parallel()
	roots := selfHostedFixture(t)
	state := seed(t, roots)
	for _, one := range []struct {
		file, id, when string
	}{
		{"older", "design-older", "2026-09-20"},
		{"newer", "design-newer", "2026-09-26"},
		{"middle", "design-middle", "2026-09-23"},
	} {
		plant(t, roots.Checkout, state+"plans/designs/"+one.file+".md",
			record("A sitting on "+one.file, "design", one.id, "draft", "ledger-sync")+
				"\n## Facts\n\n"+entry(one.when, "Wido", "something recorded", "deposit:"+one.file+"#0")+"\n")
	}

	pane, err := ReadPane(roots, readAt)
	testutil.Require(t, "the pane was read", err, nil)
	testutil.Require(t, "three were sat on", len(pane.Sittings), 3)
	testutil.Expect(t, "newest first", pane.Sittings[0].Record.ID, "design-newer")
	testutil.Expect(t, "then the middle one", pane.Sittings[1].Record.ID, "design-middle")
	testutil.Expect(t, "then the oldest", pane.Sittings[2].Record.ID, "design-older")
}

// A sitting standing on a record nobody has recorded into yet is listed, and it
// is listed first: it is the sitting a human is in the middle of.
func TestAStandingSittingWithNoEntriesIsListedFirst(t *testing.T) {
	t.Parallel()
	roots := selfHostedFixture(t)
	state := seed(t, roots)
	plant(t, roots.Checkout, state+"plans/designs/recorded.md",
		record("A sitting with entries", "design", "design-recorded", "draft", "ledger-sync")+
			"\n## Facts\n\n"+entry("2026-09-26", "Wido", "something recorded", "deposit:t1#0")+"\n")
	plant(t, roots.Checkout, state+"plans/designs/fresh.md",
		record("A draft created for a sitting", "design", "design-fresh", "draft", "")+
			"\n## Outcome\n\n## Scope\n")

	pane, err := ReadPane(roots, readAt)
	testutil.Require(t, "the pane was read", err, nil)
	testutil.Require(t, "one was sat on", len(pane.Sittings), 1)

	pane.MarkStanding("metasystem/plans/designs/fresh.md")
	testutil.Require(t, "the standing one joins it", len(pane.Sittings), 2)
	first := pane.Sittings[0]
	testutil.Expect(t, "first, because it is the one being worked", first.Record.Path,
		"metasystem/plans/designs/fresh.md")
	testutil.Expect(t, "named from the records", first.Record.Title, "A draft created for a sitting")
	testutil.Expect(t, "with its kind", first.Record.Kind, "design")
	testutil.Expect(t, "no counts, because nothing was recorded", first.Counts, PileCounts{})
	testutil.Expect(t, "no last entry either", first.LastAt, "")
	testutil.Expect(t, "and it says a sitting stands on it", first.Standing, true)
	testutil.Expect(t, "the recorded one is still there", pane.Sittings[1].Record.ID, "design-recorded")
	testutil.Expect(t, "and no sitting stands on that", pane.Sittings[1].Standing, false)
}

// A sitting standing on a record that already carries entries marks the row it
// has rather than adding a second one for the same record.
func TestAStandingSittingMarksTheRowItAlreadyHas(t *testing.T) {
	t.Parallel()
	roots := selfHostedFixture(t)
	state := seed(t, roots)
	plant(t, roots.Checkout, state+"plans/designs/standing.md",
		record("A sitting in progress", "design", "design-standing", "draft", "ledger-sync")+
			"\n## Facts\n\n"+entry("2026-09-26", "Wido", "something recorded", "deposit:t1#0")+"\n")

	pane, err := ReadPane(roots, readAt)
	testutil.Require(t, "the pane was read", err, nil)
	pane.MarkStanding("metasystem/plans/designs/standing.md")
	testutil.Require(t, "one row and not two", len(pane.Sittings), 1)
	testutil.Expect(t, "marked as standing", pane.Sittings[0].Standing, true)
	testutil.Expect(t, "with its counts intact", pane.Sittings[0].Counts, PileCounts{Facts: 1})
}

// A subject this pane's records do not carry is still the record a human is
// sitting on, so it is listed by the little that is known rather than dropped.
func TestAStandingSittingOnARecordThePaneDoesNotCarryIsStillListed(t *testing.T) {
	t.Parallel()
	roots := selfHostedFixture(t)
	seed(t, roots)

	pane, err := ReadPane(roots, readAt)
	testutil.Require(t, "the pane was read", err, nil)
	pane.MarkStanding("plans/designs/nowhere.md")
	testutil.Require(t, "it is listed", len(pane.Sittings), 1)
	testutil.Expect(t, "by its path", pane.Sittings[0].Record.Path, "plans/designs/nowhere.md")
	testutil.Expect(t, "claiming no kind", pane.Sittings[0].Record.Kind, "")
	testutil.Expect(t, "and saying a sitting stands", pane.Sittings[0].Standing, true)

	pane.MarkStanding("   ")
	testutil.Expect(t, "and nothing is listed for no subject at all", len(pane.Sittings), 1)
}

// The mark is read as a mark and not as a bracket a human typed, and the heading
// is read at any level: a record that nests its piles under a section is still
// carrying them.
func TestTheMarkAndTheHeadingsAreReadAsThisBuildWritesThem(t *testing.T) {
	t.Parallel()
	counts, last, marked := satOn([]string{
		"### Facts",
		"- 2026-09-26 · Wido · something [d:deposit:t1#0]",
		"- 2026-09-25 · Wido · a line with [d: and no close",
		"#### open questions",
		"- 2026-09-27 · Wido · a question [d:deposit:t1#1]",
		"## Something else",
		"- 2026-09-28 · Wido · not a pile at all [d:deposit:t1#2]",
	})
	testutil.Expect(t, "both piles counted at their own levels", counts,
		PileCounts{Facts: 2, Questions: 1})
	testutil.Expect(t, "the latest marked entry dates it", last, "2026-09-27")
	testutil.Expect(t, "and it is sitting material", marked, true)

	_, _, none := satOn([]string{"## Facts", "- 2026-09-26 · Wido · a line with [d: and no close"})
	testutil.Expect(t, "an unclosed bracket is not a mark", none, false)
}
