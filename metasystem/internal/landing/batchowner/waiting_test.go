package batchowner

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
)

// A goal is in a landing batch while its unit is joined there; a goal no
// batch holds is not.
func TestGoalInBatchesReadsLiveMembership(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if goalInBatchesAt(root, "waiting") {
		t.Fatal("a lane with no batches holds a goal")
	}
	store := batch.NewStore(root, nil)
	if err := store.Create(batch.Record{Schema: 1, BatchID: "01j5x00000000000000000wb01", State: batch.StateOpen, Units: []batch.Unit{
		{GoalID: "waiting", Chain: "chain-a", Claim: batch.Claim{Machine: "seat", Lineage: "seat-lineage", Epoch: 1, Revision: 2, AccountingRevision: 1}, State: batch.UnitJoined},
	}}); err != nil {
		t.Fatal(err)
	}
	if !goalInBatchesAt(root, "waiting") || goalInBatchesAt(root, "absent") {
		t.Fatalf("membership: waiting=%v absent=%v", goalInBatchesAt(root, "waiting"), goalInBatchesAt(root, "absent"))
	}
}

// The cadence's re-arm moves the lane checkout (the module's parent, when
// the module nests) at the trunk commit while holding that checkout.
func TestRearmLaneAtTrunkHoldsTheLaneCheckout(t *testing.T) {
	t.Parallel()
	top := t.TempDir()
	module := filepath.Join(top, "metasystem")
	if err := os.MkdirAll(module, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(module, "go.mod"), []byte("module x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var got string
	held := false
	err := rearmLaneAtTrunkWith(module, "commit-1", func(root, tip string) error {
		got, held = root+"@"+tip, LaneCheckoutFor(root).Held()
		return nil
	})
	if err != nil || got != top+"@commit-1" || !held {
		t.Fatalf("re-arm at %q held=%v err=%v", got, held, err)
	}
}
