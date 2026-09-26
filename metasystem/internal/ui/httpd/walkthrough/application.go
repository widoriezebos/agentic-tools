package main

// The Application section's own fixture: a synthetic known-issues register and
// the concluded work the page reads as weeks.
//
// The register is a real file this fixture plants in its checkout, exactly as
// the rulings register is, so the page reads it through the same reader the
// engine wires rather than through a canned payload: what a walkthrough proves
// about Known problems is then something about the reader as well as about the
// page.
//
// Both column sets are plantable, because there are two and a walkthrough that
// could only ever show one could only ever prove half of what the reader does.
// The kit writes `Symptom and evidence / Cost when it bites / Fix direction or
// lever`; the set an adoption ships writes `Issue / Consequence / Reopen when`.
// They are six columns in the same order of meaning, and the fifth is the one a
// reader must never rename, so -register chooses which one is on the screen.

import (
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goalbudget"
)

// The two column sets this fixture can plant.
const (
	registerKit     = "kit"
	registerAdopted = "adopted"
)

// applicationLastVisit is how long ago the walkthrough's previous visit to
// Application was.
//
// Four days, because that is where this fixture's conclusions fall either side
// of it: the newest week of concluded work lands inside the window and
// everything older falls outside it, so the block opens with dots on its first
// week and none on the five behind it — which is what the rule is for, and
// what a fixture with no previous visit could not show.
const applicationLastVisit = 4 * 24 * time.Hour

// fixtureKnownIssues is the known-issues register this fixture plants: a row of
// every shape the reader has to tell apart, under whichever column set is
// asked for.
//
// One open row, one of each of the five concluded status words, a status that
// only LOOKS like one of them, a cell holding an escaped pipe, and two rows
// with too few cells — so the block's unread line has something to count and
// the register is worth opening from it.
func fixtureKnownIssues(columns string, now time.Time) string {
	day := func(ago time.Duration) string { return now.Add(-ago).UTC().Format("2006-01-02") }
	header := "| Id | Date | Symptom and evidence | Cost when it bites | Fix direction or lever | Status |"
	if columns == registerAdopted {
		header = "| Id | Date | Issue | Consequence | Reopen when | Status |"
	}
	rows := []string{
		"| KI-1 | " + day(50*24*time.Hour) + " | The census costs 475ms on a busy laptop, six of it in separate lsof calls | A scan can outrun its own interval and refuse every dispatch behind it | Batch the cwd resolution into one call for every candidate | FIXED " + day(48*24*time.Hour) + ": census 442ms to 214ms on a 1230-process laptop |",
		"| KI-2 | " + day(49*24*time.Hour) + " | The suite's wall time grew from 2m14s to 4m38s across the same twenty items on one machine | CI cost, and a suite nobody runs locally stops catching environment defects | Profile the fixture set and split the fast suites from the slow ones | ACCEPTED " + day(47*24*time.Hour) + " with a measured trigger: a run exceeding 1.5x the recorded time prints a notice, never a failure |",
		"| KI-3 | " + day(45*24*time.Hour) + " | A supervision fixture timed out once under load in a delegate's environment; two clean runs here at 4m30s each | A flaky gate erodes trust in the suite and hides real regressions | Make the arming wait proportional to the measured census rather than a fixed cap | RETIRED " + day(44*24*time.Hour) + ": the load-immunity rework addressed the cause; a recurrence opens its own row |",
		"| KI-4 | " + day(40*24*time.Hour) + " | The register reader skipped every row holding an escaped pipe, such as `\\|\\| true` in a code span | A page claims this project knows about fewer defects than it does | Split on unescaped pipes only, and count the rows that are still wrong | RESOLVED " + day(39*24*time.Hour) + " |",
		"| KI-5 | " + day(35*24*time.Hour) + " | The watcher logged every tick at info level, four lines a second on an idle machine | Log noise nobody reads, which trains a reader to ignore the file | Move the tick to debug and keep the state changes at info | CLOSED " + day(34*24*time.Hour) + " |",
		"| KI-6 | " + day(30*24*time.Hour) + " | Evidence mirroring fails silently: every `mirror_record` call site swallows failure with `\\|\\| true`, so an unfilled evidence root means nothing is ever mirrored | Paid raws live only in a gitignored directory that `git clean` reaches, so the only copy of expensive evidence can vanish | Surface a failed mirror as a watcher-visible condition rather than a log line | OPEN |",
		"| KI-7 | " + day(25*24*time.Hour) + " | The runtime identity hash treats a CLI's self-written state as configuration, so any interactive use forces a fresh probe | Three spurious re-probes in one day, one of them misblamed on an unrelated install | Hash a filtered view that drops the CLI's own bookkeeping sections | FIX SHIPPED, PROOF INCOMPLETE " + day(24*24*time.Hour) + ": the fix is merged and correct in every observation; the 20-iteration harness proof is still owed |",
		"| KI-8 | " + day(20*24*time.Hour) + " | Two main agents in one repository share a working tree with no coordination contract | Either session can scoop the other's half-written edits into a commit | One main per checkout; the durable fix is an advisory claim on the working tree | OPEN, one observation |",
		"| KI-9 | Design-critic follow-up rounds review a stale tree: the worktree freezes at round-1 dispatch and follow-ups never sync it | OPEN |",
		"| KI-10 | " + day(15*24*time.Hour) + " | Delegate returns echo the orchestrator's session id | OPEN |",
	}
	return "# Known Issues\n\n" +
		"A standing register of defects and limitations that are recorded but not scheduled: " +
		"capability ceilings, accepted trade-offs, and dead ends that must not be silently retried.\n\n" +
		header + "\n| --- | --- | --- | --- | --- | --- |\n" +
		strings.Join(rows, "\n") + "\n"
}

