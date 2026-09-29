package diskstore

// Round B2-4 witnesses, ported from the fourth read's probes
// (b2-read4-probes, each failing on aceab8e89).

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// firmlinkSpelling is path spelled through /System/Volumes/Data when that
// names the same directory (the macOS firmlink), else "".
func firmlinkSpelling(path string) string {
	firm := filepath.Join("/System/Volumes/Data", path)
	a, errA := os.Stat(path)
	b, errB := os.Stat(firm)
	if errA != nil || errB != nil || !os.SameFile(a, b) {
		return ""
	}
	return firm
}

// F-3: a member appended to after its stage verified and before its unlink
// is kept beside its replacement and reported; its published line stays.
func TestTheDistillerKeepsAMemberWrittenAfterItsStageVerified(t *testing.T) {
	t.Parallel()
	bed := newDistillBed(t)
	log := filepath.Join(bed.bundle, "source-003-log", "run.log")
	rules := bed.rules("01R4")
	rules.interrupt = func(step, path string) bool {
		if step == "published" && path == "source-003-log/run.log" {
			file, err := os.OpenFile(log, os.O_APPEND|os.O_WRONLY, 0)
			if err != nil {
				t.Fatal(err)
			}
			file.Write([]byte("LATE LINE written after the stage verified\n"))
			file.Close()
		}
		return false
	}
	result, err := Distill(context.Background(), bed.bundle, rules, testNow)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(log)
	if err != nil || !bytes.Contains(data, []byte("LATE LINE")) {
		t.Fatalf("the changed original is kept: %v", err)
	}
	if !strings.Contains(strings.Join(result.Kept, "\n"), "source-003-log/run.log: it changed after its replacement was verified") {
		t.Fatalf("it is reported: %+v", result.Kept)
	}
	if _, lines, _, err := ReadDistilled(bed.bundle); err != nil || !lineFor(lines, "source-003-log/run.log") {
		t.Fatalf("its published line stays with its replacement: %v", err)
	}
}

func lineFor(lines []RecipeLine, path string) bool {
	for _, line := range lines {
		if line.Path == path && line.Restores() {
			return true
		}
	}
	return false
}

// F-3: the move re-checks the source just before removing it: a source
// written to after its copy verified is kept, and so is the copy.
func TestTheMoveKeepsASourceWrittenAfterItsCopyVerified(t *testing.T) {
	t.Parallel()
	bed, rules := moveBed(t)
	late := filepath.Join(bed.bundle, "source-003-log", "late.txt")
	rules.interrupt = func(step string) bool {
		if step == "renamed" {
			writeBedFile(t, late, []byte("written after the copy verified"))
		}
		return false
	}
	result, err := MoveBundle(context.Background(), bed.bundle, rules)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(late); err != nil || !strings.Contains(result.Kept, "changed after its copy verified") {
		t.Fatalf("the source is kept and reported: %+v %v", result, err)
	}
	if _, err := os.Stat(result.Destination); err != nil {
		t.Fatalf("the copy is kept: %v", err)
	}
}

// F-4: reuse of an export compares the manifest's full inventory digest
// and item, never the path's twelve-digit prefix.
func TestExportReuseComparesTheManifestsWholeInventory(t *testing.T) {
	t.Parallel()
	dir := realDir(t)
	store := BlobStore{Dir: filepath.Join(dir, ".blobs")}
	item := filepath.Join(dir, "root", "agents", "107e72c67539", "item")
	writeBedFile(t, filepath.Join(item, "one.log"), []byte("content A"))
	exportDir := filepath.Join(dir, "exports")
	first, err := Export(context.Background(), ExportRequest{Item: item, Dir: exportDir, Segment: "107e72c67539", Kind: KindChain, Blobs: store, Now: testNow, Stage: "s1"})
	if err != nil {
		t.Fatal(err)
	}
	writeBedFile(t, filepath.Join(item, "one.log"), []byte("content B, never exported"))
	files, _ := InventoryComplete(context.Background(), item)
	archive, manifest := ExportPaths(exportDir, "107e72c67539", "item", InventoryDigest(files))
	data, _ := os.ReadFile(first.Archive)
	writeBedFile(t, archive, data)
	data, _ = os.ReadFile(first.Manifest)
	writeBedFile(t, manifest, data)
	second, err := Export(context.Background(), ExportRequest{Item: item, Dir: exportDir, Segment: "107e72c67539", Kind: KindChain, Blobs: store, Now: testNow, Stage: "s2"})
	if err != nil || second.Already || second.InventoryDigest != InventoryDigest(files) {
		t.Fatalf("content B is exported afresh, never reused from A's manifest: %+v %v", second, err)
	}
}

