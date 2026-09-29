package batch

import (
	"fmt"
	"slices"
	"sort"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/board"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
)

// The facts a batch's start is decided from (batch-lane design D14, R22):
// what every seat of this host is working on and how close it is, read from
// the host board's cards, and what a separate proof costs, from the lane's
// own retained records.

// BoardPicture is what a pipeline source reads from the host: the cards it
// believes and the ones it cannot, classified by board.Classify and by the
// ledger checks only the source can make. Readable is false, with Reason,
// when the host registry cannot be read: the only case in which the lane
// falls back to its max wait.
type BoardPicture struct {
	Cards    []board.Card
	Unknown  []board.Unknown
	Readable bool
	Reason   string
}

// PipelineSource reads the host board afresh at every decision.
type PipelineSource interface {
	Board(now time.Time) BoardPicture
}

// Underway is one unit on a seat of this host in the pre-join order, with
// the time it is expected to join.
type Underway struct {
	Goal          string
	Seat          string
	Stage         board.Stage
	Round         *board.Round
	Proof         *board.Proof
	Since         time.Time
	LastProgress  time.Time
	ExpectedReady time.Time
	// Basis says whether the current stage's estimate is measured by the
	// lane (measured, n=K) or the compiled default.
	Basis string
}

// NotNear is a unit the lane never waits for, named with why.
type NotNear struct {
	Goal   string
	Seat   string
	Reason string
	// At is the stamp the reason refers to: a stalled unit's last real
	// progress.
	At time.Time
}

// PipelineFacts are everything the start decision reads.
type PipelineFacts struct {
	Underway  []Underway
	Unknown   []NotNear
	ProofCost time.Duration
	// Basis names where the proof cost came from: measured, n=K, or default.
	Basis    string
	Readable bool
	Reason   string
}

// The stage order a unit follows to its join; a revise is followed by the
// same stages as a build.
var preJoinOrder = []board.Stage{board.StageBuild, board.StageUnitProof, board.StageReview, board.StageJudgement, board.StageLandReady}

// stagesAfter are the stages a unit at stage still has to pass before it
// joins, current first; false when the stage is not in the pre-join order.
func stagesAfter(stage board.Stage) ([]board.Stage, bool) {
	if stage == board.StageRevise {
		return append([]board.Stage{board.StageRevise}, preJoinOrder[1:]...), true
	}
	index := slices.Index(preJoinOrder, stage)
	if index < 0 {
		return nil, false
	}
	return preJoinOrder[index:], true
}

// stageEstimates are the lane's measured medians where it has spans of a
// stage, else the compiled default, with the basis of each.
type stageEstimates struct {
	value map[board.Stage]time.Duration
	basis map[board.Stage]string
}

// estimateStages takes, per stage, the median of the newest n completed
// spans the retained batch records hold.
func estimateStages(records []Record, settings config.PipelineSettings) stageEstimates {
	spans := map[board.Stage][]board.StageSpan{}
	for _, record := range records {
		for _, unit := range record.Units {
			for _, span := range unit.Stages {
				if span.Until.After(span.Since) {
					spans[span.Stage] = append(spans[span.Stage], span)
				}
			}
		}
	}
	estimates := stageEstimates{value: map[board.Stage]time.Duration{}, basis: map[board.Stage]string{}}
	for _, name := range config.PipelineStages {
		stage := board.Stage(name)
		recorded := spans[stage]
		if len(recorded) == 0 {
			estimates.value[stage], estimates.basis[stage] = settings.StageDefaults[name], "default"
			continue
		}
		sort.Slice(recorded, func(i, j int) bool { return recorded[i].Until.After(recorded[j].Until) })
		recorded = recorded[:min(len(recorded), max(settings.HistoryN, 1))]
		durations := make([]time.Duration, len(recorded))
		for index, span := range recorded {
			durations[index] = span.Until.Sub(span.Since)
		}
		estimates.value[stage], estimates.basis[stage] = median(durations), fmt.Sprintf("measured, n=%d", len(durations))
	}
	return estimates
}

