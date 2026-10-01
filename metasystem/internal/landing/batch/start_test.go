package batch

import (
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/board"
)

// TestWaitLineIsOneLineEverywhereAndNamesSeats (R23, U10b-2's half): one
// line, in local time, naming each waited unit's seat, stage, round of
// limit and expected time, the proof cost and its basis.
func TestWaitLineIsOneLineEverywhereAndNamesSeats(t *testing.T) {
	t.Parallel()
	three := 3
	record := Record{BatchID: "B", State: StateOpen, Wait: &WaitState{Reason: "r", ProofCost: 40 * time.Minute, Basis: "measured, n=8", For: []Waited{
		{Goal: "goal-x", Seat: "m1b", Stage: board.StageReview, Round: &board.Round{N: 2, Max: &three}, ExpectedAt: ten.Add(8 * time.Minute)},
		{Goal: "goal-y", Seat: "m1c", Stage: board.StageUnitProof, Proof: &board.Proof{Done: 120, Planned: 189}, ExpectedAt: ten.Add(6 * time.Minute)},
	}}}
	want := "batch B waits for goal-x on m1b (review round 2 of 3, ~8 min) and goal-y on m1c (unit proof 120 of 189, ~6 min); a separate proof costs ~40 min (measured, n=8)"
	if line := WaitLine(record, ten, time.UTC); line != want || strings.Contains(line, "\n") {
		t.Fatalf("wait line\n%q\nwant\n%q", line, want)
	}
	cest := time.FixedZone("CEST", 2*60*60)
	fallback := Record{BatchID: "B", State: StateOpen, Wait: &WaitState{Reason: "f", Fallback: "registry: gone", Until: ten}}
	if line := WaitLine(fallback, ten, cest); !strings.HasSuffix(line, "starts at the max wait, 12:00") {
		t.Fatalf("times are local: %q", line)
	}
	started := Record{BatchID: "B", State: StateProving, StartReason: "goal-y on m1c joined, the last unit waited for"}
	if line := WaitLine(started, ten, time.UTC); line != "batch B started: goal-y on m1c joined, the last unit waited for" {
		t.Fatalf("started line %q", line)
	}
}
