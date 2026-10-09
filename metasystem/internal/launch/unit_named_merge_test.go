package launch

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestGoalRunsKeepsCrossWorktreeHistoryAndContinuationChecks(t *testing.T) {
	t.Parallel()
	fixture := newUnitFixture(t, "", "branch", "branch", "before")
	fixture.starter.holdKind = "build"
	first, err := fixture.runner.AdvanceNamed(fixture.plan)
	if err != nil || !first.Capped {
		t.Fatalf("held run: %+v, %v", first, err)
	}
	other := first.Record
	other.ID, other.Worktree = "other-worktree", t.TempDir()
	if err := writeUnitJSON(filepath.Join(fixture.runner.runDir(other.ID), "run.json"), other, fixture.runner.root()); err != nil {
		t.Fatal(err)
	}
	unrelated := other
	unrelated.ID, unrelated.Goal = "unrelated-goal", "another-goal"
	if err := writeUnitJSON(filepath.Join(fixture.runner.runDir(unrelated.ID), "run.json"), unrelated, fixture.runner.root()); err != nil {
		t.Fatal(err)
	}
	broken := fixture.runner.runDir("unreadable")
	if err := os.MkdirAll(broken, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(broken, "run.json"), []byte("{"), 0600); err != nil {
		t.Fatal(err)
	}
	work, unknown, err := fixture.runner.GoalRuns(first.Record.Goal)
	if err != nil || len(work) != 2 || !slices.Equal(unknown, []string{"run unreadable unavailable"}) {
		t.Fatalf("goal history: %+v, unknown=%v, %v", work, unknown, err)
	}
	for _, record := range []UnitRunRecord{first.Record, other} {
		if !slices.ContainsFunc(work, func(one NamedWork) bool {
			return one.Run == record.ID && one.Unit == record.Unit && one.Record != nil && one.Record.Worktree == record.Worktree
		}) {
			t.Fatalf("history lost run %s in %s: %+v", record.ID, record.Worktree, work)
		}
	}
	if err := fixture.runner.CheckContinuationInputs(first.Record); err != nil {
		t.Fatalf("unchanged inputs refused: %v", err)
	}
	plan, err := ReadUnitPlan(first.Record.Plan)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(plan.Build.Brief, []byte("changed build inputs\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := fixture.runner.CheckContinuationInputs(first.Record); err == nil || !strings.Contains(ErrorDetail(err), "UNIT_NAMED_INPUT_CHANGED") {
		t.Fatalf("changed inputs admitted: %v", err)
	}
	current, err := fixture.runner.Status(first.Record.ID)
	if err != nil || current.State != first.Record.State || !slices.Equal(fixture.starter.order, []string{"build"}) {
		t.Fatalf("input observation advanced work: %+v, launches=%v, %v", current, fixture.starter.order, err)
	}
}
