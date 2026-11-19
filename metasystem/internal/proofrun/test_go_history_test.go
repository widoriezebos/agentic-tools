package proofrun

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/roots"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

func historyPartitions() ([]goTestPartition, []NativeTestIdentity) {
	partitions := []goTestPartition{
		{Packages: []string{"example.invalid/history/sharded"}, Names: []string{"TestSmall"}},
		{Packages: []string{"example.invalid/history/sharded"}, Names: []string{"TestLarge"}},
		{Packages: []string{"example.invalid/history/other"}, Names: []string{"TestOther"}},
	}
	var expected []NativeTestIdentity
	for _, p := range partitions {
		expected = append(expected, NativeTestIdentity{Classname: p.Packages[0], Name: p.Names[0]})
	}
	return partitions, expected
}

// Fake Go terminals drive the real process launcher, readiness barrier,
// scheduler, result collector and advisory publication, with no timing waits.
func historyLaunches(t *testing.T, root, mode string, partitions []goTestPartition, expected []NativeTestIdentity) []int {
	t.Helper()
	bin := t.TempDir()
	script := `#!/bin/sh
added=no
for arg do
 case "$arg" in *TestAdded*) added=yes;; esac
 case "$arg" in
 *TestSmall*) name=TestSmall; elapsed=1;;
 *TestLarge*) name=TestLarge; elapsed=9;;
 *TestOther*) name=TestOther; elapsed=4;;
 esac
 pkg=$arg
done
if [ "$added" = yes ]; then
 printf '{"Action":"run","Package":"%s","Test":"TestAdded"}\n' "$pkg"
 printf '{"Action":"pass","Package":"%s","Test":"TestAdded"}\n' "$pkg"
fi
printf '{"Action":"run","Package":"%s","Test":"%s"}\n' "$pkg" "$name"
printf '{"Action":"pass","Package":"%s","Test":"%s"}\n' "$pkg" "$name"
if [ "$HISTORY_MODE" = cached ]; then
 printf '{"Action":"output","Package":"%s","Output":"ok  \\t%s\\t(cached)\\n"}\n' "$pkg" "$pkg"
fi
if [ "$HISTORY_MODE" = missing ]; then exit 0; fi
status=pass
if [ "$HISTORY_MODE" = red ]; then status=fail; fi
printf '{"Action":"%s","Package":"%s","Elapsed":%s}\n' "$status" "$pkg" "$elapsed"
if [ "$HISTORY_MODE" = red ]; then exit 1; fi
`
	if err := testexec.WriteFile(filepath.Join(bin, "go"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	group := testpolicy.Group{ID: "history", Adapter: "go"}
	history := loadGoLaunchHistory(roots.Installation(root), group, partitions)
	var order []int
	ctx := context.WithValue(t.Context(), goShardLifecycleHooksKey{}, goShardLifecycleHooks{wrapOutput: func(index int, output io.Writer) io.Writer { order = append(order, index); return output }})
	var output synchronizedBuffer
	outcome, closeErr, _, executions, launchErr := runGoPartitions(ctx, TestRunRequest{Workers: 1, nativeHistory: history}, group, root,
		append(gittree.ScrubbedEnviron(), "PATH="+bin, "HISTORY_MODE="+mode), expected, partitions, supervisorLimits{}, 20*time.Millisecond,
		filepath.Join(t.TempDir(), "history.log"), &output, goCacheFacts{})
	if closeErr != nil || launchErr != nil {
		t.Fatalf("launch: close=%v launch=%v", closeErr, launchErr)
	}
	if mode == "" && outcome.WaitErr != nil {
		t.Fatalf("unexpected launch failure: %v", outcome.WaitErr)
	}
	if mode == "red" && outcome.WaitErr == nil {
		t.Fatal("advisory history hid a red")
	}
	if mode == "missing" {
		for _, e := range executions {
			if e.Status != "missing" {
				t.Fatal("missing terminal certified")
			}
		}
	}
	return order
}

func TestGoLaunchHistoryOrdersProductionLaunchesAndKeepsUnknowns(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	partitions, expected := historyPartitions()
	if got := historyLaunches(t, root, "", partitions, expected); !reflect.DeepEqual(got, []int{0, 1, 2}) {
		t.Fatalf("cold order %v", got)
	}
	for _, mode := range []string{"", "cached", "missing", "red"} {
		if got := historyLaunches(t, root, mode, partitions, expected); !reflect.DeepEqual(got, []int{1, 2, 0}) {
			t.Fatalf("%s order %v", mode, got)
		}
		loaded := loadGoLaunchHistory(roots.Installation(root), testpolicy.Group{}, partitions)
		if got := loaded.weights([]int64{1, 1, 1}); !reflect.DeepEqual(got, []int64{1000, 9000, 4000}) {
			t.Fatalf("%s durations %v", mode, got)
		}
	}
}

func TestGoLaunchHistoryFallbackAndSelectionIdentity(t *testing.T) {
	t.Parallel()
	for _, fault := range []string{"absent", "malformed", "foreign installation", "schema", "stale", "unreadable", "variant", "platform"} {
		t.Run(fault, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			partitions, expected := historyPartitions()
			historyLaunches(t, root, "", partitions, expected)
			history := loadGoLaunchHistory(roots.Installation(root), testpolicy.Group{}, partitions)
			switch fault {
			case "absent":
				if err := os.Remove(history.path); err != nil {
					t.Fatal(err)
				}
			case "malformed":
				if err := os.WriteFile(history.path, []byte("broken"), 0600); err != nil {
					t.Fatal(err)
				}
			case "foreign installation":
				other := loadGoLaunchHistory(roots.Installation(t.TempDir()), testpolicy.Group{}, partitions)
				history.scope = other.scope
				history.save(partitions, nil)
			case "schema":
				data, err := json.Marshal(goLaunchRecord{Schema: 99, Scope: history.scope, Packages: history.packages, Written: time.Now().UTC(), Durations: history.durations})
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(history.path, data, 0600); err != nil {
					t.Fatal(err)
				}
			case "stale":
				data, err := json.Marshal(goLaunchRecord{Schema: 2, Scope: history.scope, Packages: history.packages, Written: time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC), Durations: history.durations})
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(history.path, data, 0600); err != nil {
					t.Fatal(err)
				}
			case "unreadable":
				if err := os.Remove(history.path); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(history.path, 0700); err != nil {
					t.Fatal(err)
				}
			case "variant", "platform":
				group := testpolicy.Group{}
				var environment []string
				if fault == "variant" {
					group.BuildTags = []string{"batchtest"}
				} else {
					environment = []string{"GOOS=other-platform"}
				}
				h := loadGoLaunchHistory(roots.Installation(root), group, partitions, environment)
				if got := h.weights([]int64{1, 2, 3}); !reflect.DeepEqual(got, []int64{1, 2, 3}) {
					t.Fatalf("variant weights %v", got)
				}
				return
			}
			if got := historyLaunches(t, root, "", partitions, expected); !reflect.DeepEqual(got, []int{0, 1, 2}) {
				t.Fatalf("fallback order %v", got)
			}
		})
	}
}

