package diskstore

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// disposalBed is one mirrored chain in a segment.
func disposalBed(t *testing.T, files int) (item, ledger string) {
	t.Helper()
	root := realDir(t)
	item = filepath.Join(root, "agents", "107e72c67539", "chain-a")
	writeBedFile(t, filepath.Join(item, "jobs", "chain-a.json"), []byte(`{"jobId":"chain-a"}`))
	writeBedFile(t, filepath.Join(item, "jobs", "chain-a.log"), []byte(strings.Repeat("log ", 4096)))
	writeBedFile(t, filepath.Join(item, "brief.md"), []byte("brief"))
	for index := 0; index < files; index++ {
		writeBedFile(t, filepath.Join(item, "rounds", "1", fmt.Sprintf("member-with-a-rather-long-name-so-the-inventory-grows-%06d.txt", index)), []byte{byte(index)})
	}
	return item, filepath.Join(root, "disposals", "107e72c67539.jsonl")
}

func removeStep(item, ledger, id string) DisposalStep {
	return DisposalStep{Item: item, Ledger: ledger, Stage: id,
		Receipt: DisposalReceipt{ID: id, At: testNow, Segment: "107e72c67539", Kind: KindChain, Item: "chain-a", Rule: RulePerson, By: "wido"}}
}

func TestACrashBeforeTheReceiptIsOnlyEverRolledBack(t *testing.T) {
	t.Parallel()
	for _, point := range []string{"tombstone", "aside"} {
		item, ledger := disposalBed(t, 3)
		before := treeFiles(t, filepath.Dir(item))
		step := removeStep(item, ledger, "01RECEIPT0000000000000000A")
		step.interrupt = func(at string) bool { return at == point }
		if _, err := Dispose(context.Background(), step); !errors.Is(err, errInterrupted) {
			t.Fatal(err)
		}
		if _, err := Dispose(context.Background(), removeStep(item, ledger, "01RECEIPT0000000000000000B")); !errors.Is(err, ErrDisposalOpen) {
			t.Fatalf("an open removal is settled before any other: %v", err)
		}
		settled, err := SettlePersonDisposal(context.Background(), item, ledger, Syncer{}, "01STAGE")
		if err != nil || !settled.RolledBack || settled.Finished {
			t.Fatalf("after %s: %+v %v", point, settled, err)
		}
		sameFiles(t, treeFiles(t, filepath.Dir(item)), before)
		if receipts, _ := ReadReceipts(ledger); len(receipts) != 0 {
			t.Fatalf("no receipt before the commit point: %+v", receipts)
		}
	}
}

func TestACrashAfterTheReceiptRemovesOnlyTheSetAsideCopy(t *testing.T) {
	t.Parallel()
	item, ledger := disposalBed(t, 3)
	step := removeStep(item, ledger, "01RECEIPT0000000000000000A")
	step.interrupt = func(point string) bool { return point == "receipt" }
	if _, err := Dispose(context.Background(), step); !errors.Is(err, errInterrupted) {
		t.Fatal(err)
	}
	settled, err := SettlePersonDisposal(context.Background(), item, ledger, Syncer{}, "01STAGE")
	if err != nil || !settled.Finished {
		t.Fatalf("%+v %v", settled, err)
	}
	if receipts, _ := ReadReceipts(ledger); len(receipts) != 1 || receipts[0].ID != "01RECEIPT0000000000000000A" {
		t.Fatalf("exactly the one receipt: %+v", receipts)
	}
	if entries, _ := os.ReadDir(filepath.Dir(item)); len(entries) != 1 || entries[0].Name() != filepath.Base(RemovedTombstonePath(item)) {
		t.Fatalf("only the done tombstone stays: %v", entries)
	}
	tombstone, err := ReadTombstone(RemovedTombstonePath(item))
	if err != nil || tombstone.State != StateDone {
		t.Fatalf("the tombstone is done: %+v %v", tombstone, err)
	}
	if again, err := Dispose(context.Background(), removeStep(item, ledger, "01RECEIPT0000000000000000C")); err != nil || !again.Already {
		t.Fatalf("removing a removed item succeeds and writes nothing: %+v %v", again, err)
	}
}

// A removal with a sidecar-sized inventory, crashed before its receipt and
// rolled back, leaves the item byte-identical and no record behind; a
// completed one keeps its inventory in its sidecar.
func TestALargeRemovalKeepsItsInventoryInASidecar(t *testing.T) {
	t.Parallel()
	item, ledger := disposalBed(t, 7000)
	before := treeFiles(t, filepath.Dir(item))
	step := removeStep(item, ledger, "01RECEIPT0000000000000000B")
	step.interrupt = func(point string) bool { return point == "aside" }
	if _, err := Dispose(context.Background(), step); !errors.Is(err, errInterrupted) {
		t.Fatal(err)
	}
	if _, err := os.Stat(item); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("the removal set the item aside: %v", err)
	}
	if _, err := SettlePersonDisposal(context.Background(), item, ledger, Syncer{}, "01STAGE"); err != nil {
		t.Fatal(err)
	}
	sameFiles(t, treeFiles(t, filepath.Dir(item)), before)

	result, err := Dispose(context.Background(), removeStep(item, ledger, "01RECEIPT0000000000000000C"))
	if err != nil || result.Tombstone != RemovedTombstonePath(item) {
		t.Fatalf("%+v %v", result, err)
	}
	removal, err := ReadTombstone(RemovedTombstonePath(item))
	if err != nil || removal.State != StateDone || removal.Sidecar == "" || len(removal.Files) != 0 {
		t.Fatalf("a large inventory goes to the sidecar: %+v %v", removal.Sidecar, err)
	}
	files, err := TombstoneFiles(RemovedTombstonePath(item), removal)
	if err != nil || len(files) < 7000 {
		t.Fatalf("the sidecar holds the walk: %d %v", len(files), err)
	}
	if receipts, _ := ReadReceipts(ledger); len(receipts) != 1 {
		t.Fatalf("one receipt, for the committed removal: %d", len(receipts))
	}
}

func TestInventoryNamesTheLogicalOriginalOfADistilledFile(t *testing.T) {
	t.Parallel()
	bed := newDistillBed(t)
	if _, err := Distill(context.Background(), bed.bundle, bed.rules("01STAGE0000000000000000001"), testNow); err != nil {
		t.Fatal(err)
	}
	files, err := Inventory(context.Background(), bed.bundle)
	if err != nil {
		t.Fatal(err)
	}
	var logical, physical bool
	for _, file := range files {
		if file.Path == "source-003-log/run.log" && file.Original != nil && file.Original.Replacement == "source-003-log/run.log.gz" {
			logical = true
		}
		if file.Path == "source-003-log/run.log.gz" && file.Original == nil && file.SHA256 != "" {
			physical = true
		}
	}
	if !logical || !physical {
		t.Fatalf("the inventory names the physical replacement and the logical original: %+v", files)
	}
}
