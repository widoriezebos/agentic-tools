package batch

import (
	"cmp"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
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

const earlyCheap, earlyProof = "cheap", "proof"

const noSpareSlot = "no spare proof slot"

// earlyRun is one early act of this owner in flight.
type earlyRun struct{ tree, kind string }

// useWait uses a wait the start rule just recorded, after it decided and
// holding none of its locks, when the host is not loaded: the cheap phase on
// the tip, then one early proof of it. Each act runs in the background and
// returns through the owner's loop; only one act of a batch runs at a time,
// and a join grows the shape and the acts start again once it ends. Every
// act first passes the head member's budget gate (U3-01). It never changes
// the decision.
func (owner *Owner) useWait(record Record, sample proofrun.LoadSample, at time.Time) error {
	if _, running := owner.earlyRuns[record.BatchID]; running {
		return nil
	}
	if record.State != StateOpen || record.Wait == nil || record.TipTree == "" || record.TipTree == record.BaseTree ||
		slices.ContainsFunc(record.Units, func(unit Unit) bool { return unit.State == UnitJoining }) {
		return nil
	}
	shape := goalIDs(joinedUnits(record.Units))
	if len(shape) == 0 {
		return nil
	}
	early := Early{Shape: shape, Tree: record.TipTree}
	if current := record.Early; current != nil && current.Tree == record.TipTree {
		if current.Ended != "" {
			return nil
		}
		early = *current
		if early.Cheap == earlyRunning || early.Proof == earlyRunning {
			// No run of this owner: an earlier owner's, which ends on its
			// own. The acts on this tip are over.
			early.Ended = "owner restarted"
			return owner.writeEarly(record.BatchID, early, at)
		}
	}
	if sample.Loaded() {
		early.Idle = "host loaded: " + describeLoad(sample)
		if early.Cheap == "" {
			early.Idle = "nothing (" + early.Idle + ")"
		}
		return owner.writeEarly(record.BatchID, early, at)
	}
	if early.Cheap != earlyUnavailable && early.Proof != earlyUnavailable {
		early.Idle = ""
	}
	kind := ""
	switch {
	case early.Cheap == "":
		kind = earlyCheap
	case early.Cheap == earlyGreen && early.Proof == "" && early.Finding == nil:
		kind = earlyProof
		early.Idle = owner.earlyProofRefusal(sample)
	}
	if kind == "" || early.Idle != "" {
		return owner.writeEarly(record.BatchID, early, at)
	}
	if ok, why := owner.early.Budget(record); !ok {
		early.Idle = cmp.Or(why, "no early work: the budget keeps its attempts for the batch proof")
		return owner.writeEarly(record.BatchID, early, at)
	}
	act := owner.early.Cheap
	if kind == earlyCheap {
		early.Cheap = earlyRunning
	} else {
		act, early.Proof = owner.early.Prove, earlyRunning
	}
	if err := owner.writeEarly(record.BatchID, early, at); err != nil {
		return err
	}
	owner.earlyRuns[record.BatchID] = earlyRun{tree: early.Tree, kind: kind}
	go func() {
		result, err := act(record)
		owner.earlyDone <- EarlyCompletion{ID: record.BatchID, Tree: early.Tree, Kind: kind, Result: result, Err: err}
	}()
	return nil
}

// earlyProofRefusal says why no early proof starts now, or nothing: one per
// owner at a time, never while a batch proof waits for a slot, and only when
// the host runner would still admit one more real proof beside it and every
// run of this owner, early ones included.
func (owner *Owner) earlyProofRefusal(sample proofrun.LoadSample) string {
	own := 0
	for _, run := range owner.earlyRuns {
		if run.kind == earlyProof {
			return noSpareSlot
		}
		own++
	}
	if len(owner.capped) != 0 {
		return "no early proof: a batch proof waits for a slot"
	}
	for _, runner := range owner.runners(sample, owner.admission(sample)) {
		if runner.Runner == "host" && runner.admits(owner.ownRuns(runner.Runner)+own+1) {
			return ""
		}
	}
	return noSpareSlot
}

// EarlyCompletions is where the owner's early acts finish; a loop selecting
// on it hands each to CompleteEarly.
func (owner *Owner) EarlyCompletions() <-chan EarlyCompletion {
	if owner == nil {
		return nil
	}
	return owner.earlyDone
}

// CompleteEarly records one finished early act and acts on nothing: a red is
// a finding the batch proof decides. An act whose inputs moved (the base, a
// member, the start) was abandoned: it ended on its own, its results stay
// retained, and the batch proof finds them by its tree.
func (owner *Owner) CompleteEarly(done EarlyCompletion) {
	delete(owner.earlyRuns, done.ID)
	if err := owner.completeEarly(done); err != nil {
		owner.report(done.ID, err)
	}
}

func (owner *Owner) completeEarly(done EarlyCompletion) error {
	record, err := owner.store.Load(done.ID)
	if err != nil || record.State != StateOpen || owner.inflight[done.ID] != nil || record.Early == nil || record.Early.Tree != done.Tree {
		return err
	}
	early := *record.Early
	status := earlyGreen
	switch {
	case len(done.Result.Failing) != 0:
		status = earlyRed
		early.Finding = &EarlyFinding{Group: done.Result.Failing[0].ID, Attempt: done.Result.Attempt, Log: done.Result.Failing[0].LogPath}
	case done.Err != nil:
		status = earlyUnavailable
	}
	switch {
	case done.Kind == earlyCheap && early.Cheap == earlyRunning:
		early.Cheap = status
		if status == earlyUnavailable {
			early.Idle = "cheap checks unavailable: " + done.Err.Error()
		}
	case done.Kind == earlyProof && early.Proof == earlyRunning:
		early.Proof, early.Attempt = status, done.Result.Attempt
		if status == earlyUnavailable {
			early.Idle = "early proof unavailable: " + done.Err.Error()
		}
	default:
		return nil
	}
	return owner.writeEarly(done.ID, early, owner.now())
}

func goalIDs(units []Unit) []string {
	ids := make([]string, 0, len(units))
	for _, unit := range units {
		ids = append(ids, unit.GoalID)
	}
	return ids
}

func describeLoad(sample proofrun.LoadSample) string {
	return fmt.Sprintf("load %.1f on %d cores", sample.Load1m, sample.Cores)
}

// idleKind is an idle reason without its numbers: a changed load alone is
// not a change of state (R-129).
func idleKind(idle string) string {
	kind, _, _ := strings.Cut(idle, ":")
	return kind
}

// writeEarly records the early state of the open batch whose tip it
// describes, and the line with it, only when either changed.
func (owner *Owner) writeEarly(id string, early Early, at time.Time) error {
	var line string
	err := owner.store.Update(id, func(current *Record) error {
		// A join may have grown the tip under an early proof of its prefix:
		// that proof's own record is still written.
		if current.State != StateOpen || current.TipTree != early.Tree && (current.Early == nil || current.Early.Tree != early.Tree) {
			return nil
		}
		previous := current.Early
		if previous != nil && idleKind(previous.Idle) == idleKind(early.Idle) {
			early.Idle = previous.Idle
		}
		if previous != nil && reflect.DeepEqual(*previous, early) {
			return nil
		}
		before := WaitLine(*current, at, owner.location)
		next := early
		current.Early = &next
		if after := WaitLine(*current, at, owner.location); after != before {
			current.Transition(current.State, at, "meanwhile", owner.actor, after)
			line = after
		}
		return nil
	})
	if err == nil && line != "" && owner.logWait != nil {
		owner.logWait(id, line)
	}
	return err
}

// forgetEarly drops the early work of a batch whose inputs moved, with one
// history line naming the cause; an early run in flight ends on its own.
func forgetEarly(record *Record, at time.Time, actor, cause string) {
	if record.Early == nil {
		return
	}
	record.Early = nil
	record.Transition(record.State, at, "early-forget", actor, cause)
}

// endEarly marks the acts over when the batch starts; the record keeps what
// they did for the batch proof.
func endEarly(record *Record, at time.Time, actor string) {
	if record.Early == nil || record.Early.Ended != "" {
		return
	}
	record.Early.Ended = "batch started"
	record.Transition(record.State, at, "early-end", actor, "batch started")
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
