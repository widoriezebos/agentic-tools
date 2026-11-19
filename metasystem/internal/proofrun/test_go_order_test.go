package proofrun

import (
	"context"
	"fmt"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/roots"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
	"reflect"
	"sort"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testenv"
)

func TestGoWorkerOrderLongestFirstAndNoStarvation(t *testing.T) {
	t.Parallel()
	ctx := withTestWorkerPool(t.Context(), 1)
	hold, err := acquireTestWorkers(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	defer hold()
	weights := []int64{1, 9, 4, 9, 2}
	q := newGoWorkerOrder(ctx, weights, make([][]*goPackageReadiness, len(weights)))
	type grant struct {
		index   int
		release func()
	}
	grants := make(chan grant, len(weights))
	for i := len(weights) - 1; i >= 0; i-- {
		go func() {
			release, err := q.acquire(ctx, i)
			if err == nil {
				grants <- grant{i, release}
			}
		}()
	}
	testenv.Await(t, "all launches waiting", func() bool { _, waiting := testWorkerPoolState(q.pool); return waiting == len(weights) })
	hold()
	var order []int
	for range weights {
		select {
		case next := <-grants:
			order = append(order, next.index)
			next.release()
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
	}
	if !reflect.DeepEqual(order, []int{1, 3, 2, 4, 0}) {
		t.Fatalf("grant order %v", order)
	}
	assertWorkerPoolFullyAvailable(t, ctx, 1)
}

func TestGoWorkerOrderProfilePacking(t *testing.T) {
	t.Parallel()
	// A profile-shaped finite workload: 54 long shards and 187 packages,
	// mostly short with a handful of long unsharded packages. Virtual time
	// advances only when a fake launch finishes; grants use production code.
	durations := []int64{790, 1145, 1041, 1035, 1034, 738, 722, 576, 570, 477, 462, 356, 532}
	for i := 1; i < 54; i++ {
		durations = append(durations, 109+int64((i*97)%598))
	}
	for len(durations) < 241 {
		durations = append(durations, 1+int64(len(durations)%9))
	}
	ctx := withTestWorkerPool(t.Context(), 18)
	dependencies := make([][]*goPackageReadiness, len(durations))
	producer := &goPackageReadiness{done: make(chan struct{})}
	for i := 13; i < 66; i++ {
		dependencies[i] = []*goPackageReadiness{producer}
	}
	partitions := make([]goTestPartition, len(durations))
	var executions []PackageExecution
	for i := range durations {
		partitions[i] = goTestPartition{Packages: []string{fmt.Sprintf("profile/package%d", i)}, Names: []string{"TestProfile"}}
		executions = append(executions, PackageExecution{Package: partitions[i].Packages[0], Shard: i + 1, Mode: PackageExecuted, Status: "ok", ElapsedMS: &durations[i]})
	}
	root := t.TempDir()
	loadGoLaunchHistory(roots.Installation(root), "profile", testpolicy.Group{}, partitions).save(partitions, executions)
	history := loadGoLaunchHistory(roots.Installation(root), "profile", testpolicy.Group{}, partitions)
	q := newGoWorkerOrder(ctx, history.weights(make([]int64, len(durations))), dependencies)
	type job struct {
		index   int
		finish  int64
		release func()
	}
	started := make(chan job, len(durations))
	for i := range durations {
		go func() {
			if _, err := waitGoReadiness(ctx, i, dependencies[i]); err != nil {
				return
			}
			release, err := q.acquire(ctx, i)
			if err != nil {
				return
			}
			if i == 0 {
				producer.resolve("begin running")
			}
			started <- job{index: i, release: release}
		}()
	}
	receive := func(now int64) job {
		select {
		case j := <-started:
			j.finish = now + durations[j.index]
			return j
		case <-ctx.Done():
			t.Fatal(ctx.Err())
			return job{}
		}
	}
	var active []job
	for range 18 {
		active = append(active, receive(0))
	}
	launched, completed := 18, map[int]bool{}
	var clock int64
	for len(active) > 0 {
		sort.Slice(active, func(i, j int) bool { return active[i].finish < active[j].finish })
		done := active[0]
		active = active[1:]
		clock = done.finish
		if completed[done.index] {
			t.Fatal("duplicate launch")
		}
		completed[done.index] = true
		done.release()
		if launched < len(durations) {
			active = append(active, receive(clock))
			launched++
		}
	}
	var total, longest int64
	for _, d := range durations {
		total += d
		longest = max(longest, d)
	}
	bound := max(longest, (total+17)/18)
	if len(completed) != len(durations) || clock*100 > bound*103 {
		t.Fatalf("completed=%d makespan=%d bound=%d", len(completed), clock, bound)
	}
	assertWorkerPoolFullyAvailable(t, ctx, 18)
}

func TestGoPartitionFallbackWeights(t *testing.T) {
	t.Parallel()
	partitions := []goTestPartition{
		{Packages: []string{"small"}, Names: []string{"A"}},
		{Packages: []string{"large"}, Names: []string{"A", "B", "C"}},
		{Packages: []string{"sharded"}, Names: []string{"A"}},
		{Packages: []string{"sharded"}, Names: []string{"B"}},
	}
	var expected []NativeTestIdentity
	for _, p := range partitions {
		for _, name := range p.Names {
			expected = append(expected, NativeTestIdentity{Classname: p.Packages[0], Name: name})
		}
	}
	q := newGoWorkerOrder(withTestWorkerPool(t.Context(), 1), goPartitionWeights(partitions, expected), make([][]*goPackageReadiness, len(partitions)))
	if !reflect.DeepEqual(q.order, []int{2, 3, 1, 0}) {
		t.Fatalf("fallback order %v", q.order)
	}
}

func TestGoWorkerOrderCancellationReleasesQueue(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(withTestWorkerPool(t.Context(), 1))
	defer cancel()
	hold, err := acquireTestWorkers(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	defer hold()
	q := newGoWorkerOrder(ctx, []int64{1, 2}, make([][]*goPackageReadiness, 2))
	done := make(chan error, 2)
	for i := range 2 {
		go func() { _, err := q.acquire(ctx, i); done <- err }()
	}
	testenv.Await(t, "queued launches", func() bool { _, n := testWorkerPoolState(q.pool); return n == 2 })
	cancel()
	for range 2 {
		if err := <-done; err != context.Canceled {
			t.Fatalf("cancel: %v", err)
		}
	}
	hold()
	assertWorkerPoolFullyAvailable(t, ctx, 1)
}
