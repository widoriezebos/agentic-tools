package steward

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
)

func snapshotStewardRoot(t *testing.T, root string) string {
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
		line := fmt.Sprintf("%s %s %d", strings.TrimPrefix(path, root), info.Mode(), info.ModTime().UnixNano())
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

// twoOldCancelledHandoffs makes two complete, unprotected handoffs aged past
// the cutoff.
func twoOldCancelledHandoffs(t *testing.T) (string, []string) {
	t.Helper()
	fixture := newHandoffGoalFixture(t, "claimed")
	root := fixture.root
	useHandoffNonces(t, "6800000000000021", "6800000000000022")
	var nonces []string
	for range 2 {
		handoff, err := fixture.handoff(root, handoffMainCaller(), handoffTestRecord(t, root, nil), handoffCaptureNow, filepath.Join(root, "memory", "receipts.log"))
		if err != nil {
			t.Fatal(err)
		}
		if err := CancelHandoff(root, handoff.Nonce, HandoffCanceller{Caller: handoffMainCaller()}); err != nil {
			t.Fatal(err)
		}
		ageHandoffState(t, root, handoff.Nonce, handoffCaptureNow.AddDate(0, 0, -30))
		nonces = append(nonces, handoff.Nonce)
	}
	sort.Strings(nonces)
	return root, nonces
}

// InspectHandoffs is the plan half (DL3B-05): it takes no lock, creates
// nothing, removes nothing, and lists the complete, unprotected, old nonces.
func TestInspectHandoffsObservesWithoutALock(t *testing.T) {
	root, nonces := twoOldCancelledHandoffs(t)
	lock, err := AcquireArbitration(root)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Release()
	before := snapshotStewardRoot(t, root)
	cutoff := handoffCaptureNow.AddDate(0, 0, -14)
	found, problems := InspectHandoffs(root, cutoff, handoffCaptureNow)
	if len(problems) != 0 {
		t.Fatalf("inspection problems: %v", problems)
	}
	if strings.Join(found, ",") != strings.Join(nonces, ",") {
		t.Fatalf("inspected %v, want %v", found, nonces)
	}
	if after := snapshotStewardRoot(t, root); after != before {
		t.Fatalf("the inspection changed the root:\nbefore %s\nafter  %s", before, after)
	}
	if found, _ := InspectHandoffs(root, handoffCaptureNow.AddDate(0, 0, -60), handoffCaptureNow); len(found) != 0 {
		t.Fatalf("handoffs younger than the cutoff were listed: %v", found)
	}
}

// PruneHandoffs acquires arbitration once per nonce and releases it before
// the next; a held lock stops the class with nothing removed and the rest
// pending (DL3B-04).
func TestPruneHandoffsTakesArbitrationPerNonce(t *testing.T) {
	root, nonces := twoOldCancelledHandoffs(t)
	cutoff := handoffCaptureNow.AddDate(0, 0, -14)
	held := func(string) (*ArbitrationLock, error) { return nil, ErrArbitrationHeld }
	removed, err := PruneHandoffs(context.Background(), root, nonces, cutoff, handoffCaptureNow, held)
	if !errors.Is(err, ErrArbitrationHeld) || len(removed) != 0 {
		t.Fatalf("prune under a held lock = %v, %v", removed, err)
	}
	for _, nonce := range nonces {
		if _, err := os.Stat(HandoffDir(root, nonce)); err != nil {
			t.Fatalf("a held prune removed %s: %v", nonce, err)
		}
	}
	acquisitions := 0
	counting := func(repoRoot string) (*ArbitrationLock, error) {
		acquisitions++
		// Between nonces nothing is held: a nonblocking attempt succeeds.
		probe, err := TryAcquireArbitration(repoRoot)
		if err != nil {
			return nil, fmt.Errorf("arbitration was still held between nonces: %w", err)
		}
		probe.Release()
		return TryAcquireArbitration(repoRoot)
	}
	removed, err = PruneHandoffs(context.Background(), root, nonces, cutoff, handoffCaptureNow, counting)
	if err != nil || len(removed) != 2 || acquisitions != 2 {
		t.Fatalf("prune = %v, %v after %d acquisitions; want both removed, one acquisition each", removed, err, acquisitions)
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := PruneHandoffs(cancelled, root, nonces, cutoff, handoffCaptureNow, counting); !errors.Is(err, context.Canceled) {
		t.Fatalf("a cancelled prune = %v", err)
	}
}

// ProbeArbitration never creates the lock file or its directory; an absent
// lock reads free and a held one busy.
func TestProbeArbitrationCreatesNothing(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	before := snapshotStewardRoot(t, root)
	if free, err := ProbeArbitration(root); err != nil || !free {
		t.Fatalf("an absent arbitration lock = %v, %v; want free", free, err)
	}
	if after := snapshotStewardRoot(t, root); after != before {
		t.Fatal("the probe created a file or directory")
	}
	lock, err := AcquireArbitration(root)
	if err != nil {
		t.Fatal(err)
	}
	if free, err := ProbeArbitration(root); err != nil || free {
		t.Fatalf("a held arbitration lock = %v, %v; want busy", free, err)
	}
	if _, err := TryAcquireArbitration(root); !errors.Is(err, ErrArbitrationHeld) {
		t.Fatalf("a nonblocking acquisition of a held lock = %v", err)
	}
	lock.Release()
	if free, _ := ProbeArbitration(root); !free {
		t.Fatal("a released lock reads busy")
	}
}
