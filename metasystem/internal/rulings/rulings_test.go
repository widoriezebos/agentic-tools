package rulings

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The register fixture: one row of every shape the reader has to tell apart.
//
// All four schedulable classes, an event-only row, a row whose due date and
// event travel together, a blank condition, a prose condition, an ownerless
// row, an unparsable condition, an invalid due date, a malformed row, and a
// row whose words name a goal. Every test below reads this one register, so a
// rule that answers one shape by breaking another fails here.
var fixtureRows = []string{
	"| R-1 | 2026-08-26 | Standing until further notice | given at the first review | Wido |  |",
	"| R-2 | 2026-08-27 | Temporary while the migration runs | given with the migration | Wido | class=temporary due=2026-08-29 |",
	"| R-3 | 2026-08-28 | Experimental, one arc | given on the arc | Wido | class=experimental due=2099-01-01 |",
	"| R-4 | 2026-08-29 | Delegated authority for model choice | given to the dispatch delegate | Wido | class=delegated-authority due=2026-09-30 |",
	"| R-5 | 2026-08-30 | Assumption dependent on the first report | given during the post-mortem | Wido | class=assumption-dependent event=first-measured-report-exists |",
	"| R-6 | 2026-08-31 | Both a date and an event | given with a backstop | Wido | class=temporary due=2026-09-06 event=terminal-re-arm |",
	"| R-7 | 2026-09-01 | Stands until the memory migration | the register lives in memory/ | Wido | standing |",
	"| R-8 | 2026-09-02 | Nobody was named for this one | given in passing | | class=temporary due=2026-08-29 |",
	"| R-9 | 2026-09-03 | The condition names a key nothing reads | given late | Wido | whenever=soon |",
	"| R-10 | 2026-09-04 | The due date is not a date | given late | Wido | class=temporary due=soon |",
	"| R-11 | too | few | columns |",
	"| R-12 | 2026-09-05 | The Decisions page renders g1-s44 whole | given for the browser interface | Wido | class=temporary due=2026-09-25 |",
}

func writeRegister(t *testing.T, root string, rows []string) {
	t.Helper()
	path := Path(root)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	body := "# Standing rulings register\n\n" +
		"| ID | Date | Decision | Evidence | Owner | Review |\n|---|---|---|---|---|---|\n" +
		strings.Join(rows, "\n") + "\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func fixtureRegister(t *testing.T) Register {
	t.Helper()
	root := t.TempDir()
	writeRegister(t, root, fixtureRows)
	register, err := Read(root)
	if err != nil {
		t.Fatal(err)
	}
	return register
}

func rowOf(t *testing.T, register Register, id string) Row {
	t.Helper()
	for _, row := range register.Rows {
		if row.ID == id {
			return row
		}
	}
	t.Fatalf("the register carries no row %s: %+v", id, register.Rows)
	return Row{}
}

func TestReadCarriesEveryReadableRowWholeInRegisterOrder(t *testing.T) {
	t.Parallel()
	register := fixtureRegister(t)
	var ids []string
	for _, row := range register.Rows {
		ids = append(ids, row.ID)
	}
	// R-11 is the malformed row: it has no six columns, so it has no words
	// to carry whole and is a defect instead. Everything else is a row.
	want := []string{"R-1", "R-2", "R-3", "R-4", "R-5", "R-6", "R-7", "R-8", "R-9", "R-10", "R-12"}
	if strings.Join(ids, ",") != strings.Join(want, ",") {
		t.Fatalf("rows are not the register's own, in its own order: got %v want %v", ids, want)
	}
	whole := rowOf(t, register, "R-12")
	if whole.Date != "2026-09-05" || whole.Owner != "Wido" ||
		whole.Words != "The Decisions page renders g1-s44 whole" ||
		whole.Context != "given for the browser interface" ||
		whole.Condition != "class=temporary due=2026-09-25" {
		t.Fatalf("the row did not come back whole: %+v", whole)
	}
}

