package batch

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/helm"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
)

// Batch ids chosen so A sorts before C in the queue glob.
const helmBatchA, helmBatchC = "01j5x00000000000000000aa01", "01j5x00000000000000000cc01"

// helmBed is a landing checkout's owner with batches A (seat A) and C (seat
// C), one owner pid (7), a fixed clock, and a prober the test drives.
type helmBed struct {
	*ownerBed
	prober          scriptedProber
	seatA, seatC    string
	launchedBatches []string
}

func helmRepository(t *testing.T) string {
	root := filepath.Join(t.TempDir(), "checkout")
	must(t, os.MkdirAll(filepath.Join(root, ".git"), 0o755))
	must(t, os.MkdirAll(filepath.Join(root, "metasystem"), 0o755))
	return root
}

func seatRecord(id, seat string, at time.Time) Record {
	record := ownerRecord(id, StateLanding, at.Add(-time.Minute))
	record.Units[0].SeatRoot = seat
	return record
}

func newHelmBed(t *testing.T, seatless ...bool) *helmBed {
	t.Helper()
	now := time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC) // every registration falls in this second
	bed := &helmBed{prober: scriptedProber{7: identity.Alive, 4242: identity.Alive}}
	bed.seatA, bed.seatC = filepath.Join(helmRepository(t), "metasystem"), filepath.Join(helmRepository(t), "metasystem")
	seatA := bed.seatA
	if len(seatless) != 0 {
		seatA = ""
	}
	bed.ownerBed = newOwnerBed(t, seatRecord(helmBatchA, seatA, now), now)
	must(t, bed.store.Create(seatRecord(helmBatchC, bed.seatC, now)))
	bed.owner.store.seams.prober = bed.prober
	bed.owner.lock = func(id string) *proofLock {
		return newProofLock(bed.owner.store, bed.lockDir, bed.queueDir, id, 7, func() time.Time { return now })
	}
	bed.owner.helmActive = func(root string) bool { return helm.Active(root).Active }
	bed.owner.launch = func(id string, _ proofrun.LoadSample, _ string) error {
		bed.launchedBatches = append(bed.launchedBatches, id)
		return nil
	}
	return bed
}

func (bed *helmBed) busyLock(t *testing.T) []byte {
	seedProofLock(t, bed.lockDir, "4242")
	return contents(t, filepath.Join(bed.lockDir, "owner"))
}

func (bed *helmBed) recordBytes(t *testing.T, id string) []byte {
	return contents(t, filepath.Join(bed.store.root, "artifacts", "agents", "landing-batches", id+".json"))
}

func queueEntries(t *testing.T, dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	var names []string
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names
}

func takeHelm(t *testing.T, seat string) {
	_, err := helm.Write(seat, helm.Record{By: "Wido", At: "2030-01-02T03:04:05Z", Reason: "by hand"})
	must(t, err)
}

func returnHelm(t *testing.T, seat string) {
	_, err := helm.Remove(seat)
	must(t, err)
}

func TestLandingOwnerHoldsHelmSeatBatchAcrossCheckouts(t *testing.T) {
	t.Parallel()
	t.Run("HM-4 HM-7", func(t *testing.T) {
		bed := newHelmBed(t)
		before := bed.recordBytes(t, helmBatchA)
		takeHelm(t, bed.seatA)
		bed.owner.Resume()
		if !slices.Equal(bed.launchedBatches, []string{helmBatchC}) {
			t.Fatalf("launched=%v under seat A's helm, want only C", bed.launchedBatches)
		}
		if !bytes.Equal(bed.recordBytes(t, helmBatchA), before) {
			t.Fatalf("held batch A's record changed")
		}
		if held := bed.owner.Held(); len(held) != 1 || held[0].ID != helmBatchA || held[0].Seat != bed.seatA || !held[0].New {
			t.Fatalf("Held()=%+v, want A newly held for seat A", held)
		}
		bed.owner.Resume()
		if held := bed.owner.Held(); len(held) != 1 || held[0].New {
			t.Fatalf("second held pass Held()=%+v, want A held and not new", held)
		}
		returnHelm(t, bed.seatA)
		bed.launchedBatches = nil
		bed.owner.Resume()
		if !slices.Equal(bed.launchedBatches, []string{helmBatchA, helmBatchC}) || len(bed.owner.Held()) != 0 {
			t.Fatalf("after return launched=%v held=%v, want both and none", bed.launchedBatches, bed.owner.Held())
		}
	})
}

