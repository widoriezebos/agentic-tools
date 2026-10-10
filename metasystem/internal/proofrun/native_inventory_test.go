package proofrun

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

func TestNativeCommandPartitionsUseThreeShardsPerWorker(t *testing.T) {
	t.Parallel()
	const prefix = "example.com/proof/"
	var expected []NativeTestIdentity
	for index := 0; index < 12; index++ {
		expected = append(expected, NativeTestIdentity{Classname: prefix + "cmd/metasystem", Name: fmt.Sprintf("Test%02d", index)})
	}
	expected = append(expected, NativeTestIdentity{Classname: prefix + "internal/small", Name: "TestSmall"})
	partitions := partitionNativePackages(expected, []string{"cmd/metasystem", "internal/small"}, prefix, 2)
	if len(partitions) != 7 {
		t.Fatalf("partitions=%+v; want six command shards and one small package", partitions)
	}
	seen := map[string]int{}
	for index, partition := range partitions {
		if index < 6 && (len(partition.Names) != 2 || partition.Names[0] != fmt.Sprintf("Test%02d", index) || partition.Names[1] != fmt.Sprintf("Test%02d", index+6)) {
			t.Fatalf("shard %d did not deal sorted names round robin: %+v", index, partition)
		}
		for _, name := range partition.Names {
			for _, pkg := range partition.Packages {
				seen[pkg+"/"+name]++
			}
		}
	}
	for _, identity := range expected {
		if seen[identity.Classname+"/"+identity.Name] != 1 {
			t.Fatalf("test did not appear exactly once: %+v runs=%v", identity, seen)
		}
	}
}

// Both native workers must write before either output pipe can finish.
type inventoryOverlapWriter struct {
	io.Writer
	once    sync.Once
	started chan struct{}
	other   chan struct{}
}

func (w *inventoryOverlapWriter) Write(data []byte) (int, error) {
	if strings.Contains(string(data), "native worker started") {
		w.once.Do(func() { close(w.started); <-w.other })
	}
	return w.Writer.Write(data)
}

func TestNativeInventoryReportsMissingTerminalsAsRed(t *testing.T) {
	t.Parallel()
	root, environment := namedGroupModule(t, `package named
import ("fmt"; "testing")
func TestA(t *testing.T) { t.Parallel(); fmt.Println("native worker started"); panic("deliberate panic") }
func TestB(t *testing.T) { t.Parallel(); fmt.Println("native worker started") }
func TestC(t *testing.T) { t.Parallel() }
func TestD(t *testing.T) { t.Parallel() }
func TestE(t *testing.T) { t.Parallel() }
func TestF(t *testing.T) { t.Parallel() }
func TestG(t *testing.T) { t.Parallel() }
`)
	packageRoot := filepath.Join(root, "cmd", "metasystem")
	if err := os.MkdirAll(packageRoot, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(root, "present_test.go"), filepath.Join(packageRoot, "present_test.go")); err != nil {
		t.Fatal(err)
	}
	started := []chan struct{}{make(chan struct{}), make(chan struct{})}
	ctx := context.WithValue(t.Context(), goShardLifecycleHooksKey{}, goShardLifecycleHooks{wrapOutput: func(index int, w io.Writer) io.Writer {
		if index > 1 {
			return w
		}
		return &inventoryOverlapWriter{Writer: w, started: started[index], other: started[1-index]}
	}})
	planned := 0
	completed := []PackageExecution{}
	result, err := RunNativeInventory(ctx, NativeInventoryRequest{Root: root, LogRoot: filepath.Join(t.TempDir(), "logs"), Environment: environment, Workers: 2, Packages: []string{"cmd/metasystem"}, Progress: func(total int, executions []PackageExecution) {
		if executions == nil {
			planned = total
		} else {
			if planned != 6 {
				t.Errorf("completion before its plan: %d", planned)
			}
			completed = append(completed, executions...)
		}
	}})
	if planned != 6 || len(completed) != 6 {
		t.Fatalf("live package progress planned=%d completed=%+v", planned, completed)
	}
	if err != nil || !result.Failed || len(result.Execution) != 6 {
		t.Fatalf("result=%+v error=%v", result, err)
	}
	for _, ch := range started {
		select {
		case <-ch:
		default:
			t.Fatal("native workers did not overlap")
		}
	}
	if !nativeIdentityStatus(result.Missing, "TestG", "missing-terminal") || !nativeIdentityStatus(result.Observed, "TestB", "passed") {
		t.Fatalf("panic swallowed another shard or missing test: %+v", result)
	}
	statuses := map[string]bool{}
	for _, run := range result.Execution {
		statuses[run.Status] = true
	}
	if !statuses["missing"] || !statuses["ok"] {
		t.Fatalf("shard verdicts %v", result.Execution)
	}
	if !strings.Contains(string(result.Output), `"Action":"fail","Package":"example.com/named/cmd/metasystem"`) {
		t.Fatalf("native package terminal lost: %s", result.Output)
	}
}

