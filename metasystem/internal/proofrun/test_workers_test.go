package proofrun

import (
	"bufio"
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"syscall"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

func testWorkerPoolFromContext(t *testing.T, ctx context.Context) *testWorkerPool {
	t.Helper()
	pool, ok := ctx.Value(testWorkerPoolContextKey{}).(*testWorkerPool)
	if !ok || pool == nil {
		t.Fatal("test worker pool is absent")
	}
	return pool
}

func waitForTestWorkerPoolWaiters(pool *testWorkerPool, want int) {
	for {
		pool.mu.Lock()
		got := pool.waiters
		pool.mu.Unlock()
		if got == want {
			return
		}
		runtime.Gosched()
	}
}

func testWorkerPoolState(pool *testWorkerPool) (available, waiters int) {
	pool.mu.Lock()
	defer pool.mu.Unlock()
	return pool.available, pool.waiters
}

func TestWorkerPoolValidatesRequestsAndPreservesParent(t *testing.T) {
	t.Parallel()

	if _, err := acquireTestWorkers(context.Background(), 1); err == nil {
		t.Fatal("acquire without a pool succeeded")
	}
	ctx := withTestWorkerPool(context.Background(), 2)
	ctx = withTestWorkerPool(ctx, 9)
	if _, err := acquireTestWorkers(ctx, 0); err == nil {
		t.Fatal("zero-worker acquire succeeded")
	}
	if _, err := acquireTestWorkers(ctx, 3); err == nil {
		t.Fatal("oversized acquire succeeded after a nested pool replacement")
	}
	release, err := acquireTestWorkers(ctx, 2)
	if err != nil {
		t.Fatal(err)
	}
	release()
	release()
	if available, waiters := testWorkerPoolState(testWorkerPoolFromContext(t, ctx)); available != 2 || waiters != 0 {
		t.Fatalf("idempotent release changed capacity: available=%d waiters=%d", available, waiters)
	}
	legacy := withTestWorkerPool(context.Background(), 0)
	if release, err := acquireTestWorkers(legacy, 1); err != nil {
		t.Fatalf("legacy zero did not resolve to one worker: %v", err)
	} else {
		release()
	}
}

func TestWorkerPoolGrantsAtomicallyAndCancelsQueuedAcquire(t *testing.T) {
	t.Parallel()

	ctx := withTestWorkerPool(context.Background(), 3)
	pool := testWorkerPoolFromContext(t, ctx)
	releaseTwo, err := acquireTestWorkers(ctx, 2)
	if err != nil {
		t.Fatal(err)
	}
	releaseOne, err := acquireTestWorkers(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	granted := make(chan func(), 1)
	go func() {
		release, err := acquireTestWorkers(ctx, 2)
		if err != nil {
			granted <- nil
			return
		}
		granted <- release
	}()
	waitForTestWorkerPoolWaiters(pool, 1)
	releaseOne()
	waitForTestWorkerPoolWaiters(pool, 1)
	select {
	case <-granted:
		t.Fatal("two-worker request received a partial grant")
	default:
	}
	releaseTwo()
	queuedRelease := <-granted
	if queuedRelease == nil {
		t.Fatal("queued atomic grant failed")
	}
	queuedRelease()

	cancelCtx, cancel := context.WithCancel(ctx)
	holdAll, err := acquireTestWorkers(cancelCtx, 3)
	if err != nil {
		t.Fatal(err)
	}
	canceled := make(chan error, 1)
	go func() {
		_, err := acquireTestWorkers(cancelCtx, 1)
		canceled <- err
	}()
	waitForTestWorkerPoolWaiters(pool, 1)
	cancel()
	if err := <-canceled; !errors.Is(err, context.Canceled) {
		t.Fatalf("queued acquire cancellation: %v", err)
	}
	holdAll()
	if available, waiters := testWorkerPoolState(pool); available != 3 || waiters != 0 {
		t.Fatalf("cancellation leaked capacity or waiter: available=%d waiters=%d", available, waiters)
	}
}

func TestStageWorkerPoolBoundsActiveGroups(t *testing.T) {
	t.Parallel()

	for _, workers := range []int{1, 2, 4} {
		t.Run(testWorkerCountName(workers), func(t *testing.T) {
			groupCount := workers + 2
			ids := make([]string, groupCount)
			groups := make(map[string]testpolicy.Group, groupCount)
			for index := range ids {
				ids[index] = "group-" + testWorkerCountName(index)
				groups[ids[index]] = testpolicy.Group{ID: ids[index], Kind: "unit", Adapter: "command", TargetMS: int64(groupCount - index)}
			}
			started := make(chan struct{}, groupCount)
			release := make(chan struct{})
			var mu sync.Mutex
			active, maximum := 0, 0
			runGroup := func(_ context.Context, _ TestRunRequest, group testpolicy.Group) GroupResult {
				mu.Lock()
				active++
				if active > maximum {
					maximum = active
				}
				mu.Unlock()
				started <- struct{}{}
				<-release
				mu.Lock()
				active--
				mu.Unlock()
				return GroupResult{ID: group.ID, Kind: group.Kind, Status: "passed"}
			}

			ctx := withTestWorkerPool(context.Background(), workers)
			ctx = withStageGroupDependencies(ctx, stageGroupDependencies{runGroup: runGroup})
			pool := testWorkerPoolFromContext(t, ctx)
			done := make(chan struct {
				results []GroupResult
				err     error
			}, 1)
			go func() {
				results, _, err := runStageGroups(ctx, TestRunRequest{Workers: workers, Concurrency: groupCount}, groups, ids, &progressWriter{}, false)
				done <- struct {
					results []GroupResult
					err     error
				}{results, err}
			}()
			for range workers {
				<-started
			}
			waitForTestWorkerPoolWaiters(pool, groupCount-workers)
			extraStarted := false
			select {
			case <-started:
				extraStarted = true
			default:
			}
			close(release)
			run := <-done
			if run.err != nil || len(run.results) != groupCount {
				t.Fatalf("bounded stage: results=%d err=%v", len(run.results), run.err)
			}
			mu.Lock()
			gotMaximum, gotActive := maximum, active
			mu.Unlock()
			if extraStarted || gotMaximum != workers || gotActive != 0 {
				t.Fatalf("worker bound: extra=%v maximum=%d active=%d want maximum=%d", extraStarted, gotMaximum, gotActive, workers)
			}
			if available, waiters := testWorkerPoolState(pool); available != workers || waiters != 0 {
				t.Fatalf("stage leaked worker capacity: available=%d waiters=%d", available, waiters)
			}
		})
	}
}

func testWorkerCountName(workers int) string {
	return "workers-" + strconv.Itoa(workers)
}

func TestStageReleasesRetainedPrerequisiteBeforeHeldGroupFinishes(t *testing.T) {
	t.Parallel()

	ids := []string{"build", "held", "consumer"}
	groups := map[string]testpolicy.Group{
		"build":    {ID: "build", Kind: "build", Adapter: "command", TargetMS: 200},
		"held":     {ID: "held", Kind: "unit", Adapter: "command", TargetMS: 100},
		"consumer": {ID: "consumer", Kind: "unit", Adapter: "command", TargetMS: 90, Requires: []string{"build"}},
	}
	heldStarted, consumerStarted := make(chan struct{}), make(chan struct{})
	releaseHeld := make(chan struct{})
	var launched []string
	var mu sync.Mutex
	runGroup := func(_ context.Context, _ TestRunRequest, group testpolicy.Group) GroupResult {
		mu.Lock()
		launched = append(launched, group.ID)
		mu.Unlock()
		switch group.ID {
		case "held":
			close(heldStarted)
			<-releaseHeld
		case "consumer":
			close(consumerStarted)
		}
		return GroupResult{ID: group.ID, Kind: group.Kind, Status: "passed"}
	}
	resolve := func(_ context.Context, id string) (GroupResult, bool, error) {
		if id == "build" {
			return GroupResult{ID: id, Kind: "build", Status: "reused"}, true, nil
		}
		return GroupResult{}, false, nil
	}
	done := make(chan struct {
		results []GroupResult
		err     error
	}, 1)
	go func() {
		ctx := withStageGroupDependencies(context.Background(), stageGroupDependencies{runGroup: runGroup})
		results, _, err := runStageGroupsWithPrerequisites(ctx, TestRunRequest{Workers: 2, Concurrency: 2}, groups, ids, &progressWriter{}, nil, resolve)
		done <- struct {
			results []GroupResult
			err     error
		}{results, err}
	}()
	<-heldStarted
	<-consumerStarted
	close(releaseHeld)
	run := <-done
	if run.err != nil {
		t.Fatal(run.err)
	}
	if len(run.results) != 3 || run.results[0].ID != "build" || run.results[0].Status != "reused" ||
		run.results[1].ID != "held" || run.results[2].ID != "consumer" {
		t.Fatalf("completion order changed plan-order results: %+v", run.results)
	}
	mu.Lock()
	defer mu.Unlock()
	seen := map[string]bool{}
	for _, id := range launched {
		seen[id] = true
	}
	if len(launched) != 2 || !seen["held"] || !seen["consumer"] || seen["build"] {
		t.Fatalf("retained prerequisite did not release its consumer: %v", launched)
	}
}

func TestStageCancellationDrainsQueuedWorkerGroups(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	ctx = withTestWorkerPool(ctx, 1)
	pool := testWorkerPoolFromContext(t, ctx)
	ids := []string{"one", "two", "three"}
	groups := map[string]testpolicy.Group{}
	for _, id := range ids {
		groups[id] = testpolicy.Group{ID: id, Kind: "unit", Adapter: "command"}
	}
	started := make(chan struct{}, 1)
	runGroup := func(ctx context.Context, _ TestRunRequest, group testpolicy.Group) GroupResult {
		started <- struct{}{}
		<-ctx.Done()
		return GroupResult{ID: group.ID, Kind: group.Kind, Status: "failed"}
	}
	ctx = withStageGroupDependencies(ctx, stageGroupDependencies{runGroup: runGroup})
	done := make(chan error, 1)
	go func() {
		_, _, err := runStageGroups(ctx, TestRunRequest{Workers: 1, Concurrency: 3}, groups, ids, &progressWriter{}, false)
		done <- err
	}()
	<-started
	waitForTestWorkerPoolWaiters(pool, 2)
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled stage: %v", err)
	}
	if available, waiters := testWorkerPoolState(pool); available != 1 || waiters != 0 {
		t.Fatalf("canceled stage leaked capacity or waiters: available=%d waiters=%d", available, waiters)
	}
}

func TestStageFailureDoesNotLaunchQueuedWorkerGroups(t *testing.T) {
	t.Parallel()

	ctx := withTestWorkerPool(context.Background(), 1)
	pool := testWorkerPoolFromContext(t, ctx)
	ids := []string{"one", "two", "three"}
	groups := map[string]testpolicy.Group{}
	for index, id := range ids {
		groups[id] = testpolicy.Group{ID: id, Kind: "unit", Adapter: "command", TargetMS: int64(len(ids) - index)}
	}
	started := make(chan string, len(ids))
	releaseFailure := make(chan struct{})
	runGroup := func(_ context.Context, _ TestRunRequest, group testpolicy.Group) GroupResult {
		started <- group.ID
		<-releaseFailure
		return GroupResult{ID: group.ID, Kind: group.Kind, Status: "failed"}
	}
	ctx = withStageGroupDependencies(ctx, stageGroupDependencies{runGroup: runGroup})
	type stageRun struct {
		results  []GroupResult
		haltedBy string
		err      error
	}
	done := make(chan stageRun, 1)
	go func() {
		results, haltedBy, err := runStageGroups(ctx, TestRunRequest{Workers: 1, Concurrency: 3}, groups, ids, &progressWriter{}, true)
		done <- stageRun{results: results, haltedBy: haltedBy, err: err}
	}()
	failedID := <-started
	waitForTestWorkerPoolWaiters(pool, 2)
	close(releaseFailure)
	run := <-done
	if run.err != nil || run.haltedBy != failedID {
		t.Fatalf("failed stage: haltedBy=%q want=%q err=%v", run.haltedBy, failedID, run.err)
	}
	select {
	case id := <-started:
		t.Fatalf("queued group %s launched after %s failed", id, failedID)
	default:
	}
	for _, result := range run.results {
		if result.ID == failedID {
			if result.Status != "failed" {
				t.Fatalf("failing group result: %+v", result)
			}
			continue
		}
		if result.Status != "not-run" || result.NotRunReason != haltReason(failedID) {
			t.Fatalf("queued result was not halted by %s: %+v", failedID, result)
		}
	}
	if available, waiters := testWorkerPoolState(pool); available != 1 || waiters != 0 {
		t.Fatalf("failed stage leaked capacity or waiters: available=%d waiters=%d", available, waiters)
	}
}

func TestDeliveryStopCauseSurvivesEarlierCompletionDelivery(t *testing.T) {
	t.Parallel()

	const firstStop = "failure-a"
	ids := []string{firstStop, "failure-b", "queued", "later-stage"}
	groups := make([]testpolicy.Group, 0, len(ids))
	for index, id := range ids {
		groups = append(groups, testpolicy.Group{ID: id, Kind: "unit", Adapter: "command", TargetMS: int64(len(ids) - index)})
	}
	contract := testpolicy.Contract{SchemaVersion: 1, Groups: groups}
	plan := testpolicy.Plan{Purpose: testpolicy.PurposeDelivery, SelectedGroups: ids,
		Stages: []testpolicy.Stage{{ID: "first", Groups: ids[:3]}, {ID: "later", Groups: ids[3:]}}}

	started := make(chan string, 2)
	releaseA, releaseB := make(chan struct{}), make(chan struct{})
	runGroup := func(_ context.Context, _ TestRunRequest, group testpolicy.Group) GroupResult {
		started <- group.ID
		switch group.ID {
		case firstStop:
			<-releaseA
		case "failure-b":
			<-releaseB
		}
		return GroupResult{ID: group.ID, Kind: group.Kind, Status: "failed"}
	}

	aRecordedStop := make(chan struct{})
	releaseACompletion := make(chan struct{})
	bDelivered := make(chan struct{})
	deliverCompletion := func(completion chan<- stageGroupCompletion, outcome stageGroupCompletion) {
		switch ids[outcome.index] {
		case firstStop:
			close(aRecordedStop)
			<-releaseACompletion
			completion <- outcome
		case "failure-b":
			completion <- outcome
			close(bDelivered)
		default:
			completion <- outcome
		}
	}
	ctx := withStageGroupDependencies(context.Background(), stageGroupDependencies{
		runGroup: runGroup, deliverCompletion: deliverCompletion,
	})

	done := make(chan struct {
		result TestResult
		status int
		err    error
	}, 1)
	go func() {
		result, status, err := RunTestPlan(ctx, TestRunRequest{Contract: contract, Plan: plan, Workers: 2, Concurrency: 2})
		done <- struct {
			result TestResult
			status int
			err    error
		}{result: result, status: status, err: err}
	}()

	seenStarted := map[string]bool{<-started: true, <-started: true}
	if !seenStarted[firstStop] || !seenStarted["failure-b"] {
		t.Fatalf("unexpected initial launches: %v", seenStarted)
	}
	close(releaseA)
	<-aRecordedStop
	close(releaseB)
	<-bDelivered
	close(releaseACompletion)
	run := <-done
	if run.err != nil || run.status != 1 {
		t.Fatalf("delivery run: status=%d err=%v", run.status, run.err)
	}
	if len(run.result.Groups) != len(ids) {
		t.Fatalf("delivery result groups = %+v", run.result.Groups)
	}
	for index, id := range ids {
		result := run.result.Groups[index]
		if result.ID != id {
			t.Fatalf("result order at %d: got %q want %q", index, result.ID, id)
		}
		if id == firstStop || id == "failure-b" {
			if result.Status != "failed" {
				t.Fatalf("completed failure %s = %+v", id, result)
			}
			continue
		}
		if result.Status != "not-run" || result.NotRunReason != haltReason(firstStop) {
			t.Fatalf("%s did not retain the gate-closing cause %s: %+v", id, firstStop, result)
		}
	}
	select {
	case id := <-started:
		t.Fatalf("group %s launched after the delivery stop", id)
	default:
	}
}

func TestStageRetainsProgressErrorAfterDeliveryHalt(t *testing.T) {
	t.Parallel()

	const firstStop = "failure-a"
	ids := []string{firstStop, "failure-b"}
	groups := map[string]testpolicy.Group{
		firstStop:   {ID: firstStop, Kind: "unit", Adapter: "command", TargetMS: 2},
		"failure-b": {ID: "failure-b", Kind: "unit", Adapter: "command", TargetMS: 1},
	}
	progressDirectory := filepath.Join(t.TempDir(), "progress")
	if err := os.Mkdir(progressDirectory, 0o755); err != nil {
		t.Fatal(err)
	}
	progressPath := filepath.Join(progressDirectory, "events.jsonl")

	started := make(chan string, len(ids))
	releaseA, releaseB := make(chan struct{}), make(chan struct{})
	runGroup := func(_ context.Context, _ TestRunRequest, group testpolicy.Group) GroupResult {
		started <- group.ID
		if group.ID == firstStop {
			<-releaseA
		} else {
			<-releaseB
		}
		return GroupResult{ID: group.ID, Kind: group.Kind, Status: "failed"}
	}

	aHaltAcknowledged := make(chan struct{})
	releaseACompletion := make(chan struct{})
	bEndWriteAttempted := make(chan struct{})
	deliverCompletion := func(completion chan<- stageGroupCompletion, outcome stageGroupCompletion) {
		switch ids[outcome.index] {
		case firstStop:
			close(aHaltAcknowledged)
			<-releaseACompletion
		case "failure-b":
			close(bEndWriteAttempted)
		}
		completion <- outcome
	}
	ctx := withStageGroupDependencies(context.Background(), stageGroupDependencies{
		runGroup: runGroup, deliverCompletion: deliverCompletion,
	})

	done := make(chan struct {
		results  []GroupResult
		haltedBy string
		err      error
	}, 1)
	go func() {
		results, haltedBy, err := runStageGroups(ctx, TestRunRequest{Workers: 2, Concurrency: 2}, groups, ids,
			&progressWriter{path: progressPath}, true)
		done <- struct {
			results  []GroupResult
			haltedBy string
			err      error
		}{results: results, haltedBy: haltedBy, err: err}
	}()

	seenStarted := map[string]bool{<-started: true, <-started: true}
	if !seenStarted[firstStop] || !seenStarted["failure-b"] {
		t.Fatalf("unexpected initial launches: %v", seenStarted)
	}
	close(releaseA)
	<-aHaltAcknowledged
	if err := os.Rename(progressDirectory, progressDirectory+"-closed"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(progressDirectory, []byte("closed"), 0o600); err != nil {
		t.Fatal(err)
	}
	close(releaseB)
	<-bEndWriteAttempted
	close(releaseACompletion)
	run := <-done
	if run.haltedBy != firstStop {
		t.Fatalf("haltedBy=%q want=%q", run.haltedBy, firstStop)
	}
	if run.err == nil {
		t.Fatal("delivery halt concealed the later progress end-write failure")
	}
	if len(run.results) != len(ids) || run.results[0].Status != "failed" || run.results[1].Status != "failed" {
		t.Fatalf("draining results = %+v", run.results)
	}
}

func TestStageStopPreventsQueuedMixedAdapterDispatch(t *testing.T) {
	t.Parallel()

	ctx := withTestWorkerPool(context.Background(), 1)
	pool := testWorkerPoolFromContext(t, ctx)
	ids := []string{"go-failure", "command-a", "command-b", "go-queued"}
	groups := map[string]testpolicy.Group{
		"go-failure": {ID: "go-failure", Kind: "unit", Adapter: "go", TargetMS: 4},
		"command-a":  {ID: "command-a", Kind: "unit", Adapter: "command", TargetMS: 3},
		"command-b":  {ID: "command-b", Kind: "unit", Adapter: "command", TargetMS: 2},
		"go-queued":  {ID: "go-queued", Kind: "unit", Adapter: "go", TargetMS: 1},
	}
	goAtDispatch := make(chan struct{})
	stopEstablished := make(chan struct{})
	beforeDispatch := func(id string) {
		if id != "go-queued" {
			return
		}
		close(goAtDispatch)
		<-stopEstablished
	}

	launched := make(chan string, len(ids))
	fail := make(chan struct{})
	commandStarted := make(chan struct{})
	releaseCommand := make(chan struct{})
	var commandOnce sync.Once
	runGroup := func(_ context.Context, _ TestRunRequest, group testpolicy.Group) GroupResult {
		launched <- group.ID
		if group.ID == "go-failure" {
			<-fail
			return GroupResult{ID: group.ID, Kind: group.Kind, Status: "failed"}
		}
		commandOnce.Do(func() { close(commandStarted) })
		<-releaseCommand
		return GroupResult{ID: group.ID, Kind: group.Kind, Status: "passed"}
	}
	ctx = withStageGroupDependencies(ctx, stageGroupDependencies{runGroup: runGroup, beforeDispatch: beforeDispatch})

	done := make(chan struct {
		results  []GroupResult
		haltedBy string
		err      error
	}, 1)
	go func() {
		results, haltedBy, err := runStageGroups(ctx, TestRunRequest{Workers: 1, Concurrency: 4}, groups, ids, &progressWriter{}, true)
		done <- struct {
			results  []GroupResult
			haltedBy string
			err      error
		}{results: results, haltedBy: haltedBy, err: err}
	}()

	<-commandStarted
	<-goAtDispatch
	waitForTestWorkerPoolWaiters(pool, 1)
	close(fail)
	waitForTestWorkerPoolWaiters(pool, 0)
	close(stopEstablished)
	close(releaseCommand)
	run := <-done
	if run.err != nil || run.haltedBy != "go-failure" {
		t.Fatalf("mixed stage stop: haltedBy=%q err=%v", run.haltedBy, run.err)
	}
	close(launched)
	launchedSet := map[string]bool{}
	for id := range launched {
		launchedSet[id] = true
	}
	if launchedSet["go-queued"] || !launchedSet["go-failure"] || launchedSet["command-a"] == launchedSet["command-b"] {
		t.Fatalf("mixed native launches = %v; want failing Go, one draining command, and no queued Go", launchedSet)
	}
	for _, result := range run.results {
		if result.ID == "go-failure" {
			if result.Status != "failed" {
				t.Fatalf("failing Go result: %+v", result)
			}
			continue
		}
		if launchedSet[result.ID] {
			if result.Status != "passed" {
				t.Fatalf("already launched command did not drain: %+v", result)
			}
			continue
		}
		if result.Status != "not-run" || result.NotRunReason != haltReason("go-failure") {
			t.Fatalf("queued mixed-adapter result was not halted: %+v", result)
		}
	}
	if available, waiters := testWorkerPoolState(pool); available != 1 || waiters != 0 {
		t.Fatalf("mixed stage stop leaked capacity: available=%d waiters=%d", available, waiters)
	}
}

func TestPrerequisiteStageKeepsIndependentFailureIsolation(t *testing.T) {
	t.Parallel()

	ids := []string{"failed-prerequisite", "blocked-consumer", "independent-failure"}
	groups := map[string]testpolicy.Group{
		"failed-prerequisite": {ID: "failed-prerequisite", Kind: "unit", Adapter: "go", TargetMS: 3},
		"blocked-consumer":    {ID: "blocked-consumer", Kind: "unit", Adapter: "command", Requires: []string{"failed-prerequisite"}, TargetMS: 2},
		"independent-failure": {ID: "independent-failure", Kind: "unit", Adapter: "command", TargetMS: 1},
	}
	launched := map[string]bool{}
	var launchedMu sync.Mutex
	runGroup := func(_ context.Context, _ TestRunRequest, group testpolicy.Group) GroupResult {
		launchedMu.Lock()
		launched[group.ID] = true
		launchedMu.Unlock()
		return GroupResult{ID: group.ID, Kind: group.Kind, Status: "failed"}
	}
	resolve := func(_ context.Context, _ string) (GroupResult, bool, error) {
		return GroupResult{}, false, nil
	}

	ctx := withStageGroupDependencies(context.Background(), stageGroupDependencies{runGroup: runGroup})
	results, haltedBy, err := runStageGroupsWithPrerequisites(ctx, TestRunRequest{Workers: 2, Concurrency: 2},
		groups, ids, &progressWriter{}, nil, resolve)
	if err != nil || haltedBy != "" {
		t.Fatalf("independent schema-two stage: haltedBy=%q err=%v", haltedBy, err)
	}
	launchedMu.Lock()
	defer launchedMu.Unlock()
	if !launched["failed-prerequisite"] || !launched["independent-failure"] || launched["blocked-consumer"] {
		t.Fatalf("independent launches = %v", launched)
	}
	if results[0].Status != "failed" || results[1].Status != "blocked" ||
		len(results[1].BlockingGroups) != 1 || results[1].BlockingGroups[0] != "failed-prerequisite" || results[1].NativeLaunched ||
		results[2].Status != "failed" {
		t.Fatalf("independent schema-two results = %+v", results)
	}
}

func TestPerformanceGroupAcquiresWholePoolAfterOtherGroupsDrain(t *testing.T) {
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
	performanceStarted := make(chan struct{})
	performanceAvailable := -1
	runGroup := func(_ context.Context, _ TestRunRequest, group testpolicy.Group) GroupResult {
		if group.Kind == "performance" {
			performanceAvailable, _ = testWorkerPoolState(pool)
			close(performanceStarted)
		} else {
			unitsStarted <- struct{}{}
			<-releaseUnits
		}
		return GroupResult{ID: group.ID, Kind: group.Kind, Status: "passed"}
	}
	ctx = withStageGroupDependencies(ctx, stageGroupDependencies{runGroup: runGroup})
	done := make(chan error, 1)
	go func() {
		_, _, err := runStageGroups(ctx, TestRunRequest{Workers: 3, Concurrency: 3}, groups, ids, &progressWriter{}, false)
		done <- err
	}()
	<-unitsStarted
	<-unitsStarted
	select {
	case <-performanceStarted:
		t.Fatal("performance group started before ordinary groups drained")
	default:
	}
	close(releaseUnits)
	<-performanceStarted
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if performanceAvailable != 0 {
		t.Fatalf("performance group did not own the whole worker pool: available=%d", performanceAvailable)
	}
	if available, waiters := testWorkerPoolState(pool); available != 3 || waiters != 0 {
		t.Fatalf("performance group leaked capacity: available=%d waiters=%d", available, waiters)
	}
}

func TestRunTestPlanStartsDependentBeforeHeldPeerFinishes(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	runTestResultGit(t, root, "init", "-q", "-b", "main")
	runTestResultGit(t, root, "config", "user.name", "fixture")
	runTestResultGit(t, root, "config", "user.email", "fixture@example.invalid")
	eventsPath, releasePath := filepath.Join(root, "events.fifo"), filepath.Join(root, "release.fifo")
	ids := []string{"prerequisite", "held", "consumer"}
	groups := make([]testpolicy.Group, 0, len(ids))
	for index, id := range ids {
		before := ""
		if id == "held" {
			before = "printf 'held\\n' > \"$EVENT_FIFO\"\nread -r release < \"$RELEASE_FIFO\"\n"
		} else if id == "consumer" {
			before = "printf 'consumer\\n' > \"$EVENT_FIFO\"\n"
		}
		body := "<testsuite><testcase classname=\"fixture\" name=\"" + id + "\"></testcase></testsuite>"
		script := "#!/bin/sh\nset -eu\n" + before + "mkdir -p reports-" + id + "\nprintf '%s\\n' " + strconv.Quote(body) + " > reports-" + id + "/tests.xml\n"
		path := filepath.Join(root, "scripts", id+".sh")
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := testexec.WriteFile(path, []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
		group := testpolicy.Group{ID: id, Kind: "unit", Adapter: "command", Phase: "acceptance", EnvironmentMode: "inherit", CWD: ".",
			Inputs: []string{"scripts/" + id + ".sh"}, Outputs: []string{"reports-" + id}, Tools: []testpolicy.Tool{}, Obligations: []string{id},
			Platforms: []string{"any"}, TargetMS: int64(100 - index*10), Argv: []string{"sh", "scripts/" + id + ".sh"},
			Env: map[string]string{"EVENT_FIFO": eventsPath, "RELEASE_FIFO": releasePath}, Reports: []string{"reports-" + id}, Format: "junit-xml",
			ExpectedTests: []testpolicy.ExpectedTest{{Report: "reports-" + id + "/tests.xml", Classname: "fixture", Name: id}}}
		if id == "consumer" {
			group.Requires = []string{"prerequisite"}
		}
		groups = append(groups, group)
	}
	runTestResultGit(t, root, "add", ".")
	runTestResultGit(t, root, "commit", "-qm", "fixture")
	tree := runTestResultGit(t, root, "rev-parse", "HEAD^{tree}")
	if err := syscall.Mkfifo(eventsPath, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(releasePath, 0o600); err != nil {
		t.Fatal(err)
	}
	events, err := os.OpenFile(eventsPath, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer events.Close()
	release, err := os.OpenFile(releasePath, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer release.Close()

	contract := testpolicy.Contract{SchemaVersion: testpolicy.ExecutionContractSchemaVersion,
		ProjectRisk: testpolicy.ProjectRisk{Severity: 1, Exposure: 1, Reversibility: "revert", Detection: "immediate", Recovery: "bounded"},
		Surfaces:    []testpolicy.Surface{{ID: "app", Paths: []string{"scripts/**"}, Standard: ids, Critical: []string{"prerequisite"}}}, Groups: groups,
		Always: testpolicy.Always{Canary: []string{"prerequisite"}}, Unknown: []string{"prerequisite"}, Cadence: ids}
	if err := contract.Validate(); err != nil {
		t.Fatal(err)
	}
	plan := testpolicy.Plan{Purpose: testpolicy.PurposeDiagnostic, RequestedMode: testpolicy.ModeStandard, RequiredMode: testpolicy.ModeStandard,
		ExecutedMode: testpolicy.ModeStandard, RequiredGroups: ids, SelectedGroups: ids, Stages: []testpolicy.Stage{{ID: "standard", Groups: ids}}}
	done := make(chan struct {
		result TestResult
		status int
		err    error
	}, 1)
	go func() {
		result, status, err := RunTestPlan(context.Background(), TestRunRequest{ProjectRoot: root, CandidateTree: tree, BaseCommit: "HEAD", PolicyBaseCommit: "HEAD",
			Contract: contract, Plan: plan, AttemptID: "attempt", LogRoot: filepath.Join(root, "logs"), Workers: 2, Concurrency: 2})
		done <- struct {
			result TestResult
			status int
			err    error
		}{result, status, err}
	}()
	scanner := bufio.NewScanner(events)
	seen := map[string]bool{}
	for range 2 {
		if !scanner.Scan() {
			t.Fatalf("read group event: %v", scanner.Err())
		}
		seen[scanner.Text()] = true
	}
	if !seen["held"] || !seen["consumer"] {
		t.Fatalf("dependent did not start while peer was held: %v", seen)
	}
	if _, err := release.WriteString("release\n"); err != nil {
		t.Fatal(err)
	}
	run := <-done
	if run.err != nil || run.status != 0 || len(run.result.Groups) != 3 {
		t.Fatalf("public scheduled run: status=%d groups=%+v err=%v", run.status, run.result.Groups, run.err)
	}
	for index, id := range ids {
		if run.result.Groups[index].ID != id || run.result.Groups[index].Status != "passed" {
			t.Fatalf("public result order or verdict changed: %+v", run.result.Groups)
		}
	}
}

func TestStageProgressStartFailureReleasesWorkers(t *testing.T) {
	t.Parallel()

	ctx := withTestWorkerPool(context.Background(), 1)
	pool := testWorkerPoolFromContext(t, ctx)
	called := false
	runGroup := func(_ context.Context, _ TestRunRequest, group testpolicy.Group) GroupResult {
		called = true
		return GroupResult{ID: group.ID, Status: "passed"}
	}
	ctx = withStageGroupDependencies(ctx, stageGroupDependencies{runGroup: runGroup})
	group := testpolicy.Group{ID: "group", Kind: "unit", Adapter: "command"}
	results, _, err := runStageGroups(ctx, TestRunRequest{Workers: 1, Concurrency: 1}, map[string]testpolicy.Group{"group": group}, []string{"group"},
		&progressWriter{path: t.TempDir()}, false)
	if err == nil || called || len(results) != 1 || results[0].Status != "not-run" {
		t.Fatalf("progress start failure: called=%v results=%+v err=%v", called, results, err)
	}
	if available, waiters := testWorkerPoolState(pool); available != 1 || waiters != 0 {
		t.Fatalf("progress failure leaked capacity: available=%d waiters=%d", available, waiters)
	}
}
