package evidence

// Round B2-4 witnesses, ported from the fourth read's probes
// (b2-read4-probes, each failing on aceab8e89).

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
)

// F-1: an export directory naming the root's agents directory through the
// macOS firmlink is refused; neither the item nor an export is lost.
func TestAnExportDirectoryInTheRootByTheFirmlinkIsRefused(t *testing.T) {
	t.Parallel()
	bed := newPersonBed(t)
	item := bed.chain(t, "only-copy", 300, 50, "g-done")
	agents := filepath.Join(bed.root, "agents")
	firm := filepath.Join("/System/Volumes/Data", agents)
	a, errA := os.Stat(agents)
	b, errB := os.Stat(firm)
	if errA != nil || errB != nil || !os.SameFile(a, b) {
		t.Skip("no firmlink spelling on this volume")
	}
	if _, refusal := bed.env.ExportDirFor(firm, nil); refusal == "" || !strings.Contains(refusal, "outside every evidence root") {
		t.Fatalf("the firmlink spelling of the root is refused: %q", refusal)
	}
	if gone(item) {
		t.Fatal("nothing was removed")
	}
}

// F-1(b): a removal whose export does not hold the content being removed
// is refused (content written after the export).
func TestDisposeNeverRemovesContentItsExportDidNotHold(t *testing.T) {
	t.Parallel()
	bed := newPersonBed(t)
	item := bed.chain(t, "grows", 300, 50, "g-done")
	exported, err := diskstore.Export(context.Background(), diskstore.ExportRequest{Item: item, Dir: bed.export, Segment: bed.segment.Git, Kind: diskstore.KindChain,
		Blobs: bed.env.Blobs, Now: boundNow, Stage: "s1"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(item, "late.log"), []byte("written after the export"), 0o644); err != nil {
		t.Fatal(err)
	}
	id, _ := diskstore.NewReceiptID(boundNow, strings.NewReader(strings.Repeat("z", 64)))
	_, err = diskstore.Dispose(context.Background(), diskstore.DisposalStep{Item: item, Ledger: bed.segment.Ledger(), Stage: "s2", PlannedDigest: exported.InventoryDigest,
		Receipt: diskstore.DisposalReceipt{ID: id, Item: "grows", Kind: diskstore.KindChain, Rule: diskstore.RulePerson, Export: exported.Ref(bed.export)}})
	if err == nil || gone(item) {
		t.Fatalf("content the export never held is kept: %v", err)
	}
}
