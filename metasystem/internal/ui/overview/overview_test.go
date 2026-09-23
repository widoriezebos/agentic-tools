package overview

import (
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/backlog"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/notifications"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/project"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/snapshot"
)

// The composition rules, one test per rule the design states.
//
// Every fixture is stamped relative to `now` below rather than written as a
// date, so a test that passes today passes next year: every rule here is about
// a window, and a fixed date drifts out of one.

var now = time.Date(2026, 9, 23, 14, 37, 0, 0, time.UTC)

// ago is an instant this long before the tests' now, as a record writes one.
func ago(d time.Duration) string { return now.Add(-d).Format(time.RFC3339) }

// visitedYesterday is the window the tests that do not care about the window
// use: the previous visit ended last evening.
var visitedYesterday = now.Add(-20 * time.Hour)

func goalRow(id string, lane backlog.Lane, intent string) backlog.Row {
	return backlog.Row{
		Ref:   backlog.Ref{Kind: "goal", ID: id, Revision: 3},
		Where: backlog.WhereLive, Lane: lane, Intent: intent,
		Priority: 2, Sequence: 1, OpenedAt: ago(30 * 24 * time.Hour),
		Labels: []string{}, BlockedBy: []string{}, OpenBlockers: []string{}, Gaps: []string{},
	}
}

func ranked(row backlog.Row, priority uint8, sequence uint64) backlog.Row {
	row.Priority, row.Sequence = priority, sequence
	return row
}

func approved(row backlog.Row) backlog.Row {
	row.Approved = &backlog.Approval{By: "human:Wido", At: ago(3 * 24 * time.Hour), Authority: "proven"}
	return row
}

func record(kind, id, status string, goals ...string) project.Record {
	return project.Record{
		Kind: kind, ID: id, Status: status, Goals: goals,
		Title: id, Path: "plans/" + kind + "s/" + id + ".md", Home: "plans/" + kind + "s",
		Slices: []string{},
	}
}

// composed is Compose over one busy workspace, which most of the rules below
// read one block out of.
func composed(t *testing.T, change func(*Inputs)) Page {
	t.Helper()
	in := busy()
	if change != nil {
		change(&in)
	}
	return Compose(in, now)
}

// busy is a workspace with something in every block.
func busy() Inputs {
	return Inputs{
		Project: project.Pane{
			SchemaVersion: project.SchemaVersion,
			Goals: []project.Goal{
				{ID: "g1-s9", Title: "g1-s9", State: "done"},
				{ID: "g1-s10", Title: "g1-s10", State: "done"},
				{ID: "g1-s12", Title: "g1-s12", State: "queued"},
				{ID: "g1-s15", Title: "g1-s15", State: "claimed"},
			},
			Records: []project.Record{
				record("design", "design-shell", "accepted", "g1-s9", "g1-s10"),
				record("design", "design-reader", "accepted", "g1-s9", "g1-s12"),
				record("design", "design-board", "draft", "g1-s12"),
				record("design", "design-old", "done", "g1-s9"),
				record("design", "design-unbound", "accepted"),
				record("decision", "decision-one-binary", "accepted"),
				record("decision", "decision-answering", "draft", "g1-s15"),
			},
			Intent: project.Book{
				Index: &project.Record{Kind: "intent", ID: "intent-index",
					Summary: "The MetaSystem is the machinery a human runs a fleet with. It exists so one person can hold the intent."},
				Chapters: []project.Chapter{{ID: "a"}, {ID: "b"}, {ID: "c"}},
			},
			Doctrine: project.Book{
				Index:    &project.Record{Kind: "doctrine", ID: "doctrine-index", Summary: "The rules every design is held to."},
				Chapters: []project.Chapter{{ID: "d"}, {ID: "e"}},
			},
			Questions: []project.Question{
				{ID: "q-old", Opened: ago(20 * 24 * time.Hour), Question: "Old", Status: "open", Goals: []string{}},
				{ID: "q-new", Opened: ago(2 * time.Hour), Question: "New", Status: "open", Goals: []string{}},
				{ID: "q-shut", Opened: ago(time.Hour), Question: "Answered", Status: "answered", Goals: []string{}},
			},
			Problems: []project.Problem{},
		},
		Rows: []backlog.Row{
			ranked(goalRow("g1-s12", backlog.LaneToDo, "The board"), 2, 1),
			ranked(goalRow("g1-s18", backlog.LaneToDo, "The application section"), 1, 1),
			approved(ranked(goalRow("g1-s14", backlog.LaneReady, "The Fleet section"), 2, 5)),
			goalRow("g1-s15", backlog.LaneInProgress, "The Decisions section"),
		},
		Closed:  []backlog.Row{},
		Counts:  map[backlog.Lane]int{backlog.LaneToDo: 2, backlog.LaneReady: 1, backlog.LaneInProgress: 1},
		Ledger:  Ledger{Freshness: snapshot.FreshnessCurrent, AtTip: true, SyncedAt: now.Add(-time.Minute)},
		Journal: []notifications.Notice{},
		Human:   Standing{Proven: true},
		Since:   visitedYesterday,
	}
}