func TestReadFillsTheScheduledPartsOnlyWhereTheConditionParses(t *testing.T) {
	t.Parallel()
	register := fixtureRegister(t)
	for _, expected := range []struct{ id, class, due, event string }{
		{"R-1", "", "", ""},
		{"R-2", ClassTemporary, "2026-08-29", ""},
		{"R-3", ClassExperimental, "2099-01-01", ""},
		{"R-4", ClassDelegatedAuthority, "2026-09-30", ""},
		{"R-5", ClassAssumptionDependent, "", "first-measured-report-exists"},
		{"R-6", ClassTemporary, "2026-09-06", "terminal-re-arm"},
		// Prose is not a class, so the row keeps its words and claims no
		// schedule. Its condition still reads as the register wrote it.
		{"R-7", "", "", ""},
		{"R-9", "", "", ""},
		{"R-10", "", "", ""},
	} {
		row := rowOf(t, register, expected.id)
		if row.Class != expected.class || row.Due != expected.due || row.Event != expected.event {
			t.Errorf("%s parsed as class=%q due=%q event=%q, want class=%q due=%q event=%q",
				expected.id, row.Class, row.Due, row.Event, expected.class, expected.due, expected.event)
		}
	}
	if condition := rowOf(t, register, "R-7").Condition; condition != "standing" {
		t.Fatalf("a prose review condition was not carried as written: %q", condition)
	}
}

func TestReadExposesTheSameScheduledSubsetTheSweepConsumes(t *testing.T) {
	t.Parallel()
	register := fixtureRegister(t)
	var got []string
	for _, review := range register.Reviews {
		got = append(got, review.ID+"/"+review.Class+"/"+review.Due+"/"+review.Event)
	}
	// R-8 is ownerless and R-9, R-10 and R-7 are refused by the parser, so
	// none of the four is a review the sweep may schedule.
	want := []string{
		"R-2/temporary/2026-08-29/",
		"R-3/experimental/2099-01-01/",
		"R-4/delegated-authority/2026-09-30/",
		"R-5/assumption-dependent//first-measured-report-exists",
		"R-6/temporary/2026-09-06/terminal-re-arm",
		"R-12/temporary/2026-09-25/",
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("the scheduled subset moved: got %v want %v", got, want)
	}
	if owner := register.Reviews[0].Owner; owner != "Wido" {
		t.Fatalf("a review lost its accountable owner: %q", owner)
	}
}

func TestReadNamesEveryDefectInTheStewardsOwnWords(t *testing.T) {
	t.Parallel()
	register := fixtureRegister(t)
	var got []string
	for _, defect := range register.Defects {
		line := defect.Label + " defect=" + defect.Reason
		if defect.Ownerless {
			line += " choice=adopt|withdraw"
		}
		got = append(got, line)
	}
	// Prose in the review column is a defect under this grammar and always
	// was: the sweep's digest has said so since the parser was written, and
	// lifting the reader did not widen it. The row is still carried whole
	// above, so the page shows the ruling and the defect both.
	want := []string{
		`R-7 defect=review condition token "standing" is not key=value`,
		"R-8 defect=no accountable owner choice=adopt|withdraw",
		`R-9 defect=unknown review condition key "whenever"`,
		`R-10 defect=review due date "soon" is invalid`,
		"row=11 defect=wrong column count: got 4, want 6",
	}
	if strings.Join(got, "; ") != strings.Join(want, "; ") {
		t.Fatalf("the defect wording or its order moved: got %v want %v", got, want)
	}
}

