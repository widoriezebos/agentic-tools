package main

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/hostload"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/readsubject"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

func TestRunRevisionRetainsComposedBriefAcrossHostHold(t *testing.T) {
	t.Parallel()
	bed := newEvidenceBed(t)
	bed.manager.Supervisor = &stopReadStarter{bed: bed, reads: [][]readsubject.Finding{{stopFinding("regression", "one.go")}, {}}}
	brief := bed.brief("own.md", "Read each round: yes\nBuild the declared behavior.\n")
	code, built, _ := bed.work("work", "build", bed.id, "--work", "u1", "--brief", brief, "--lines", "20", "--read-tool-calls", "48")
	if code != 0 {
		t.Fatalf("build: code=%d outcome=%s summary=%s", code, built.Outcome, built.Summary)
	}
	run := resultData(t, built)["run"].(string)
	runner := &launch.UnitRunner{Root: bed.unitRoot}
	record, err := runner.Status(run)
	if err != nil {
		t.Fatal(err)
	}
	finding := record.Rounds[0].Reads[0].Findings[0].ID
	correction := "Keep this correction's exact prose.\n## Decisions on round 1\n| " + finding + " | accepted | repair the defect | one.go:12 |\n"
	fix := bed.brief("fix.md", correction)
	bed.manager.CapacitySources.Load = func(at time.Time) hostload.Sample {
		return hostload.Sample{At: at.Format(time.RFC3339Nano), Available: true, Load1m: 9}
	}
	launched := len(bed.starter.launched())
	for range 2 {
		code, held, _ := bed.work("work", "revise", "run:"+run, "--brief", fix)
		if code != 1 || held.Outcome != intentRefused || !strings.Contains(resultWords(held), "host.load-max") || held.Next == nil || !slices.Equal(held.Next.Argv, []string{"metasystem", "work", "build", "run:" + run}) {
			t.Fatalf("revision did not retain the build retry remedy: code=%d outcome=%s summary=%s next=%+v", code, held.Outcome, held.Summary, held.Next)
		}
		record, err = runner.Status(run)
		if err != nil || len(record.Rounds) != 2 || len(record.Revisions) != 1 || record.Rounds[1].Steps[0].State != launch.StepStarting || len(bed.starter.launched()) != launched {
			t.Fatalf("hold consumed another correction or launched: rounds=%d revisions=%d launches=%d error=%v", len(record.Rounds), len(record.Revisions), len(bed.starter.launched())-launched, err)
		}
	}
	planPath := filepath.Join(bed.unitRoot, run, "round-2", "plan.json")
	plan, err := launch.ReadUnitPlan(planPath)
	if err != nil {
		t.Fatal(err)
	}
	frozen, err := os.ReadFile(plan.Build.Brief)
	if err != nil || !strings.Contains(string(frozen), "[begin supplementary brief]") || !strings.Contains(string(frozen), "Keep this correction's exact prose.") {
		t.Fatalf("held correction lost composition: %s %v", frozen, err)
	}
	bed.manager.CapacitySources.Load = func(at time.Time) hostload.Sample {
		return hostload.Sample{At: at.Format(time.RFC3339Nano), Available: true}
	}
	code, resumed, _ := bed.work("work", "build", "run:"+run)
	if code != 0 || resultData(t, resumed)["round"] != float64(2) || composedRound(t, bed, run, 2, correction) != string(frozen) {
		t.Fatalf("build remedy did not resume the frozen correction: code=%d outcome=%s summary=%s", code, resumed.Outcome, resumed.Summary)
	}
}

func TestWorkInvocationKeepsSharedRunnerAuthorityAndCompositionSeparate(t *testing.T) {
	t.Parallel()
	bed := newEvidenceBed(t)
	owners := bed.workOwners()
	shared := owners.work.units(stateroot.Layout{})
	shared.Actor = "person"
	owners.work.units = func(stateroot.Layout) *launch.UnitRunner { return shared }
	bed.manager.CapacitySources.Load = func(at time.Time) hostload.Sample {
		return hostload.Sample{At: at.Format(time.RFC3339Nano), Available: true, Load1m: 9}
	}
	owners.prove = enrolledPersonProver(t, bed.root(), bed.manager.Now())
	brief := bed.brief("own.md", "Read each round: yes\nBuild the declared behavior.\n")
	code, built := bed.runJSON(owners, "work", "build", bed.id, "--work", "u1", "--brief", brief, "--lines", "20", "--read-tool-calls", "48")
	if code != 0 {
		t.Fatalf("person's build: code=%d outcome=%s summary=%s", code, built.Outcome, built.Summary)
	}
	composedRound(t, bed, resultData(t, built)["run"].(string), 1, string(mustRead(t, filepath.Join(bed.root(), brief))))
	releaseFleetBuild(t, bed, built)
	owners.prove = func(string, int64, humanauthority.Reader, string, string, time.Time) (humanauthority.Proof, error) {
		return humanauthority.Proof{}, errors.New("agent process")
	}
	launched := len(bed.starter.launched())
	code, held := bed.runJSON(owners, "work", "build", bed.id, "--work", "u2", "--brief", brief, "--lines", "20", "--read-tool-calls", "48")
	if code != 1 || held.Outcome != intentRefused || !strings.Contains(resultWords(held), "host.load-max") || len(bed.starter.launched()) != launched {
		t.Fatalf("agent inherited the person's build authority: code=%d outcome=%s summary=%s launches=%d", code, held.Outcome, held.Summary, len(bed.starter.launched())-launched)
	}
	if shared.Actor != "person" || shared.ComposeUnitBrief != nil || shared.FreezeCheck != nil || shared.Manager != bed.manager || bed.manager.BuildPolicy.Policy != nil {
		t.Fatal("invocation modified the shared runner or manager")
	}
}
