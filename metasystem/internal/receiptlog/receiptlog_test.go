package receiptlog

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/registry"
)

// An engine append waits while the bound holds its lock exclusively
// (3.12 clause 2, DL4D-03): the line is either seen by the judgement in
// flight or lands after it. The test's registry home is testenv's own.
func TestAnAppendWaitsForTheBoundsJudgement(t *testing.T) {
	t.Parallel()
	registryPath, err := registry.DefaultPath()
	if err != nil {
		t.Fatal(err)
	}
	lock, err := diskstore.BoundExclusive(diskstore.BoundLockPath(filepath.Dir(registryPath)))
	if err != nil {
		t.Fatal(err)
	}
	installation := t.TempDir()
	if err := os.WriteFile(filepath.Join(installation, "metasystem.conf"), []byte("evidence.root="+filepath.Join(t.TempDir(), "evidence")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ledger := filepath.Join(installation, "memory", "receipts.log")
	if err := os.MkdirAll(filepath.Dir(ledger), 0o755); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- AppendLine(ledger, "a line\n", Options{Create: true}) }()
	select {
	case err := <-done:
		lock.Release()
		t.Fatalf("the append did not wait for the exclusive holder: %v", err)
	case <-time.After(200 * time.Millisecond):
	}
	if data, _ := os.ReadFile(ledger); len(data) != 0 {
		t.Fatalf("nothing lands while the judgement holds the lock: %q", data)
	}
	lock.Release()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(ledger); string(data) != "a line\n" {
		t.Fatalf("the line lands once the judgement released: %q", data)
	}
}