/* ------------------------------------------------------------- needs you -- */

// The approvals row is the goals nobody has said yes to, in the order a seat
// would take them: the band first, then the position in it.
func TestGoalsAwaitingApprovalAreRankedAndCappedAtThree(t *testing.T) {
	t.Parallel()

	page := composed(t, func(in *Inputs) {
		in.Rows = []backlog.Row{
			ranked(goalRow("second", backlog.LaneToDo, "b"), 2, 1),
			ranked(goalRow("fourth", backlog.LaneToDo, "d"), 2, 3),
			ranked(goalRow("first", backlog.LaneToDo, "a"), 1, 1),
			ranked(goalRow("third", backlog.LaneToDo, "c"), 2, 2),
			// Ranked in no band at all, which is after every band and not
			// before band 1.
			ranked(goalRow("last", backlog.LaneToDo, "e"), 0, 0),
			// Carries an approval, so it is not waiting on one.
			approved(ranked(goalRow("admitted", backlog.LaneToDo, "f"), 1, 2)),
			// Another lane entirely.
			ranked(goalRow("elsewhere", backlog.LaneReady, "g"), 1, 3),
		}
	})

	testutil.Expect(t, "how many are waiting on an approval", page.NeedsYou.Approvals.Count, 5)
	testutil.Require(t, "how many are shown", len(page.NeedsYou.Approvals.Items), 3)
	testutil.Expect(t, "the first three, by band then position",
		[]string{
			page.NeedsYou.Approvals.Items[0].ID,
			page.NeedsYou.Approvals.Items[1].ID,
			page.NeedsYou.Approvals.Items[2].ID,
		},
		[]string{"first", "second", "third"})
	testutil.Expect(t, "where the first one opens",
		page.NeedsYou.Approvals.Items[0].Where, Where{Kind: WhereGoal, ID: "first"})
}

func TestOpenQuestionsAreNewestFirst(t *testing.T) {
	t.Parallel()

	page := composed(t, nil)

	testutil.Expect(t, "how many are open", page.NeedsYou.Questions.Count, 2)
	testutil.Require(t, "how many are shown", len(page.NeedsYou.Questions.Items), 2)
	testutil.Expect(t, "the newest first", page.NeedsYou.Questions.Items[0].ID, "q-new")
	testutil.Expect(t, "then the older", page.NeedsYou.Questions.Items[1].ID, "q-old")
	testutil.Expect(t, "where one opens",
		page.NeedsYou.Questions.Items[0].Where, Where{Kind: WhereQuestion, ID: "q-new"})
}

func TestDraftsAreEveryRecordDeclaringDraft(t *testing.T) {
	t.Parallel()

	page := composed(t, nil)

	testutil.Expect(t, "how many drafts", page.NeedsYou.Drafts.Count, 2)
	testutil.Require(t, "how many are shown", len(page.NeedsYou.Drafts.Items), 2)
	testutil.Expect(t, "a draft of any kind is one",
		[]string{page.NeedsYou.Drafts.Items[0].ID, page.NeedsYou.Drafts.Items[1].ID},
		[]string{"design-board", "decision-answering"})
	testutil.Expect(t, "the kind is the note beside it", page.NeedsYou.Drafts.Items[1].Note, "decision")
	testutil.Expect(t, "where a draft opens",
		page.NeedsYou.Drafts.Items[0].Where,
		Where{Kind: WhereDocument, ID: "plans/designs/design-board.md"})
}