// concludedWeeks is thirty concluded goals across six weeks, which is what
// makes the block weeks rather than a list.
//
// The dates are stamped back from this process's own clock rather than written
// down, because a fixed date drifts out of every week the day after it is
// written and the walkthrough would then show six groups nobody can read as
// "this week" and "the one before". Five a week for six weeks, with the labels
// repeating so a chip narrows to a family rather than to one row, one arc so
// an open row has one to show, and one conclusion that records an
// administrative end rather than something that shipped — which is the whole
// reason the block is called what concluded.
var concludedWeeks = []struct {
	id       string
	intent   string
	conclude string
	labels   []string
	arc      string
	// weeksAgo is which week this conclusion falls in, and dayIn its place
	// within that group's one day, so the six groups each hold five rows
	// however this fixture is run.
	weeksAgo int
	dayIn    int
}{
	{"g0-s30", "The Decisions inbox collapses into groups", "landed in 9017baa. One line per group, one group open, and the queue worked in a sitting.", []string{"browser-interface"}, "", 0, 0},
	{"g0-s29", "The goal editor saves a queued goal in place", "landed in f57c242. The sheet prefills from the row and the engine's own allowlist refuses the rest.", []string{"browser-interface"}, "", 0, 1},
	{"g0-s28", "The document reader anchors a heading", "landed in b50bd8d. A heading is an address, and the outline walks it.", []string{"browser-interface"}, "", 0, 2},
	{"g0-s27", "The notification stream reconnects by itself", "landed in 519fdb4. One EventSource, reconnected by the browser, and no timer anywhere.", []string{"browser-interface", "robustness"}, "", 0, 3},
	{"g0-s26", "A machine publishes its phase with every tick", "landed in 6d9529d. The presence record carries the chain, and the fleet page reads it.", []string{"headless-fleet"}, "", 0, 4},
	{"g0-s25", "The label chips are drawn from the rows on screen", "landed in 2ad91f0. Four chips on the line and the rest on asking.", []string{"browser-interface"}, "", 1, 0},
	{"g0-s24", "Selecting many goals approves them one at a time", "landed in 44de0a1. One publication per goal, and a refusal stops the rest.", []string{"browser-interface", "robustness"}, "", 1, 1},
	{"g0-s23", "The header counts what is asked of you and what waits", "landed in 9cc2e31. Two counts that sum to the inbox, because the page splits one list.", []string{"browser-interface"}, "", 1, 2},
	{"g0-s22", "The queue row opens in place and shows the whole intent", "landed in 7b1c9de. No buttons on a line; the acts are in the open row.", []string{"browser-interface"}, "", 1, 3},
	{"g0-s21", "A second reader for the census format", "Obsolete: self-declared duplicate holding no work, absorbed into g0-s16.", []string{}, "", 1, 4},
	{"g0-s20", "The seat census answers which machines are alive", "landed in 1f0aa54. One scan, batched, and a verdict with the instant it was made.", []string{"headless-fleet"}, "", 2, 0},
	{"g0-s19", "The fleet page reads a seat's whole chain", "landed in 3cc81de. The newest non-terminal chain, with the job and the round.", []string{"headless-fleet"}, "", 2, 1},
	{"g0-s18", "A stopped seat says why it stopped, in the engine's words", "landed in 8ad0192. The fence's own reason, carried rather than reworded.", []string{"headless-fleet", "robustness"}, "", 2, 2},
	{"g0-s17", "The launch sheet proposes a destination as the nickname is typed", "landed in 0c1e9ab. The parent directory and the remote name, joined on the page.", []string{"headless-fleet"}, "", 2, 3},
	{"g0-s16", "The presence copy says where it came from", "landed in 5d2c7fe. The interface's own namespace, the tick's copy, or local refs.", []string{"headless-fleet"}, "", 2, 4},
	{"g0-s15", "The board reorders inside a priority band", "landed in a91f3cd. The engine renumbers the band, so the act is never about one record.", []string{"browser-interface"}, "covenant-harvest", 3, 0},
	{"g0-s14", "The goal page reads the whole record", "landed in cc90b12. Every field the projection carries, and its history beneath it.", []string{"browser-interface"}, "covenant-harvest", 3, 1},
	{"g0-s13", "Approve and withdraw publish from the browser", "landed in 4ee1a07. Under the hand that reached the route, and never otherwise.", []string{"browser-interface"}, "", 3, 2},
	{"g0-s12", "The Project section reads the checkout", "landed in 77b3c41. A kind at a time, out of the homes the resolver names.", []string{"browser-interface"}, "", 3, 3},
	{"g0-s11", "A second bundler beside the first", "Withdrawn: the requirement was absorbed into g0-s06 before any work began.", []string{}, "", 3, 4},
	{"g0-s10", "The Overview reads what needs a human", "landed in 1b4da77. Five readers, one composition, and a window per human.", []string{"browser-interface"}, "", 4, 0},
	{"g0-s09", "The fleet channel gateway opens", "landed in e07c518. A seat asks, a human answers, and the answer carries their own code.", []string{"headless-fleet"}, "", 4, 1},
	{"g0-s08", "The steward delivers a notification once", "landed in 2f19b60. The journal is the record of every attempt, refusals included.", []string{"robustness"}, "", 4, 2},
	{"g0-s07", "The interface signs a human in with a one-time code", "landed in 9de4a13. A floor on the step, so a code cannot be replayed.", []string{"browser-interface", "robustness"}, "", 4, 3},
	{"g0-s06", "The seat roster is read from the registry", "landed in 5a7cd90. One roster, read where it is written.", []string{"headless-fleet"}, "", 4, 4},
	{"g0-s05", "The frontend toolchain and the committed bundle", "landed in cbb4e2f. The bundle is committed inside the package that embeds it.", []string{"browser-interface"}, "", 5, 0},
	{"g0-s04", "The application shell, the rail and the header", "landed in 3a01c8d. One rail, one header, and a work area beneath them.", []string{"browser-interface"}, "", 5, 1},
	{"g0-s03", "The backlog's data path and the list", "landed in d1e2f03. One observation per request, and no second capture anywhere.", []string{"browser-interface"}, "", 5, 2},
	{"g0-s02", "The accepted ledger is fetched on a loop", "landed in 60ca4b8. The loop keeps it current, so no page ever polls to stay so.", []string{"robustness"}, "", 5, 3},
	{"g0-s01", "The workspace says which application it is", "landed in 4c7e2b1. The layout decides the mode and the adoption line the provenance.", []string{}, "", 5, 4},
}

