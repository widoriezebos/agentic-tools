package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
)

func TestProcessStoppedStepRetainsCostsAndStatus(t *testing.T) {
	t.Parallel()
	bed, processes := failedCommandBed(t, 2)
	want := []string{"go", "test", "-timeout", "30m", "-run", "^TestDeclared$", "./..."}
	bed.declaredCheap = shellCommand(want)
	unitHook := bed.workOwnersHook
	page, accepted := processEstimatePage(t, bed, "70", "10")
	accepted = []byte(strings.ReplaceAll(strings.ReplaceAll(string(accepted), "| evidence |", "| outcome |"), "| 250 |", "| 1 |"))
	if err := os.WriteFile(page, accepted, 0600); err != nil {
		t.Fatal(err)
	}
	processCommittedPage(t, bed, page, accepted)
	pageHook := bed.workOwnersHook
	bed.workOwnersHook = func(owners *intentWorkOwners) {
		unitHook(owners)
		pageHook(owners)
	}
	code, result := failedCommandBuild(t, bed)
	record := failedCommandRecord(t, bed, result)
	round := record.Rounds[0]
	if code != 1 || round.Stop == nil || round.Stop.Cause.Kind != "environment" || round.Result == nil || round.Result.Tree == "" || round.Result.ProofIdentity == "" || len(processes.commands) != 2 {
		t.Fatalf("stopped round lost its result or retry: exit=%d round=%+v", code, round)
	}
	plan, err := launch.ReadUnitPlan(filepath.Join(round.Directory, "plan.json"))
	if err != nil || plan.Estimate == nil || plan.Estimate.ElapsedMinutes != 70 {
		t.Fatalf("stopped run lost the frozen estimate: %+v %v", plan.Estimate, err)
	}
	step := round.Steps[1]
	if plan.Check == nil || plan.Check.Cheap != shellCommand(want) || step.Kind != "attest" || step.Command == nil || len(step.Command.Argv) != 7 || !slices.Equal(step.Command.Argv[1:6], []string{"test", "run", "--unit-run", record.ID, "--repo"}) || step.Command.Name != "unit-check" || step.Retained == nil || len(step.LaunchIDs) != 2 {
		t.Fatalf("step lost command evidence or retained retry inputs: %+v", step)
	}
	execution, err := bed.manager.Store.Read(step.LaunchID)
	if err != nil || step.ExecutionStartedAt == "" || step.ExecutionEndedAt == "" || step.ExecutionStartedAt != execution.StartedAt || step.ExecutionEndedAt != execution.FinishedAt || step.FinishedAt == "" {
		t.Fatalf("failed step lost execution and collection intervals: %+v %v", step, err)
	}
	code, status, _ := bed.work("goal", "status", bed.id)
	if code != 0 {
		t.Fatalf("status exit=%d: %+v", code, status)
	}
	data := resultData(t, status)
	views := data["work"].([]any)
	if len(views) != 1 {
		t.Fatalf("status work: %+v", views)
	}
	view := views[0].(map[string]any)
	stop, ok := view["stop"].(map[string]any)
	if !ok || stop["decision"] != "stop" || view["run"] != record.ID || view["state"] != "awaiting-judgement" {
		t.Fatalf("status lost the stop or current run: %+v", view)
	}
	for _, raw := range []any{view["measures"], data["measures"]} {
		measure := processReportMeasures(t, raw)
		if measure.EstimateMinutes == nil || *measure.EstimateMinutes != 70 || measure.Hours["build"] == nil || measure.Hours["attest"] == nil || !slices.Contains(measure.Sources, step.LaunchID) {
			t.Fatalf("status lost costs beside the stop: %+v", measure)
		}
	}
}