// A design is waiting on a human when every goal it names has landed and
// nobody has closed it. The three near misses are each a design that is not.
func TestDesignsWhoseWorkHasAllLanded(t *testing.T) {
	t.Parallel()

	page := composed(t, nil)

	testutil.Expect(t, "how many designs are waiting to be closed", page.NeedsYou.Designs.Count, 1)
	testutil.Require(t, "how many are shown", len(page.NeedsYou.Designs.Items), 1)
	testutil.Expect(t, "the one whose goals all landed", page.NeedsYou.Designs.Items[0].ID, "design-shell")
	testutil.Expect(t, "why it is here", page.NeedsYou.Designs.Items[0].Note, "every goal landed")
}

func TestADesignNamingAGoalTheCheckoutCannotSeeHasNotLanded(t *testing.T) {
	t.Parallel()

	page := composed(t, func(in *Inputs) {
		in.Project.Records = []project.Record{record("design", "design-stranger", "accepted", "g1-s9", "nobody-knows")}
	})

	testutil.Expect(t, "a design naming an unknown goal", page.NeedsYou.Designs.Count, 0)
}

func TestAlertsAndHandoffsInsideSevenDays(t *testing.T) {
	t.Parallel()

	page := composed(t, func(in *Inputs) {
		in.Journal = []notifications.Notice{
			{ID: "n1", At: ago(10 * time.Minute), Source: "handoff", Message: "your turn", Delivered: true},
			{ID: "n2", At: ago(2 * time.Hour), Source: "alert", Message: "unhealthy", Delivered: true},
			{ID: "n3", At: ago(3 * time.Hour), Source: "steward", Message: "reaped 3 workers", Delivered: true},
			{ID: "n4", At: ago(6 * 24 * time.Hour), Source: "alert", Message: "older alert", Delivered: true},
			{ID: "n5", At: ago(8 * 24 * time.Hour), Source: "alert", Message: "outside the window", Delivered: true},
			{ID: "n6", At: "not a stamp", Source: "alert", Message: "undated", Delivered: true},
		}
	})

	testutil.Expect(t, "how many are addressed to a human and recent", page.NeedsYou.Alerts.Count, 3)
	testutil.Require(t, "how many are shown", len(page.NeedsYou.Alerts.Items), 3)
	testutil.Expect(t, "the journal's own order is kept",
		[]string{
			page.NeedsYou.Alerts.Items[0].ID,
			page.NeedsYou.Alerts.Items[1].ID,
			page.NeedsYou.Alerts.Items[2].ID,
		},
		[]string{"n1", "n2", "n4"})
	testutil.Expect(t, "an alert opens the panel at its own row",
		page.NeedsYou.Alerts.Items[0].Where, Where{Kind: WhereNotification, ID: "n1"})
}

func TestSignInIsTrueWhenNothingProvesAHuman(t *testing.T) {
	t.Parallel()

	testutil.Expect(t, "a proven seat", composed(t, nil).NeedsYou.SignIn, false)
	testutil.Expect(t, "a seat nothing proves",
		composed(t, func(in *Inputs) { in.Human = Standing{} }).NeedsYou.SignIn, true)
}

// Nothing needs you is Total zero: the sign-in row is a row of its own and is
// not counted into it.
func TestNothingNeedsYou(t *testing.T) {
	t.Parallel()

	page := composed(t, func(in *Inputs) {
		in.Project.Records = []project.Record{}
		in.Project.Questions = []project.Question{}
		in.Rows = []backlog.Row{approved(goalRow("g1-s14", backlog.LaneReady, "The Fleet section"))}
		in.Human = Standing{}
	})

	testutil.Expect(t, "the total", page.NeedsYou.Total, 0)
	testutil.Expect(t, "and the sign-in row beside it", page.NeedsYou.SignIn, true)
}

/* ----------------------------------------------------- since the last visit -- */

