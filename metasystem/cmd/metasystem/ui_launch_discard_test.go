package main

import (
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/seat/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/lifecycle"
)

// The server's discarder marks the record in this checkout, and a second
// press answers the same mark.
func TestLaunchDiscarderMarksTheRecordInThisCheckout(t *testing.T) {
	t.Parallel()
	checkout := t.TempDir()
	const id = "01K5ZZZZZZZZZZZZZZZZZZZZZZ"
	if err := launch.Save(checkout, launch.Record{Launch: id, Machine: "m1f", Outcome: launch.OutcomeFailed}); err != nil {
		t.Fatal(err)
	}
	discard := launchDiscarder(lifecycle.Roots{Checkout: checkout})
	first, err := discard(id)
	if err != nil || first.DiscardedAt == nil {
		t.Fatalf("discard = %+v, %v", first, err)
	}
	second, err := discard(id)
	if err != nil || second.DiscardedAt == nil || *second.DiscardedAt != *first.DiscardedAt {
		t.Fatalf("second discard = %+v, %v", second, err)
	}
	read, err := launch.Load(checkout, id)
	if err != nil || read.DiscardedAt == nil {
		t.Fatalf("record on disk = %+v, %v", read, err)
	}
}
