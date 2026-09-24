package proofrun

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

func readyAdmissionWorkers(workers int) *int { return &workers }

func readyAdmissionResult(group testpolicy.Group, status string) GroupResult {
	identity := NativeTestIdentity{Classname: "fixture", Name: group.ID, Status: status}
	return GroupResult{ID: group.ID, Kind: group.Kind, Status: status, NativeLaunched: true,
		Expected: []NativeTestIdentity{identity}, Observed: []NativeTestIdentity{identity}}
}

func TestReadyWorkerAdmissionUsesFreeCapacityAndDrainsForOldest(t *testing.T) {
	t.Parallel()

	ids := []string{"running", "wide-a", "wide-b", "wide-c", "narrow"}
	groups := make(map[string]testpolicy.Group, len(ids))
	for index, id := range ids {
		workers := 4
		if id == "running" || id == "narrow" {
			workers = 1
		}
		groups[id] = testpolicy.Group{ID: id, Kind: "unit", Adapter: "command", TargetMS: int64(len(ids) - index),
			Resources: testpolicy.GroupResources{Workers: readyAdmissionWorkers(workers)}}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx = withTestWorkerPool(ctx, 4)
	pool := testWorkerPoolFromContext(t, ctx)
	releaseThree, err := acquireTestWorkers(ctx, 3)
	if err != nil {
		t.Fatal(err)
	}
	defer releaseThree()
	releaseOne, err := acquireTestWorkers(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	defer releaseOne()

	runningStarted := make(chan struct{})
	narrowStarted := make(chan struct{})
	wideStarted := make(chan string, 3)
	releaseRunning := make(chan struct{})
	releaseNarrow := make(chan struct{})
	releaseWide := make(chan struct{})
	runGroup := func(ctx context.Context, _ TestRunRequest, group testpolicy.Group) GroupResult {
		switch group.ID {
		case "running":
			close(runningStarted)
			select {
			case <-releaseRunning:
			case <-ctx.Done():
			}
		case "narrow":
			close(narrowStarted)
			select {
			case <-releaseNarrow:
			case <-ctx.Done():
			}
		default:
			wideStarted <- group.ID
			select {
			case <-releaseWide:
			case <-ctx.Done():
			}
		}
		status := "passed"
		if group.ID == "wide-b" {
			status = "failed"
		}
		return readyAdmissionResult(group, status)
	}
	ctx = withStageGroupDependencies(ctx, stageGroupDependencies{runGroup: runGroup})
	type stageRun struct {
		results []GroupResult
		err     error
	}
	done := make(chan stageRun, 1)
	go func() {
		results, _, err := runStageGroups(ctx, TestRunRequest{Workers: 4, Concurrency: 4}, groups, ids, &progressWriter{}, false)
		done <- stageRun{results: results, err: err}
	}()

	releaseOne()
	<-runningStarted
	releaseThree()
	<-narrowStarted
	close(releaseNarrow)
	select {
	case id := <-wideStarted:
		t.Fatalf("wide group %q started with only three workers free", id)
	default:
	}
	close(releaseRunning)
	for _, want := range []string{"wide-a", "wide-b", "wide-c"} {
		if got := <-wideStarted; got != want {
			t.Fatalf("wide admission=%q want=%q", got, want)
		}
		releaseWide <- struct{}{}
	}
	run := <-done
	if run.err != nil {
		t.Fatal(run.err)
	}
	wantStatus := []string{"passed", "passed", "failed", "passed", "passed"}
	if len(run.results) != len(ids) {
		t.Fatalf("result census=%d want=%d: %+v", len(run.results), len(ids), run.results)
	}
	for index, result := range run.results {
		if result.ID != ids[index] || result.Status != wantStatus[index] || !result.NativeLaunched ||
			len(result.Expected) != 1 || len(result.Observed) != 1 ||
			result.Expected[0].Name != ids[index] || result.Observed[0].Status != wantStatus[index] {
			t.Fatalf("result %d lost plan identity, census, or outcome: %+v", index, result)
		}
	}
	if available, waiters := testWorkerPoolState(pool); available != 4 || waiters != 0 {
		t.Fatalf("stage leaked worker capacity: available=%d waiters=%d", available, waiters)
	}
}

func TestReadyWorkerAdmissionDrainsBeforeNewlyReadyFittingGroup(t *testing.T) {
	t.Parallel()

	ids := []string{"running", "probe", "wide", "bypass", "late"}
	groups := map[string]testpolicy.Group{
		"running": {ID: "running", Kind: "unit", Adapter: "command", TargetMS: 50,
			Resources: testpolicy.GroupResources{Workers: readyAdmissionWorkers(1)}},
		"probe": {ID: "probe", Kind: "unit", Adapter: "go", TargetMS: 40},
		"wide": {ID: "wide", Kind: "unit", Adapter: "command", TargetMS: 30,
			Resources: testpolicy.GroupResources{Workers: readyAdmissionWorkers(4)}},
		"bypass": {ID: "bypass", Kind: "unit", Adapter: "command", TargetMS: 20,
			Resources: testpolicy.GroupResources{Workers: readyAdmissionWorkers(1)}},
		"late": {ID: "late", Kind: "unit", Adapter: "command", TargetMS: 10, Requires: []string{"bypass"},
			Resources: testpolicy.GroupResources{Workers: readyAdmissionWorkers(1)}},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx = withTestWorkerPool(ctx, 4)
	pool := testWorkerPoolFromContext(t, ctx)
	releaseThree, err := acquireTestWorkers(ctx, 3)
	if err != nil {
		t.Fatal(err)
	}
	defer releaseThree()
	releaseOne, err := acquireTestWorkers(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	defer releaseOne()

	started := make(chan string, len(ids))
	releases := map[string]chan struct{}{
		"running": make(chan struct{}),
		"probe":   make(chan struct{}),
		"wide":    make(chan struct{}),
		"bypass":  make(chan struct{}),
		"late":    make(chan struct{}),
	}
	afterAdmission := make(chan struct{})
	releaseAdmission := make(chan struct{})
	checkpointSent := false
	ctx = withStageGroupDependencies(ctx, stageGroupDependencies{
		runGroup: func(ctx context.Context, _ TestRunRequest, group testpolicy.Group) GroupResult {
			started <- group.ID
			select {
			case <-releases[group.ID]:
			case <-ctx.Done():
			}
			return readyAdmissionResult(group, "passed")
		},
		afterAdmission: func(completed map[string]GroupResult) {
			if checkpointSent {
				return
			}
			if _, ok := completed["probe"]; !ok {
				return
			}
			checkpointSent = true
			select {
			case afterAdmission <- struct{}{}:
			case <-ctx.Done():
				return
			}
			select {
			case <-releaseAdmission:
			case <-ctx.Done():
			}
		},
	})
	type stageRun struct {
		results []GroupResult
		err     error
	}
	done := make(chan stageRun, 1)
	go func() {
		results, _, err := runStageGroups(ctx, TestRunRequest{Workers: 4, Concurrency: len(ids)}, groups, ids, &progressWriter{}, false)
		done <- stageRun{results: results, err: err}
	}()

	releaseOne()
	initial := map[string]bool{<-started: true, <-started: true}
	if !initial["running"] || !initial["probe"] {
		t.Fatalf("initial admissions=%v want running and probe", initial)
	}
	waitForTestWorkerPoolWaiters(pool, 2)
	releaseThree()
	if got := <-started; got != "bypass" {
		t.Fatalf("allowed bypass admission=%q want bypass", got)
	}
	pool.mu.Lock()
	bypassReleaseChanged := pool.changed
	pool.mu.Unlock()

	close(releases["bypass"])
	<-bypassReleaseChanged
	close(releases["probe"])
	<-afterAdmission

	availableDuringDrain, waitersDuringDrain := testWorkerPoolState(pool)
	select {
	case got := <-started:
		t.Fatalf("group %q received a forbidden grant during the drain decision", got)
	default:
	}
	close(releases["running"])
	close(releaseAdmission)
	if got := <-started; got != "wide" {
		t.Fatalf("first admission after drain barrier=%q want wide", got)
	}
	close(releases["wide"])
	if got := <-started; got != "late" {
		t.Fatalf("admission after wide=%q want late", got)
	}
	close(releases["late"])

	run := <-done
	if run.err != nil {
		t.Fatal(run.err)
	}
	if availableDuringDrain != 3 {
		t.Errorf("newly ready fitting group consumed capacity during drain: available=%d want=3", availableDuringDrain)
	}
	if waitersDuringDrain != 1 {
		t.Errorf("oldest wide group was not the sole pending worker request: waiters=%d want=1", waitersDuringDrain)
	}
	if len(run.results) != len(ids) {
		t.Fatalf("result census=%d want=%d: %+v", len(run.results), len(ids), run.results)
	}
	for index, result := range run.results {
		if result.ID != ids[index] || result.Status != "passed" || !result.NativeLaunched ||
			len(result.Expected) != 1 || len(result.Observed) != 1 ||
			result.Expected[0].Name != ids[index] || result.Observed[0].Status != "passed" {
			t.Fatalf("result %d lost plan identity, census, or outcome: %+v", index, result)
		}
	}
	if available, waiters := testWorkerPoolState(pool); available != 4 || waiters != 0 {
		t.Fatalf("stage leaked worker capacity: available=%d waiters=%d", available, waiters)
	}
}

func TestReadyWorkerAdmissionPreservesRecordedStopCauseDuringCanceledAcquire(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name          string
		progressError bool
	}{
		{name: "stop at first failure"},
		{name: "progress record failure", progressError: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			ids := []string{"failed-go", "waiting-command"}
			groups := map[string]testpolicy.Group{
				"failed-go":       {ID: "failed-go", Kind: "unit", Adapter: "go", TargetMS: 2},
				"waiting-command": {ID: "waiting-command", Kind: "unit", Adapter: "command", TargetMS: 1},
			}
			progress := &progressWriter{}
			if test.progressError {
				progress.path = t.TempDir()
			}
			ctx := withTestWorkerPool(context.Background(), 1)
			pool := testWorkerPoolFromContext(t, ctx)
			stopRecorded := make(chan struct{})
			ctx = withStageGroupDependencies(ctx, stageGroupDependencies{
				runGroup: func(_ context.Context, _ TestRunRequest, group testpolicy.Group) GroupResult {
					return readyAdmissionResult(group, "failed")
				},
				deliverCompletion: func(completion chan<- stageGroupCompletion, outcome stageGroupCompletion) {
					close(stopRecorded)
					completion <- outcome
				},
			})
			type stageRun struct {
				results  []GroupResult
				haltedBy string
				err      error
			}
			done := make(chan stageRun, 1)
			pool.mu.Lock()
			poolLocked := true
			defer func() {
				if poolLocked {
					pool.mu.Unlock()
				}
			}()
			go func() {
				results, haltedBy, err := runStageGroups(ctx, TestRunRequest{Workers: 1, Concurrency: 2},
					groups, ids, progress, !test.progressError)
				done <- stageRun{results: results, haltedBy: haltedBy, err: err}
			}()

			<-stopRecorded
			pool.mu.Unlock()
			poolLocked = false
			run := <-done
			if test.progressError {
				if run.haltedBy != "" || run.err == nil || errors.Is(run.err, context.Canceled) ||
					!strings.Contains(run.err.Error(), "open suite progress journal") ||
					len(run.results) != len(ids) || run.results[0].Status != "not-run" || run.results[0].NativeLaunched ||
					run.results[1].Status != "not-run" || run.results[1].NativeLaunched {
					t.Fatalf("progress stop cause changed: haltedBy=%q results=%+v err=%v", run.haltedBy, run.results, run.err)
				}
			} else {
				if run.err != nil || run.haltedBy != "failed-go" {
					t.Fatalf("recorded stop cause changed: haltedBy=%q err=%v", run.haltedBy, run.err)
				}
				if len(run.results) != len(ids) || run.results[0].ID != "failed-go" || run.results[0].Status != "failed" ||
					!run.results[0].NativeLaunched || run.results[1].ID != "waiting-command" ||
					run.results[1].Status != "not-run" || run.results[1].NativeLaunched ||
					run.results[1].NotRunReason != haltReason("failed-go") {
					t.Fatalf("recorded stop results changed: %+v", run.results)
				}
			}
			if available, waiters := testWorkerPoolState(pool); available != 1 || waiters != 0 {
				t.Fatalf("recorded stop leaked worker state: available=%d waiters=%d", available, waiters)
			}
		})
	}
}

func TestReadyWorkerAdmissionCancellationReleasesReservation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	ctx = withTestWorkerPool(ctx, 1)
	pool := testWorkerPoolFromContext(t, ctx)
	reserved := make(chan struct{})
	continueDispatch := make(chan struct{})
	launched := make(chan struct{}, 1)
	ctx = withStageGroupDependencies(ctx, stageGroupDependencies{
		beforeDispatch: func(string) {
			close(reserved)
			<-continueDispatch
		},
		runGroup: func(_ context.Context, _ TestRunRequest, group testpolicy.Group) GroupResult {
			launched <- struct{}{}
			return readyAdmissionResult(group, "passed")
		},
	})
	group := testpolicy.Group{ID: "reserved", Kind: "unit", Adapter: "command"}
	done := make(chan struct {
		results []GroupResult
		err     error
	}, 1)
	go func() {
		results, _, err := runStageGroups(ctx, TestRunRequest{Workers: 1, Concurrency: 1},
			map[string]testpolicy.Group{group.ID: group}, []string{group.ID}, &progressWriter{}, false)
		done <- struct {
			results []GroupResult
			err     error
		}{results, err}
	}()
	<-reserved
	if available, _ := testWorkerPoolState(pool); available != 0 {
		t.Fatalf("group reached dispatch without owning its worker: available=%d", available)
	}
	cancel()
	close(continueDispatch)
	run := <-done
	if !errors.Is(run.err, context.Canceled) || len(run.results) != 1 || run.results[0].Status != "not-run" {
		t.Fatalf("canceled reservation: results=%+v err=%v", run.results, run.err)
	}
	select {
	case <-launched:
		t.Fatal("canceled reserved group reached native launch")
	default:
	}
	if available, waiters := testWorkerPoolState(pool); available != 1 || waiters != 0 {
		t.Fatalf("cancellation leaked reservation: available=%d waiters=%d", available, waiters)
	}
}

func TestReadyWorkerAdmissionRejectsImpossibleGrant(t *testing.T) {
	t.Parallel()

	group := testpolicy.Group{ID: "impossible", Kind: "unit", Adapter: "command",
		Resources: testpolicy.GroupResources{Workers: readyAdmissionWorkers(3)}}
	launched := make(chan struct{}, 1)
	ctx := withStageGroupDependencies(context.Background(), stageGroupDependencies{runGroup: func(_ context.Context, _ TestRunRequest, group testpolicy.Group) GroupResult {
		launched <- struct{}{}
		return readyAdmissionResult(group, "passed")
	}})
	results, _, err := runStageGroups(ctx, TestRunRequest{Workers: 2, Concurrency: 1},
		map[string]testpolicy.Group{group.ID: group}, []string{group.ID}, &progressWriter{}, false)
	if err == nil || !strings.Contains(err.Error(), "requests 3 workers from attempt allowance 2") ||
		len(results) != 1 || results[0].Status != "not-run" || results[0].NativeLaunched {
		t.Fatalf("impossible grant: results=%+v err=%v", results, err)
	}
	select {
	case <-launched:
		t.Fatal("impossible group reached native launch")
	default:
	}
}

func TestReadyWorkerAdmissionLeavesGoWorkersToShards(t *testing.T) {
	t.Parallel()

	ctx := withTestWorkerPool(context.Background(), 2)
	pool := testWorkerPoolFromContext(t, ctx)
	releaseSetup, err := acquireTestWorkers(ctx, 2)
	if err != nil {
		t.Fatal(err)
	}
	defer releaseSetup()
	started := make(chan struct{})
	group := testpolicy.Group{ID: "go", Kind: "unit", Adapter: "go"}
	ctx = withStageGroupDependencies(ctx, stageGroupDependencies{runGroup: func(_ context.Context, _ TestRunRequest, group testpolicy.Group) GroupResult {
		close(started)
		return readyAdmissionResult(group, "passed")
	}})
	done := make(chan error, 1)
	go func() {
		results, _, err := runStageGroups(ctx, TestRunRequest{Workers: 2, Concurrency: 1},
			map[string]testpolicy.Group{group.ID: group}, []string{group.ID}, &progressWriter{}, false)
		if err == nil && (len(results) != 1 || results[0].Status != "passed") {
			err = errors.New("Go group result census or outcome changed")
		}
		done <- err
	}()
	<-started
	if available, _ := testWorkerPoolState(pool); available != 0 {
		t.Fatalf("Go parent changed the shard-owned pool: available=%d", available)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestReadyWorkerAdmissionKeepsPerformanceExclusive(t *testing.T) {
	t.Parallel()

	ctx := withTestWorkerPool(context.Background(), 3)
	pool := testWorkerPoolFromContext(t, ctx)
	ids := []string{"unit-one", "performance", "unit-two"}
	groups := map[string]testpolicy.Group{
		"unit-one":    {ID: "unit-one", Kind: "unit", Adapter: "command", TargetMS: 20},
		"performance": {ID: "performance", Kind: "performance", Adapter: "command", TargetMS: 100},
		"unit-two":    {ID: "unit-two", Kind: "unit", Adapter: "command", TargetMS: 10},
	}
	unitsStarted := make(chan struct{}, 2)
	releaseUnits := make(chan struct{})
	performanceStarted := make(chan int, 1)
	runGroup := func(_ context.Context, _ TestRunRequest, group testpolicy.Group) GroupResult {
		if group.Kind == "performance" {
			available, _ := testWorkerPoolState(pool)
			performanceStarted <- available
		} else {
			unitsStarted <- struct{}{}
			<-releaseUnits
		}
		return readyAdmissionResult(group, "passed")
	}
	ctx = withStageGroupDependencies(ctx, stageGroupDependencies{runGroup: runGroup})
	done := make(chan struct {
		results []GroupResult
		err     error
	}, 1)
	go func() {
		results, _, err := runStageGroups(ctx, TestRunRequest{Workers: 3, Concurrency: 3}, groups, ids, &progressWriter{}, false)
		done <- struct {
			results []GroupResult
			err     error
		}{results, err}
	}()
	<-unitsStarted
	<-unitsStarted
	select {
	case <-performanceStarted:
		t.Fatal("performance group started before ordinary groups drained")
	default:
	}
	close(releaseUnits)
	if available := <-performanceStarted; available != 0 {
		t.Fatalf("performance group did not own the whole pool: available=%d", available)
	}
	run := <-done
	if run.err != nil || len(run.results) != len(ids) {
		t.Fatalf("performance run: results=%+v err=%v", run.results, run.err)
	}
	for index, result := range run.results {
		if result.ID != ids[index] || result.Status != "passed" || !result.NativeLaunched {
			t.Fatalf("performance result %d changed: %+v", index, result)
		}
	}
	if available, waiters := testWorkerPoolState(pool); available != 3 || waiters != 0 {
		t.Fatalf("performance stage leaked capacity: available=%d waiters=%d", available, waiters)
	}
}
