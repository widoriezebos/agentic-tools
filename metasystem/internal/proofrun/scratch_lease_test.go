package proofrun

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
)

// The lease tests hold several live runs at once; each body runs with forks
// excluded (lockedScratch) so no child of a parallel test ever holds a copy
// of one of their writer locks when a cleanup takes it.
// testLeasePolicy is a lease namespace; the leases themselves know no policy.
const testLeasePolicy = "scratch-environment/v2"

func newLeaseRun(t *testing.T, control string) *ScratchRun {
	t.Helper()
	run, err := CreateScratchRun(control)
	if err != nil {
		t.Fatal(err)
	}
	return run
}

// Sequential runs of one key see the same slot path; a run overlapping a
// live owner takes the next slot; a run re-claiming its own key gets its own
// slot back; normal cleanup empties the slot and frees it, keeping the path.
func TestScratchLeaseSlotsAreRunInvariantAndExclusive(t *testing.T) {
	t.Parallel()
	lockedScratch(t, func() {
		control := t.TempDir()
		first := newLeaseRun(t, control)
		slot0, err := first.ClaimLease(testLeasePolicy, "key")
		if err != nil {
			t.Fatal(err)
		}
		if again, err := first.ClaimLease(testLeasePolicy, "key"); err != nil || again != slot0 {
			t.Fatalf("re-claim = %q %v, want %q", again, err, slot0)
		}
		if err := os.WriteFile(filepath.Join(slot0, "left-by-first"), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		overlapping := newLeaseRun(t, control)
		slot1, err := overlapping.ClaimLease(testLeasePolicy, "key")
		if err != nil || filepath.Base(slot1) != "1" || filepath.Dir(slot1) != filepath.Dir(slot0) {
			t.Fatalf("overlapping claim = %q %v, want slot 1 beside %q", slot1, err, slot0)
		}
		cleanupLeaseRuns(t, first)
		if entries, err := os.ReadDir(slot0); err != nil || len(entries) != 0 {
			t.Fatalf("released slot 0 = %v %v, want an empty kept directory", entries, err)
		}
		next := newLeaseRun(t, control)
		if slot, err := next.ClaimLease(testLeasePolicy, "key"); err != nil || slot != slot0 {
			t.Fatalf("sequential claim = %q %v, want %q", slot, err, slot0)
		}
		cleanupLeaseRuns(t, overlapping, next)
		store, err := filepath.EvalSymlinks(ScratchStore(control))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(slot0, first.ID()) || filepath.Dir(filepath.Dir(filepath.Dir(slot0))) != filepath.Join(store, scratchLeaseDir) {
			t.Fatalf("slot %q is not a run-invariant lease path", slot0)
		}
	})
}

func cleanupLeaseRuns(t *testing.T, runs ...*ScratchRun) {
	t.Helper()
	for _, run := range runs {
		if err := run.Cleanup(nil); err != nil {
			t.Fatal(err)
		}
	}
}

// A crashed owner keeps its slot while its record stands; recovery removes
// its recorded worktrees, releases the lease, and only then is the slot free.
// A slot a later run owns is never touched by the dead run's recovery.
func TestScratchLeaseOfACrashedRunIsReleasedOnlyByRecovery(t *testing.T) {
	t.Parallel()
	lockedScratch(t, func() {
		control := t.TempDir()
		probe := newScratchProbe(t)
		var slot string
		crashed := newLeaseRun(t, control)
		_ = crashed.writer.Close()
		record := readScratchRecord(t, control, crashed.ID())
		_, record.Launcher = probe.ref(t, 900001)
		writeScratchRecord(t, control, record)
		slot, err := crashed.ClaimLease(testLeasePolicy, "key")
		if err != nil {
			t.Fatal(err)
		}
		tuple := ScratchWorktree{Group: "groups", Parent: filepath.Join(slot, scratchLeaseWorktree), Top: filepath.Join(slot, scratchLeaseWorktree, "wt-key-0"),
			Control: "/c", Common: "/c/.git", State: ScratchWorktreeReserved}
		if err := crashed.mutate(func(r *ScratchRecord) error { r.Worktrees = append(r.Worktrees, tuple); return nil }); err != nil {
			t.Fatal(err)
		}
		waiting := newLeaseRun(t, control)
		if other, err := waiting.ClaimLease(testLeasePolicy, "key"); err != nil || other == slot {
			t.Fatalf("claim beside a crashed owner = %q %v, want another slot", other, err)
		}
		var removed []gittree.WorktreeTuple
		options := ScratchOptions{Prober: probe, GroupMembers: func(int64) ([]int64, error) { return nil, nil }, Self: waiting.ID(),
			RemoveWorktree: func(tuple gittree.WorktreeTuple, _ gittree.Workspace) (string, error) {
				removed = append(removed, tuple)
				return gittree.WorktreeRemoved, nil
			}}
		if outcome := scratchOutcome(ReconcileScratch(control, options), record.Run); outcome.Action != ReconcileScratchRemoved || len(removed) != 1 {
			t.Fatalf("recovery = %+v removed=%v", outcome, removed)
		}
		later := newLeaseRun(t, control)
		if again, err := later.ClaimLease(testLeasePolicy, "key"); err != nil || again != slot {
			t.Fatalf("claim after recovery = %q %v, want %q", again, err, slot)
		}
		// The dead run's tuple now names a path the later run owns.
		stale := ScratchRecord{Root: record.Root, Leases: []string{slot}, Worktrees: []ScratchWorktree{tuple}}
		if reason, err := removeRecordedWorktrees(stale, options); err != nil || len(removed) != 1 {
			t.Fatalf("stale tuple under a foreign lease was removed: %s %v %v", reason, err, removed)
		}
		cleanupLeaseRuns(t, waiting, later)
	})
}
