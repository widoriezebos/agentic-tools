package steward

import (
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/census"
)

func TestOwnedLiveProcessesCountAsLive(t *testing.T) {
	v := census.Verdict{Verdict: "SUCCESS", Inventory: []census.InventoryItem{
		{Class: "CUSTODY"}, {Class: "ANNOUNCED"},
	}}
	w := workersFromVerdict(v)
	if w.Live != 2 || !w.CensusComplete || w.Untracked != 0 {
		t.Fatalf("custody and announced are live under a complete scan: %+v", w)
	}
}

func TestUntrackedProcessBlocksADeathProof(t *testing.T) {
	v := census.Verdict{Verdict: "SUCCESS", Inventory: []census.InventoryItem{{Class: "UNTRACKED"}}}
	w := workersFromVerdict(v)
	if w.Untracked != 1 || w.Live != 0 {
		t.Fatalf("an unaccounted live process must block the proof: %+v", w)
	}
}

func TestFailedCensusIsAnIncompleteScan(t *testing.T) {
	v := census.Verdict{Verdict: "CENSUS-FAILED"}
	w := workersFromVerdict(v)
	if w.CensusComplete {
		t.Fatalf("a failed census proves nothing: %+v", w)
	}
}

func TestUnknownInventoryClassIsUnprovable(t *testing.T) {
	v := census.Verdict{Verdict: "SUCCESS", Inventory: []census.InventoryItem{{Class: "SOMETHING-NEW"}}}
	w := workersFromVerdict(v)
	if w.Unprovable != 1 {
		t.Fatalf("an unrecognized class must not silently vanish: %+v", w)
	}
}