func TestLandingOwnerNeverHoldsUnitWithoutSeat(t *testing.T) {
	t.Parallel()
	t.Run("HM-7", func(t *testing.T) {
		bed := newHelmBed(t, true)
		bed.owner.helmActive = func(string) bool { return true }
		bed.owner.Resume()
		if !slices.Contains(bed.launchedBatches, helmBatchA) || slices.Contains(bed.launchedBatches, helmBatchC) {
			t.Fatalf("launched=%v, want A (no seat, never held) and not C", bed.launchedBatches)
		}
	})
}

// queueTwo queues A and C behind the busy foreign lock in one second.
func (bed *helmBed) queueTwo(t *testing.T) {
	bed.owner.Resume()
	names := queueEntries(t, bed.queueDir)
	if len(names) != 2 || !strings.Contains(names[0], helmBatchA) || !strings.Contains(names[1], helmBatchC) ||
		!strings.HasSuffix(names[0], "-7") || !strings.HasSuffix(names[1], "-7") || len(bed.launchedBatches) != 0 {
		t.Fatalf("queue=%v launched=%v, want A then C, each ending in the owner pid", names, bed.launchedBatches)
	}
}

func TestHeldBatchWithdrawsQueueRegistration(t *testing.T) {
	t.Parallel()
	t.Run("HM-7 HM-11", func(t *testing.T) {
		bed := newHelmBed(t)
		bed.busyLock(t)
		before := bed.recordBytes(t, helmBatchA)
		bed.queueTwo(t)
		takeHelm(t, bed.seatA)
		bed.prober[4242] = identity.Dead
		bed.owner.Resume()
		names := queueEntries(t, bed.queueDir)
		if slices.ContainsFunc(names, func(name string) bool { return strings.Contains(name, helmBatchA) }) {
			t.Fatalf("queue=%v, held A's registration was not withdrawn", names)
		}
		if !slices.Equal(bed.launchedBatches, []string{helmBatchC}) {
			t.Fatalf("launched=%v, want C (A held, C not stranded behind it)", bed.launchedBatches)
		}
		if !bytes.Equal(bed.recordBytes(t, helmBatchA), before) {
			t.Fatalf("held batch A's record changed")
		}
		if held := bed.owner.Held(); len(held) != 1 || !strings.Contains(held[0].Entry, helmBatchA) {
			t.Fatalf("Held()=%+v, want A naming its withdrawn entry", held)
		}
	})
}

func TestHoldLeavesRunningProofLockUntouched(t *testing.T) {
	t.Parallel()
	t.Run("HM-11", func(t *testing.T) {
		bed := newHelmBed(t)
		owner := bed.busyLock(t)
		bed.queueTwo(t)
		takeHelm(t, bed.seatA)
		bed.owner.Resume()
		if got := contents(t, filepath.Join(bed.lockDir, "owner")); !bytes.Equal(got, owner) {
			t.Fatalf("running job's lock owner changed: %q", got)
		}
		if names := queueEntries(t, bed.queueDir); len(names) != 1 || !strings.Contains(names[0], helmBatchC) {
			t.Fatalf("queue=%v, want only C's registration left", names)
		}
		if len(bed.launchedBatches) != 0 {
			t.Fatalf("launched=%v while a foreign job holds the lock", bed.launchedBatches)
		}
	})
}

func TestWithdrawThenReregister(t *testing.T) {
	t.Parallel()
	t.Run("HM-7", func(t *testing.T) {
		bed := newHelmBed(t)
		owner := bed.busyLock(t)
		bed.queueTwo(t)
		first, withdrawn, err := bed.owner.Withdraw()
		must(t, err)
		if names := queueEntries(t, bed.queueDir); len(names) != 0 || !first || len(withdrawn) != 2 {
			t.Fatalf("after Withdraw queue=%v first=%v withdrawn=%v", names, first, withdrawn)
		}
		if again, _, _ := bed.owner.Withdraw(); again {
			t.Fatalf("second Withdraw reported a transition")
		}
		if got := contents(t, filepath.Join(bed.lockDir, "owner")); !bytes.Equal(got, owner) {
			t.Fatalf("foreign lock changed: %q", got)
		}
		bed.queueTwo(t)
		bed.prober[4242] = identity.Dead
		bed.owner.Resume()
		if !slices.Equal(bed.launchedBatches, []string{helmBatchA, helmBatchC}) {
			t.Fatalf("launched=%v, want A then C", bed.launchedBatches)
		}
	})
}
