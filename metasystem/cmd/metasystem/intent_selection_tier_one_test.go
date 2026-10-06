package main

import (
	"slices"
	"testing"
)

// A tier-1 goal advances through its declared builds without a review.
func TestSelectionTierOneWorkAdvancesUntilFinished(t *testing.T) {
	t.Parallel()
	bed, inv, state := nextStepBed(t)
	nextStepDesign(bed, "u1", "u2")
	state.ReadsWaived = true
	state.Status.Units = state.Status.Units[:1]
	state.Status.Units[0].ReadState = ""
	item := manualWorkItem{Unit: "u1", Commit: "first", Goal: bed.id, ReadsWaived: true}
	next, reason := inv.manualContinuation(bed.id, item)
	if !slices.Equal(next, inv.publicArgv("work", "build", bed.id, "--work", "u2", "--brief", "FILE", "--check", "COMMAND")) || reason == "" {
		t.Fatalf("unfinished tier-1 goal: next=%q reason=%q", next, reason)
	}
	state.Status.Units = append(state.Status.Units, state.Status.Units[0])
	state.Status.Units[1].Unit = "u2"
	next, reason = inv.manualContinuation(bed.id, item)
	if !slices.Equal(next, inv.publicArgv("work", "land", bed.id)) || reason != "goal "+bed.id+" is finished: every unit it declares is built and read. This is its one hand-in; `goal done` follows when it has landed." {
		t.Fatalf("finished tier-1 goal: next=%q reason=%q", next, reason)
	}
}
