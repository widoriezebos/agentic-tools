package steward

// The box behind what a seat is doing: one goal's consumption against its
// limits, projected for the tick that publishes presence and for the
// interface that draws the same block.
//
// It is here and not in internal/seat for one reason. A goal's consumption is
// dispatch's projection, read from the goal file at the accepted tip, and the
// presence package opens no repository and knows nothing of dispatch — it
// composes a record from job files and a clock. So the component that owns
// the tick supplies the reader, and a test supplies a fake.
//
// Two things it deliberately does not do. It computes no elapsed time,
// because ProjectConsumption computes none: elapsed belongs to the authority
// lens and this is the spending view. And it never answers zeros for a
// projection it could not make — an unknown projection carries its own
// reason, because a goal that has spent nothing and a goal nobody could count
// look identical in numbers and are not the same fact.

import (
	"sync"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/seat"
)

// SeatBox reads the accepted tip once and answers for each goal from it.
//
// Once, because the tip is one reading: two captures could count one goal's
// attempts from one commit and another goal's limits from the next, and the
// block would be a join of two ledgers. The read is lazy, so a tick whose
// jobs name no goal never opens the repository at all.
func SeatBox(repoRoot string, now time.Time) seat.BoxReader {
	var (
		once    sync.Once
		live    map[string]*goal.GoalFile
		problem string
	)
	return func(id string) *seat.Box {
		once.Do(func() { live, problem = acceptedGoals(repoRoot) })
		if problem != "" {
			return &seat.Box{Problem: problem}
		}
		file, present := live[id]
		if !present || file == nil {
			return &seat.Box{Problem: "the accepted ledger carries no live goal " + id}
		}
		// A goal with no budget tuple has no box, which is a different thing
		// from a box nobody could project: the page says so in its own words
		// and shows no bars at all.
		if file.Budget == nil {
			return nil
		}
		return boxOf(dispatch.ProjectConsumption(repoRoot, file, now))
	}
}

// acceptedGoals is the live goals at the accepted tip, or the reason there
// are none to read.
func acceptedGoals(repoRoot string) (map[string]*goal.GoalFile, string) {
	tip, exists, err := goal.AcceptedLedgerTip(repoRoot)
	if err != nil {
		return nil, "the accepted ledger tip is unreadable: " + err.Error()
	}
	if !exists {
		return nil, "this checkout has no accepted ledger to project a box from"
	}
	projection, err := goal.ProjectAt(repoRoot, tip)
	if err != nil {
		return nil, "the accepted ledger tree is unreadable: " + err.Error()
	}
	if projection.Tree == nil {
		return nil, "the accepted ledger tree carries no goals"
	}
	return projection.Tree.Live, ""
}

// boxOf maps the projection onto the box a record carries: its attempts and
// its reserved job minutes against the goal's own limits, and nothing it did
// not count.
func boxOf(projected dispatch.ConsumptionProjection) *seat.Box {
	if projected.Status != dispatch.BudgetKnown {
		return &seat.Box{Problem: unknownBoxReason(projected)}
	}
	attempts := int64(projected.Attempts)
	attemptLimit := int64(projected.Limits.AttemptLimit)
	reserved := int64(projected.ReservedJobMinutes)
	reservedLimit := int64(projected.Limits.ReservedJobMinutesLimit)
	return &seat.Box{
		Attempts: &attempts, AttemptLimit: &attemptLimit,
		ReservedMinutes: &reserved, ReservedMinutesLimit: &reservedLimit,
	}
}

// unknownBoxReason is the projection's own words, with the record it names
// where it named one. The projection is the owner of why it could not count,
// and a second account written here would be a second account.
func unknownBoxReason(projected dispatch.ConsumptionProjection) string {
	if projected.Unknown == nil {
		return "this goal's consumption could not be projected"
	}
	reason := projected.Unknown.Reason
	if reason == "" {
		reason = "this goal's consumption could not be projected"
	}
	if projected.Unknown.Record != "" {
		reason += " (" + projected.Unknown.Record + ")"
	}
	return reason
}
