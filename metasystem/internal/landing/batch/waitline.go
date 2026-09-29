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
		line := "batch " + record.BatchID + " " + waitText(*record.Wait, now, location)
		if clause := earlyClause(record.Early); clause != "" {
			line += "; meanwhile: " + clause
		}
		return line
	case record.StartReason != "":
		line := "batch " + record.BatchID + " started: " + record.StartReason
		if lastHistory(record).Verb == "cap" {
			line += "; first in line for a slot"
		}
		if reused, of, ok := earlyReuse(record); ok {
			line += fmt.Sprintf("; reusing %d of %d groups from the early proof", reused, of)
		}
		return line
	}
	return ""
}

func waitText(wait WaitState, now time.Time, location *time.Location) string {
	if len(wait.Changes) != 0 {
		return fmt.Sprintf("waits for a goal member: %s %s on a goal's proof", joinAnd(wait.Changes), map[bool]string{true: "rides", false: "ride"}[len(wait.Changes) == 1])
	}
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

// stageText is a stage as a person reads it, the board's own rendering.
func stageText(stage board.Stage, round *board.Round, proof *board.Proof) string {
	return board.StageText(stage, round, proof)
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

// earlyReuse counts the batch proof's groups the retained verifier resolved
// to the early proof's attempt; ok once the proof's sources are recorded.
func earlyReuse(record Record) (reused, of int, ok bool) {
	if record.Early == nil || record.Early.Attempt == "" || record.Proof == nil || record.Proof.Sources == nil {
		return 0, 0, false
	}
	for _, source := range record.Proof.Sources {
		if source.Kind == SourceReused && source.Attempt == record.Early.Attempt {
			reused++
		}
	}
	return reused, len(record.Proof.SelectedGroups), true
}
