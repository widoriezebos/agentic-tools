package diskstore

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// blobBed is one distilled bundle in a segment of an evidence root, whose
// engine went to the host's blob store.
type blobBed struct {
	root, bundle, installation string
	blobs                      BlobStore
	digest                     string
}

func newBlobBed(t *testing.T, name string, root string) blobBed {
	t.Helper()
	if root == "" {
		root = realDir(t)
	}
	bed := blobBed{root: root, installation: filepath.Join(root, "checkout", "metasystem"),
		blobs: BlobStore{Dir: filepath.Join(root, "home", "metasystem-evidence", ".blobs")}}
	bed.bundle = filepath.Join(root, "evidence", "suite-failures", "107e72c67539", name)
	writeBedFile(t, filepath.Join(bed.bundle, "bin", "engine"), append([]byte("\x7fELF"), bytes.Repeat([]byte("e"), testMiB)...))
	result, err := Distill(context.Background(), bed.bundle, DistillRules{CompressAbove: testMiB, Blobs: bed.blobs, Referrer: "107e72c67539-" + name,
		Installation: bed.installation, Segment: "107e72c67539", Stage: "01STAGE-" + name}, testNow)
	if err != nil || len(result.Replaced) != 1 {
		t.Fatalf("%+v %v", result, err)
	}
	bed.digest = result.Replaced[0].SHA256
	return bed
}

func (bed blobBed) check(now time.Time, armed bool) BlobCheck {
	return BlobCheck{Blobs: bed.blobs, Now: now, Grace: 24 * time.Hour, AgeFloor: 90 * 24 * time.Hour, By: "steward m1e",
		Armed: func(string) bool { return armed }}
}

func (bed blobBed) blobPresent() bool {
	_, err := os.Stat(bed.blobs.Path(bed.digest))
	return err == nil
}

func TestABlobWhoseRecipeStandsSurvivesEverySweep(t *testing.T) {
	t.Parallel()
	bed := newBlobBed(t, "20260901T000000Z-watchdog-a", "")
	result := bed.check(testNow.Add(400*24*time.Hour), true).Run(context.Background())
	if !bed.blobPresent() || result.Kept != 1 || len(result.Dropped) != 0 {
		t.Fatalf("a distilled bundle keeps its blob through the sweep: %+v", result)
	}
}

func TestASharedBlobSurvivesTheFirstBundlesRemoval(t *testing.T) {
	t.Parallel()
	first := newBlobBed(t, "20260901T000000Z-watchdog-a", "")
	second := newBlobBed(t, "20260902T000000Z-watchdog-b", first.root)
	if refs, _ := first.blobs.Refs(first.digest); len(refs) != 2 {
		t.Fatalf("an older blob reused by a new bundle gains a second reference: %+v", refs)
	}
	ledger := filepath.Join(first.root, "evidence", "disposals", "107e72c67539.jsonl")
	if _, err := Dispose(context.Background(), DisposalStep{Item: first.bundle, Ledger: ledger, Stage: "01R",
		Receipt: DisposalReceipt{ID: "01RECEIPTR", Kind: KindBundle, Item: filepath.Base(first.bundle), Segment: "107e72c67539", Rule: RulePerson}}); err != nil {
		t.Fatal(err)
	}
	result := first.check(testNow.Add(400*24*time.Hour), true).Run(context.Background())
	if len(result.Dropped) != 1 || !strings.Contains(result.Dropped[0], "20260901T000000Z-watchdog-a") {
		t.Fatalf("a committed removal's reference is dropped: %+v", result)
	}
	if !second.blobPresent() {
		t.Fatal("the blob the second bundle still references survives")
	}
}