func TestNativeInventoryTestMainFailureNamesItsPackage(t *testing.T) {
	t.Parallel()
	root, environment := namedGroupModule(t, `package named
import ("os"; "testing")
func TestA(t *testing.T) { t.Parallel() }
func TestB(t *testing.T) { t.Parallel() }
func TestMain(m *testing.M) { m.Run(); os.Exit(3) }
`)
	for name, source := range map[string]string{"internal/good/good.go": "package good\n", "internal/good/good_test.go": "package good\nimport \"testing\"\nfunc TestGood(t *testing.T) {t.Parallel()}\n"} {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(source), 0600); err != nil {
			t.Fatal(err)
		}
	}
	result, err := RunNativeInventory(t.Context(), NativeInventoryRequest{Root: root, LogRoot: t.TempDir(), Environment: environment, Workers: 2, Packages: []string{"./..."}})
	if err != nil || !result.Failed || len(result.Missing) != 0 {
		t.Fatalf("TestMain result=%+v err=%v", result, err)
	}
	good, bad := false, false
	for _, run := range result.Execution {
		if run.Package == "example.com/named" {
			bad = true
			if run.Status != "fail" {
				t.Fatalf("TestMain package green: %+v", run)
			}
		}
		if run.Package == "example.com/named/internal/good" && run.Status == "ok" {
			good = true
		}
	}
	if !good || !bad || !nativeIdentityStatus(result.Observed, "TestA", "passed") || !nativeIdentityStatus(result.Observed, "TestB", "passed") {
		t.Fatalf("TestMain hid package failure: %+v", result)
	}
}

func TestNativeInventoryIgnoresNonTestSelections(t *testing.T) {
	t.Parallel()
	for _, names := range [][]string{
		{"TestMain"},
		{"Test"},
		{"TestMain", "Test", "Testhelper", "TestéHelper", "TestPresent"},
		{"TestMain", "TestPresent", "TestAbsent"},
	} {
		t.Run(strings.Join(names, ","), func(t *testing.T) {
			t.Parallel()
			root, environment := namedGroupModule(t, `package named
import ("os"; "testing")
func TestMain(m *testing.M) { os.WriteFile("main-ran", []byte("ran"), 0600); os.Exit(m.Run()) }
func Test(t *testing.T) { t.Parallel(); t.Log("bare Test ran") }
func TestPresent(t *testing.T) { t.Parallel(); t.Log("selected test ran") }
func TestNoise(t *testing.T) { t.Parallel(); t.Fatal("unselected test ran") }
`)
			result, err := RunNativeInventory(t.Context(), NativeInventoryRequest{Root: root, LogRoot: t.TempDir(), Environment: environment, Workers: 2, Packages: []string{"."}, Tests: names})
			wantMissing := slices.Contains(names, "TestAbsent")
			missingCount := 0
			if wantMissing {
				missingCount = 1
			}
			if err != nil || result.Failed != wantMissing || len(result.Missing) != missingCount {
				t.Fatalf("selection=%v failed=%t missing=%+v observed=%+v err=%v", names, result.Failed, result.Missing, result.Observed, err)
			}
			if wantMissing && result.Missing[0].Name != "TestAbsent" {
				t.Fatalf("only a real missing test may be red: %+v", result.Missing)
			}
			if slices.Contains(names, "TestPresent") && !nativeIdentityStatus(result.Observed, "TestPresent", "passed") {
				t.Fatalf("selected test did not run: observed=%+v", result.Observed)
			}
			if slices.Contains(names, "Test") && !nativeIdentityStatus(result.Observed, "Test", "passed") {
				t.Fatalf("bare Test did not run: observed=%+v", result.Observed)
			}
			if _, err := os.Stat(filepath.Join(root, "main-ran")); err != nil {
				t.Fatalf("TestMain must still execute: %v", err)
			}
			// Replay groups use the same discovery boundary as native inventory.
			encoded, err := json.Marshal(names)
			if err != nil {
				t.Fatal(err)
			}
			group := testpolicy.Group{ID: "selected", Adapter: "go", CWD: ".", Packages: []string{"."}, Tests: encoded}
			results, err := RunNamedGroups(t.Context(), root, testpolicy.Contract{SchemaVersion: 2, Groups: []testpolicy.Group{group}}, []string{group.ID}, environment)
			wantStatus := "green"
			if wantMissing {
				wantStatus = "red"
			}
			if err != nil || len(results) != 1 || results[0].Status != wantStatus || slices.Contains(results[0].Reasons, "missing test TestMain") {
				t.Fatalf("named selection=%v results=%+v err=%v", names, results, err)
			}
		})
	}
}
