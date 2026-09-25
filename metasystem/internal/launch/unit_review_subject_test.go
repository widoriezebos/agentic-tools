package launch

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

const reviewDiff = "diff --git a/code.go b/code.go\nindex 1111111..2222222 100644\n--- a/code.go\n+++ b/code.go\n@@ -1 +1 @@\n-one\n+two\n"

func reviewNotCalled(t *testing.T) func(UnitReview, func(UnitSubject) error) error {
	return func(review UnitReview, _ func(UnitSubject) error) error {
		t.Errorf("bound a round that is not ready: %+v", review.Round)
		return nil
	}
}

// ReviewSubject hands the caller the latest completed round's retained
// result, and retain binds a subject to that round only. A repeat sees the
// bound subject, a later round sees the earlier committed one as its prior,
// and a retained diff that no longer matches its subject is refused.
func TestReviewSubjectBindsTheLatestCompletedRound(t *testing.T) {
	t.Parallel()
	fixture := newUnitFixture(t, reviewDiff, "branch", "round", "branch", "round")
	first, err := fixture.runner.Advance(UnitRequest{Plan: fixture.plan})
	if err != nil || first.Record.Rounds[0].Outcome != "green" {
		t.Fatalf("result=%+v err=%v", first, err)
	}
	plan, _ := ReadUnitPlan(fixture.plan)
	sum := sha256.Sum256([]byte(reviewDiff))
	wantDigest := hex.EncodeToString(sum[:])
	if empty := UnitResultDigest(""); empty != "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855" {
		t.Fatalf("empty result digest=%s", empty)
	}
	// The run store alone is enough to review a run.
	reviewer := &UnitRunner{Root: fixture.runner.Root}
	bound := UnitSubject{Round: 1, Operation: "op-1", ExpectedParent: "head", ResultDigest: UnitResultDigest(""), DiffDigest: wantDigest}
	err = reviewer.ReviewSubject(first.Record.ID, func(review UnitReview, retain func(UnitSubject) error) error {
		if review.Record.ID != first.Record.ID || review.Round.Number != 1 || review.Head != "head" || review.Result != "" || review.Legacy ||
			string(review.Diff) != reviewDiff || review.DiffDigest != wantDigest || review.Base != "base" || review.BuildBrief != plan.Build.Brief ||
			review.Subject != nil || review.Prior != nil {
			t.Fatalf("review=%+v", review)
		}
		if err := retain(UnitSubject{Round: 2, DiffDigest: wantDigest}); err == nil || !strings.Contains(err.Error(), "latest completed round 1") {
			t.Fatalf("retain of another round: %v", err)
		}
		return retain(bound)
	})
	if err != nil {
		t.Fatal(err)
	}
	if record, err := fixture.runner.Status(first.Record.ID); err != nil || !reflect.DeepEqual(record.Subjects, []UnitSubject{bound}) {
		t.Fatalf("subjects=%+v err=%v", record.Subjects, err)
	}

	committed := bound
	committed.Commit, committed.Tip = "commit-1", "commit-1"
	err = reviewer.ReviewSubject(first.Record.ID, func(review UnitReview, retain func(UnitSubject) error) error {
		if review.Subject == nil || !reflect.DeepEqual(*review.Subject, bound) || review.Prior != nil {
			t.Fatalf("repeat review subject=%+v prior=%+v", review.Subject, review.Prior)
		}
		return retain(committed)
	})
	if err != nil {
		t.Fatal(err)
	}
	if record, _ := fixture.runner.Status(first.Record.ID); !reflect.DeepEqual(record.Subjects, []UnitSubject{committed}) {
		t.Fatalf("repeat appended instead of replacing: %+v", record.Subjects)
	}

	follow := writeFollowUp(t)
	second, err := fixture.runner.Advance(UnitRequest{Resume: first.Record.ID, FollowUp: follow})
	if err != nil || len(second.Record.Rounds) != 2 || second.Record.Rounds[1].Outcome != "green" {
		t.Fatalf("result=%+v err=%v", second, err)
	}
	amend := UnitSubject{Round: 2, Operation: "op-2", DiffDigest: wantDigest, Amends: "commit-1"}
	err = reviewer.ReviewSubject(first.Record.ID, func(review UnitReview, retain func(UnitSubject) error) error {
		if review.Round.Number != 2 || review.Subject != nil || review.Prior == nil || !reflect.DeepEqual(*review.Prior, committed) {
			t.Fatalf("second round subject=%+v prior=%+v", review.Subject, review.Prior)
		}
		return retain(amend)
	})
	if err != nil {
		t.Fatal(err)
	}
	if record, _ := fixture.runner.Status(first.Record.ID); !reflect.DeepEqual(record.Subjects, []UnitSubject{committed, amend}) {
		t.Fatalf("subjects=%+v", record.Subjects)
	}

	if err := os.WriteFile(filepath.Join(second.Record.Rounds[1].Directory, "worktree.diff"), []byte(reviewDiff+"+more\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	err = reviewer.ReviewSubject(first.Record.ID, reviewNotCalled(t))
	if err == nil || !strings.Contains(err.Error(), "UNIT_RESULT_CHANGED") || !strings.Contains(err.Error(), "round=2") {
		t.Fatalf("err=%v", err)
	}
}

// Only a completed round whose proof passed and whose retained snapshot,
// diff and plan are all present can be reviewed; a busy run is refused.
func TestReviewSubjectRefusesARoundThatIsNotReady(t *testing.T) {
	t.Parallel()
	if err := (&UnitRunner{}).ReviewSubject("run", reviewNotCalled(t)); err == nil || !strings.Contains(err.Error(), "store is unavailable") {
		t.Fatalf("err=%v", err)
	}
	for _, row := range []struct {
		name, want string
		events     []string
		prepare    func(fixture unitFixture)
		spoil      func(t *testing.T, fixture unitFixture, record UnitRunRecord) func()
	}{
		{name: "unknown", want: "UNIT_RUN_UNKNOWN", events: []string{"branch", "round"},
			spoil: func(t *testing.T, fixture unitFixture, record UnitRunRecord) func() {
				os.RemoveAll(fixture.runner.runDir(record.ID))
				return nil
			}},
		{name: "running", want: "state=running", events: []string{"branch"},
			prepare: func(fixture unitFixture) { fixture.starter.holdKind = "build" }},
		{name: "proof-red", want: "outcome=proof-red", events: []string{"branch", "round"},
			prepare: func(fixture unitFixture) { fixture.starter.failKind = "proof" }},
		{name: "snapshot-missing", want: "no retained result snapshot", events: []string{"branch", "round"},
			spoil: func(t *testing.T, fixture unitFixture, record UnitRunRecord) func() {
				os.Remove(filepath.Join(record.Rounds[0].Directory, "proof-after.json"))
				return nil
			}},
		{name: "diff-missing", want: "no retained result diff", events: []string{"branch", "round"},
			spoil: func(t *testing.T, fixture unitFixture, record UnitRunRecord) func() {
				os.Remove(filepath.Join(record.Rounds[0].Directory, "worktree.diff"))
				return nil
			}},
		{name: "plan-missing", want: "plan is unreadable", events: []string{"branch", "round"},
			spoil: func(t *testing.T, fixture unitFixture, record UnitRunRecord) func() {
				os.Remove(record.Plan)
				return nil
			}},
		{name: "busy", want: "UNIT_RUN_BUSY", events: []string{"branch", "round"},
			spoil: func(t *testing.T, fixture unitFixture, record UnitRunRecord) func() {
				lock, err := fixture.runner.lock(record.ID)
				if err != nil {
					t.Fatal(err)
				}
				return func() { releaseUnitLock(lock) }
			}},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			fixture := newUnitFixture(t, reviewDiff, row.events...)
			if row.prepare != nil {
				row.prepare(fixture)
			}
			result, err := fixture.runner.Advance(UnitRequest{Plan: fixture.plan})
			if err != nil {
				t.Fatal(err)
			}
			if row.spoil != nil {
				if release := row.spoil(t, fixture, result.Record); release != nil {
					defer release()
				}
			}
			before, _ := os.ReadFile(filepath.Join(fixture.runner.runDir(result.Record.ID), "run.json"))
			err = fixture.runner.ReviewSubject(result.Record.ID, reviewNotCalled(t))
			if err == nil || !strings.Contains(err.Error(), row.want) {
				t.Fatalf("err=%v, want %s", err, row.want)
			}
			if after, _ := os.ReadFile(filepath.Join(fixture.runner.runDir(result.Record.ID), "run.json")); string(after) != string(before) {
				t.Fatalf("refused review rewrote the run: %s", after)
			}
		})
	}
}

