package diskstore

// Round B2-3 (scope cut) witnesses, ported from the third read's probes
// (b2-read3-probes, each failing on 3dd7f7b9f).

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func receiptLine(t *testing.T, id, item string) []byte {
	t.Helper()
	line, err := json.Marshal(DisposalReceipt{Schema: ReceiptSchema, ID: id, Item: item, Step: StepRemove, Rule: RulePerson})
	if err != nil {
		t.Fatal(err)
	}
	return line
}

// Rule 5: a symlinked ledger is refused and never written; the real
// ledger keeps every byte even when the append would have failed.
func TestASymlinkedLedgerIsRefusedAndNeverWritten(t *testing.T) {
	t.Parallel()
	dir := realDir(t)
	real := filepath.Join(dir, "elsewhere", "107e72c67539.jsonl")
	var ledger bytes.Buffer
	for _, id := range []string{"01A", "01B", "01C", "01D"} {
		ledger.Write(append(receiptLine(t, id, "item-"+id), '\n'))
	}
	writeBedFile(t, real, ledger.Bytes())
	link := filepath.Join(dir, "root", "disposals", "107e72c67539.jsonl")
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	for _, sync := range []Syncer{{}, {File: func(*os.File) error { return errors.New("EIO") }}} {
		err := AppendReceipt(link, DisposalReceipt{Schema: ReceiptSchema, ID: "01E", Item: "item-01E"}, sync)
		if err == nil || !strings.Contains(err.Error(), "not a regular file") {
			t.Fatalf("a symlinked ledger is refused: %v", err)
		}
	}
	if after, _ := os.ReadFile(real); !bytes.Equal(after, ledger.Bytes()) {
		t.Fatalf("the real ledger is untouched: %q", after)
	}
}

// Rule 5: a last line without its newline that parses as a whole receipt
// is whole: the append writes the newline first and keeps it.
func TestAWholeLastLineWithoutNewlineIsKeptAndTheAppendCompletesIt(t *testing.T) {
	t.Parallel()
	ledger := filepath.Join(realDir(t), "disposals", "107e72c67539.jsonl")
	writeBedFile(t, ledger, append(append(receiptLine(t, "01A", "a"), '\n'), receiptLine(t, "01B", "b")...))
	if committed, err := ReceiptCommitted(ledger, "01B", "b"); err != nil || !committed {
		t.Fatalf("the whole last line is committed: %v %v", committed, err)
	}
	if err := AppendReceipt(ledger, DisposalReceipt{Schema: ReceiptSchema, ID: "01C", Item: "c"}, Syncer{}); err != nil {
		t.Fatalf("the append completes the whole line and appends: %v", err)
	}
	receipts, err := ReadReceipts(ledger)
	if err != nil || len(receipts) != 3 || receipts[1].ID != "01B" || receipts[2].ID != "01C" {
		t.Fatalf("every receipt stands: %+v %v", receipts, err)
	}
}

// Rule 5: an unparseable last line is torn: nothing is appended and the
// named repair keeps every parseable receipt.
func TestATornLastLineIsRefusedWithARepairThatKeepsEveryReceipt(t *testing.T) {
	t.Parallel()
	ledger := filepath.Join(realDir(t), "disposals", "107e72c67539.jsonl")
	whole := append(receiptLine(t, "01A", "a"), '\n')
	writeBedFile(t, ledger, append(append([]byte(nil), whole...), []byte(`{"schema":3,"id":"01B","it`)...))
	err := AppendReceipt(ledger, DisposalReceipt{Schema: ReceiptSchema, ID: "01C", Item: "c"}, Syncer{})
	if err == nil || !strings.Contains(err.Error(), "truncate -s 0") && !strings.Contains(err.Error(), "truncate -s "+itoa(len(whole))) {
		t.Fatalf("a torn tail is refused with its repair: %v", err)
	}
	if !strings.Contains(err.Error(), "truncate -s "+itoa(len(whole))+" ") {
		t.Fatalf("the repair keeps the whole receipt 01A: %v", err)
	}
}

func itoa(n int) string {
	data, _ := json.Marshal(n)
	return string(data)
}

// Rule 5: a ledger that cannot be read holds: nothing is appended.
func TestAnUnreadableLedgerHolds(t *testing.T) {
	t.Parallel()
	ledger := filepath.Join(realDir(t), "disposals", "107e72c67539.jsonl")
	writeBedFile(t, ledger, receiptLine(t, "01A", "a"))
	if err := os.Chmod(ledger, 0o200); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(ledger, 0o644) })
	if err := AppendReceipt(ledger, DisposalReceipt{Schema: ReceiptSchema, ID: "01C", Item: "c"}, Syncer{}); err == nil {
		t.Fatal("an unreadable ledger holds the append")
	}
	os.Chmod(ledger, 0o644)
	if data, _ := os.ReadFile(ledger); !bytes.Equal(data, receiptLine(t, "01A", "a")) {
		t.Fatalf("nothing was written: %q", data)
	}
}

