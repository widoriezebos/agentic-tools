package decisions

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/backlog"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/project"
)

// What "new" means, against a window the caller injects.
//
// The dates the kinds carry are uneven and this page is honest about that
// rather than exact: a goal's opening instant is an instant, a question's
// opened column is a calendar date, and some rows carry nothing at all. Each
// of the three is a rule a human can be told, and each of the three is here.

// window is the boundary these tests compare against: a day and a half before
// the instant everything else is composed at.
var window = observed.Add(-36 * time.Hour)

func TestAnInstantIsNewWhenItIsAfterTheWindow(t *testing.T) {
	t.Parallel()

	in := everyKind()
	in.Since = window
	page := Compose(in, observed)

	// The alert is ninety minutes old and the stopped goal four hours old;
	// both fall inside a window that opened a day and a half ago.
	if !needOf(t, page, KindAlert).New {
		t.Errorf("an alert ninety minutes old is not new: %+v", needOf(t, page, KindAlert))
	}
	if !needOf(t, page, KindStopped).New {
		t.Errorf("a goal stopped four hours ago is not new")
	}
	// The draft changed seventy hours ago, which is before the window opened.
	if needOf(t, page, KindDraft).New {
		t.Errorf("a draft seventy hours old is new")
	}
	// And the boundary itself is not inside it: after, not at or after.
	at := everyKind()
	at.Since = window
	at.Journal[0].At = stamp(window)
	edge := Compose(at, observed)
	if needOf(t, edge, KindAlert).New {
		t.Errorf("a row recorded exactly at the boundary is new")
	}
}

func TestACalendarDateIsNewFromTheWindowsOwnDay(t *testing.T) {
	t.Parallel()

	// A question's opened column is a day rather than an instant. The window
	// opened at 23:00 on 2026-09-23; a question opened on 2026-09-23 is new,
	// because the day the window opened on is a day the human may not have
	// seen the whole of, and reading the day as midnight would call this
	// morning's question old.
	in := everyKind()
	in.Since = time.Date(2026, 9, 23, 23, 0, 0, 0, time.UTC)
	in.Project.Questions = []project.Question{
		{ID: "Q-same", Opened: "2026-09-23", Question: "Opened on the window's own day", Status: statusOpen},
		{ID: "Q-after", Opened: "2026-09-24", Question: "Opened the day after", Status: statusOpen},
		{ID: "Q-before", Opened: "2026-09-22", Question: "Opened the day before", Status: statusOpen},
	}
	page := Compose(in, observed)

	want := map[string]bool{"Q-same": true, "Q-after": true, "Q-before": false}
	for _, need := range page.NeedsYou {
		if need.Kind != KindQuestion {
			continue
		}
		expected, named := want[need.ID]
		if !named {
			t.Errorf("an unexpected question: %q", need.ID)
			continue
		}
		if need.New != expected {
			t.Errorf("%s: new = %v, want %v", need.ID, need.New, expected)
		}
		delete(want, need.ID)
	}
	if len(want) != 0 {
		t.Errorf("the inbox lost a question: %v", want)
	}
}

func TestARowNothingDatedIsNeverNewAndNeitherIsAnyRowWithoutAWindow(t *testing.T) {
	t.Parallel()

	// A record with no changed instant carries no since, and a page that
	// guessed one would be marking rows new on no evidence.
	in := everyKind()
	in.Since = window
	for index := range in.Project.Records {
		if in.Project.Records[index].ID == "d-open" {
			in.Project.Records[index].ChangedAt = ""
		}
	}
	page := Compose(in, observed)
	draft := needOf(t, page, KindDraft)
	if draft.Since != "" || draft.New {
		t.Errorf("a row nothing dated is new: since %q, new %v", draft.Since, draft.New)
	}

	// And a page composed over no window at all marks nothing new: that is
	// what a build with no marker store answers, and "everything ever
	// recorded is new" is the one answer it must not give.
	none := Compose(everyKind(), observed)
	for _, need := range none.NeedsYou {
		if need.New {
			t.Errorf("%s %s is new on a page composed over no window", need.Kind, need.ID)
		}
	}
	if none.Visit.Since != "" || none.Visit.First {
		t.Errorf("a page with no window named one: %+v", none.Visit)
	}
}

// The window travels, so the page can say what it means by new rather than
// leaving a reader to infer a boundary from the dots.
func TestThePageNamesTheWindowItDecidedNewAgainst(t *testing.T) {
	t.Parallel()

	in := everyKind()
	in.Since, in.First = window, true
	page := Compose(in, observed)

	if page.Visit.Since != window.Format(time.RFC3339) {
		t.Errorf("the page names the window at %q, want %q", page.Visit.Since, window.Format(time.RFC3339))
	}
	if !page.Visit.First {
		t.Errorf("the page did not say the window is a first visit's")
	}
}

/* ----------------------------------------- what the rows now carry -- */

