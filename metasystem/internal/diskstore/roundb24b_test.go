package diskstore

// Round B2-4 follow-up witnesses (the coordinator's gap and read-4 D3).

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// A source that changes after it was set aside is kept with its own blob
// references; the copy has its own too. A person's removal of the copy
// then leaves the blob check keeping every blob the source names.
func TestAKeptMoveSourceKeepsItsBlobReferencesThroughTheCopysRemoval(t *testing.T) {
	t.Parallel()
	bed, rules := moveBed(t)
	_, lines, _, err := ReadDistilled(bed.bundle)
	if err != nil {
		t.Fatal(err)
	}
	var digests []string
	for _, line := range lines {
		if line.Kind == RecipeBlob {
			digests = append(digests, line.SHA256)
		}
	}
	if len(digests) == 0 {
		t.Fatal("setup: the bundle names a blob")
	}
	rules.interrupt = func(step string) bool {
		if step == "set-aside" {
			writeBedFile(t, filepath.Join(MovedSourceName(bed.bundle, rules.Stage), "late.txt"), []byte("written after the set-aside"))
		}
		return false
	}
	result, err := MoveBundle(context.Background(), bed.bundle, rules)
	if err != nil || result.Kept == "" {
		t.Fatalf("the changed source is kept: %+v %v", result, err)
	}
	if _, err := os.Stat(filepath.Join(bed.bundle, "late.txt")); err != nil {
		t.Fatalf("the source is back at its path: %v", err)
	}
	ledger := filepath.Join(filepath.Dir(filepath.Dir(rules.SegmentDir)), "disposals", "107e72c67539.jsonl")
	step := DisposalStep{Item: result.Destination, Ledger: ledger, Stage: "01D",
		Receipt: DisposalReceipt{ID: "01RECEIPTCOPY0000000000000", At: testNow, Segment: "107e72c67539", Kind: KindBundle, Item: filepath.Base(result.Destination), Rule: RulePerson, By: "wido"}}
	if _, err := Dispose(context.Background(), step); err != nil {
		t.Fatal(err)
	}
	check := BlobCheck{Blobs: bed.blobs, Now: testNow.Add(400 * 24 * time.Hour), Grace: time.Hour, AgeFloor: time.Hour, By: "steward",
		Armed: func(string) bool { return true }}
	check.Run(context.Background())
	check.Now = check.Now.Add(48 * time.Hour)
	check.Run(context.Background())
	for _, digest := range digests {
		if _, err := os.Stat(bed.blobs.Path(digest)); err != nil {
			t.Fatalf("the blob %s the kept source names survives: %v", digest[:12], err)
		}
		refs, _ := bed.blobs.Refs(digest)
		found := false
		for _, ref := range refs {
			found = found || ref.Recipe == filepath.Join(bed.bundle, DistilledName)
		}
		if !found {
			t.Fatalf("the kept source's reference to %s stands: %+v", digest[:12], refs)
		}
	}
}

// Read-4 D3: the move never creates an absent evidence root (an
// unmounted volume's mount point); it holds and says so.
func TestTheMoveNeverCreatesAnAbsentEvidenceRoot(t *testing.T) {
	t.Parallel()
	dir := realDir(t)
	root := filepath.Join(dir, "Volumes", "Backup", "metasystem-evidence", "agentic-tools")
	if err := os.MkdirAll(filepath.Join(dir, "Volumes", "Backup"), 0o755); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(dir, "checkout", "suite-failures", "20260101T000000Z-detached-x-1")
	writeBedFile(t, filepath.Join(source, "run.log"), []byte("evidence"))
	result, err := MoveBundle(context.Background(), source, MoveRules{SegmentDir: filepath.Join(root, "suite-failures", "107e72c67539"),
		Blobs: BlobStore{Dir: filepath.Join(dir, ".blobs")}, Referrer: "r", Stage: "s"})
	if err == nil || result.Moved || !strings.Contains(err.Error(), "does not exist") {
		t.Fatalf("an absent evidence root holds the move: %+v %v", result, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "Volumes", "Backup", "metasystem-evidence")); !os.IsNotExist(err) {
		t.Fatalf("nothing is created under the mount point: %v", err)
	}
	if _, err := os.Stat(filepath.Join(source, "run.log")); err != nil {
		t.Fatalf("the source stays: %v", err)
	}
}
