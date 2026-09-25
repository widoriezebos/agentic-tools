package main

// The Decisions section's own fixture: a synthetic rulings register and this
// seat's open channel asks.
//
// The register is a real file this fixture plants in its checkout, so the page
// reads it through the same reader the steward's sweep uses rather than
// through a canned payload: what a walkthrough proves about the Rulings tab
// is then something about the reader as well as about the page.

import (
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goalbudget"
)

// fixtureRulings is eight rulings and one broken row.
//
// One of each shape the tab has to render: all four schedulable classes, one
// review already due and two still inside their window, an event condition
// nothing here judges, two rulings that stand with no condition at all, a
// prose condition the grammar refuses, an ownerless row, and a row with the
// wrong number of columns — so the defects line has something to say — plus
// rulings whose words name goals of the canned ledger, which is what a
// mention chip is drawn from.
func fixtureRulings(calm bool, now time.Time) string {
	day := func(ago time.Duration) string { return now.Add(-ago).UTC().Format("2006-01-02") }
	ahead := func(in time.Duration) string { return now.Add(in).UTC().Format("2006-01-02") }
	rows := []string{
		"| R-1 | " + day(40*24*time.Hour) + " | Two rosters exist and must never be confused: development work names its models in the seat's own configuration, and a benchmark run hosts on the cheap tier | Given after three healthy jobs were cancelled under a benchmark's cost ceiling | Wido |  |",
		"| R-2 | " + day(30*24*time.Hour) + " | The browser interface's frontend is the one exception to the Go-only rule: its React and TypeScript source, its npm toolchain, and the built bundle committed inside the package that embeds it | Given when g1-s8 landed the toolchain | Wido |  |",
		"| R-3 | " + day(20*24*time.Hour) + " | The board's two drag moves are the only acts the browser publishes, and g1-s12 is where they land | Given with the board design | Wido | class=temporary due=" + day(6*24*time.Hour) + " |",
		"| R-4 | " + day(14*24*time.Hour) + " | Model and effort choices within the recorded lanes are the dispatch delegate's judgement, on one condition: every choice is a thought-through decision recorded where it lands | Given after the lane map was confirmed | Wido | class=delegated-authority due=" + ahead(30*24*time.Hour) + " |",
		"| R-5 | " + day(12*24*time.Hour) + " | The steward runs the ruling sweep at most once a day, and rows rotate behind a five-item attention ceiling | Given when the digest first overflowed | Wido | class=experimental due=" + ahead(60*24*time.Hour) + " |",
		"| R-6 | " + day(10*24*time.Hour) + " | Same-family agreement does not authorize a consequence by itself; the executable review policy stays empty until the first measured report exists | Given during the post-mortem dispatch | Wido | class=assumption-dependent event=first-measured-report-exists |",
		"| R-7 | " + day(8*24*time.Hour) + " | Commit and push freely on the interface branch, and append a receipt in the same commit as the work it describes | Given on the first day of the interface arc | Wido | standing |",
		"| R-8 | " + day(5*24*time.Hour) + " | A temporary enrolment by remote word carries the word and the date on the identity record, and the human re-arms at a terminal | Given from away, a week before the terminal | | class=temporary due=" + day(2*24*time.Hour) + " |",
		"| R-9 | too | few | columns |",
	}
	if calm {
		// The calm workspace is the page's good outcome: no review has come
		// due, no row is ownerless, and no row is malformed.
		rows = []string{
			"| R-1 | " + day(40*24*time.Hour) + " | Two rosters exist and must never be confused: development work names its models in the seat's own configuration, and a benchmark run hosts on the cheap tier | Given after three healthy jobs were cancelled under a benchmark's cost ceiling | Wido |  |",
			"| R-2 | " + day(30*24*time.Hour) + " | The browser interface's frontend is the one exception to the Go-only rule | Given when g1-s8 landed the toolchain | Wido |  |",
			"| R-4 | " + day(14*24*time.Hour) + " | Model and effort choices within the recorded lanes are the dispatch delegate's judgement | Given after the lane map was confirmed | Wido | class=delegated-authority due=" + ahead(30*24*time.Hour) + " |",
			"| R-5 | " + day(12*24*time.Hour) + " | The steward runs the ruling sweep at most once a day | Given when the digest first overflowed | Wido | class=experimental due=" + ahead(60*24*time.Hour) + " |",
		}
	}
	return "# Standing rulings register\n\n" +
		"One entry per human ruling: id, date, the ruling as close to verbatim as\n" +
		"the session record allows, context, accountable owner, and an optional\n" +
		"review condition.\n\n" +
		"| ID | Date | Decision | Evidence | Owner | Review |\n|---|---|---|---|---|---|\n" +
		strings.Join(rows, "\n") + "\n"
}

// fixtureAsks is the two channel questions this seat is holding.
//
// They are values rather than files, because this fixture has no channel to
// open one on and the page reads them through the same shape the engine hands
// it. One carries a proposed budget and one does not, which are the two
// shapes the row has to render, and both carry the options, their
// consequences and a recommendation, which the seat-communication law
// requires of every ask.
func fixtureAsks(calm bool, now time.Time) []channel.Question {
	if calm {
		return nil
	}
	return []channel.Question{
		{
			ID: "q-budget-g1-s21", Goal: "g1-s21", Kind: "budget-above-norm", Machine: "m1e",
			OpenedAt: now.Add(-6 * time.Hour), State: "open",
			Facts: []string{
				"the second attempt reached the elapsed limit with the pane half built",
				"what is left is the empty state and the phone layout",
			},
			Wants: "one more review round and two more hours",
			Options: []channel.Option{
				{Label: "raise", Consequence: "the goal runs on under the larger box and the arc slips by an afternoon"},
				{Label: "stop", Consequence: "the goal parks with what is built and the slice is re-cut"},
			},
			Recommendation: "raise it once; what is left is small and well understood",
			Budget: &goalbudget.Budget{
				ElapsedLimit: "8h", AttemptLimit: 10, ReservedJobMinutesLimit: 1200,
				ActiveJobLimit: 1, ReviewRoundLimit: 3,
			},
		},
		{
			ID: "q-decision-g1-s22", Goal: "g1-s22", Kind: "decision", Machine: "m2a",
			OpenedAt: now.Add(-28 * time.Hour), State: "open",
			Facts: []string{
				"the census format has two readers and they disagree about the seat key",
				"nothing has been written to the registry under either spelling",
			},
			Wants: "the seat key spelled as machine+lineage, or as the machine alone",
			Options: []channel.Option{
				{Label: "machine+lineage", Consequence: "two seats on one host are told apart and every existing record is rewritten"},
				{Label: "machine", Consequence: "the existing records stand and two seats on one host collide"},
			},
			Recommendation: "machine+lineage; the rewrite is mechanical and the collision is not",
		},
	}
}
