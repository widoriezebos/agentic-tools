package batch

import "testing"

// An entry whose check could not run is not a red on main: it never holds
// a landing, whatever its class.
func TestUnavailableEntryDoesNotHoldALanding(t *testing.T) {
	t.Parallel()
	if !(OpenEntry{ID: "tr-1", Group: "section/deep"}).HoldsLanding() {
		t.Fatal("a trunk red no longer holds a landing")
	}
	if (OpenEntry{ID: "tr-2", Group: "section/deep", Unavailable: true}).HoldsLanding() {
		t.Fatal("an unavailable check holds a landing as if main were red")
	}
}

// The lane's own record commits move main without touching what a batch
// proves: they never reopen it (and so never re-prove or re-arm it).
func TestMovedBaseIgnoresLaneRecordCommits(t *testing.T) {
	t.Parallel()
	record := Record{State: StateProving}
	records := []string{"metasystem/plans/goals/trunk-red.json", "metasystem/plans/goals/standing-validation.md"}
	if DecideMovedBase(record, records, "metasystem").Reopen {
		t.Fatal("a lane record move reopened a proving batch")
	}
	if !DecideMovedBase(record, append(records, "metasystem/README.md"), "metasystem").Reopen {
		t.Fatal("a move with a real change no longer reopens a proving batch without a proof")
	}
}
