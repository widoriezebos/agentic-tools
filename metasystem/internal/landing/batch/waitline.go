package batch

import (
	"fmt"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/board"
)

// WaitLine is the one line every reader prints for a batch's start (D14,
// R23): what an open batch waits for, naming each unit's seat, stage, round
// and expected time, the separate-proof cost and its basis, or why it
// started. Times are local for people. Empty when the batch has neither.
func WaitLine(record Record, now time.Time, location *time.Location) string {
	if location == nil {
		location = time.Local
	}
	switch {
	case record.Wait != nil:
		return "batch " + record.BatchID + " " + waitText(*record.Wait, now, location)
	case record.StartReason != "":
		line := "batch " + record.BatchID + " started: " + record.StartReason
		if lastHistory(record).Verb == "cap" {
			line += "; first in line for a slot"
		}
		return line
	}
	return ""
}

func waitText(wait WaitState, now time.Time, location *time.Location) string {
	if wait.Fallback != "" {
		return fmt.Sprintf("waits: board unreadable (%s); starts at the max wait, %s", wait.Fallback, localClock(wait.Until, location))
	}
	var units []string
	for _, waited := range wait.For {
		units = append(units, fmt.Sprintf("%s on %s (%s, ~%s)", waited.Goal, waited.Seat, stageText(waited.Stage, waited.Round, waited.Proof), minutes(waited.ExpectedAt.Sub(now))))
	}
	basis := wait.Basis
	if basis == "" {
		basis = "default"
	}
	return fmt.Sprintf("waits for %s; a separate proof costs ~%s (%s)", joinAnd(units), minutes(wait.ProofCost), basis)
}

// stageText is a stage as a person reads it: review round 2 of 3, unit
// proof 120 of 189.
func stageText(stage board.Stage, round *board.Round, proof *board.Proof) string {
	text := strings.ReplaceAll(string(stage), "-", " ")
	if proof != nil && proof.Planned > 0 && stage == board.StageUnitProof {
		text += fmt.Sprintf(" %d of %d", proof.Done, proof.Planned)
	}
	if round != nil && (stage == board.StageReview || stage == board.StageRevise) {
		if round.Max != nil {
			text += fmt.Sprintf(" round %d of %d", round.N, *round.Max)
		} else {
			text += fmt.Sprintf(" round %d", round.N)
		}
	}
	return text
}

func joinAnd(items []string) string {
	switch len(items) {
	case 0:
		return "nothing"
	case 1:
		return items[0]
	}
	return strings.Join(items[:len(items)-1], ", ") + " and " + items[len(items)-1]
}

func lastHistory(record Record) HistoryEntry {
	if len(record.History) == 0 {
		return HistoryEntry{}
	}
	return record.History[len(record.History)-1]
}