func TestWhatChangedSinceTheWindowOpened(t *testing.T) {
	t.Parallel()

	page := composed(t, func(in *Inputs) {
		concluded := goalRow("g1-s9", backlog.LaneDone, "The application shell")
		concluded.DoneAt = ago(6 * time.Hour)
		older := goalRow("g1-s8", backlog.LaneDone, "The toolchain")
		older.DoneAt = ago(40 * 24 * time.Hour)
		moved := goalRow("g1-s15", backlog.LaneInProgress, "The Decisions section")
		moved.LastChangeAt, moved.LastVerb = ago(90*time.Minute), "claim"
		still := goalRow("g1-s12", backlog.LaneToDo, "The board")
		still.LastChangeAt, still.LastVerb = ago(30*24*time.Hour), "open"

		in.Rows = []backlog.Row{moved, still}
		in.Closed = []backlog.Row{concluded, older}
		in.Project.Records = []project.Record{
			changedAt(record("design", "design-shell", "accepted", "g1-s9"), ago(3*time.Hour)),
			changedAt(record("design", "design-board", "accepted", "g1-s12"), ago(9*24*time.Hour)),
			record("design", "design-undated", "accepted", "g1-s12"),
		}
		in.Journal = []notifications.Notice{
			{ID: "n1", At: ago(time.Hour), Source: "steward", Message: "advanced", Delivered: true},
			{ID: "n2", At: ago(5 * time.Hour), Source: "steward", Message: "reaped", Delivered: true},
			{ID: "n3", At: ago(40 * time.Hour), Source: "steward", Message: "armed", Delivered: true},
		}
	})

	testutil.Expect(t, "goals concluded in the window", page.Changed.Concluded.Count, 1)
	testutil.Require(t, "the concluded shown", len(page.Changed.Concluded.Items), 1)
	testutil.Expect(t, "which one", page.Changed.Concluded.Items[0].ID, "g1-s9")
	testutil.Expect(t, "goals that moved", page.Changed.Moved.Count, 1)
	testutil.Require(t, "the moved shown", len(page.Changed.Moved.Items), 1)
	testutil.Expect(t, "the ledger's last verb", page.Changed.Moved.Items[0].Note, "claim")
	testutil.Expect(t, "and when", page.Changed.Moved.Items[0].At, ago(90*time.Minute))
	testutil.Expect(t, "records written in the window", page.Changed.Records.Count, 1)
	testutil.Require(t, "the records shown", len(page.Changed.Records.Items), 1)
	testutil.Expect(t, "which record", page.Changed.Records.Items[0].ID, "design-shell")
	testutil.Expect(t, "the steward's messages", page.Changed.Messages, 2)
	testutil.Expect(t, "the whole of it", page.Changed.Total, 5)
}

// A goal that concluded in the window is in the concluded list and not also in
// the moved one: concluding is the change, and counting it twice would make
// the page's arithmetic disagree with itself.
func TestAConcludedGoalIsNotAlsoAGoalThatMoved(t *testing.T) {
	t.Parallel()

	page := composed(t, func(in *Inputs) {
		row := goalRow("g1-s9", backlog.LaneDone, "The application shell")
		row.DoneAt, row.LastChangeAt, row.LastVerb = ago(6*time.Hour), ago(6*time.Hour), "done"
		in.Rows = []backlog.Row{row}
		in.Closed = []backlog.Row{}
	})

	testutil.Expect(t, "concluded", page.Changed.Concluded.Count, 1)
	testutil.Expect(t, "moved", page.Changed.Moved.Count, 0)
}

func TestTheChangedListsAreCappedAtFiveWithTheWholeCount(t *testing.T) {
	t.Parallel()

	page := composed(t, func(in *Inputs) {
		rows := []backlog.Row{}
		for _, id := range []string{"a", "b", "c", "d", "e", "f", "g"} {
			row := goalRow(id, backlog.LaneDone, id)
			row.DoneAt = ago(time.Hour)
			rows = append(rows, row)
		}
		in.Rows, in.Closed = rows, []backlog.Row{}
	})

	testutil.Expect(t, "the whole count", page.Changed.Concluded.Count, 7)
	testutil.Expect(t, "how many are shown", len(page.Changed.Concluded.Items), 5)
}

/* -------------------------------------------------------------- work now -- */

