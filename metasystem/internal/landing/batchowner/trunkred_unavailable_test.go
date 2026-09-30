package batchowner

import (
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

func TestLedgerOpenEntryMarksAnUnavailableCheck(t *testing.T) {
	t.Parallel()
	unavailable := goal.TrunkRedEntry{ID: "tr-1", Group: "section/deep", Status: "unavailable", Failures: []goal.TrunkRedFailure{}}
	red := goal.TrunkRedEntry{ID: "tr-2", Group: "section/deep", Status: "failed", Failures: []goal.TrunkRedFailure{{Name: "deep"}}}
	if entry := openEntryFrom(unavailable); !entry.Unavailable || entry.HoldsLanding() {
		t.Fatalf("unavailable check read as %+v", entry)
	}
	if entry := openEntryFrom(red); entry.Unavailable || !entry.HoldsLanding() {
		t.Fatalf("red read as %+v", entry)
	}
}
