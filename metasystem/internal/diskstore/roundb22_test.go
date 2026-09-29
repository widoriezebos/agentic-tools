package diskstore

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func agedRefs(t *testing.T, bed blobBed) {
	old := testNow.Add(-48 * time.Hour)
	_ = os.Chtimes(filepath.Join(bed.blobs.Dir, "refs", bed.digest), old, old)
}

// F-2 variant: a junk, non-JSON, non-dot entry in the refs directory.
func TestAJunkRefEntryHoldsTheBlob(t *testing.T) {
	t.Parallel()
	bed := newBlobBed(t, "20260901T000000Z-watchdog-a", "")
	os.WriteFile(filepath.Join(bed.blobs.Dir, "refs", bed.digest, "notes.txt"), []byte("not json"), 0o644)
	agedRefs(t, bed)
	result := bed.check(testNow, true).Run(context.Background())
	t.Logf("%+v", result)
	if !bed.blobPresent() || result.Pending == "" {
		t.Fatalf("junk ref entry: blob present=%v result=%+v", bed.blobPresent(), result)
	}
}

// F-2 variant: a dangling symlink in the refs directory.
func TestASymlinkRefEntryHoldsTheBlob(t *testing.T) {
	t.Parallel()
	bed := newBlobBed(t, "20260901T000000Z-watchdog-a", "")
	os.Symlink("/nonexistent/target", filepath.Join(bed.blobs.Dir, "refs", bed.digest, "linked"))
	agedRefs(t, bed)
	result := bed.check(testNow, true).Run(context.Background())
	t.Logf("%+v", result)
	if !bed.blobPresent() || result.Pending == "" {
		t.Fatalf("symlink ref entry: blob present=%v result=%+v", bed.blobPresent(), result)
	}
}

// F-2 variant: the ONLY reference is a dot-named file (all references
// skipped) -> is a blob whose recipe still names it kept?
func TestABlobWhoseOnlyReferenceIsDotNamedIsKept(t *testing.T) {
	t.Parallel()
	bed := newBlobBed(t, "20260901T000000Z-watchdog-a", "")
	refDir := filepath.Join(bed.blobs.Dir, "refs", bed.digest)
	entries, _ := os.ReadDir(refDir)
	for _, entry := range entries {
		os.Rename(filepath.Join(refDir, entry.Name()), filepath.Join(refDir, "."+entry.Name()))
	}
	agedRefs(t, bed)
	result := bed.check(testNow, true).Run(context.Background())
	t.Logf("%+v", result)
	if !bed.blobPresent() {
		t.Fatalf("DATA LOSS: blob swept while DISTILLED.txt of %s names it (its only reference was dot-named)", bed.bundle)
	}
}

// Fail-closed class: the bundle is present but its DISTILLED.txt cannot be
// read (a newer engine's schema). The check reads that as "bundle gone",
// marks the reference dangling, drops it past the age floor and sweeps
// the blob: the distilled original is lost.
func TestAPresentBundleWithAnUnreadableRecipeKeepsItsBlob(t *testing.T) {
	t.Parallel()
	bed := newBlobBed(t, "20260901T000000Z-watchdog-a", "")
	manifest := filepath.Join(bed.bundle, DistilledName)
	data, _ := os.ReadFile(manifest)
	os.WriteFile(manifest, []byte(strings.Replace(string(data), DistilledSchema, DistilledSchema+"-v2", 1)), 0o644)
	r1 := bed.check(testNow, true).Run(context.Background())
	r2 := bed.check(testNow.Add(91*24*time.Hour), true).Run(context.Background())
	r3 := bed.check(testNow.Add(93*24*time.Hour), true).Run(context.Background())
	t.Logf("r1=%+v\nr2=%+v\nr3=%+v", r1, r2, r3)
	if _, err := os.Stat(bed.bundle); err != nil {
		t.Fatal("setup: bundle must still be present")
	}
	if !bed.blobPresent() {
		t.Fatalf("DATA LOSS: the bundle %s is present (its DISTILLED.txt unreadable) and its blob %s was swept", bed.bundle, bed.digest[:12])
	}
}

// Same class for a person's export and removal: Inventory ignores an
// unreadable DISTILLED.txt, so the export carries no blob and "verifies".
func TestABundleWithAnUnreadableRecipeIsNotExported(t *testing.T) {
	t.Parallel()
	bed := newBlobBed(t, "20260901T000000Z-watchdog-a", "")
	manifest := filepath.Join(bed.bundle, DistilledName)
	data, _ := os.ReadFile(manifest)
	os.WriteFile(manifest, []byte(strings.Replace(string(data), DistilledSchema, DistilledSchema+"-v2", 1)), 0o644)
	files, err := Inventory(context.Background(), bed.bundle)
	originals := 0
	for _, file := range files {
		if file.Original != nil {
			originals++
		}
	}
	t.Logf("inventory err=%v files=%d originals=%d", err, len(files), originals)
	result, err := Export(context.Background(), ExportRequest{Item: bed.bundle, Dir: filepath.Join(bed.root, "exports"), Segment: "107e72c67539",
		Kind: KindBundle, Blobs: bed.blobs, Now: testNow, Stage: "01E"})
	t.Logf("export err=%v result=%+v", err, result)
	if err == nil {
		m, _ := readExportManifest(result.Manifest)
		t.Fatalf("EXPORT INCOMPLETE BUT VERIFIED: blobs in manifest=%d recipes=%d; the bundle's blob-backed original is not in the archive", len(m.Blobs), len(m.Recipes))
	}
}