// An ownerless row raises exactly one defect, even when its condition is
// unreadable too: the steward reports the missing owner and stops on that
// row, and a second sentence about the same row would be a digest line the
// sweep never wrote.
func TestAnOwnerlessRowRaisesTheOwnerDefectAndNoSecondOne(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeRegister(t, root, []string{"| R-x | 2026-09-05 | words | context | | whenever=soon |"})
	register, err := Read(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(register.Defects) != 1 || register.Defects[0].Reason != "no accountable owner" || !register.Defects[0].Ownerless {
		t.Fatalf("an ownerless row with a bad condition did not raise one owner defect: %+v", register.Defects)
	}
	if len(register.Rows) != 1 || register.Rows[0].Words != "words" {
		t.Fatalf("an ownerless row was dropped instead of carried whole: %+v", register.Rows)
	}
}

func TestParseReviewConditionKeepsTheAcceptanceRulesAndTheRefusalWording(t *testing.T) {
	t.Parallel()
	for _, accepted := range []struct{ condition, class, due, event string }{
		{"", "", "", ""},
		{"class=temporary due=2026-08-29", ClassTemporary, "2026-08-29", ""},
		{"class=experimental event=some-token", ClassExperimental, "", "some-token"},
		{"class=delegated-authority due=2026-09-30 event=first-enrolled-session", ClassDelegatedAuthority, "2026-09-30", "first-enrolled-session"},
	} {
		class, due, event, err := ParseReviewCondition(accepted.condition)
		if err != nil || class != accepted.class || due != accepted.due || event != accepted.event {
			t.Errorf("%q parsed as class=%q due=%q event=%q err=%v", accepted.condition, class, due, event, err)
		}
	}
	for _, refused := range []struct{ condition, reason string }{
		{"standing", `review condition token "standing" is not key=value`},
		{"class=", `review condition token "class=" is not key=value`},
		{"whenever=soon", `unknown review condition key "whenever"`},
		{"class=standing due=2026-08-29", `review class "standing" is not schedulable`},
		{"due=2026-08-29", `review class "" is not schedulable`},
		{"class=temporary", "review condition needs due= or event="},
		{"class=temporary due=soon", `review due date "soon" is invalid`},
	} {
		class, due, event, err := ParseReviewCondition(refused.condition)
		if err == nil || err.Error() != refused.reason {
			t.Errorf("%q was refused as %v, want %q", refused.condition, err, refused.reason)
		}
		if class != "" || due != "" || event != "" {
			t.Errorf("%q was refused and still reported class=%q due=%q event=%q", refused.condition, class, due, event)
		}
	}
}

func TestDuePassedIsADateJudgementAndNeverAnEventOne(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 9, 25, 11, 0, 0, 0, time.UTC)
	for _, judged := range []struct {
		due    string
		passed bool
	}{
		{"", false},
		{"2026-09-24", true},
		{"2026-09-25", true},
		{"2026-09-26", false},
		{"soon", false},
	} {
		if passed := DuePassed(judged.due, now); passed != judged.passed {
			t.Errorf("due %q at %s judged %v, want %v", judged.due, now, passed, judged.passed)
		}
	}
	// The day is the observing clock's own UTC day: an hour before midnight
	// somewhere else is still today here.
	if !DuePassed("2026-09-25", time.Date(2026, 9, 25, 23, 59, 0, 0, time.UTC)) {
		t.Fatal("a due date fell out of its own day")
	}
}

// The reader is optional in an adopted repository: a checkout that records no
// rulings has an empty register, and its steward is not degraded by that.
func TestAnAdoptedRepositoryWithNoRegisterReadsEmpty(t *testing.T) {
	t.Parallel()
	register, err := Read(t.TempDir())
	if err != nil || register.Rows != nil || register.Reviews != nil || register.Defects != nil {
		t.Fatalf("a checkout without the register did not read empty: %+v %v", register, err)
	}
	reviews, defects, err := ReadReviews(t.TempDir())
	if err != nil || reviews != nil || defects != nil {
		t.Fatalf("the steward's view of a missing register was not empty: %+v %+v %v", reviews, defects, err)
	}
}

// This repository's own register, read whole: every row has an owner, and
// every scheduled review is one of the four typed classes.
func TestThisRegisterHasOwnersAndOnlyTypedScheduledReviews(t *testing.T) {
	t.Parallel()
	register, err := Read(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	if len(register.Rows) == 0 {
		t.Fatal("the register read no rows at all")
	}
	if len(register.Reviews) == 0 {
		t.Fatal("the register has no scheduled review conditions")
	}
	for _, review := range register.Reviews {
		if review.Owner == "" {
			t.Errorf("scheduled review %s has no accountable owner", review.ID)
		}
		switch review.Class {
		case ClassTemporary, ClassExperimental, ClassDelegatedAuthority, ClassAssumptionDependent:
		default:
			t.Errorf("scheduled review %s carries the untyped class %q", review.ID, review.Class)
		}
	}
}
