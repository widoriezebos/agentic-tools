package receiptlog

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/registry"
)

// An engine append holds the bound lock shared around its write (3.12
// clause 2, DL4D-03): while the line is written the bound cannot take the
// lock exclusively, so a judgement either sees the line or runs before it.
// The test's registry home is testenv's own.
func TestAnAppendHoldsTheBoundLockSharedAroundItsWrite(t *testing.T) {
	t.Parallel()
	registryPath, err := registry.DefaultPath()
	if err != nil {
		t.Fatal(err)
	}
	boundLock := diskstore.BoundLockPath(filepath.Dir(registryPath))
	installation := t.TempDir()
	if err := os.WriteFile(filepath.Join(installation, "metasystem.conf"), []byte("evidence.root="+filepath.Join(t.TempDir(), "evidence")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ledger := filepath.Join(installation, "memory", "receipts.log")
	if err := os.MkdirAll(filepath.Dir(ledger), 0o755); err != nil {
		t.Fatal(err)
	}
	heldDuringWrite := false
	err = AppendLine(ledger, "a line\n", Options{Create: true, Sync: func(file *os.File) error {
		lock, err := diskstore.TryBoundExclusive(boundLock)
		if err == nil {
			lock.Release()
		}
		heldDuringWrite = errors.Is(err, diskstore.ErrBoundHeld)
		return file.Sync()
	}})
	if err != nil {
		t.Fatal(err)
	}
	if !heldDuringWrite {
		t.Fatal("the bound could take its lock exclusively while the line was being written")
	}
	lock, err := diskstore.TryBoundExclusive(boundLock)
	if err != nil {
		t.Fatalf("the shared hold is released after the append: %v", err)
	}
	lock.Release()
	if data, _ := os.ReadFile(ledger); string(data) != "a line\n" {
		t.Fatalf("%q", data)
	}
}