// Rule 2, N3-2: a committed tombstone with no set-aside name is refused
// and reported; neither the item path nor its parent is removed.
func TestACommittedTombstoneWithNoSetAsideNameRemovesNothing(t *testing.T) {
	t.Parallel()
	root := realDir(t)
	segment := filepath.Join(root, "agents", "107e72c67539")
	item := filepath.Join(segment, "chain-a")
	own := filepath.Join(item, "jobs", "chain-a.json")
	sibling := filepath.Join(segment, "chain-b", "jobs", "chain-b.json")
	writeBedFile(t, own, []byte(`{"jobId":"chain-a"}`))
	writeBedFile(t, sibling, []byte(`{"jobId":"chain-b"}`))
	ledger := filepath.Join(root, "disposals", "107e72c67539.jsonl")
	for _, disposing := range []string{"", "..", "chain-b", "chain-a.disposing-01/../../x"} {
		tombstone := Tombstone{Schema: TombstoneSchema, Item: "chain-a", Kind: KindChain, Segment: "107e72c67539", History: []HistoryEntry{},
			Step: StepRemove, Rule: RulePerson, By: "Wido", At: testNow, Receipt: "01RECEIPT", State: StateBegun, Disposing: disposing}
		data, _ := json.Marshal(tombstone)
		writeBedFile(t, RemovedTombstonePath(item), data)
		if _, err := os.Stat(ledger); err != nil {
			if err := AppendReceipt(ledger, DisposalReceipt{Schema: ReceiptSchema, ID: "01RECEIPT", Item: "chain-a"}, Syncer{}); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := SettlePersonDisposal(context.Background(), item, ledger, Syncer{}, "01S"); err == nil || !strings.Contains(err.Error(), "no valid set-aside name") {
			t.Fatalf("set-aside name %q is refused: %v", disposing, err)
		}
		for _, path := range []string{own, sibling} {
			if _, err := os.Stat(path); err != nil {
				t.Fatalf("set-aside name %q removed %s", disposing, path)
			}
		}
	}
}

// Rule 2: a committed removal whose item path was taken again after the
// crash removes only the recorded set-aside copy, never the new entry.
func TestACommittedRemovalNeverRemovesWhatNowStandsAtTheItemPath(t *testing.T) {
	t.Parallel()
	item, ledger := disposalBed(t, 3)
	step := removeStep(item, ledger, "01RECEIPT0000000000000000A")
	step.interrupt = func(point string) bool { return point == "receipt" }
	if _, err := Dispose(context.Background(), step); !errors.Is(err, errInterrupted) {
		t.Fatal(err)
	}
	fresh := filepath.Join(item, "jobs", "chain-a-2.log")
	writeBedFile(t, fresh, []byte("written after the crash"))
	settled, err := SettlePersonDisposal(context.Background(), item, ledger, Syncer{}, "01S")
	if err != nil || !settled.Finished {
		t.Fatalf("%+v %v", settled, err)
	}
	if _, err := os.Stat(fresh); err != nil {
		t.Fatalf("the new entry at the item path stays: %v", err)
	}
	matches, _ := filepath.Glob(item + ".disposing-*")
	if len(matches) != 0 {
		t.Fatalf("the set-aside copy is gone: %v", matches)
	}
}

// Rule 2, N3-3: an uncommitted removal with both its set-aside copy and a
// new entry at the item path keeps its tombstone and reports.
func TestARollbackWithBothCopiesPresentKeepsTheTombstone(t *testing.T) {
	t.Parallel()
	item, ledger := disposalBed(t, 3)
	step := removeStep(item, ledger, "01RECEIPT0000000000000000A")
	step.interrupt = func(point string) bool { return point == "aside" }
	if _, err := Dispose(context.Background(), step); !errors.Is(err, errInterrupted) {
		t.Fatal(err)
	}
	writeBedFile(t, filepath.Join(item, "placeholder.txt"), []byte("x"))
	if _, err := SettlePersonDisposal(context.Background(), item, ledger, Syncer{}, "01S"); err == nil || !strings.Contains(err.Error(), "both") {
		t.Fatalf("both present is reported: %v", err)
	}
	if _, err := os.Stat(RemovedTombstonePath(item)); err != nil {
		t.Fatalf("the tombstone is kept: %v", err)
	}
	if matches, _ := filepath.Glob(item + ".disposing-*"); len(matches) != 1 {
		t.Fatalf("the set-aside copy is kept: %v", matches)
	}
	if open := OpenRemovals(filepath.Dir(item)); len(open) != 1 {
		t.Fatalf("it stays listed as open: %v", open)
	}
}

// N3-6: a restart finds an original rewritten after its line was
// published: both are kept and reported, nothing is unlinked.
func TestARestartKeepsAnOriginalThatChangedAfterItsLinePublished(t *testing.T) {
	t.Parallel()
	bed := newDistillBed(t)
	rules := bed.rules("01STAGEA")
	target := "source-003-log/run.log"
	rules.interrupt = func(step, path string) bool { return step == "published" && path == target }
	if _, err := Distill(context.Background(), bed.bundle, rules, testNow); !errors.Is(err, errInterrupted) {
		t.Fatal(err)
	}
	original := filepath.Join(bed.bundle, filepath.FromSlash(target))
	changed := append(bytes.Repeat([]byte("a log line of a failing run\n"), 2*testMiB/28+1), []byte("--- FAIL: TestAppendedAfterTheCrash\n")...)
	if err := os.WriteFile(original, changed, 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := Distill(context.Background(), bed.bundle, bed.rules("01STAGEB"), testNow)
	if err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(original); !bytes.Equal(data, changed) {
		t.Fatal("the changed original is kept")
	}
	if _, lines, _, err := ReadDistilled(bed.bundle); err != nil || !strings.Contains(func() string { data, _ := json.Marshal(lines); return string(data) }(), target) {
		t.Fatalf("its published line is kept: %v", err)
	}
	if !strings.Contains(strings.Join(result.Kept, "\n"), "no longer matches its published line") {
		t.Fatalf("the restart reports it: %+v", result.Kept)
	}
}