// addConcludedWeeks plants those thirty rows in the canned tree.
func addConcludedWeeks(tree *goal.TreeGoals, now time.Time) {
	for _, one := range concludedWeeks {
		row := walkthroughGoal(one.id, goal.StateDone, one.intent)
		row.Labels = one.labels
		row.Arc = one.arc
		row.Budget = &goalbudget.Budget{
			ElapsedLimit: "4h", AttemptLimit: 6, ReservedJobMinutesLimit: 720, ActiveJobLimit: 1, ReviewRoundLimit: 2,
		}
		// Every row of one group lands on ONE day, a day and a half back and
		// then a week at a time, so each group is five rows inside one Monday
		// to Sunday however late in the day this fixture is started.
		at := now.Add(-time.Duration(one.weeksAgo)*7*24*time.Hour - 36*time.Hour - time.Duration(one.dayIn)*time.Hour)
		concluded(row, at)
		row.Conclude = one.conclude
		tree.Done[row.Id] = row
	}
}

// The three documents the Application page's "What it is" links to.
//
// They are short on purpose: what the block proves in a browser is that the
// link opens the checkout's own document in the reader, and a fixture that
// carried none of them could only ever show the line that says so.
const (
	walkthroughReadme = `# walkthrough

The fixture application this interface is walked through against. It records
its work in a ledger, keeps its defects in a register, and serves this
interface over both.

## What it is for

Standing in front of a page with real readers behind it, on a canned
workspace, so that what a walkthrough proves is something about the interface
rather than about a payload somebody typed.
`
	walkthroughConcepts = `# Concepts

The words this workspace uses in its own way.

## Goal

One unit of work with an intent, a state and, when it ends, a conclusion. A
conclusion records an end — sometimes an administrative one — and dates that
end rather than a capability that still stands.

## Register

A table of rows this project keeps by hand: the rulings it has made, and the
defects and limitations it has accepted.
`
	walkthroughGlossary = `# Glossary

- **Arc**: a family of goals planned together, named on each of its members.
- **Box**: the limits one goal's work is admitted under.
- **Seat**: one machine of the fleet, with its own presence record.
- **Tier**: how much a goal is allowed to cost before a human sees it again.
`
)