// proofCost is the median wall time of the newest n batch proofs that ran
// on runner, from each retained record's planned-to-verdict history pair; a
// proof without a runner counts for none.
func proofCost(records []Record, runner string, settings config.PipelineSettings) (time.Duration, string) {
	type proof struct {
		ended time.Time
		wall  time.Duration
	}
	var proofs []proof
	for _, record := range records {
		if record.Proof == nil || record.Proof.Runner != runner || runner == "" {
			continue
		}
		var planned time.Time
		for _, entry := range record.History {
			if entry.Verb != "prove" {
				continue
			}
			at, err := time.Parse(time.RFC3339Nano, entry.At)
			if err != nil {
				continue
			}
			switch entry.Detail {
			case "planned":
				planned = at
			case "green", "red":
				if !planned.IsZero() && at.After(planned) {
					proofs = append(proofs, proof{ended: at, wall: at.Sub(planned)})
				}
				planned = time.Time{}
			}
		}
	}
	if len(proofs) == 0 {
		return settings.ProofCost, "default"
	}
	sort.Slice(proofs, func(i, j int) bool { return proofs[i].ended.After(proofs[j].ended) })
	proofs = proofs[:min(len(proofs), max(settings.HistoryN, 1))]
	walls := make([]time.Duration, len(proofs))
	for index, proof := range proofs {
		walls[index] = proof.wall
	}
	return median(walls), fmt.Sprintf("measured, n=%d", len(walls))
}

// median is the middle value, or the mean of the two middle values.
func median(values []time.Duration) time.Duration {
	sorted := slices.Clone(values)
	slices.Sort(sorted)
	middle := len(sorted) / 2
	if len(sorted)%2 == 1 {
		return sorted[middle]
	}
	return (sorted[middle-1] + sorted[middle]) / 2
}

// expectedReady is when a unit is expected to join: the current stage's
// remainder from its structural marker when it has one (a proof's sections
// done of planned), else from the time it has run, clamped to the stage's
// estimate and never below zero, plus the estimates of every following
// stage; never earlier than now.
func expectedReady(card board.Card, estimates stageEstimates, now time.Time) (time.Time, bool) {
	stages, ok := stagesAfter(card.Stage)
	if !ok {
		return time.Time{}, false
	}
	current := estimates.value[card.Stage]
	var remaining time.Duration
	switch {
	case card.Proof != nil && card.Proof.Planned > 0:
		done := min(max(card.Proof.Done, 0), card.Proof.Planned)
		remaining = time.Duration(float64(current) * (1 - float64(done)/float64(card.Proof.Planned)))
	case card.Job != nil && (card.Job.Phase == "reservation" || card.Job.Phase == "handshake"):
		remaining = current
	default:
		remaining = current - now.Sub(card.Since)
	}
	remaining = min(max(remaining, 0), current)
	ready := now.Add(remaining)
	for _, following := range stages[1:] {
		ready = ready.Add(estimates.value[following])
	}
	if ready.Before(now) {
		ready = now
	}
	return ready, true
}

// Facts turns a board picture and the lane's retained records into the
// facts a start is decided from: every believed card in the pre-join order
// with its expected join, every card the lane cannot believe with its
// reason, and the cost of a separate proof on runner.
func Facts(picture BoardPicture, records []Record, settings config.PipelineSettings, runner string, now time.Time) PipelineFacts {
	facts := PipelineFacts{Readable: picture.Readable, Reason: picture.Reason}
	facts.ProofCost, facts.Basis = proofCost(records, runner, settings)
	estimates := estimateStages(records, settings)
	for _, card := range picture.Cards {
		ready, inOrder := expectedReady(card, estimates, now)
		if !inOrder {
			continue
		}
		facts.Underway = append(facts.Underway, Underway{Goal: card.Goal, Seat: card.Seat.Machine, Stage: card.Stage, Round: card.Round,
			Proof: card.Proof, Since: card.Since, LastProgress: card.LastProgressAt, ExpectedReady: ready, Basis: estimates.basis[card.Stage]})
	}
	for _, unknown := range picture.Unknown {
		notNear := NotNear{Goal: unknown.Goal, Seat: unknown.Seat.Machine, Reason: unknown.Reason}
		if unknown.Card != nil {
			if unknown.Card.Stage.Terminal() || unknown.Card.Stage == board.StageLanding {
				continue
			}
			if unknown.Reason == board.ReasonStalled {
				notNear.At = unknown.Card.LastProgressAt
			}
		}
		facts.Unknown = append(facts.Unknown, notNear)
	}
	sort.SliceStable(facts.Underway, func(i, j int) bool { return facts.Underway[i].ExpectedReady.Before(facts.Underway[j].ExpectedReady) })
	return facts
}

// Reachable are the underway units expected to join no later than the
// oldest join plus the cost of a separate proof: the units already joined
// never wait longer, in total, than one proof of their own would have cost.
func (facts PipelineFacts) Reachable(oldestJoin time.Time) []Underway {
	bound := oldestJoin.Add(facts.ProofCost)
	var reachable []Underway
	for _, unit := range facts.Underway {
		if !unit.ExpectedReady.After(bound) {
			reachable = append(reachable, unit)
		}
	}
	return reachable
}