func TestGoLaunchHistoryChangedSelectionsAndUnseenPackages(t *testing.T) {
	t.Parallel()
	for _, change := range []string{"added shard test", "unsharded selection", "unseen package"} {
		t.Run(change, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			partitions, expected := historyPartitions()
			historyLaunches(t, root, "", partitions, expected)
			wantWeights := []int64{5000, 9000, 4000}
			wantOrder := []int{1, 0, 2}
			switch change {
			case "added shard test":
				partitions[0].Names = append(partitions[0].Names, "TestAdded")
				expected = append(expected, NativeTestIdentity{Classname: partitions[0].Packages[0], Name: "TestAdded"})
			case "unsharded selection":
				partitions[2].Names = append(partitions[2].Names, "TestAdded")
				expected = append(expected, NativeTestIdentity{Classname: partitions[2].Packages[0], Name: "TestAdded"})
				wantWeights = []int64{1000, 9000, 4000}
				wantOrder = []int{1, 2, 0}
			case "unseen package":
				partitions[2].Packages = []string{"example.invalid/history/unseen"}
				expected[2].Classname = partitions[2].Packages[0]
				wantWeights = []int64{1000, 9000, 9001}
				wantOrder = []int{2, 1, 0}
			}
			h := loadGoLaunchHistory(roots.Installation(root), testpolicy.Group{}, partitions)
			if got := h.weights([]int64{1, 1, 1}); !reflect.DeepEqual(got, wantWeights) {
				t.Fatalf("weights %v, want %v", got, wantWeights)
			}
			if got := historyLaunches(t, root, "", partitions, expected); !reflect.DeepEqual(got, wantOrder) {
				t.Fatalf("order %v, want %v", got, wantOrder)
			}
		})
	}
}