func TestAnUncommittedRemovalKeepsEveryReference(t *testing.T) {
	t.Parallel()
	bed := newBlobBed(t, "20260901T000000Z-watchdog-a", "")
	ledger := filepath.Join(bed.root, "evidence", "disposals", "107e72c67539.jsonl")
	step := DisposalStep{Item: bed.bundle, Ledger: ledger, Stage: "01R",
		Receipt: DisposalReceipt{ID: "01RECEIPTR", Kind: KindBundle, Item: filepath.Base(bed.bundle), Segment: "107e72c67539", Rule: RulePerson}}
	step.interrupt = func(point string) bool { return point == "aside" }
	if _, err := Dispose(context.Background(), step); !errors.Is(err, errInterrupted) {
		t.Fatal(err)
	}
	later := testNow.Add(400 * 24 * time.Hour)
	if result := bed.check(later, true).Run(context.Background()); result.Kept != 1 || !bed.blobPresent() {
		t.Fatalf("a bundle set aside under a begun tombstone keeps its reference: %+v", result)
	}
	if _, err := SettlePersonDisposal(context.Background(), bed.bundle, ledger, Syncer{}, "01S"); err != nil {
		t.Fatal(err)
	}
	if result := bed.check(later.Add(48*time.Hour), true).Run(context.Background()); len(result.Removed) != 0 || !bed.blobPresent() {
		t.Fatalf("after the rollback the references are intact and the sweep removes nothing: %+v", result)
	}
}

func TestTheCheckIsPendingOnALiveDistillerAndKeepsAnInterruptedOne(t *testing.T) {
	t.Parallel()
	bed := newDistillBed(t)
	rules := bed.rules("01STAGE0000000000000000001")
	rules.interrupt = func(step, path string) bool { return step == "referenced" }
	if _, err := Distill(context.Background(), bed.bundle, rules, testNow); !errors.Is(err, errInterrupted) {
		t.Fatal(err)
	}
	check := BlobCheck{Blobs: bed.blobs, Now: testNow.Add(400 * 24 * time.Hour), Grace: time.Hour, AgeFloor: 90 * 24 * time.Hour, Armed: func(string) bool { return true }}
	// A distiller in its transaction holds the lock shared.
	release, err := bed.blobs.TryShared()
	if err != nil {
		t.Fatal(err)
	}
	if result := check.Run(context.Background()); result.Pending == "" {
		t.Fatalf("the check is pending while a transaction is live: %+v", result)
	}
	release()
	// The distiller died between its reference and its recipe line: the
	// bundle is present, so its references stand (Round B2-2, R2).
	result := check.Run(context.Background())
	if len(result.Dropped) != 0 {
		t.Fatalf("a present bundle's reference is never dropped: %+v", result)
	}
	if _, err := os.Stat(filepath.Join(bed.bundle, "source-001-tmp", "work", "README")); err != nil {
		t.Fatalf("the original is intact: %v", err)
	}
	if _, err := Distill(context.Background(), bed.bundle, bed.rules("01STAGE0000000000000000002"), testNow); err != nil {
		t.Fatal(err)
	}
	sameFiles(t, restoreBundle(t, bed.bundle, bed.blobs), bed.original)
}

func TestAHandDeletedBundleIsReportedAndItsReferencesNeverDropped(t *testing.T) {
	t.Parallel()
	bed := newBlobBed(t, "20260901T000000Z-watchdog-a", "")
	// The segment stays reachable: another bundle is in it.
	writeBedFile(t, filepath.Join(filepath.Dir(bed.bundle), "20260902T000000Z-watchdog-b", "copy-note.txt"), []byte("x"))
	if err := os.RemoveAll(bed.bundle); err != nil {
		t.Fatal(err)
	}
	for _, at := range []time.Time{testNow, testNow.Add(91 * 24 * time.Hour), testNow.Add(400 * 24 * time.Hour)} {
		result := bed.check(at, true).Run(context.Background())
		if len(result.Dangling) != 1 || len(result.Dropped) != 0 || len(result.Removed) != 0 || !bed.blobPresent() {
			t.Fatalf("at %s: a bundle gone without a committed removal is reported, its references kept (Round B2-2, R2): %+v", at, result)
		}
	}
}

// Round B2-2, R2: an evidence root that is not reachable (its segment
// directory empty or unreadable, as an unmounted volume leaves it) holds
// the whole check.
func TestAnUnreachableSegmentHoldsTheWholeCheck(t *testing.T) {
	t.Parallel()
	bed := newBlobBed(t, "20260901T000000Z-watchdog-a", "")
	if err := os.RemoveAll(bed.bundle); err != nil {
		t.Fatal(err)
	}
	result := bed.check(testNow.Add(400*24*time.Hour), true).Run(context.Background())
	if result.Pending == "" || len(result.Dropped) != 0 || !bed.blobPresent() {
		t.Fatalf("%+v", result)
	}
}