func TestWorkNowReadsTheLanesTheBoardPlaced(t *testing.T) {
	t.Parallel()

	page := composed(t, func(in *Inputs) {
		claimed := goalRow("g1-s15", backlog.LaneInProgress, "The Decisions section")
		claimed.Phase = backlog.PhaseNotRecorded
		claimed.Claim = &backlog.Claim{Machine: "m1e", Lineage: "coordinator", At: ago(5 * time.Hour)}
		held := goalRow("g1-s22", backlog.LaneWaiting, "The Fleet census")
		held.Waiting = &backlog.Waiting{Reason: "the format is still being decided", Since: ago(30 * time.Hour)}
		newer := goalRow("g1-s23", backlog.LaneWaiting, "The seat page")
		newer.Waiting = &backlog.Waiting{Reason: "waiting on the census", Since: ago(2 * time.Hour)}
		blocked := goalRow("g1-s24", backlog.LaneWaiting, "The job page")
		blocked.OpenBlockers = []string{"g1-s22"}
		blocked.Waiting = &backlog.Waiting{From: "approved"}
		concluded := goalRow("g1-s9", backlog.LaneDone, "The shell")
		concluded.DoneAt = now.Add(-3 * time.Hour).Format(time.RFC3339)
		yesterday := goalRow("g1-s8", backlog.LaneDone, "The toolchain")
		yesterday.DoneAt = now.Add(-26 * time.Hour).Format(time.RFC3339)

		in.Rows = []backlog.Row{
			claimed, held, newer, blocked,
			approved(ranked(goalRow("g1-s14", backlog.LaneReady, "The Fleet section"), 1, 1)),
			approved(ranked(goalRow("g1-s16", backlog.LaneReady, "The Overview"), 2, 1)),
			approved(ranked(goalRow("g1-s17", backlog.LaneReady, "Decisions"), 2, 2)),
			approved(ranked(goalRow("g1-s20", backlog.LaneReady, "Application"), 3, 1)),
		}
		in.Closed = []backlog.Row{concluded, yesterday}
		in.Counts = map[backlog.Lane]int{
			backlog.LaneToDo: 5, backlog.LaneReady: 4, backlog.LaneInProgress: 1,
			backlog.LaneReview: 1, backlog.LaneWaiting: 3, backlog.LaneDone: 2,
		}
	})

	testutil.Require(t, "how many are in progress", len(page.Work.InProgress), 1)
	testutil.Expect(t, "the seat that holds it", page.Work.InProgress[0].Seat,
		Seat{Machine: "m1e", Lineage: "coordinator"})
	testutil.Expect(t, "the phase", page.Work.InProgress[0].Phase, backlog.PhaseNotRecorded)
	testutil.Expect(t, "when it was claimed", page.Work.InProgress[0].At, ago(5*time.Hour))

	testutil.Require(t, "how many are next up", len(page.Work.Next), 3)
	testutil.Expect(t, "the first three of Ready by rank",
		[]string{page.Work.Next[0].ID, page.Work.Next[1].ID, page.Work.Next[2].ID},
		[]string{"g1-s14", "g1-s16", "g1-s17"})

	testutil.Expect(t, "how much is held", page.Work.Waiting.Count, 3)
	testutil.Expect(t, "the oldest hold", page.Work.Waiting.ID, "g1-s22")
	testutil.Expect(t, "and its reason", page.Work.Waiting.Reason, "the format is still being decided")

	testutil.Expect(t, "the lane strip", page.Work.Lanes, []Lane{
		{ID: "to-do", Count: 5}, {ID: "ready", Count: 4}, {ID: "in-progress", Count: 1},
		{ID: "review", Count: 1}, {ID: "waiting", Count: 3}, {ID: LaneDoneToday, Count: 1},
	})
}

// A hold nothing dated is never the oldest one, and where it is the only one
// its reason is the blockers the record names.
func TestAnUndatedHoldNamesItsBlockers(t *testing.T) {
	t.Parallel()

	page := composed(t, func(in *Inputs) {
		blocked := goalRow("g1-s24", backlog.LaneWaiting, "The job page")
		blocked.OpenBlockers = []string{"g1-s22", "g1-s23"}
		blocked.Waiting = &backlog.Waiting{From: "approved"}
		in.Rows = []backlog.Row{blocked}
	})

	testutil.Expect(t, "the oldest hold", page.Work.Waiting.ID, "g1-s24")
	testutil.Expect(t, "its reason", page.Work.Waiting.Reason, "blocked by g1-s22, g1-s23")
	testutil.Expect(t, "and nothing dated it", page.Work.Waiting.Since, "")
}