// F-1(b): a removal with an export is refused unless the export's
// inventory digest equals the planned one and the one taken for the
// removal.
func TestARemovalIsRefusedUnlessItsExportHoldsWhatIsRemoved(t *testing.T) {
	t.Parallel()
	item, ledger := disposalBed(t, 3)
	exported, err := Export(context.Background(), ExportRequest{Item: item, Dir: filepath.Join(filepath.Dir(ledger), "..", "..", "exports"), Segment: "107e72c67539",
		Kind: KindChain, Now: testNow, Stage: "01E"})
	if err != nil {
		t.Fatal(err)
	}
	planned := exported.InventoryDigest
	writeBedFile(t, filepath.Join(item, "late.log"), []byte("written after the export"))
	step := removeStep(item, ledger, "01RECEIPT0000000000000000A")
	step.Receipt.Export, step.PlannedDigest = exported.Ref(filepath.Dir(exported.Archive)), planned
	if _, err := Dispose(context.Background(), step); err == nil || !strings.Contains(err.Error(), "does not hold what would be removed") {
		t.Fatalf("content the export never held is refused: %v", err)
	}
	if _, err := os.Stat(filepath.Join(item, "late.log")); err != nil {
		t.Fatal("nothing was removed")
	}
	if receipts, _ := ReadReceipts(ledger); len(receipts) != 0 {
		t.Fatalf("no receipt: %+v", receipts)
	}
	os.Remove(filepath.Join(item, "late.log"))
	step = removeStep(item, ledger, "01RECEIPT0000000000000000B")
	step.Receipt.Export, step.PlannedDigest = exported.Ref(filepath.Dir(exported.Archive)), "another-plan"
	if _, err := Dispose(context.Background(), step); err == nil || !strings.Contains(err.Error(), "does not hold what would be removed") {
		t.Fatalf("an export of content other than the planned is refused: %v", err)
	}
	step = removeStep(item, ledger, "01RECEIPT0000000000000000C")
	step.Receipt.Export, step.PlannedDigest = exported.Ref(filepath.Dir(exported.Archive)), planned
	if _, err := Dispose(context.Background(), step); err != nil {
		t.Fatalf("the export that holds the planned content lets it go: %v", err)
	}
}

// F-1(a): an export directory is judged by file identity: a spelling of
// an evidence root through a symlink or the firmlink is inside it.
func TestAnExportDirectoryIsJudgedByFileIdentity(t *testing.T) {
	t.Parallel()
	dir := realDir(t)
	root := filepath.Join(dir, "evidence")
	writeBedFile(t, filepath.Join(root, "agents", "107e72c67539", "item", "one.log"), []byte("x"))
	link := filepath.Join(dir, "elsewhere")
	if err := os.Symlink(filepath.Join(root, "agents"), link); err != nil {
		t.Fatal(err)
	}
	spellings := []string{filepath.Join(link, "107e72c67539", "item", "exports-not-yet-made")}
	if firm := firmlinkSpelling(filepath.Join(root, "agents")); firm != "" {
		spellings = append(spellings, filepath.Join(firm, "exports"))
	}
	for _, spelling := range spellings {
		if problem := ExportDirProblem(spelling, []string{root}, nil, nil); problem == "" {
			t.Fatalf("%s is inside the evidence root", spelling)
		}
	}
	if problem := ExportDirProblem(filepath.Join(dir, "backup", "exports"), []string{root}, nil, nil); problem != "" {
		t.Fatalf("a directory outside is accepted: %s", problem)
	}
}

// F-1(c): an export whose archive would lie inside the item is refused,
// whatever the spelling of the directory.
func TestAnExportNeverLiesInsideItsItem(t *testing.T) {
	t.Parallel()
	dir := realDir(t)
	item := filepath.Join(dir, "evidence", "agents", "107e72c67539", "item")
	writeBedFile(t, filepath.Join(item, "one.log"), []byte("x"))
	link := filepath.Join(dir, "into-item")
	if err := os.Symlink(item, link); err != nil {
		t.Fatal(err)
	}
	_, err := Export(context.Background(), ExportRequest{Item: item, Dir: link, Segment: "107e72c67539", Kind: KindChain, Now: testNow, Stage: "01E"})
	if err == nil || !strings.Contains(err.Error(), "inside the item") {
		t.Fatalf("an export into the item is refused: %v", err)
	}
	entries, _ := os.ReadDir(item)
	if len(entries) != 1 {
		t.Fatalf("nothing is written into the item: %v", entries)
	}
}
