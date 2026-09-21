// Package backlog places every goal of one accepted ledger tree in exactly
// one lane, with the record's own facts beside it and a named gap wherever
// the record does not answer. It is the projection both a browser and a
// terminal read, so it lives outside the interface and depends on no
// transport: the ledger tree, the approval horizon, and the engine's own
// claim-admission verdict go in, and rows come out. It judges nothing the
// engine judges — readiness, expiry, and refusal all arrive as answers.
package backlog

// Lane is where a goal stands in the flow of work. The set is closed: a
// record this build cannot place goes to unknown with the reason on the row
// rather than disappearing from the board.
type Lane string

const (
	// LaneDraft is a proposal that has not passed intake. No engine reader
	// owns proposals yet, so this lane carries a statement and no rows.
	LaneDraft Lane = "draft"
	// LaneToDo is work not yet authorized for execution: queued, or approved
	// with an approval the claim gate will not act on.
	LaneToDo Lane = "to-do"
	// LaneReady is approved work the claim gate would admit right now.
	LaneReady Lane = "ready"
	// LaneInProgress is claimed work being executed.
	LaneInProgress Lane = "in-progress"
	// LaneReview is claimed work built and waiting to land.
	LaneReview Lane = "review"
	// LaneWaiting is work an authoritative blocker holds: a park, an open
	// dependency, or a breach fence.
	LaneWaiting Lane = "waiting"
	// LaneDone is a recorded completed goal.
	LaneDone Lane = "done"
	// LaneAbandoned is work dropped with its recorded reason.
	LaneAbandoned Lane = "abandoned"
	// LaneUnknown holds a record this build cannot place. It is reachable
	// only when the engine cannot answer admission or gains a state this
	// projection does not know; a goal is never dropped to reach it.
	LaneUnknown Lane = "unknown"
)

// LaneOrder is the order the lanes are read in, from proposal to conclusion.
var LaneOrder = []Lane{
	LaneDraft, LaneToDo, LaneReady, LaneInProgress,
	LaneReview, LaneWaiting, LaneDone, LaneAbandoned, LaneUnknown,
}

// ClosedLanes are the lanes whose goals have left the live ledger.
var ClosedLanes = []Lane{LaneDone, LaneAbandoned}
