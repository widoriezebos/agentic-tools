package diskstore

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Round B2, F-2: a stray non-reference entry (Finder's .DS_Store) in a
// blob's refs directory is not a reference and never makes the sweep see
// zero references for a blob a recipe still depends on.
func TestADotFileInARefsDirectoryNeverSweepsAReferencedBlob(t *testing.T) {
	t.Parallel()
	bed := newBlobBed(t, "20260901T000000Z-watchdog-a", "")
	if refs, err := bed.blobs.Refs(bed.digest); err != nil || len(refs) != 1 {
		t.Fatalf("setup: %v %v", refs, err)
	}
	stray := filepath.Join(bed.blobs.Dir, "refs", bed.digest, ".DS_Store")
	if err := os.WriteFile(stray, []byte{0, 0, 0, 1, 'B', 'u', 'd', '1'}, 0o644); err != nil {
		t.Fatal(err)
	}
	// Age the refs directory past the grace (as 25 h of wall time would).
	old := testNow.Add(-48 * time.Hour)
	_ = os.Chtimes(filepath.Join(bed.blobs.Dir, "refs", bed.digest), old, old)
	result := bed.check(testNow, true).Run(context.Background())
	t.Logf("result: %+v", result)
	if !bed.blobPresent() {
		t.Fatalf("DATA LOSS: the blob %s was swept while DISTILLED.txt at %s still names it (the original was unlinked by distillation)", bed.digest[:12], bed.bundle)
	}
}

// Round B2, F-5: a bundle captured from a failing test's temp directory
// may hold a file named like the engine's stages; the distiller removes
// only the stages its manifest lists, never captured content.
func TestDistillNeverDeletesACapturedPartialNamedFile(t *testing.T) {
	t.Parallel()
	root := realDir(t)
	bundle := filepath.Join(root, "suite-failures", "20260901T000000Z-watchdog-x")
	captured := filepath.Join(bundle, "source-001-TestFoo", "state.json.partial-01J9ABC")
	writeBedFile(t, captured, []byte("the half-written state the failing test left behind"))
	writeBedFile(t, filepath.Join(bundle, "copy-note.txt"), []byte("copied-bytes=10\n"))
	blobs := BlobStore{Dir: filepath.Join(root, "home", "metasystem-evidence", ".blobs")}
	result, err := Distill(context.Background(), bundle, DistillRules{CompressAbove: testMiB, Blobs: blobs, Referrer: "seg-x",
		Installation: root, Segment: "107e72c67539", Stage: "01STAGE"}, testNow)
	t.Logf("result: %+v err: %v", result, err)
	if _, statErr := os.Stat(captured); statErr != nil {
		t.Fatalf("DATA LOSS: the distiller removed captured evidence %s (discarded=%v)", captured, result.Discarded)
	}
}

// A refs directory that cannot be read keeps its blob and stops the sweep
// for the pass (Round B2, F-2: fail closed).
func TestUnreadableReferencesKeepTheBlobAndStopTheSweep(t *testing.T) {
	t.Parallel()
	bed := newBlobBed(t, "20260901T000000Z-watchdog-a", "")
	refDir := filepath.Join(bed.blobs.Dir, "refs", bed.digest)
	if err := os.Chmod(refDir, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(refDir, 0o755) })
	old := testNow.Add(-48 * time.Hour)
	_ = os.Chtimes(refDir, old, old)
	result := bed.check(testNow, true).Run(context.Background())
	os.Chmod(refDir, 0o755)
	if !bed.blobPresent() || result.Pending == "" || len(result.Unreadable) == 0 || len(result.Removed) != 0 {
		t.Fatalf("an unreadable refs directory keeps the blob and sweeps nothing: %+v", result)
	}
}

// Round B2, F-9: the export's barrier syncs every directory it created and
// the first one that existed before it, whose entry names the new chain.
func TestAnExportSyncsEveryDirectoryItCreatedAndTheirParent(t *testing.T) {
	t.Parallel()
	bed := newDistillBed(t)
	volume := realDir(t)
	dir := filepath.Join(volume, "backup", "exports")
	synced := map[string]bool{}
	request := exportRequest(bed, dir, "01E")
	request.Sync = Syncer{Dir: func(path string) error { synced[path] = true; return syncDirectory(path) }}
	result, err := Export(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	for directory := filepath.Dir(result.Archive); ; directory = filepath.Dir(directory) {
		if !synced[directory] {
			t.Fatalf("%s was not synced (synced: %v)", directory, synced)
		}
		if directory == volume {
			break
		}
	}
}

// Round B2, F-12: an append whose sync fails leaves the ledger as it was.
func TestAFailedReceiptAppendLeavesTheLedgerAsItWas(t *testing.T) {
	t.Parallel()
	ledger := filepath.Join(realDir(t), "disposals", "107e72c67539.jsonl")
	if err := AppendReceipt(ledger, DisposalReceipt{ID: "01FIRST", Item: "a"}, Syncer{}); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(ledger)
	failing := Syncer{File: func(*os.File) error { return os.ErrInvalid }}
	if err := AppendReceipt(ledger, DisposalReceipt{ID: "01SECOND", Item: "b"}, failing); err == nil {
		t.Fatal("the failed sync is an error")
	}
	after, _ := os.ReadFile(ledger)
	if string(after) != string(before) {
		t.Fatalf("the failed append is truncated away:\n%s", after)
	}
	if committed, _ := ReceiptCommitted(ledger, "01SECOND", "b"); committed {
		t.Fatal("an unsynced receipt is never committed")
	}
}