// F-5 variant: a captured DIRECTORY named like a stage, nested, with
// content; distilled twice (a restart).
func TestANestedStageNamedCapturedDirSurvivesRestarts(t *testing.T) {
	t.Parallel()
	root := realDir(t)
	bundle := filepath.Join(root, "suite-failures", "20260901T000000Z-watchdog-y")
	captured := filepath.Join(bundle, "source-001-TestFoo", "cache.partial-01ABC", "deep.partial-x", "state.json")
	writeBedFile(t, captured, []byte("captured"))
	writeBedFile(t, filepath.Join(bundle, "source-001-TestFoo", "big.log.gz.partial-01OTHER"), []byte(strings.Repeat("q", 2*testMiB)))
	writeBedFile(t, filepath.Join(bundle, "copy-note.txt"), []byte("x\n"))
	blobs := BlobStore{Dir: filepath.Join(root, "home", "metasystem-evidence", ".blobs")}
	for _, stage := range []string{"01STAGEA", "01STAGEB"} {
		result, err := Distill(context.Background(), bundle, DistillRules{CompressAbove: testMiB, Blobs: blobs, Referrer: "seg-y",
			Installation: root, Segment: "107e72c67539", Stage: stage}, testNow)
		t.Logf("%s: %+v err=%v", stage, result, err)
	}
	if _, err := os.Stat(captured); err != nil {
		t.Fatalf("DATA LOSS: nested captured stage-named dir content removed")
	}
}

// F-12 variant: a ledger whose last line is torn (no trailing newline, as a
// crash or an older writer leaves it). The next append joins the torn line.
func TestATornLedgerTailRefusesTheAppend(t *testing.T) {
	t.Parallel()
	ledger := filepath.Join(realDir(t), "disposals", "107e72c67539.jsonl")
	os.MkdirAll(filepath.Dir(ledger), 0o755)
	os.WriteFile(ledger, []byte(`{"schema":"x","id":"01A","item":"a"}`+"\n"+`{"schema":"x","id":"01B","it`), 0o644)
	err := AppendReceipt(ledger, DisposalReceipt{ID: "01C", Item: "c"}, Syncer{})
	committed, readErr := ReceiptCommitted(ledger, "01C", "c")
	t.Logf("append err=%v committed=%v readErr=%v", err, committed, readErr)
	if err == nil && readErr != nil {
		t.Fatalf("POISONED: the append succeeded (the disposal proceeds past its commit point) but the ledger no longer reads: %v", readErr)
	}
	if err == nil || !strings.Contains(err.Error(), "truncate -s") || committed {
		t.Fatalf("a torn tail refuses the append and names the repair: %v", err)
	}
}

// F-9 variant: the export's directory chain was created by an earlier run
// that crashed before any sync; this run treats it as pre-existing and
// never syncs the entries of those directories in their parents.
func TestAnExportSyncsAPreexistingUnsyncedChain(t *testing.T) {
	t.Parallel()
	bed := newDistillBed(t)
	volume := realDir(t)
	dir := filepath.Join(volume, "backup", "exports")
	crashed := filepath.Join(dir, "107e72c67539", filepath.Base(bed.bundle))
	os.MkdirAll(crashed, 0o755) // an earlier export crashed after MkdirAll
	synced := map[string]bool{}
	request := exportRequest(bed, dir, "01E")
	request.Sync = Syncer{Dir: func(path string) error { synced[path] = true; return syncDirectory(path) }}
	result, err := Export(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	var missed []string
	for directory := filepath.Dir(result.Archive); directory != volume; directory = filepath.Dir(directory) {
		if !synced[filepath.Dir(directory)] {
			missed = append(missed, filepath.Dir(directory)+" (holds the entry of "+filepath.Base(directory)+")")
		}
	}
	t.Logf("synced=%v", synced)
	if len(missed) > 0 {
		t.Fatalf("NOT DURABLE: directories created by the crashed run were never synced into their parents: %v", missed)
	}
}

// Round B2-2, L-1: a rollback whose rename back fails keeps the tombstone,
// the only record of where the item was set aside.
func TestARollbackThatCannotRenameBackKeepsTheTombstone(t *testing.T) {
	t.Parallel()
	item, ledger := disposalBed(t, 3)
	step := removeStep(item, ledger, "01RECEIPTL1")
	step.interrupt = func(point string) bool { return point == "aside" }
	if _, err := Dispose(context.Background(), step); err == nil {
		t.Fatal("setup: the step stops after the set-aside")
	}
	parent := filepath.Dir(item)
	if err := os.Chmod(parent, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(parent, 0o755) })
	_, err := SettlePersonDisposal(context.Background(), item, ledger, Syncer{}, "01S")
	os.Chmod(parent, 0o755)
	if err == nil {
		t.Fatal("a rollback that cannot rename back is an error")
	}
	if _, statErr := os.Stat(RemovedTombstonePath(item)); statErr != nil {
		t.Fatalf("the tombstone is kept: %v", statErr)
	}
}
