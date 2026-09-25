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
// projection it could not make — an unknown projection says WHICH class of
// evidence stopped it, because a goal that has spent nothing and a goal
// nobody could count look identical in numbers and are not the same fact.
//
// What it says is a fixed sentence and never the projection's own words. The
// string this file produces is written onto the presence record, which the
// tick publishes to a remote; the projection builds most of its reasons from
// an err.Error(), so a file this machine could not read would put an absolute
// path on that record. Every sentence here is a constant for that reason.

import (
	"strings"
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
			return &seat.Box{Problem: boxUnknownGoal}
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

// The bounded descriptions a ledger this machine could not read may carry.
// They are fixed for the reason the ones below are: a git error's text is
// built at this machine and names paths on it, and this string is written
// onto a record every other seat reads.
const (
	boxUnknownTip    = "this goal's box could not be projected: this machine could not read its accepted ledger"
	boxUnknownLedger = "this machine has no accepted ledger to project a box from"
	boxUnknownGoal   = "this goal is not live at the accepted ledger this machine read"
)

// acceptedGoals is the live goals at the accepted tip, or why there are none
// to read, in words that name no path.
func acceptedGoals(repoRoot string) (map[string]*goal.GoalFile, string) {
	tip, exists, err := goal.AcceptedLedgerTip(repoRoot)
	if err != nil {
		return nil, boxUnknownTip
	}
	if !exists {
		return nil, boxUnknownLedger
	}
	projection, err := goal.ProjectAt(repoRoot, tip)
	if err != nil || projection.Tree == nil {
		return nil, boxUnknownTip
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

// The bounded descriptions an unknown box may carry, one per class of
// evidence the projection distinguishes.
//
// They are fixed sentences and never the projection's own words. Most of its
// reasons are built from an err.Error(), so a file this machine could not
// read puts a machine-local ABSOLUTE PATH in one — and this string is written
// onto the presence record, which the tick publishes to a remote every other
// seat reads. The seat package's rule is that a record carries no free text
// for exactly that reason: every field is an identifier, a number, a hash or
// a time, so a record can carry neither a secret nor a path off this host.
//
// So the class is chosen from the Unknown evidence and the words are these.
// The goal id is already beside them on the record, in `working.goal`, so
// nothing here repeats it.
const (
	boxUnknownProjection      = "this goal's box could not be projected"
	boxUnknownGoalRecord      = "this goal's box could not be projected from its goal record"
	boxUnknownJobRecords      = "this goal's box could not be projected from its delegate job records"
	boxUnknownProofRecords    = "this goal's box could not be projected from its proof-attempt records"
	boxUnknownGovernedRecords = "this goal's box could not be projected from its governed-run records"
	boxUnknownWeightRecord    = "this goal's box could not be projected from the validation-weight record"
	boxUnknownConfiguration   = "this goal's box could not be projected from this machine's configuration"
)

// unknownBoxReason names the class of evidence that stopped the projection,
// chosen from the record the projection named and never copied from it.
//
// The raw reason does not travel and is not kept: this component has no local
// log and no alert path of its own, and a reason that cannot be published and
// has nowhere local to go is dropped rather than smuggled onto the record. A
// human who needs the exact words reads the projection at this machine's own
// terminal, where it is the projection that says them.
func unknownBoxReason(projected dispatch.ConsumptionProjection) string {
	if projected.Unknown == nil {
		return boxUnknownProjection
	}
	switch record := projected.Unknown.Record; {
	case strings.HasPrefix(record, "plans/goals"):
		return boxUnknownGoalRecord
	case strings.HasPrefix(record, "artifacts/agents/jobs"):
		return boxUnknownJobRecords
	case strings.HasPrefix(record, "artifacts/agents/proof-runs"):
		return boxUnknownProofRecords
	case strings.HasPrefix(record, "artifacts/agents/governed-obligations"):
		return boxUnknownGovernedRecords
	case strings.HasPrefix(record, "artifacts/agents/validation-weight"):
		return boxUnknownWeightRecord
	case strings.HasPrefix(record, "metasystem.conf"):
		return boxUnknownConfiguration
	default:
		return boxUnknownProjection
	}
}
