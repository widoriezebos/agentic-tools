package batch

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// TestQueueRegistrationsAreDistinctPerBatch is the lock's own defect (HM8-01),
// helm-free: two batches of one owner polled in one second must not share a
// queue registration, or the first to acquire strands the second.
func TestQueueRegistrationsAreDistinctPerBatch(t *testing.T) {
	t.Parallel()
	t.Run("HM-11", func(t *testing.T) {
		root, now := t.TempDir(), time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
		lockDir, queueDir := filepath.Join(root, "lock"), filepath.Join(root, "queue")
		prober := scriptedProber{7: identity.Alive, 4242: identity.Alive}
		seedProofLock(t, lockDir, "4242")
		store := NewStore("", prober)
		clock := func() time.Time { return now }
		first := newProofLock(store, lockDir, queueDir, "01j5x00000000000000000aa01", 7, clock)
		second := newProofLock(store, lockDir, queueDir, "01j5x00000000000000000cc01", 7, clock)
		polls(t, first, lockQueued)
		polls(t, second, lockQueued)
		names := queueEntries(t, queueDir)
		if len(names) != 2 || !strings.HasSuffix(names[0], "-7") || !strings.HasSuffix(names[1], "-7") {
			t.Fatalf("queue=%v, want two registrations each ending in the owner pid", names)
		}
		prober[4242] = identity.Dead
		polls(t, first, lockAcquired)
		must(t, first.whileHeld(func() error { return nil }))
		polls(t, second, lockAcquired)
		must(t, second.release())
	})
}

func TestProofLockRefusesKeyWithDash(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	lock := newProofLock(NewStore("", scriptedProber{}), filepath.Join(root, "lock"), filepath.Join(root, "queue"), "batch-1", 7, time.Now)
	if _, err := lock.poll(); err == nil || !strings.Contains(err.Error(), "batch-1") {
		t.Fatalf("poll error=%v, want the key refused", err)
	}
	if _, err := os.Stat(filepath.Join(root, "queue")); !os.IsNotExist(err) {
		t.Fatalf("a refused key registered in the queue: %v", err)
	}
}