// A ruling review used to be an id and a sentence about an id; the words a
// human actually ruled sat in the register beside it under the same id, and
// reading the row meant opening the register. Now the row carries them.
func TestARulingReviewCarriesTheRegisterRowItNames(t *testing.T) {
	t.Parallel()

	page := Compose(everyKind(), observed)
	review := needOf(t, page, KindRulingReview)

	if review.ID != "R-2" {
		t.Fatalf("the review is not the one whose date passed: %+v", review)
	}
	if review.Words != "A temporary ruling whose review has come round" {
		t.Errorf("the review carries no words: %q", review.Words)
	}
	if review.Context != "given with the migration" {
		t.Errorf("the review carries no context: %q", review.Context)
	}
	if review.Owner != "Wido" || review.Class != "temporary" || review.Due != "2026-09-20" {
		t.Errorf("the review lost its schedule: owner %q class %q due %q",
			review.Owner, review.Class, review.Due)
	}
}

// A review whose register row the reader could not read carries the schedule
// and empty words, rather than nothing at all: the review is still due.
func TestAReviewWithNoRegisterRowStillCarriesItsSchedule(t *testing.T) {
	t.Parallel()

	in := everyKind()
	in.Register.Rows = nil
	review := needOf(t, Compose(in, observed), KindRulingReview)

	if review.ID != "R-2" || review.Due != "2026-09-20" || review.Owner != "Wido" {
		t.Fatalf("the review lost its schedule with its row: %+v", review)
	}
	if review.Words != "" || review.Context != "" {
		t.Errorf("the review invented words nobody wrote: %q / %q", review.Words, review.Context)
	}
}

// The two kinds that are about a record carry the record's path, so the open
// row shows where the file is without taking it out of a destination.
func TestTheDraftAndTheLandedDesignCarryTheRecordsPath(t *testing.T) {
	t.Parallel()

	page := Compose(everyKind(), observed)

	if path := needOf(t, page, KindDraft).Path; path != "plans/designs/draft.md" {
		t.Errorf("the draft carries no path: %q", path)
	}
	landed := needOf(t, page, KindLanded)
	if landed.Path != "plans/designs/landed.md" {
		t.Errorf("the landed design carries no path: %q", landed.Path)
	}
	// And the landed design carries the goals it named, with where each
	// stands, which is the whole of the evidence for marking it done.
	if len(landed.Goals) != 2 {
		t.Fatalf("the landed design lost its goals: %+v", landed.Goals)
	}
	for index, want := range []GoalState{{ID: "g1-s9", State: "done"}, {ID: "g1-s10", State: "done"}} {
		if landed.Goals[index] != want {
			t.Errorf("goal %d: got %+v, want %+v", index, landed.Goals[index], want)
		}
	}
}

// A goal a record names that this checkout has no goal for is listed with no
// state, rather than dropped: the record names work that is not here, and
// hiding it would hide the reason the design is not done.
func TestAGoalTheLedgerHasNoRowForIsListedWithNoState(t *testing.T) {
	t.Parallel()

	in := everyKind()
	for index := range in.Project.Records {
		if in.Project.Records[index].ID == "d-landed" {
			in.Project.Records[index].Goals = []string{"g1-s9", "g1-s10", "g1-elsewhere"}
		}
	}
	// A record whose goals are not all done is not a landed design at all, so
	// the ledger is told the missing goal is done too — which is what a
	// checkout that carries the record and not the goal file reads like.
	in.Project.Goals = append(in.Project.Goals, project.Goal{ID: "g1-elsewhere", State: "done"})
	landed := needOf(t, Compose(in, observed), KindLanded)
	if len(landed.Goals) != 3 || landed.Goals[2].ID != "g1-elsewhere" {
		t.Fatalf("the goals a record names are not the goals it lists: %+v", landed.Goals)
	}

	in.Project.Goals = in.Project.Goals[:len(in.Project.Goals)-1]
	for _, need := range Compose(in, observed).NeedsYou {
		if need.Kind == KindLanded {
			t.Fatalf("a design whose named goal is not done is offered as landed: %+v", need)
		}
	}
}

// Every field is written, including the ones a row has nothing for, so a
// reader never has to tell an absent field from an empty one.
func TestEveryRowWritesEveryFieldEvenTheOnesItHasNothingFor(t *testing.T) {
	t.Parallel()

	in := everyKind()
	in.Since = window
	encoded, err := json.Marshal(Compose(in, observed))
	if err != nil {
		t.Fatalf("the page did not encode: %v", err)
	}
	var read struct {
		Visit    Visit                        `json:"visit"`
		NeedsYou []map[string]json.RawMessage `json:"needsYou"`
	}
	if err := json.Unmarshal(encoded, &read); err != nil {
		t.Fatalf("the page did not read back: %v", err)
	}
	if read.Visit.Since != window.Format(time.RFC3339) {
		t.Errorf("the encoded page names no window: %+v", read.Visit)
	}
	for _, row := range read.NeedsYou {
		for _, field := range []string{"new", "words", "context", "owner", "class", "due", "path", "goals"} {
			raw, written := row[field]
			if !written {
				t.Fatalf("a row does not write %q: %v", field, row)
			}
			if field == "goals" && string(raw) == "null" {
				t.Errorf("a row writes its goals as nothing rather than as an empty list")
			}
		}
	}
}

// The row a goal carries is untouched by any of this: the sheets prefill from
// it, and an approval's row is still the whole row.
func TestAnApprovalStillCarriesItsWholeRow(t *testing.T) {
	t.Parallel()

	in := everyKind()
	in.Since = window
	approval := needOf(t, Compose(in, observed), KindApproval)
	if approval.Row == nil || approval.Row.Lane != backlog.LaneToDo {
		t.Fatalf("an approval lost its row: %+v", approval)
	}
}
