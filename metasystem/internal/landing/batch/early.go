package batch

import (
	"fmt"
	"strings"
	"time"
)

// Early is what the owner did with a batch's wait (D14, R27; U10b-3): the
// join's cheap phase on the tip the joins recorded, then at most one early
// delivery proof of it on spare capacity. Shape is the joined members that
// tip holds, in join order (U3-02). A red is a finding said in the lines:
// no diagnostic runs in the wait, and the batch proof's D1 decides it. The
// early proof is an ordinary retained attempt: the batch proof takes from it
// only what identity-exact reuse takes, and it is never the batch's own
// tip attempt.
type Early struct {
	Shape   []string      `json:"shape"`
	Tree    string        `json:"tree"`
	Cheap   string        `json:"cheap,omitempty"`
	Proof   string        `json:"proof,omitempty"`
	Attempt string        `json:"attempt,omitempty"`
	Finding *EarlyFinding `json:"finding,omitempty"`
	// Idle says why nothing more runs meanwhile, in the line's words.
	Idle string `json:"idle,omitempty"`
	// Ended names why the acts ended while the record keeps what they did.
	Ended string `json:"ended,omitempty"`
}

// EarlyFinding is a red of the partial tip: said in the lines and decided at
// the batch proof, never a return and never a landing.
type EarlyFinding struct {
	Group   string `json:"group"`
	Attempt string `json:"attempt"`
	Log     string `json:"log,omitempty"`
}

// EarlyResult is one early act's outcome: its attempt and its red groups.
type EarlyResult struct {
	Attempt string
	Failing []RedGroup
}

// EarlySeams are the early acts' effects outside the record. Cheap runs the
// join's cheap phase on the record's tip tree; Prove runs one delivery proof
// of it, launched as the tip proof is but reserving no diagnostic headroom,
// since it is nobody's tip; Budget says whether the head member, whom every
// early act is charged to, keeps after one more attempt everything the batch
// proof needs (U3-01), and in words why not.
type EarlySeams struct {
	Cheap  func(Record) (EarlyResult, error)
	Prove  func(Record) (EarlyResult, error)
	Budget func(Record) (bool, string)
}

// EarlyCompletion is an early act as it returns to the owner's loop.
type EarlyCompletion struct {
	ID, Tree, Kind string
	Result         EarlyResult
	Err            error
}

const earlyGreen, earlyRed, earlyRunning, earlyUnavailable = "green", "red", "running", "unavailable"

// forgetEarly drops the early work of a batch whose inputs moved, with one
// history line naming the cause; an early run in flight ends on its own.
func forgetEarly(record *Record, at time.Time, actor, cause string) {
	if record.Early == nil {
		return
	}
	record.Early = nil
	record.Transition(record.State, at, "early-forget", actor, cause)
}

// leaveCause is the history word for a member leaving by outcome.
func leaveCause(goalID, outcome string) string {
	switch outcome {
	case UnitEjected:
		return goalID + " ejected"
	case UnitWithdrawn, UnitWithdrawnBudget:
		return goalID + " withdrawn"
	}
	return goalID + " returned"
}

// earlyClause is the wait line's meanwhile clause.
func earlyClause(early *Early) string {
	if early == nil || early.Ended != "" {
		return ""
	}
	members := strings.Join(early.Shape, "+")
	switch {
	case early.Finding != nil:
		return fmt.Sprintf("partial red: %s on %s; decided at the batch proof", early.Finding.Group, members)
	case early.Cheap == "" || early.Cheap == earlyUnavailable:
		return early.Idle
	}
	clause := members + ": cheap checks " + early.Cheap
	if early.Proof != "" && early.Proof != earlyUnavailable {
		clause += ", early proof " + early.Proof
	}
	if early.Idle != "" {
		clause += "; " + early.Idle
	}
	return clause
}
