package batch

import (
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/board"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
)

var ten = time.Date(2026, 9, 29, 10, 0, 0, 0, time.UTC)

func pipelineCard(goal, seat string, stage board.Stage, since time.Time) board.Card {
	return board.Card{Seat: board.Seat{Machine: seat, Installation: "/c/" + seat + "/metasystem"}, Goal: goal, Stage: stage, Since: since, LastProgressAt: since}
}

func readyOf(t *testing.T, card board.Card, records []Record, now time.Time) Underway {
	t.Helper()
	facts := Facts(BoardPicture{Cards: []board.Card{card}, Readable: true}, records, config.DefaultPipelineSettings(), "host", now)
	if len(facts.Underway) != 1 {
		t.Fatalf("card %+v is not underway: %+v", card, facts)
	}
	return facts.Underway[0]
}

// spanRecord is a retained batch record whose one unit carries spans of
// stage with the given lengths, the newest last.
func spanRecord(stage board.Stage, ends time.Time, lengths ...time.Duration) Record {
	unit := Unit{GoalID: "g"}
	for index, length := range lengths {
		until := ends.Add(time.Duration(index) * time.Hour)
		unit.Stages = append(unit.Stages, board.StageSpan{Stage: stage, Since: until.Add(-length), Until: until})
	}
	return Record{Units: []Unit{unit}}
}

// provedRecord is a retained record whose proof ran on runner for wall,
// ending at ended.
func provedRecord(runner string, ended time.Time, wall time.Duration) Record {
	record := Record{Proof: &Proof{Runner: runner, Status: "green"}}
	record.History = []HistoryEntry{
		{At: ended.Add(-wall).Format(time.RFC3339Nano), Verb: "prove", Detail: "planned"},
		{At: ended.Format(time.RFC3339Nano), Verb: "prove", Detail: "green"},
	}
	return record
}

// TestExpectedReadyComesFromStageAndMarkers (R22, U10b-1): the expected join
// is the current stage's remainder, from a structural marker where one
// exists, clamped to [0, estimate], plus every following stage of the
// pre-join order, never in the past; estimates are the medians of the newest
// n recorded spans, else the defaults; the proof cost is the median of the
// newest n same-runner proofs.
func TestExpectedReadyComesFromStageAndMarkers(t *testing.T) {
	t.Parallel()
	// A review at round 2 of 3 since 10:00: 18 + judgement 5 + land-ready 3.
	three := 3
	review := pipelineCard("goal-x", "m1b", board.StageReview, ten)
	review.Round = &board.Round{N: 2, Max: &three}
	if got := readyOf(t, review, nil, ten); !got.ExpectedReady.Equal(ten.Add(26*time.Minute)) || got.Basis != "default" {
		t.Fatalf("review expected %v (%s), want 10:26 from the defaults", got.ExpectedReady, got.Basis)
	}
	// A unit proof with 120 of 189 sections ended: 5 min x 69/189 left, then
	// review, judgement and land-ready.
	proof := pipelineCard("goal-y", "m1c", board.StageUnitProof, ten.Add(-time.Hour))
	proof.Proof = &board.Proof{Attempt: "a", Done: 120, Planned: 189}
	want := ten.Add(proportion(5*time.Minute, 69, 189) + 26*time.Minute)
	if got := readyOf(t, proof, nil, ten); !got.ExpectedReady.Equal(want) {
		t.Fatalf("unit proof expected %v, want %v", got.ExpectedReady, want)
	}
	// A job still in its handshake has its whole estimate left.
	handshake := pipelineCard("goal-h", "m1b", board.StageBuild, ten.Add(-30*time.Minute))
	handshake.Job = &board.Job{ID: "j", Phase: "handshake"}
	if got := readyOf(t, handshake, nil, ten); !got.ExpectedReady.Equal(ten.Add((10 + 5 + 18 + 5 + 3) * time.Minute)) {
		t.Fatalf("handshake expected %v", got.ExpectedReady)
	}
	// A build since 10:00 read at 10:30 with estimate 10 has zero left, not
	// -20, and is expected after the following stages, not in the past.
	build := pipelineCard("goal-b", "m1b", board.StageBuild, ten)
	now := ten.Add(30 * time.Minute)
	if got := readyOf(t, build, nil, now); !got.ExpectedReady.Equal(now.Add((5 + 18 + 5 + 3) * time.Minute)) {
		t.Fatalf("overrun build expected %v, want its remainder clamped at zero", got.ExpectedReady)
	}
	landReady := pipelineCard("goal-l", "m1b", board.StageLandReady, ten)
	if got := readyOf(t, landReady, nil, now); !got.ExpectedReady.Equal(now) {
		t.Fatalf("an overrun last stage is expected now, never in the past: %v", got.ExpectedReady)
	}
	// A judgement is in the order: a review is followed by it.
	judgement := pipelineCard("goal-j", "m1b", board.StageJudgement, ten)
	if got := readyOf(t, judgement, nil, ten); !got.ExpectedReady.Equal(ten.Add(8 * time.Minute)) {
		t.Fatalf("judgement expected %v", got.ExpectedReady)
	}
	// Nine retained records with review spans of 5..90 min, the oldest 45:
	// the median of the newest eight (5..35 and 90) is 22.5 min, where
	// their mean is 28.75 and a median over all nine 25.
	var records []Record
	for index, minutes := range []int{45, 5, 10, 15, 20, 25, 30, 35, 90} {
		records = append(records, spanRecord(board.StageReview, ten.Add(time.Duration(index)*24*time.Hour), time.Duration(minutes)*time.Minute))
	}
	if got := readyOf(t, review, records, ten); !got.ExpectedReady.Equal(ten.Add(22*time.Minute+30*time.Second+8*time.Minute)) || got.Basis != "measured, n=8" {
		t.Fatalf("measured review expected %v (%s)", got.ExpectedReady, got.Basis)
	}
	// The proof cost: the newest eight host proofs; a proof without a runner
	// and one on the VM are not counted.
	var proofs []Record
	for index := range 10 {
		proofs = append(proofs, provedRecord("host", ten.Add(time.Duration(index)*time.Hour), time.Duration(20+index)*time.Minute))
	}
	proofs = append(proofs, provedRecord("", ten.Add(20*time.Hour), time.Hour), provedRecord("vm", ten.Add(21*time.Hour), 2*time.Hour))
	facts := Facts(BoardPicture{Readable: true}, proofs, config.DefaultPipelineSettings(), "host", ten)
	if facts.ProofCost != 25*time.Minute+30*time.Second || facts.Basis != "measured, n=8" {
		t.Fatalf("proof cost %v (%s), want the median of the newest eight host proofs, 25m30s", facts.ProofCost, facts.Basis)
	}
	if none := Facts(BoardPicture{Readable: true}, nil, config.DefaultPipelineSettings(), "host", ten); none.ProofCost != 40*time.Minute || none.Basis != "default" {
		t.Fatalf("no measured proof: %v (%s)", none.ProofCost, none.Basis)
	}
}