func TestGoLaunchHistoryConcurrentWritersPublishWholeRecords(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	partitions, _ := historyPartitions()
	var wg sync.WaitGroup
	for writer := range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			h := loadGoLaunchHistory(roots.Installation(root), testpolicy.Group{}, partitions)
			var executions []PackageExecution
			for i, p := range partitions {
				elapsed := int64(writer + 1)
				executions = append(executions, PackageExecution{Shard: i + 1, Package: p.Packages[0], Mode: PackageExecuted, Status: "ok", ElapsedMS: &elapsed})
			}
			h.save(partitions, executions)
		}()
	}
	wg.Wait()
	h := loadGoLaunchHistory(roots.Installation(root), testpolicy.Group{}, partitions)
	if len(h.durations) != len(partitions) {
		t.Fatal("unreadable concurrent publication")
	}
	first := h.durations[h.keys[0]]
	for _, duration := range h.durations {
		if duration != first {
			t.Fatal("torn publication")
		}
	}
}

func TestNativeHistorySurvivesScratchAndSourceChanges(t *testing.T) {
	t.Parallel()
	root, historyRoot := t.TempDir(), t.TempDir()
	files := map[string]string{
		"go.mod":         "module example.invalid/nativehistory\n\ngo 1.27\n",
		"a/tiny_test.go": "package a\nimport \"testing\"\nfunc TestTiny(t *testing.T) { t.Parallel() }\n",
		"b/tiny_test.go": "package b\nimport \"testing\"\nfunc TestTiny(t *testing.T) { t.Parallel() }\n",
	}
	for name, contents := range files {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
			t.Fatal(err)
		}
	}
	var order []int
	ctx := context.WithValue(t.Context(), goShardLifecycleHooksKey{}, goShardLifecycleHooks{wrapOutput: func(index int, output io.Writer) io.Writer { order = append(order, index); return output }})
	for run := range 2 {
		order = nil
		result, err := RunNativeInventory(ctx, NativeInventoryRequest{Root: root, HistoryRoot: roots.Installation(historyRoot), LogRoot: t.TempDir(), Packages: []string{"./..."}, Workers: 1, Environment: gittree.ScrubbedEnviron()})
		if err != nil || result.Failed || len(result.Execution) != 2 {
			t.Fatalf("native result failed=%t err=%v", result.Failed, err)
		}
		wantOrder := []int{0, 1}
		if run == 1 {
			wantOrder = []int{1, 0}
		}
		if !reflect.DeepEqual(order, wantOrder) {
			t.Fatalf("run %d order %v, want %v", run, order, wantOrder)
		}
		if run == 1 {
			break
		}
		paths, err := filepath.Glob(filepath.Join(historyRoot, "artifacts/agents/proof-runs/advisory/*.json"))
		if err != nil || len(paths) != 1 {
			t.Fatalf("history files: %d %v", len(paths), err)
		}
		var record goLaunchRecord
		data, err := os.ReadFile(paths[0])
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(data, &record); err != nil || len(record.Durations) != 2 {
			t.Fatalf("record: %v", err)
		}
		// Fix durations to make the next hand-out independent of host timing.
		for key := range record.Durations {
			record.Durations[key] = 1000
			if record.Packages[key] == goHistoryDigest([]string{"example.invalid/nativehistory/b"}) {
				record.Durations[key] = 9000
			}
		}
		data, err = json.Marshal(record)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(paths[0], data, 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "a/tiny_test.go"), []byte(files["a/tiny_test.go"]+"// changed source contents\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
}

func TestGoLaunchHistoryCannotChangeIdentityOrReuse(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"inherit", "explicit"} {
		group := testpolicy.Group{ID: "history", Adapter: "go", EnvironmentMode: mode}
		request := TestRunRequest{Workers: 1}
		environment := []string{"FIXTURE=one"}
		before := groupExecutionIdentity(request, group, "", "inputs", digestGroupEnvironment(group, environment), nil, nil)
		countBefore, reasonBefore := reusePolicy(request, group, 1, goCacheFacts{})
		request.nativeHistory = &goLaunchHistory{durations: map[string]int64{"anything": 999999}}
		environment = append(environment, NativeHistoryRootEnvironment+"=/another-installation")
		after := groupExecutionIdentity(request, group, "", "inputs", digestGroupEnvironment(group, environment), nil, nil)
		countAfter, reasonAfter := reusePolicy(request, group, 1, goCacheFacts{})
		if before != after || countBefore != countAfter || reasonBefore != reasonAfter {
			t.Fatal("advisory scheduling affected proof identity or reuse")
		}
	}
}
