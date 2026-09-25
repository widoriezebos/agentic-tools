package proofrun

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

type performanceScheduleRun struct {
	launches, faults []string
	results          []GroupResult
}

func runPerformanceSchedule(t *testing.T, fail bool) performanceScheduleRun {
	t.Helper()
	ids := []string{"unit-a", "performance-a", "unit-b", "performance-b", "unit-c"}
	groups := map[string]testpolicy.Group{
		"unit-a":        {ID: "unit-a", Kind: "unit", TargetMS: 1},
		"unit-b":        {ID: "unit-b", Kind: "unit", TargetMS: 3},
		"unit-c":        {ID: "unit-c", Kind: "unit", TargetMS: 2},
		"performance-a": {ID: "performance-a", Kind: "performance", TargetMS: 100},
		"performance-b": {ID: "performance-b", Kind: "performance", TargetMS: 90},
	}
	var mu sync.Mutex
	active := 0
	performanceActive := false
	var launches, faults []string
	var unitBarrier sync.WaitGroup
	unitBarrier.Add(3)
	unitsCompleted := make(chan struct{})
	firstPerformanceStarted := make(chan struct{})
	releaseFirstPerformance := make(chan struct{})
	coordinatorDone := make(chan struct{})
	coordinatorContext, cancelCoordinator := context.WithCancel(context.Background())
	var signalUnitsCompleted sync.Once
	var releasePerformance sync.Once
	go func() {
		defer close(coordinatorDone)
		select {
		case <-unitsCompleted:
		case <-coordinatorContext.Done():
			return
		}
		select {
		case <-firstPerformanceStarted:
			releasePerformance.Do(func() { close(releaseFirstPerformance) })
		case <-coordinatorContext.Done():
		}
	}()
	runGroup := func(_ context.Context, _ TestRunRequest, group testpolicy.Group) GroupResult {
		if group.ID == "performance-b" {
			<-firstPerformanceStarted
		}
		mu.Lock()
		launches = append(launches, group.ID)
		if group.Kind == "performance" {
			if active != 0 {
				faults = append(faults, group.ID+" entered with another group active")
			}
			performanceActive = true
		} else {
			if performanceActive {
				faults = append(faults, group.ID+" entered during performance")
			}
		}
		active++
		mu.Unlock()
		if group.ID == "performance-a" {
			close(firstPerformanceStarted)
		}
		if group.ID == "performance-b" {
			releasePerformance.Do(func() { close(releaseFirstPerformance) })
		}
		if group.Kind != "performance" {
			unitBarrier.Done()
			unitBarrier.Wait()
		} else if group.ID == "performance-a" {
			<-releaseFirstPerformance
		}
		mu.Lock()
		if group.Kind == "performance" {
			if active != 1 {
				faults = append(faults, group.ID+" exited with another group active")
			}
			performanceActive = false
		}
		active--
		mu.Unlock()
		status := "passed"
		if fail && group.ID == "unit-a" {
			status = "failed"
		}
		return GroupResult{ID: group.ID, Kind: group.Kind, Status: status}
	}
	afterAdmission := func(completed map[string]GroupResult) {
		if _, ok := completed["unit-a"]; !ok {
			return
		}
		if _, ok := completed["unit-b"]; !ok {
			return
		}
		if _, ok := completed["unit-c"]; !ok {
			return
		}
		signalUnitsCompleted.Do(func() { close(unitsCompleted) })
	}
	ctx := withStageGroupDependencies(context.Background(), stageGroupDependencies{
		runGroup:       runGroup,
		afterAdmission: afterAdmission,
	})
	results, _, err := runStageGroups(ctx, TestRunRequest{Workers: 3, Concurrency: 3}, groups, ids, &progressWriter{}, fail)
	cancelCoordinator()
	<-coordinatorDone
	if err != nil {
		t.Fatal(err)
	}
	return performanceScheduleRun{launches: launches, faults: faults, results: results}
}
func TestPerformanceScheduling(t *testing.T) {
	run := runPerformanceSchedule(t, false)
	checks := map[string]bool{
		"W1Exclusivity":     len(run.faults) != 0,
		"W2Ordering":        strings.Contains(strings.Join(run.launches[:3], ","), "performance") || run.launches[3] != "performance-a" || run.launches[4] != "performance-b",
		"W4PerformancePair": len(run.faults) != 0,
		"W5PlanOrder":       run.results[0].ID != "unit-a" || run.results[1].ID != "performance-a" || run.results[2].ID != "unit-b" || run.results[3].ID != "performance-b" || run.results[4].ID != "unit-c",
	}
	for name, failed := range checks {
		t.Run(name, func(t *testing.T) {
			if failed {
				t.Fatalf("%s failed: launches=%v faults=%v", name, run.launches, run.faults)
			}
		})
	}
	t.Run("W3Halt", func(t *testing.T) {
		stopped := runPerformanceSchedule(t, true)
		if stopped.results[1].Status != "not-run" || stopped.results[3].Status != "not-run" || stopped.results[1].NotRunReason != haltReason("unit-a") || stopped.results[3].NotRunReason != haltReason("unit-a") || strings.Contains(strings.Join(stopped.launches, ","), "performance") {
			t.Fatalf("W3Halt failed: %v", stopped.launches)
		}
	})
}
