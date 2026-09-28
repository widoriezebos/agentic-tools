package usage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

func snapshotUsageRoot(t *testing.T, root string) string {
	t.Helper()
	var lines []string
	if err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		line := fmt.Sprintf("%s %s %d %d", strings.TrimPrefix(path, root), info.Mode(), info.Size(), info.ModTime().UnixNano())
		if info.Mode().IsRegular() {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			sum := sha256.Sum256(data)
			line += " " + hex.EncodeToString(sum[:])
		}
		lines = append(lines, line)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	sort.Strings(lines)
	return strings.Join(lines, "\n")
}

// removeUsageLocks takes every lock file out of a fixture, so the witness
// covers a first-ever inspection on a store with no lock files at all.
func removeUsageLocks(t *testing.T, root string) {
	t.Helper()
	var locks []string
	if err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err == nil && !entry.IsDir() && strings.HasSuffix(path, ".lock") {
			locks = append(locks, path)
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
	for _, lock := range locks {
		if err := os.Remove(lock); err != nil {
			t.Fatal(err)
		}
	}
}

// interruptRetirement leaves a retirement journal behind by failing the
// cursor unlink of a real retirement.
func interruptRetirement(t *testing.T, root, session string, before time.Time) string {
	t.Helper()
	old := before.Add(-4 * time.Hour)
	seedRetirementCall(t, root, session, old)
	setRetirementPairTimes(t, root, "claude", session, old, old)
	cursorPath := CursorPath(root, "claude", session)
	originalRemove := removeCallStorePath
	removeCallStorePath = func(path string) error {
		if path == cursorPath {
			return errors.New("injected unlink failure")
		}
		return originalRemove(path)
	}
	defer func() { removeCallStorePath = originalRemove }()
	if _, err := PruneCallSessions(root, before); err == nil {
		t.Fatal("the injected interruption did not interrupt")
	}
	if _, err := os.Stat(callRetirementPath(cursorPath)); err != nil {
		t.Fatalf("no journal was left: %v", err)
	}
	return cursorPath
}

// The inspection is read-only (DL3B-05): over a root with no lock files, an
// interrupted journal and an eligible pair it lists both, recovers nothing,
// publishes no boundary and creates no file or directory.
func TestInspectCallSessionsCreatesAndRecoversNothing(t *testing.T) {
	root := t.TempDir()
	before := retirementTestCutoff()
	now := before.Add(callRetentionWindow + 48*time.Hour)
	interrupted := interruptRetirement(t, root, "interrupted", before)
	old := before.Add(-4 * time.Hour)
	seedRetirementCall(t, root, "old", old)
	setRetirementPairTimes(t, root, "claude", "old", old, old)
	removeUsageLocks(t, root)

	snapshot := snapshotUsageRoot(t, root)
	inspection, err := InspectCallSessions(root, before, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(inspection.Candidates) != 1 || inspection.Candidates[0].Session != "old" {
		t.Fatalf("candidates = %+v", inspection.Candidates)
	}
	if len(inspection.Interrupted) != 1 || inspection.Interrupted[0] != interrupted {
		t.Fatalf("interrupted = %v, want %s", inspection.Interrupted, interrupted)
	}
	if inspection.MaintenanceHeld {
		t.Fatal("an absent maintenance lock reads held")
	}
	if after := snapshotUsageRoot(t, root); after != snapshot {
		t.Fatalf("the inspection changed the store:\nbefore %s\nafter  %s", snapshot, after)
	}
	if free, err := ProbeMaintenance(root); err != nil || !free {
		t.Fatalf("probe of an absent maintenance lock = %v, %v", free, err)
	}
	if after := snapshotUsageRoot(t, root); after != snapshot {
		t.Fatal("the maintenance probe created a file")
	}
	if _, err := InspectCallSessions(root, before, before.Add(callRetentionWindow)); err == nil {
		t.Fatal("a cutoff inside the retention window of the given now was accepted")
	}
}

// RetireCallSessions carries its caller's clock into the cutoff check, takes
// the maintenance lock without waiting, retires at most limit pairs, and
// releases the lock between calls (DL2-16, DL3B-04).
func TestRetireCallSessionsIsLimitedNonblockingAndClocked(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	before := retirementTestCutoff()
	old := before.Add(-4 * time.Hour)
	for _, session := range []string{"first", "second"} {
		seedRetirementCall(t, root, session, old)
		setRetirementPairTimes(t, root, "claude", session, old, old)
	}
	now := before.Add(callRetentionWindow + 48*time.Hour)

	held, err := lockCallMaintenance(root, true, false)
	if err != nil {
		t.Fatal(err)
	}
	if free, _ := ProbeMaintenance(root); free {
		t.Fatal("a held maintenance lock reads free")
	}
	var busy *CallStoreBusyError
	if removed, err := RetireCallSessions(context.Background(), root, before, now, 1); !errors.As(err, &busy) || removed != 0 {
		t.Fatalf("retire under a held lock = %d, %v; want busy at once", removed, err)
	}
	unlockCallFile(held)

	for round, want := range []int{1, 1, 0} {
		removed, err := RetireCallSessions(context.Background(), root, before, now, 1)
		if err != nil || removed != want {
			t.Fatalf("round %d retired %d, %v; want %d", round, removed, err, want)
		}
		if free, err := ProbeMaintenance(root); err != nil || !free {
			t.Fatalf("round %d kept the maintenance lock: %v %v", round, free, err)
		}
	}
	assertRetirementPairAbsent(t, root, "claude", "first")
	assertRetirementPairAbsent(t, root, "claude", "second")

	// The cutoff is judged against the given now, never the wall clock.
	if _, err := RetireCallSessions(context.Background(), root, before, before.Add(callRetentionWindow-time.Hour), 1); err == nil {
		t.Fatal("a cutoff inside the window of the given now was accepted")
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := RetireCallSessions(cancelled, root, before, now, 1); !errors.Is(err, context.Canceled) {
		t.Fatalf("a cancelled retirement = %v", err)
	}
	if _, err := RetireCallSessions(context.Background(), root, before, now, 0); err == nil {
		t.Fatal("a zero limit was accepted")
	}
}
