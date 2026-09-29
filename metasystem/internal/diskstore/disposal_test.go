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

func chainKept(rel string) bool {
	return rel == "brief.md" || strings.HasPrefix(rel, "jobs/") && strings.HasSuffix(rel, ".json")
}

func compactStep(item, ledger, id string) DisposalStep {
	return DisposalStep{Item: item, Ledger: ledger, Kept: chainKept, Verdict: []byte("1|chain-a|implementer|completed\n"), Stage: id,
		Receipt: DisposalReceipt{ID: id, At: testNow, Segment: "107e72c67539", Kind: KindChain, Item: "chain-a", Rule: RuleBound, By: "steward m1e"}}
}

func removeStep(item, ledger, id string) DisposalStep {
	return DisposalStep{Item: item, Ledger: ledger, Stage: id,
		Receipt: DisposalReceipt{ID: id, At: testNow, Segment: "107e72c67539", Kind: KindChain, Item: "chain-a", Rule: RulePerson, By: "wido"}}
}

func TestACrashBeforeTheReceiptRollsBackAndLeavesTheItemWhole(t *testing.T) {
	t.Parallel()
	item, ledger := disposalBed(t, 3)
	before := treeFiles(t, filepath.Dir(item))
	step := compactStep(item, ledger, "01RECEIPT0000000000000000A")
	step.interrupt = func(point string) bool { return point == "tombstone" }
	if _, err := Dispose(context.Background(), step); !errors.Is(err, errInterrupted) {
		t.Fatal(err)
	}
	if _, err := Dispose(context.Background(), compactStep(item, ledger, "01RECEIPT0000000000000000B")); !errors.Is(err, ErrDisposalOpen) {
		t.Fatalf("an open disposal is recovered before any other: %v", err)
	}
	result, err := RecoverDisposal(context.Background(), item, ledger, step.Receipt, Syncer{}, "01STAGE", func(Tombstone) Recovery {
		return Recovery{Reason: "goal G was reopened"}
	})
	if err != nil || result.RolledBack != "goal G was reopened" {
		t.Fatalf("%+v %v", result, err)
	}
	sameFiles(t, treeFiles(t, filepath.Dir(item)), before)
	if receipts, _ := ReadReceipts(ledger); len(receipts) != 0 {
		t.Fatalf("no receipt before the commit point: %+v", receipts)
	}
}

func TestACrashAfterTheReceiptFinishesWithThatOneReceipt(t *testing.T) {
	t.Parallel()
	item, ledger := disposalBed(t, 3)
	step := compactStep(item, ledger, "01RECEIPT0000000000000000A")
	step.interrupt = func(point string) bool { return point == "receipt" }
	if _, err := Dispose(context.Background(), step); !errors.Is(err, errInterrupted) {
		t.Fatal(err)
	}
	result, err := RecoverDisposal(context.Background(), item, ledger, step.Receipt, Syncer{}, "01STAGE", func(Tombstone) Recovery {
		t.Fatal("a committed disposal is never re-judged")
		return Recovery{}
	})
	if err != nil || result.RolledBack != "" {
		t.Fatalf("%+v %v", result, err)
	}
	if receipts, _ := ReadReceipts(ledger); len(receipts) != 1 || receipts[0].ID != "01RECEIPT0000000000000000A" {
		t.Fatalf("exactly the one receipt: %+v", receipts)
	}
	if _, err := os.Stat(filepath.Join(item, "jobs", "chain-a.log")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("the dropped members go: %v", err)
	}
	tombstone, err := ReadTombstone(filepath.Join(item, CompactTombstoneName))
	if err != nil || tombstone.State != StateDone {
		t.Fatalf("the tombstone is done: %+v %v", tombstone, err)
	}
	if again, err := Dispose(context.Background(), compactStep(item, ledger, "01RECEIPT0000000000000000C")); err != nil || !again.Already {
		t.Fatalf("compacting a compacted item succeeds and writes nothing: %+v %v", again, err)
	}
}

// A removal of a compacted item with a sidecar-sized inventory, crashed
// before its receipt and rolled back, leaves the compacted item, its
// tombstone and its sidecar byte-identical (DL4E-06); a completed removal
// carries the compaction whole in its history and its sidecar in its own.
func TestARemovalCopiesTheCompactionSidecarAndRollsBackCleanly(t *testing.T) {
	t.Parallel()
	item, ledger := disposalBed(t, 7000)
	if _, err := Dispose(context.Background(), compactStep(item, ledger, "01RECEIPT0000000000000000A")); err != nil {
		t.Fatal(err)
	}
	compaction, err := ReadTombstone(filepath.Join(item, CompactTombstoneName))
	if err != nil || compaction.Sidecar != CompactSidecarName || len(compaction.Files) != 0 {
		t.Fatalf("a large inventory goes to the sidecar: %+v %v", compaction.Sidecar, err)
	}
	before := treeFiles(t, filepath.Dir(item))
	step := removeStep(item, ledger, "01RECEIPT0000000000000000B")
	step.interrupt = func(point string) bool { return point == "aside" }
	if _, err := Dispose(context.Background(), step); !errors.Is(err, errInterrupted) {
		t.Fatal(err)
	}
	if _, err := os.Stat(item); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("the removal set the item aside: %v", err)
	}
	if _, err := RecoverDisposal(context.Background(), item, ledger, step.Receipt, Syncer{}, "01STAGE", func(Tombstone) Recovery {
		return Recovery{Reason: "export not verified after restart"}
	}); err != nil {
		t.Fatal(err)
	}
	sameFiles(t, treeFiles(t, filepath.Dir(item)), before)

	result, err := Dispose(context.Background(), removeStep(item, ledger, "01RECEIPT0000000000000000C"))
	if err != nil || result.Tombstone != RemovedTombstonePath(item) {
		t.Fatalf("%+v %v", result, err)
	}
	if _, err := os.Stat(item); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("the item is removed: %v", err)
	}
	removal, err := ReadTombstone(RemovedTombstonePath(item))
	if err != nil || removal.State != StateDone || len(removal.History) != 1 || removal.History[0].Receipt != "01RECEIPT0000000000000000A" {
		t.Fatalf("the removal carries the compaction whole: %+v %v", removal.History, err)
	}
	files, err := TombstoneFiles(RemovedTombstonePath(item), removal)
	if err != nil {
		t.Fatal(err)
	}
	own, history := 0, 0
	for _, file := range files {
		if file.History == 1 {
			history++
		} else {
			own++
		}
	}
	if own == 0 || history < 7000 {
		t.Fatalf("the removal's sidecar holds its own walk and the compaction's copied in: own=%d history=%d", own, history)
	}
	if again, err := Dispose(context.Background(), removeStep(item, ledger, "01RECEIPT0000000000000000D")); err != nil || !again.Already {
		t.Fatalf("removing a removed item succeeds and writes nothing: %+v %v", again, err)
	}
	if receipts, _ := ReadReceipts(ledger); len(receipts) != 2 {
		t.Fatalf("two receipts, one per committed disposal: %d", len(receipts))
	}
}

func TestInventoryNamesTheLogicalOriginalOfADistilledFile(t *testing.T) {
	t.Parallel()
	bed := newDistillBed(t)
	if _, err := Distill(context.Background(), bed.bundle, bed.rules("01STAGE0000000000000000001"), testNow); err != nil {
		t.Fatal(err)
	}
	files, err := Inventory(context.Background(), bed.bundle, nil)
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
