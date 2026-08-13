package dispatch

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// codex-5: dispatch's record lock is bounded like the lease lock — a
// wedged holder produces a refusal naming the job, not a hang under the
// global lease lock.
func TestWithRecordLockIsBounded(t *testing.T) {
	t.Setenv("METASYSTEM_LEASE_LOCK_WAIT_SEC", "0.2")
	root := t.TempDir()
	locks := filepath.Join(root, "artifacts", "agents", "record-locks")
	if err := os.MkdirAll(locks, 0o755); err != nil {
		t.Fatal(err)
	}
	holder, err := os.OpenFile(filepath.Join(locks, "wedged.lock"), os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	defer holder.Close()
	if err := unix.Flock(int(holder.Fd()), unix.LOCK_EX); err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	err = withRecordLock(root, "wedged", func(string) error { return nil })
	if err == nil || !strings.Contains(err.Error(), "wedged") || !strings.Contains(err.Error(), "busy") {
		t.Fatalf("a held record lock must refuse by name: %v", err)
	}
	if elapsed := time.Since(started); elapsed > 5*time.Second {
		t.Fatalf("the bound did not release the caller: %v", elapsed)
	}
}
