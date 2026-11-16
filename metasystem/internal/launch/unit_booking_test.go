package launch

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestBookEndedReadCollectsWithoutDrivingARunningRound(t *testing.T) {
	t.Parallel()
	// U2 mutation: skipping collection leaves ended launches unaccounted.
	fixture := newUnitFixture(t, "")
	result, err := fixture.runner.Advance(UnitRequest{Plan: fixture.plan})
	if err != nil {
		t.Fatal(err)
	}
	record := result.Record
	// Preservation: launch stamps keep the retained run's schema unchanged.
	data, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{`"UnitRun"`, `"UnitInstallation"`} {
		if bytes.Contains(data, []byte(field)) {
			t.Fatalf("run record retained the launch stamp field %s", field)
		}
	}
	record.State = "running"
	if err := fixture.runner.save(record); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(fixture.runner.runDir(record.ID), "run.json")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	collected := map[string]bool{}
	fixture.runner.CollectLaunch = func(_ UnitRunRecord, execution Record, _ string) error { collected[execution.ID] = true; return nil }
	launched := len(fixture.starter.ids)
	if err := fixture.runner.BookEndedRead(record.ID); err != nil {
		t.Fatal(err)
	}
	for _, step := range record.Rounds[0].Steps {
		if !collected[step.LaunchID] {
			t.Fatalf("ended launch was not collected: %s", step.LaunchID)
		}
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(before, after) || len(fixture.starter.ids) != launched {
		t.Fatalf("booking drove or rewrote a running round: %v", err)
	}
}

func TestBookEndedReadSettlesACancelledRunsUnstartedStep(t *testing.T) {
	t.Parallel()
	fixture := newUnitFixture(t, "")
	result, err := fixture.runner.Advance(UnitRequest{Plan: fixture.plan})
	if err != nil {
		t.Fatal(err)
	}
	record := result.Record
	record.State = "running"
	record.Rounds[0].Steps = []UnitStep{{Name: "build", State: StepStarting, LaunchID: "unstarted"}}
	var collected []Record
	fixture.runner.CollectLaunch = func(_ UnitRunRecord, execution Record, _ string) error {
		collected = append(collected, execution)
		return nil
	}
	if err := fixture.runner.save(record); err != nil {
		t.Fatal(err)
	}
	if err := fixture.runner.BookEndedRead(record.ID); err != nil || len(collected) != 0 {
		t.Fatalf("a live run's starting step was collected: %v %v", collected, err)
	}
	record.Rounds[0].Steps[0].LaunchIDs = []string{"unstarted"}
	if err := fixture.runner.save(record); err != nil {
		t.Fatal(err)
	}
	if err := fixture.runner.BookEndedRead(record.ID); err != nil || len(collected) != 0 {
		t.Fatalf("a live run's retained starting id was collected: %v %v", collected, err)
	}
	record.Rounds[0].Steps[0].LaunchIDs = nil
	record.State = "cancelled"
	if err := fixture.runner.save(record); err != nil {
		t.Fatal(err)
	}
	for repeat := 0; repeat < 2; repeat++ {
		fixture.manager.Sleep(time.Minute)
		collected = nil
		if err := fixture.runner.BookEndedRead(record.ID); err != nil {
			t.Fatal(err)
		}
		if len(collected) != 1 || collected[0].ID != "unstarted" || collected[0].State != Cancelled || collected[0].Executed() || collected[0].StartedAt == "" || collected[0].StartedAt != collected[0].FinishedAt {
			t.Fatalf("an uncreated launch was not collected once as never executed: %+v", collected)
		}
		// The current id can also already be among the step's retained ids.
		record.Rounds[0].Steps[0].LaunchIDs = []string{"unstarted"}
		if err := fixture.runner.save(record); err != nil {
			t.Fatal(err)
		}
	}
}
