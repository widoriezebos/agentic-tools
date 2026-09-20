package batch

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

func TestGLEProofLockAcquiresAfterDeadPredecessorInOnePoll(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	lockDir, queueDir := filepath.Join(root, "lock"), filepath.Join(root, "queue")
	now := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	prober := scriptedProber{7: identity.Alive, 9: identity.Dead}
	if err := os.Mkdir(queueDir, 0o755); err != nil {
		t.Fatal(err)
	}
	deadEntry := filepath.Join(queueDir, "1893455999-m1e-9")
	if err := os.WriteFile(deadEntry, []byte("m1e 9 2029-12-31T23:59:59Z hand\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	seedProofLock(t, lockDir, "9")
	lock := testProofLock(prober, lockDir, queueDir, 7, &now)
	polled, err := lock.poll()
	if err != nil || polled != lockAcquired {
		t.Fatalf("first poll after dead predecessor = %v, %v; want acquired", polled, err)
	}
	if err := lock.release(); err != nil {
		t.Fatal(err)
	}
	if entries, err := os.ReadDir(queueDir); err != nil || len(entries) != 0 {
		t.Fatalf("queue after release = %v, %v; want empty", entries, err)
	}
}

func TestGLEBatchTickOnceReleasesWaitingQueueEntry(t *testing.T) {
	t.Parallel()
	now := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	bed := newOwnerBed(t, ownerRecord(testBatchID, StateOpen, now.Add(-time.Minute)), now)
	seedProofLock(t, bed.lockDir, "7")
	if err := bed.owner.TickOnce(testBatchID); err != nil {
		t.Fatal(err)
	}
	if entries, err := os.ReadDir(bed.queueDir); err != nil || len(entries) != 0 {
		t.Fatalf("one-shot waiter left queue entries: %v, %v", entries, err)
	}
	if bed.launches != 0 {
		t.Fatalf("launched through live lock: %d", bed.launches)
	}
	if owner := string(contents(t, filepath.Join(bed.lockDir, "owner"))); owner == "" {
		t.Fatal("one-shot waiter removed the live lock owner")
	}
	if err := os.RemoveAll(bed.lockDir); err != nil {
		t.Fatal(err)
	}
	if err := bed.owner.TickOnce(testBatchID); err != nil {
		t.Fatal(err)
	}
	if bed.launches != 1 {
		t.Fatalf("next one-shot tick launched %d times, want once", bed.launches)
	}
}
