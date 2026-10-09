package proofrun

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

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
`)
	started := []chan struct{}{make(chan struct{}), make(chan struct{})}
	ctx := context.WithValue(t.Context(), goShardLifecycleHooksKey{}, goShardLifecycleHooks{wrapOutput: func(index int, w io.Writer) io.Writer {
		return &inventoryOverlapWriter{Writer: w, started: started[index], other: started[1-index]}
	}})
	planned := 0
	completed := []PackageExecution{}
	result, err := RunNativeInventory(ctx, NativeInventoryRequest{Root: root, LogRoot: filepath.Join(t.TempDir(), "logs"), Environment: environment, Workers: 2, Packages: []string{"."}, Progress: func(total int, executions []PackageExecution) {
		if executions == nil {
			planned = total
		} else {
			if planned != 2 {
				t.Errorf("completion before its plan: %d", planned)
			}
			completed = append(completed, executions...)
		}
	}})
	if planned != 2 || len(completed) != 2 {
		t.Fatalf("live package progress planned=%d completed=%+v", planned, completed)
	}
	if err != nil || !result.Failed || len(result.Execution) != 2 {
		t.Fatalf("result=%+v error=%v", result, err)
	}
	for _, ch := range started {
		select {
		case <-ch:
		default:
			t.Fatal("native workers did not overlap")
		}
	}
	if !nativeIdentityStatus(result.Missing, "TestC", "missing-terminal") || !nativeIdentityStatus(result.Observed, "TestB", "passed") {
		t.Fatalf("panic swallowed another shard or missing test: %+v", result)
	}
	statuses := map[string]bool{}
	for _, run := range result.Execution {
		statuses[run.Status] = true
	}
	if !statuses["missing"] || !statuses["ok"] {
		t.Fatalf("shard verdicts %v", result.Execution)
	}
	if !strings.Contains(string(result.Output), `"Action":"fail","Package":"example.com/named"`) {
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
