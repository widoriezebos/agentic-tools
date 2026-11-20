package main

import (
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
)

// Build, check, and cancellation go through the public verbs and real stores;
// only the provider process and Git adapters are fixtures.
func TestSeatContinuationReadsThePublicWorkRecord(t *testing.T) {
	t.Parallel()
	bed := newWorkBed(t)
	brief := bed.brief("progress.md", "Build the unit.\n\n| Unit | Lines |\n| --- | --- |\n| first | 40 |\n")
	code, result, _ := bed.work(append([]string{"work", "build", bed.id, "first", "--brief", brief}, workCheck...)...)
	if code != 0 {
		t.Fatalf("build: %+v", result)
	}
	run := resultData(t, result)["run"].(string)
	var cfg steward.TickConfig
	wireStewardSeat(&cfg, bed.workOwners())
	units, err := cfg.Units(bed.root(), bed.id)
	if err != nil || len(units) != 1 || !units[0].Continuable {
		t.Fatalf("completed work has no durable continuation: %+v %v", units, err)
	}
	foundBuild, foundCheck := false, false
	for _, mark := range units[0].Advances {
		foundBuild = foundBuild || strings.HasSuffix(mark, "/build")
		foundCheck = foundCheck || strings.Contains(mark, "/proof:")
	}
	if !foundBuild || !foundCheck {
		t.Fatalf("work milestones absent: %+v", units)
	}
	runner := &launch.UnitRunner{Manager: bed.manager, Root: bed.unitRoot, Git: workGit{bed}}
	if _, err := runner.CancelRun(run); err != nil {
		t.Fatal(err)
	}
	units, err = cfg.Units(bed.root(), bed.id)
	if err != nil || len(units) != 1 || units[0].Continuable {
		t.Fatalf("cancelled unit was called continuable: %+v %v", units, err)
	}
}
