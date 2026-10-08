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
