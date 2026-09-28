package batch

import (
	"os"
	"path/filepath"
	"strings"
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
	must(t, os.MkdirAll(bed.lockDir, 0o755))
	seedProofLock(t, filepath.Join(bed.lockDir, "batch-"+testBatchID), "7")
	if err := bed.owner.TickOnce(testBatchID); err != nil {
		t.Fatal(err)
	}
	if entries, err := os.ReadDir(filepath.Join(bed.queueDir, "batch-"+testBatchID)); err != nil || len(entries) != 0 {
		t.Fatalf("one-shot waiter left queue entries: %v, %v", entries, err)
	}
	if bed.launches != 0 {
		t.Fatalf("launched through live lock: %d", bed.launches)
	}
	if owner := string(contents(t, filepath.Join(bed.lockDir, "batch-"+testBatchID, "owner"))); owner == "" {
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

// TestBatchLocksAreIndependent: each batch's mutex is its own directory under
// the lock root, so a batch whose proof holds its lock does not queue another
// batch behind it, and no lock names the retired host-wide test-run lock.
func TestBatchLocksAreIndependent(t *testing.T) {
	t.Parallel()
	const other = "01j5x00000000000000000bb01"
	now := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	bed := newOwnerBed(t, ownerRecord(testBatchID, StateSealed, now.Add(-time.Minute)), now)
	second := ownerRecord(other, StateSealed, now.Add(-time.Minute))
	second.Units[0].GoalID, second.Units[0].Chain, second.History[0].Detail = "goal-b", "chain-b", "goal-b joined"
	must(t, bed.store.Create(second))
	held := bed.owner.lock(testBatchID)
	if polled, err := held.poll(); err != nil || polled != lockAcquired {
		t.Fatalf("batch A lock = %v, %v; want acquired", polled, err)
	}
	must(t, bed.owner.Tick(other))
	if bed.launches != 1 {
		t.Fatalf("batch B launched %d times while A held its own lock, want once", bed.launches)
	}
	if !strings.HasPrefix(ProofLockOwner(bed.lockDir), "landing-batch-owner 7 ") {
		t.Fatalf("lock root owners=%q, want A's lock reported", ProofLockOwner(bed.lockDir))
	}
	must(t, held.release())
	defaults := bed.owner.lock(other)
	for _, lock := range []*proofLock{held, defaults, newBatchProofLock(bed.store, "", "", other, 7, time.Now)} {
		for _, path := range []string{lock.lockDir, lock.queueDir, DefaultProofLockDir} {
			if strings.Contains(path, "metasystem-testrun-lock") || strings.Contains(path, "metasystem-testrun-queue") {
				t.Fatalf("lock path %s names the retired host-wide test-run lock", path)
			}
		}
	}
	if held.lockDir != filepath.Join(bed.lockDir, "batch-"+testBatchID) || defaults.lockDir != filepath.Join(bed.lockDir, "batch-"+other) {
		t.Fatalf("lock directories %s and %s, want <lockDir>/batch-<id>", held.lockDir, defaults.lockDir)
	}
}