// Done today is the observation's own calendar day, not the last twenty-four
// hours: a conclusion written last night is yesterday's, however few hours ago
// it was.
func TestDoneTodayIsTheObservationDay(t *testing.T) {
	t.Parallel()

	page := composed(t, func(in *Inputs) {
		thisMorning := goalRow("a", backlog.LaneDone, "a")
		thisMorning.DoneAt = time.Date(2026, 9, 23, 2, 0, 0, 0, time.UTC).Format(time.RFC3339)
		lastNight := goalRow("b", backlog.LaneDone, "b")
		lastNight.DoneAt = time.Date(2026, 9, 22, 23, 30, 0, 0, time.UTC).Format(time.RFC3339)
		undated := goalRow("c", backlog.LaneDone, "c")
		in.Rows = []backlog.Row{}
		in.Closed = []backlog.Row{thisMorning, lastNight, undated}
	})

	strip := page.Work.Lanes
	testutil.Expect(t, "done today", strip[len(strip)-1], Lane{ID: LaneDoneToday, Count: 1})
}

/* --------------------------------------------------------------- memory -- */

func TestTheProjectsMemory(t *testing.T) {
	t.Parallel()

	page := composed(t, nil)

	testutil.Expect(t, "intent chapters", page.Memory.Intent.Chapters, 3)
	testutil.Expect(t, "the intent index's first sentence", page.Memory.Intent.Summary,
		"The MetaSystem is the machinery a human runs a fleet with.")
	testutil.Expect(t, "doctrine chapters", page.Memory.Doctrine.Chapters, 2)
	testutil.Expect(t, "decisions", page.Memory.Decisions, Tally{Total: 2, Drafts: 1})
	testutil.Expect(t, "designs on the shelf", page.Memory.Designs.Total, 5)
	testutil.Expect(t, "designs marked done", page.Memory.Designs.Done, 1)
	testutil.Expect(t, "designs in flight", page.Memory.Designs.InFlight, 3)
	testutil.Expect(t, "how far each has got", page.Memory.Designs.Progress, []Progress{
		{ID: "design-shell", Title: "design-shell", Path: "plans/designs/design-shell.md", Done: 2, Goals: 2},
		{ID: "design-reader", Title: "design-reader", Path: "plans/designs/design-reader.md", Done: 1, Goals: 2},
		{ID: "design-board", Title: "design-board", Path: "plans/designs/design-board.md", Done: 0, Goals: 1},
	})
	testutil.Expect(t, "open questions", page.Memory.Questions, 2)
}

func TestABookWithNoIndexIsSaidRatherThanInvented(t *testing.T) {
	t.Parallel()

	page := composed(t, func(in *Inputs) {
		in.Project.Intent = project.Book{Chapters: []project.Chapter{}}
	})

	testutil.Expect(t, "the book", page.Memory.Intent, Book{})
}

/* --------------------------------------------------------------- health -- */

func TestHealthIsOneLineWhenEverythingAnswered(t *testing.T) {
	t.Parallel()

	page := composed(t, nil)

	testutil.Expect(t, "health", page.Health.OK, true)
	testutil.Expect(t, "when the ledger last synced", page.Health.SyncedAt, stamp(now.Add(-time.Minute)))
	testutil.Expect(t, "problems", page.Health.Problems, Group{Count: 0, Items: []Item{}})
}

func TestEveryFailingQuestionIsReported(t *testing.T) {
	t.Parallel()

	page := composed(t, func(in *Inputs) {
		in.Ledger = Ledger{Freshness: snapshot.FreshnessFailed, AtTip: false, Statement: "the remote does not answer"}
		in.Project.Problems = []project.Problem{
			{Path: "plans/designs/board.md", Line: 4, Message: "Status: proposed is not a status"},
		}
		in.Journal = []notifications.Notice{
			{ID: "n1", At: ago(2 * time.Hour), Source: "alert", Message: "unhealthy",
				Delivered: false, Error: "notification not delivered: exit status 1"},
			{ID: "n2", At: ago(40 * 24 * time.Hour), Source: "alert", Message: "old",
				Delivered: false, Error: "outside the window"},
		}
	})

	testutil.Expect(t, "health", page.Health.OK, false)
	testutil.Require(t, "how many problems", page.Health.Problems.Count, 3)
	items := page.Health.Problems.Items
	testutil.Expect(t, "the ledger's own words", items[0].Title, "the remote does not answer")
	testutil.Expect(t, "where a ledger problem is fixed", items[0].Where, Where{Kind: WhereBacklog})
	testutil.Expect(t, "the record's refusal", items[1].Title, "Status: proposed is not a status")
	testutil.Expect(t, "anchored where it is", items[1].Note, "plans/designs/board.md:4")
	testutil.Expect(t, "where a refusal is fixed", items[1].Where,
		Where{Kind: WhereDocument, ID: "plans/designs/board.md"})
	testutil.Expect(t, "the delivery that failed", items[2].Title, "notification not delivered: exit status 1")
	testutil.Expect(t, "where it is read", items[2].Where, Where{Kind: WhereNotification, ID: "n1"})
}

