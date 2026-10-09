package main

import (
	"os"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
)

func TestStatusAndTheSeatPromptShareOneUnitList(t *testing.T) {
	t.Parallel()
	bed := newWorkBed(t)
	brief := bed.brief("shared.md", "Working Mode: implement\n\nBuild the unit.\n\n| Unit | Lines |\n| --- | --- |\n| first | 40 |\n| second | 40 |\n")
	for _, name := range []string{"first", "second"} {
		code, result, _ := bed.work(append([]string{"work", "build", bed.id, name, "--brief", brief}, workCheck...)...)
		if code != 0 {
			t.Fatalf("build: %+v", result)
		}
		if name == "first" {
			if _, err := (&launch.UnitRunner{Manager: bed.manager, Root: bed.unitRoot, Git: workGit{bed}}).CancelRun(resultData(t, result)["run"].(string)); err != nil {
				t.Fatal(err)
			}
		}
	}
	var config steward.TickConfig
	wireStewardSeat(&config, bed.workOwners())
	units, err := config.Units(bed.root(), bed.id)
	if err != nil || len(units) != 2 {
		t.Fatalf("seat units=%+v err=%v", units, err)
	}
	for _, argv := range [][]string{{"status", bed.id}, {"work", "status", bed.id}} {
		code, stdout, stderr := bed.run(bed.workOwners(), argv...)
		if code != 0 {
			t.Fatalf("status: %s %s", stdout, stderr)
		}

		for _, unit := range units {
			if !strings.Contains(stdout, strings.TrimSpace(unit.Line)) || !strings.Contains(unit.Line, unit.Stage) {
				t.Fatalf("status and seat disagree: %+v %s", units, stdout)
			}
		}
	}
	// The same answers are insufficient if a second stage loop can drift.
	source, err := os.ReadFile("steward_seat.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(source), "inv.goalUnitStages(id)") {
		t.Fatal("the seat's unit reader must call the shared goalUnitStages owner")
	}
}

func TestGoalStatusKeepsRunReviewActAndContinuationRefusal(t *testing.T) {
	t.Parallel()
	bed := newWorkBed(t)
	bed.starter.hold = "build"
	brief := bed.brief("status.md", "Build the unit.\n")
	code, built, _ := bed.work(append([]string{"work", "build", bed.id, "status-unit", "--brief", brief, "--lines", "5"}, workCheck...)...)
	if code != 3 {
		t.Fatalf("held build: code=%d %+v", code, built)
	}
	run := resultData(t, built)["run"].(string)
	runner := &launch.UnitRunner{Manager: bed.manager, Root: bed.unitRoot, Git: workGit{bed}}
	record, err := runner.Status(run)
	if err != nil {
		t.Fatal(err)
	}
	act := launch.UnitReviewAct{Key: launch.ReviewActKey(record), Round: len(record.Rounds), State: "prepared", Summary: "review the retained result"}
	if err := runner.RetainReviewAct(run, act); err != nil {
		t.Fatal(err)
	}
	plan, err := launch.ReadUnitPlan(record.Plan)
	if err != nil {
		t.Fatal(err)
	}
	restore := tamper(t, plan.Build.Brief)
	defer restore()
	launches := len(bed.starter.launched())
	for _, argv := range [][]string{{"status", bed.id}, {"work", "status", bed.id}} {
		code, status, _ := bed.work(argv...)
		if code != 0 {
			t.Fatalf("status: code=%d %+v", code, status)
		}
		views := resultData(t, status)["work"].([]any)
		if len(views) != 1 {
			t.Fatalf("unit views: %+v", views)
		}
		view := views[0].(map[string]any)
		if view["run"] != run || view["state"] != record.State {
			t.Fatalf("status lost the run identity: %+v", view)
		}
		if retained, ok := view["reviewAct"].(map[string]any); !ok || retained["key"] != act.Key || retained["state"] != act.State {
			t.Fatalf("status lost the review action: %+v", view)
		}
		if stage, ok := view["stage"].(string); !ok || !strings.Contains(stage, "continuation refused:") || !strings.Contains(stage, "UNIT_NAMED_INPUT_CHANGED") {
			t.Fatalf("status hid changed continuation inputs: %+v", view)
		}
	}
	if len(bed.starter.launched()) != launches {
		t.Fatalf("status launched work: %v", bed.starter.launched())
	}
}