// A proof card written from a journal that ended one section every minute
// for thirty minutes carries done 1 and its first end as progress; the lane
// estimates from that marker and the stall bound, never from the repeats.
func TestExpectedReadyTrustsTheDistinctCount(t *testing.T) {
	t.Parallel()
	stuck := pipelineCard("goal-s", "m1b", board.StageUnitProof, ten)
	stuck.Proof = &board.Proof{Attempt: "a", Done: 1, Planned: 189}
	stuck.LastProgressAt = ten.Add(time.Minute)
	got := readyOf(t, stuck, nil, ten.Add(30*time.Minute))
	left := proportion(5*time.Minute, 188, 189)
	if !got.ExpectedReady.Equal(ten.Add(30*time.Minute + left + 26*time.Minute)) {
		t.Fatalf("stuck proof expected %v", got.ExpectedReady)
	}
	if !got.LastProgress.Equal(ten.Add(time.Minute)) {
		t.Fatalf("last progress %v", got.LastProgress)
	}
}

func proportion(total time.Duration, part, whole int) time.Duration {
	return time.Duration(float64(total) * (1 - float64(whole-part)/float64(whole)))
}

// TestCompletedProofRecordsItsRunner (R22, U10b-1): the proof a dispatched
// run planned carries the runner that ran it once the run completes, so the
// lane's proof cost is measured per runner.
func TestCompletedProofRecordsItsRunner(t *testing.T) {
	t.Parallel()
	bed, _, _ := dispatchBed(t, StateSealed, testBatchID)
	bed.owner.launch = func(request Dispatch) error {
		return bed.store.Update(request.ID, func(record *Record) error {
			record.Proof = &Proof{Status: "green", Token: request.Token}
			return nil
		})
	}
	must(t, bed.owner.Tick(testBatchID))
	bed.owner.settle()
	record, err := bed.store.Load(testBatchID)
	must(t, err)
	if record.Proof == nil || record.Proof.Runner != "host" {
		t.Fatalf("proof = %+v, want the host runner stamped", record.Proof)
	}
}