// Status reads a run's record as it is, without advancing a running round.
func TestStatusReadsARunWithoutAdvancingIt(t *testing.T) {
	t.Parallel()
	fixture := newUnitFixture(t, "", "branch")
	fixture.starter.holdKind = "build"
	result, err := fixture.runner.Advance(UnitRequest{Plan: fixture.plan})
	if err != nil || !result.Capped {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	fixture.manager.Store.Update(result.Launch, func(record *Record) error { record.State = Completed; return nil })
	for range 2 {
		record, err := fixture.runner.Status(result.Record.ID)
		if err != nil || record.State != "running" || len(record.Rounds) != 1 || stepNamed(t, record.Rounds[0], "build").State != StepRunning {
			t.Fatalf("status=%+v err=%v", record, err)
		}
	}
	requireLaunchedOnce(t, fixture, 1)
	if _, err := fixture.runner.Status("unknown-run"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("err=%v", err)
	}
	if _, err := fixture.runner.Status("../" + result.Record.ID); err == nil || !strings.Contains(err.Error(), "invalid unit run id") {
		t.Fatalf("err=%v", err)
	}
}

// A page's declared units are the rows of its units table, with the same
// estimate build admission reads, including a witness allowance.
func TestDeclaredUnitsReadThePagesUnitsTable(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	page := filepath.Join(directory, "design.md")
	writeFile(t, page, "# Design\n\n| Unit | Purpose | Estimated changed lines |\n| --- | --- | --- |\n"+
		"| parser | reads | 120 |\n| parser.tests | proves | 40 |\n| writer | writes | 30 plus witness 12 |\n\nAfter the table.\n| stray | row | 999 |\n")
	units, err := DeclaredUnits(page)
	want := []UnitSize{{Name: "parser", Lines: 120}, {Name: "parser.tests", Lines: 40}, {Name: "writer", Lines: 42}}
	if err != nil || !slices.Equal(units, want) {
		t.Fatalf("units=%+v err=%v", units, err)
	}
	for unit, lines := range map[string]int64{"parser": 120, "writer": 42} {
		if got, err := DeclaredUnitLines(page, unit); err != nil || got != lines {
			t.Fatalf("unit=%s lines=%d err=%v, want %d", unit, got, err, lines)
		}
	}
	if _, err := DeclaredUnitLines(page, "reader"); err == nil || !strings.Contains(err.Error(), "unit=reader missing=row") {
		t.Fatalf("err=%v", err)
	}
	prose := filepath.Join(directory, "prose.md")
	writeFile(t, prose, "No table here.\n")
	if _, err := DeclaredUnits(prose); err == nil || !strings.Contains(err.Error(), "missing=units-table") {
		t.Fatalf("err=%v", err)
	}
	missing := filepath.Join(directory, "missing.md")
	if _, err := DeclaredUnits(missing); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("err=%v", err)
	}
	if _, err := DeclaredUnitLines(missing, "parser"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("err=%v", err)
	}
}
