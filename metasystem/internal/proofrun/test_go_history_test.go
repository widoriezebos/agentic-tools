package proofrun

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
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
func historyLaunches(t *testing.T, root, tree, mode string, partitions []goTestPartition, expected []NativeTestIdentity) []int {
	t.Helper()
	bin := t.TempDir()
	script := `#!/bin/sh
for arg do
 case "$arg" in
 *TestSmall*) name=TestSmall; elapsed=1;;
 *TestLarge*) name=TestLarge; elapsed=9;;
 *TestOther*) name=TestOther; elapsed=4;;
 esac
 pkg=$arg
done
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
	history := loadGoLaunchHistory(roots.Installation(root), tree, group, partitions)
	var order []int
	ctx := context.WithValue(t.Context(), goShardLifecycleHooksKey{}, goShardLifecycleHooks{wrapOutput: func(index int, output io.Writer) io.Writer { order = append(order, index); return output }})
	var output synchronizedBuffer
	outcome, closeErr, _, executions, launchErr := runGoPartitions(ctx, TestRunRequest{Workers: 1, nativeHistory: history}, group, root,
		append(gittree.ScrubbedEnviron(), "PATH="+bin, "HISTORY_MODE="+mode), expected, partitions, supervisorLimits{}, 20*time.Millisecond,
		filepath.Join(t.TempDir(), "history.log"), &output, goCacheFacts{})
	if closeErr != nil || launchErr != nil {
		t.Fatalf("launch: close=%v launch=%v", closeErr, launchErr)
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
	if got := historyLaunches(t, root, "source", "", partitions, expected); !reflect.DeepEqual(got, []int{0, 1, 2}) {
		t.Fatalf("cold order %v", got)
	}
	for _, mode := range []string{"", "cached", "missing", "red"} {
		if got := historyLaunches(t, root, "source", mode, partitions, expected); !reflect.DeepEqual(got, []int{1, 2, 0}) {
			t.Fatalf("%s order %v", mode, got)
		}
		loaded := loadGoLaunchHistory(roots.Installation(root), "source", testpolicy.Group{}, partitions)
		if got := loaded.weights([]int64{1, 1, 1}); !reflect.DeepEqual(got, []int64{1000, 9000, 4000}) {
			t.Fatalf("%s durations %v", mode, got)
		}
	}
}

func TestGoLaunchHistoryFallbackAndSelectionIdentity(t *testing.T) {
	t.Parallel()
	for _, fault := range []string{"absent", "malformed", "foreign", "stale", "unreadable", "changed selection", "variant"} {
		t.Run(fault, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			partitions, expected := historyPartitions()
			historyLaunches(t, root, "source", "", partitions, expected)
			history := loadGoLaunchHistory(roots.Installation(root), "source", testpolicy.Group{}, partitions)
			switch fault {
			case "absent":
				if err := os.Remove(history.path); err != nil {
					t.Fatal(err)
				}
			case "malformed":
				if err := os.WriteFile(history.path, []byte("broken"), 0600); err != nil {
					t.Fatal(err)
				}
			case "foreign":
				history.tree = "other"
				history.save(partitions, nil)
			case "stale":
				data, err := json.Marshal(goLaunchRecord{Schema: 1, Tree: "source", Written: time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC), Durations: history.durations})
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
			case "changed selection":
				partitions[1].Names = []string{"TestLargeChanged"}
				expected[1].Name = "TestLargeChanged"
				h := loadGoLaunchHistory(roots.Installation(root), "source", testpolicy.Group{}, partitions)
				if got := h.weights([]int64{1, 2, 3}); !reflect.DeepEqual(got, []int64{1000, 2, 4000}) {
					t.Fatalf("selection weights %v", got)
				}
				return
			case "variant":
				h := loadGoLaunchHistory(roots.Installation(root), "source", testpolicy.Group{BuildTags: []string{"batchtest"}}, partitions)
				if got := h.weights([]int64{1, 2, 3}); !reflect.DeepEqual(got, []int64{1, 2, 3}) {
					t.Fatalf("variant weights %v", got)
				}
				return
			}
			if got := historyLaunches(t, root, "source", "", partitions, expected); !reflect.DeepEqual(got, []int{0, 1, 2}) {
				t.Fatalf("fallback order %v", got)
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
			h := loadGoLaunchHistory(roots.Installation(root), "source", testpolicy.Group{}, partitions)
			var executions []PackageExecution
			for i, p := range partitions {
				elapsed := int64(writer + 1)
				executions = append(executions, PackageExecution{Shard: i + 1, Package: p.Packages[0], Mode: PackageExecuted, Status: "ok", ElapsedMS: &elapsed})
			}
			h.save(partitions, executions)
		}()
	}
	wg.Wait()
	h := loadGoLaunchHistory(roots.Installation(root), "source", testpolicy.Group{}, partitions)
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

func TestNativeHistorySurvivesScratchAndForeignSourceFallsBack(t *testing.T) {
	t.Parallel()
	root, historyRoot := t.TempDir(), t.TempDir()
	files := map[string]string{"go.mod": "module example.invalid/nativehistory\n\ngo 1.27\n", "tiny_test.go": "package tiny\nimport \"testing\"\nfunc TestTiny(t *testing.T) { t.Parallel() }\n"}
	for name, contents := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(contents), 0600); err != nil {
			t.Fatal(err)
		}
	}
	for range 2 {
		result, err := RunNativeInventory(t.Context(), NativeInventoryRequest{Root: root, HistoryRoot: roots.Installation(historyRoot), LogRoot: t.TempDir(), Packages: []string{"./..."}, Workers: 1, Environment: gittree.ScrubbedEnviron()})
		if err != nil || result.Failed || len(result.Execution) != 1 {
			t.Fatalf("native result failed=%t err=%v", result.Failed, err)
		}
	}
	paths, err := filepath.Glob(filepath.Join(historyRoot, "artifacts/agents/proof-runs/advisory/*.json"))
	if err != nil || len(paths) != 1 {
		t.Fatalf("history did not survive native scratch: %d %v", len(paths), err)
	}
	var record goLaunchRecord
	data, err := os.ReadFile(paths[0])
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &record); err != nil || len(record.Durations) != 1 {
		t.Fatalf("record: %v", err)
	}
	discovery := goDiscovery{Inputs: []string{"go.mod", "tiny_test.go"}}
	before := goHistoryTree(root, discovery)
	if err := os.WriteFile(filepath.Join(root, "tiny_test.go"), []byte(files["tiny_test.go"]+"// changed\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if after := goHistoryTree(root, discovery); before == after || strings.TrimSpace(after) == "" {
		t.Fatal("source change did not change history scope")
	}
	fmt.Println("native history survived two independent log roots")
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
