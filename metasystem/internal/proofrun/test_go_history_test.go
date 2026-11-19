package proofrun

import (
	"context"
	"encoding/json"
	"fmt"
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
func historyLaunches(t *testing.T, root, mode string, partitions []goTestPartition, expected []NativeTestIdentity, completeSelection ...bool) []int {
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
if [ "$HISTORY_MODE" = zero-other ] && [ "$name" = TestOther ]; then elapsed=0; fi
if [ "$HISTORY_MODE" = red ]; then status=fail; elapsed=0.001; fi
printf '{"Action":"%s","Package":"%s","Elapsed":%s}\n' "$status" "$pkg" "$elapsed"
if [ "$HISTORY_MODE" = red ]; then exit 1; fi
`
	if err := testexec.WriteFile(filepath.Join(bin, "go"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	group := testpolicy.Group{ID: "history", Adapter: "go"}
	history := loadGoLaunchHistory(roots.Installation(root), group, partitions)
	history.completeSelection = mode != "narrow"
	if len(completeSelection) > 0 {
		history.completeSelection = completeSelection[0]
	}
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
				data, err := json.Marshal(goLaunchRecord{Schema: 99, Scope: history.scope, Entries: history.entries, Means: history.means})
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(history.path, data, 0600); err != nil {
					t.Fatal(err)
				}
			case "stale":
				for key, entry := range history.entries {
					entry.Observed = time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
					history.entries[key] = entry
				}
				for key, entry := range history.means {
					entry.Observed = time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
					history.means[key] = entry
				}
				data, err := json.Marshal(goLaunchRecord{Schema: 3, Scope: history.scope, Entries: history.entries, Means: history.means})
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
	if len(h.entries) != len(partitions) {
		t.Fatal("unreadable concurrent publication")
	}
	first := h.entries[h.keys[0]].Duration
	for _, entry := range h.entries {
		if entry.Duration != first {
			t.Fatal("torn publication")
		}
	}
}

func TestNativeHistorySurvivesScratchAndSourceChanges(t *testing.T) {
	t.Parallel()
	root, historyRoot := t.TempDir(), t.TempDir()
	files := map[string]string{
		"go.mod":         "module example.invalid/nativehistory\n\ngo 1.27\n",
		"a/tiny_test.go": "package a\nimport \"testing\"\nfunc TestTiny(t *testing.T) { t.Parallel() }\nfunc TestOne(t *testing.T) { t.Parallel() }\nfunc TestTwo(t *testing.T) { t.Parallel() }\n",
		"b/tiny_test.go": "package b\nimport \"testing\"\nfunc TestTiny(t *testing.T) { t.Parallel() }\nfunc TestOne(t *testing.T) { t.Parallel() }\nfunc TestTwo(t *testing.T) { t.Parallel() }\n",
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
		if err := json.Unmarshal(data, &record); err != nil || len(record.Entries) != 2 {
			t.Fatalf("record: %v", err)
		}
		// Fix durations to make the next hand-out independent of host timing.
		for key, entry := range record.Entries {
			entry.Duration = 1000
			if entry.Package == goHistoryDigest([]string{"example.invalid/nativehistory/b"}) {
				entry.Duration = 9000
			}
			record.Entries[key] = entry
		}
		for pkg, mean := range record.Means {
			mean.Duration = 1000
			if pkg == goHistoryDigest([]string{"example.invalid/nativehistory/b"}) {
				mean.Duration = 9000
			}
			record.Means[pkg] = mean
		}
		data, err = json.Marshal(record)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(paths[0], data, 0600); err != nil {
			t.Fatal(err)
		}
		replay, err := RunNativeInventory(t.Context(), NativeInventoryRequest{Root: root, HistoryRoot: roots.Installation(historyRoot), LogRoot: t.TempDir(), Packages: []string{"./b"}, Tests: []string{"TestOne", "TestTwo"}, Unsharded: true, Workers: 1, Environment: gittree.ScrubbedEnviron()})
		if err != nil || replay.Failed || len(replay.Execution) != 1 {
			t.Fatalf("replay failed=%t err=%v", replay.Failed, err)
		}
		after := readHistoryRecord(t, paths[0])
		if len(after.Entries) != 3 || !reflect.DeepEqual(after.Means, record.Means) {
			t.Fatal("narrow inventory changed complete history")
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
		request.nativeHistory = &goLaunchHistory{entries: map[string]goLaunchObservation{"anything": {Duration: 999999}}}
		environment = append(environment, NativeHistoryRootEnvironment+"=/another-installation")
		after := groupExecutionIdentity(request, group, "", "inputs", digestGroupEnvironment(group, environment), nil, nil)
		countAfter, reasonAfter := reusePolicy(request, group, 1, goCacheFacts{})
		if before != after || countBefore != countAfter || reasonBefore != reasonAfter {
			t.Fatal("advisory scheduling affected proof identity or reuse")
		}
	}
}

func TestGoLaunchHistoryNarrowRunPreservesCompleteOrder(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	partitions, expected := historyPartitions()
	partitions[1].Names = append(partitions[1].Names, "TestAdded")
	expected = append(expected, NativeTestIdentity{Classname: partitions[1].Packages[0], Name: "TestAdded"})
	historyLaunches(t, root, "", partitions, expected)
	first := loadGoLaunchHistory(roots.Installation(root), testpolicy.Group{}, partitions).weights([]int64{1, 1, 1})
	narrow := []goTestPartition{{Packages: partitions[0].Packages, Names: []string{"TestSmall", "TestAdded"}}}
	selected := []NativeTestIdentity{{Classname: narrow[0].Packages[0], Name: "TestSmall"}, {Classname: narrow[0].Packages[0], Name: "TestAdded"}}
	historyLaunches(t, root, "narrow", narrow, selected)
	h := loadGoLaunchHistory(roots.Installation(root), testpolicy.Group{}, partitions)
	if got := h.weights([]int64{1, 1, 1}); !reflect.DeepEqual(got, first) {
		t.Fatalf("narrow run changed complete weights: got %v want %v", got, first)
	}
	changed := []goTestPartition{{Packages: partitions[0].Packages, Names: []string{"TestChanged"}}}
	if got := loadGoLaunchHistory(roots.Installation(root), testpolicy.Group{}, changed).weights([]int64{1}); got[0] != 5000 {
		t.Fatalf("narrow run changed package mean: %v", got)
	}
	if got := historyLaunches(t, root, "", partitions, expected); !reflect.DeepEqual(got, []int{1, 2, 0}) {
		t.Fatalf("complete run after replay order %v", got)
	}
}

func readHistoryRecord(t *testing.T, path string) goLaunchRecord {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var record goLaunchRecord
	if err := json.Unmarshal(data, &record); err != nil {
		t.Fatal(err)
	}
	return record
}

func writeHistoryRecord(t *testing.T, path string, record goLaunchRecord) {
	t.Helper()
	data, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
}

func TestGoLaunchHistoryExpiresEntriesAndBoundsRecord(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"age", "bound"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			partitions, expected := historyPartitions()
			historyLaunches(t, root, "", partitions, expected)
			h := loadGoLaunchHistory(roots.Installation(root), testpolicy.Group{}, partitions)
			record := readHistoryRecord(t, h.path)
			fresh := record.Entries[h.keys[1]]
			if mode == "age" {
				old := record.Entries[h.keys[0]]
				old.Observed = time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
				record.Entries[h.keys[0]] = old
				record.Means[h.packageKeys[0]] = old
			} else {
				older := fresh
				older.Observed = older.Observed.Add(-time.Hour)
				for i := range goHistoryLimit + 1 {
					key := goHistoryDigest(fmt.Sprintf("older-%d", i))
					record.Entries[key] = older
					record.Means[key] = older
				}
			}
			writeHistoryRecord(t, h.path, record)
			// Cached terminals must neither refresh ages nor hide save-time pruning.
			historyLaunches(t, root, "cached", partitions, expected, false)
			saved := readHistoryRecord(t, h.path)
			if saved.Entries[h.keys[1]] != fresh {
				t.Fatal("fresh observation changed without execution")
			}
			if mode == "age" {
				if _, ok := saved.Entries[h.keys[0]]; ok {
					t.Fatal("old entry survived save")
				}
				if _, ok := saved.Means[h.packageKeys[0]]; ok {
					t.Fatal("old package mean survived save")
				}
				if len(saved.Entries) != 2 || len(saved.Means) != 1 {
					t.Fatal("fresh history was discarded")
				}
			} else if len(saved.Entries) != goHistoryLimit || len(saved.Means) != goHistoryLimit {
				t.Fatalf("record exceeded bounds: entries=%d means=%d", len(saved.Entries), len(saved.Means))
			}
		})
	}
}

func TestGoLaunchHistoryMergeReadsCurrentRecord(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	partitions, expected := historyPartitions()
	historyLaunches(t, root, "", partitions, expected)
	loaded := loadGoLaunchHistory(roots.Installation(root), testpolicy.Group{}, partitions)
	narrow := []goTestPartition{{Packages: partitions[0].Packages, Names: []string{"TestSmall", "TestAdded"}}}
	selected := []NativeTestIdentity{{Classname: narrow[0].Packages[0], Name: "TestSmall"}, {Classname: narrow[0].Packages[0], Name: "TestAdded"}}
	historyLaunches(t, root, "narrow", narrow, selected)
	before := readHistoryRecord(t, loaded.path)
	loaded.save(partitions, nil)
	after := readHistoryRecord(t, loaded.path)
	if len(after.Entries) != 4 || !reflect.DeepEqual(before, after) {
		t.Fatal("save lost observations published after load")
	}
}

func TestGoLaunchHistoryUnmeasuredPackageDoesNotBlockMeans(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	partitions, expected := historyPartitions()
	historyLaunches(t, root, "zero-other", partitions, expected)
	changed := []goTestPartition{{Packages: partitions[0].Packages, Names: []string{"TestChanged"}}}
	if got := loadGoLaunchHistory(roots.Installation(root), testpolicy.Group{}, changed).weights([]int64{1}); got[0] != 5000 {
		t.Fatalf("unmeasured package blocked measured package mean: %v", got)
	}
}
