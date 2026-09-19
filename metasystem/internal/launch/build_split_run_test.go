package launch

import (
	"encoding/json"
	"os"
	"slices"
	"strings"
	"testing"
)

func TestOversizeBuildRunsSerialGroupsInOrder(t *testing.T) {
	t.Parallel()
	fixture := threeUnitBuildFixture(t)
	completedBeforeSecond := false
	builds := 0
	fixture.starter.onStart = func(record Record) error {
		if record.Kind != "build" {
			return nil
		}
		builds++
		if builds == 2 {
			first, err := fixture.manager.Store.Read(fixture.starter.ids[0])
			completedBeforeSecond = err == nil && first.State == Completed && first.ExitCode != nil && *first.ExitCode == 0
		}
		return nil
	}
	result, err := fixture.runner.Advance(UnitRequest{Plan: fixture.plan})
	if err != nil {
		t.Fatal(err)
	}
	round := result.Record.Rounds[0]
	var groups [][]string
	for _, step := range round.Steps {
		if strings.HasPrefix(step.Name, "build") {
			groups = append(groups, step.Units)
			data, readErr := os.ReadFile(step.Brief)
			if readErr != nil || !strings.Contains(string(data), "Build units: "+strings.Join(step.Units, ",")) {
				t.Fatalf("brief=%s data=%q err=%v", step.Brief, data, readErr)
			}
		}
	}
	if len(groups) != 2 || !slices.Equal(groups[0], []string{"a", "b"}) || !slices.Equal(groups[1], []string{"c"}) || !completedBeforeSecond || round.Outcome != "green" {
		t.Fatalf("groups=%v completed-before-second=%t outcome=%s", groups, completedBeforeSecond, round.Outcome)
	}
}

func TestFailedFirstBuildGroupStopsTheRound(t *testing.T) {
	t.Parallel()
	fixture := threeUnitBuildFixture(t)
	fixture.starter.failKind = "build"
	result, err := fixture.runner.Advance(UnitRequest{Plan: fixture.plan})
	if err != nil {
		t.Fatal(err)
	}
	round := result.Record.Rounds[0]
	if round.Outcome != "build-failed" || len(fixture.starter.ids) != 1 || round.Steps[1].State != StepSkipped || round.Steps[1].Reason != "build-failed" {
		t.Fatalf("launches=%v round=%+v", fixture.starter.ids, round)
	}
}

func threeUnitBuildFixture(t *testing.T) unitFixture {
	t.Helper()
	fixture := newUnitFixture(t, "")
	fixture.manager.Settings.BuildLinesCap = 1500
	data, err := os.ReadFile(fixture.plan)
	if err != nil {
		t.Fatal(err)
	}
	var plan UnitPlan
	if err := json.Unmarshal(data, &plan); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(plan.Build.UnitsPage, []byte("| Unit | Lines |\n|---|---|\n| a | 700 |\n| b | 700 |\n| c | 700 |\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	plan.Build.Units = []string{"a", "b", "c"}
	data, err = json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fixture.plan, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return fixture
}