// The page says which of the three freshness states it is looking at, so the
// pill can read the word rather than read the sentence back.
func TestHealthCarriesTheFreshnessItWasJudgedOn(t *testing.T) {
	t.Parallel()

	for _, one := range []struct {
		name  string
		state snapshot.Freshness
	}{
		{name: "current", state: snapshot.FreshnessCurrent},
		{name: "behind", state: snapshot.FreshnessBehind},
		{name: "failed", state: snapshot.FreshnessFailed},
	} {
		t.Run(one.name, func(t *testing.T) {
			t.Parallel()

			page := composed(t, func(in *Inputs) {
				in.Ledger = Ledger{Freshness: one.state, AtTip: one.state == snapshot.FreshnessCurrent}
			})

			testutil.Expect(t, "the freshness the page was judged on", page.Health.Freshness, one.state)
		})
	}
}

// A freshness that is not current is a problem even where nothing wrote a
// sentence about it: a reader told something is wrong must be told what.
func TestAFreshnessWithNoSentenceIsStillSaidPlainly(t *testing.T) {
	t.Parallel()

	for _, one := range []struct {
		name     string
		state    snapshot.Freshness
		sentence string
	}{
		{
			name: "a loop that has not landed lately", state: snapshot.FreshnessBehind,
			sentence: "the last fetch of the canonical branch has not landed lately",
		},
		{
			name: "a loop whose last look failed", state: snapshot.FreshnessFailed,
			sentence: "the last fetch of the canonical branch failed",
		},
	} {
		t.Run(one.name, func(t *testing.T) {
			t.Parallel()

			page := composed(t, func(in *Inputs) {
				in.Ledger = Ledger{Freshness: one.state, AtTip: false}
			})

			testutil.Require(t, "how many problems", page.Health.Problems.Count, 1)
			testutil.Expect(t, "what it says", page.Health.Problems.Items[0].Title, one.sentence)
		})
	}
}

/* ----------------------------------------------------------- the whole -- */

func TestThePageCarriesItsSchemaAndItsWindow(t *testing.T) {
	t.Parallel()

	page := composed(t, func(in *Inputs) { in.First = true })

	testutil.Expect(t, "the schema version", page.SchemaVersion, SchemaVersion)
	testutil.Expect(t, "when it was read", page.ReadAt, stamp(now))
	testutil.Expect(t, "the window it compares against", page.Since, stamp(visitedYesterday))
	testutil.Expect(t, "whether it is a first visit", page.First, true)
}

func TestFirstSentence(t *testing.T) {
	t.Parallel()

	for _, one := range []struct{ name, summary, sentence string }{
		{"one sentence of several", "One. Two. Three.", "One."},
		{"a summary with no stop", "A line with no full stop", "A line with no full stop"},
		{"a stop inside a number", "Version 1.2 shipped. Then more.", "Version 1.2 shipped."},
		{"a question", "Is it ready? It is.", "Is it ready?"},
		{"one sentence only", "Just the one.", "Just the one."},
		{"nothing at all", "", ""},
	} {
		t.Run(one.name, func(t *testing.T) {
			t.Parallel()
			testutil.Expect(t, "the first sentence", firstSentence(one.summary), one.sentence)
		})
	}
}

func changedAt(record project.Record, at string) project.Record {
	record.ChangedAt = at
	return record
}

func TestLedeIsTheFirstSentenceAndNeverLong(t *testing.T) {
	t.Parallel()
	cases := []struct{ in, want string }{
		{"Finish these repairs. Then push.", "Finish these repairs."},
		{"One line with no stop", "One line with no stop"},
		{"Spread   over\nlines. Next.", "Spread over lines."},
		{strings.Repeat("word ", 40) + "end", strings.TrimRight(strings.Repeat("word ", 28), " ") + "…"},
	}
	for _, c := range cases {
		if got := lede(c.in); got != c.want {
			t.Errorf("lede(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
