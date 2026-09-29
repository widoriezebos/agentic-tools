package batch

import (
	"cmp"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy/adapter"
)

// Early is what the owner did with a batch's wait (D14, R27; U10b-3): the
// join's cheap phase on the tip the joins recorded, then at most one early
// delivery proof of it on spare capacity, and a red judged by D1's first two
// steps. Shape is the joined members that tip holds, in join order: a red is
// judged against them alone, never against a member that joined later
// (U3-02). The early proof is an ordinary retained attempt: the batch proof
// takes from it only what identity-exact reuse takes from any attempt, and
// it is never the batch's own tip attempt.
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

// EarlyFinding is a partial red nobody was named for: said in the lines and
// decided at the batch proof, never a return and never a landing.
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
// since it is nobody's tip; Budget says whether the head member's budget
// keeps the batch proof's attempts and headroom after one more attempt
// (U3-01), and in words why not; Adapter names a red group's language
// adapter for naming by owner unit (nil: unknown).
type EarlySeams struct {
	Cheap   func(Record) (EarlyResult, error)
	Prove   func(Record) (EarlyResult, error)
	Budget  func(Record) (bool, string)
	Adapter func(RedGroup) (adapter.Adapter, bool)
}

// EarlyCompletion is an early proof as it returns to the owner's loop.
type EarlyCompletion struct {
	ID, Tree string
	Result   EarlyResult
	Err      error
}

const earlyGreen, earlyRed, earlyRunning, earlyUnavailable = "green", "red", "running", "unavailable"

const noSpareSlot = "no spare proof slot"

// useWait uses a wait the start rule just recorded, after it decided and
// holding none of its locks, when the host is not loaded: the cheap phase on
// the tip, then one early proof of it when the runner would still admit a
// real proof beside it. While an early proof of the batch runs, nothing new
// starts; a join grows the shape and the acts start again once it ends. It
// never changes the decision.
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
	if current := record.Early; current != nil && current.Tree == record.TipTree && current.Ended == "" {
		early = *current
	}
	if sample.Loaded() {
		early.Idle = "host loaded: " + describeLoad(sample)
		if early.Cheap == "" {
			early.Idle = "nothing (" + early.Idle + ")"
		}
		return owner.writeEarly(record.BatchID, early, at)
	}
	early.Idle = ""
	if early.Cheap == "" {
		result, err := owner.early.Cheap(record)
		if err != nil {
			early.Cheap, early.Idle = earlyUnavailable, "cheap checks unavailable: "+err.Error()
			return owner.writeEarly(record.BatchID, early, at)
		}
		early.Cheap = earlyGreen
		if len(result.Failing) != 0 {
			early.Cheap = earlyRed
		}
		if early.Cheap == earlyRed {
			if err := owner.writeEarly(record.BatchID, early, at); err != nil {
				return err
			}
			return owner.judgeEarlyRed(record.BatchID, early, result, at)
		}
	}
	if early.Cheap != earlyGreen || early.Proof != "" || early.Finding != nil {
		return owner.writeEarly(record.BatchID, early, at)
	}
	if early.Idle = owner.earlyProofRefusal(record, sample); early.Idle != "" {
		return owner.writeEarly(record.BatchID, early, at)
	}
	early.Proof = earlyRunning
	if err := owner.writeEarly(record.BatchID, early, at); err != nil {
		return err
	}
	owner.earlyRuns[record.BatchID] = early.Tree
	go func() {
		result, err := owner.early.Prove(record)
		owner.earlyDone <- EarlyCompletion{ID: record.BatchID, Tree: early.Tree, Result: result, Err: err}
	}()
	return nil
}

// earlyProofRefusal says why no early proof starts now, or nothing: one per
// owner at a time, only when the host runner would still admit one more real
// proof beside it and every run of this owner, and only when the head
// member's budget keeps the batch proof's headroom (U3-01).
func (owner *Owner) earlyProofRefusal(record Record, sample proofrun.LoadSample) string {
	if len(owner.earlyRuns) != 0 {
		return noSpareSlot
	}
	room := false
	for _, runner := range owner.runners(sample, owner.admission(sample)) {
		if runner.Runner == "host" {
			room = runner.admits(owner.ownRuns(runner.Runner) + 1)
		}
	}
	if !room {
		return noSpareSlot
	}
	if ok, why := owner.early.Budget(record); !ok {
		return cmp.Or(why, "no early proof: the budget keeps its attempts for the batch proof")
	}
	return ""
}

// EarlyCompletions is where the owner's early proofs finish; a loop
// selecting on it hands each to CompleteEarly.
func (owner *Owner) EarlyCompletions() <-chan EarlyCompletion {
	if owner == nil {
		return nil
	}
	return owner.earlyDone
}

// CompleteEarly applies one finished early proof. An early proof whose
// inputs moved (the base, a member, the start) was abandoned: it ended on
// its own, its results stay retained, and nothing here reads them.
func (owner *Owner) CompleteEarly(done EarlyCompletion) {
	delete(owner.earlyRuns, done.ID)
	if err := owner.completeEarly(done); err != nil {
		owner.report(done.ID, err)
	}
}

func (owner *Owner) completeEarly(done EarlyCompletion) error {
	record, err := owner.store.Load(done.ID)
	if err != nil || record.State != StateOpen || owner.inflight[done.ID] != nil ||
		record.Early == nil || record.Early.Tree != done.Tree || record.Early.Proof != earlyRunning {
		return err
	}
	early, at := *record.Early, owner.now()
	early.Attempt = done.Result.Attempt
	switch {
	case len(done.Result.Failing) != 0:
		early.Proof = earlyRed
	case done.Err != nil:
		early.Proof, early.Idle = earlyUnavailable, "early proof unavailable: "+done.Err.Error()
	default:
		early.Proof = earlyGreen
	}
	if early.Proof == earlyRed && early.Ended != "" {
		// The batch started while it ran: nothing is judged any more, and
		// the red is kept for its batch proof (U3-03).
		early.Finding = &EarlyFinding{Group: done.Result.Failing[0].ID, Attempt: done.Result.Attempt, Log: done.Result.Failing[0].LogPath}
	}
	if err := owner.writeEarly(done.ID, early, at); err != nil || early.Proof != earlyRed || early.Ended != "" {
		return err
	}
	return owner.judgeEarlyRed(done.ID, early, done.Result, at)
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

// judgeEarlyRed decides a red of the tip the joined members made by D1's
// first two steps, against the members of the red's own shape: the failing
// groups run on the base; red twice is a trunk red that holds; a member
// named by owner unit is ejected now and the survivors reassemble; anything
// else is a finding. The classification (D1 step 3) never runs here.
func (owner *Owner) judgeEarlyRed(id string, early Early, result EarlyResult, at time.Time) error {
	record, err := owner.store.Load(id)
	if err != nil || record.State != StateOpen || record.Early == nil || record.Early.Tree != early.Tree || len(result.Failing) == 0 {
		return err
	}
	units := slices.DeleteFunc(joinedUnits(record.Units), func(unit Unit) bool { return !slices.Contains(early.Shape, unit.GoalID) })
	finding := func() error {
		found := *record.Early
		found.Finding = &EarlyFinding{Group: result.Failing[0].ID, Attempt: result.Attempt, Log: result.Failing[0].LogPath}
		return owner.writeEarly(id, found, at)
	}
	if len(units) == 0 {
		return finding()
	}
	authority := units[len(units)-1]
	run := func(groups []RedGroup, fresh []string) (DiagnosticResult, error) {
		return owner.runDiagnostic(id, DiagnosticRequest{Tree: record.BaseTree, GoalID: authority.GoalID, Groups: redGroupIDs(groups),
			Claim: authority.Claim, NeverReuse: true, Fresh: fresh}, authority.Claim)
	}
	base, err := run(result.Failing, nil)
	if err != nil {
		owner.report(id, fmt.Errorf("early red: base check unavailable: %w", err))
		return finding()
	}
	if !base.Green() {
		second, err := run(base.Groups, freshExecution(base.Groups, owner.early.Adapter))
		if err != nil {
			owner.report(id, fmt.Errorf("early red: second base run unavailable: %w", err))
			return finding()
		}
		if second.Green() {
			// Red then green on main: nobody is named, and a partial red
			// takes nothing into the register.
			return finding()
		}
		baseCommit, err := owner.baseCommit(record.BaseTree)
		if err != nil {
			return err
		}
		opid, err := owner.mint()
		if err != nil {
			return err
		}
		red := TrunkRed{AttemptID: second.AttemptID, BaseCommit: baseCommit, BaseTree: record.BaseTree, Groups: second.Groups}
		if err := owner.store.holdEarlyTrunkRed(id, red, opid, at, owner.actor); err != nil {
			return err
		}
		_, err = owner.store.EnsureTrunkRedRecorded(id, owner.mint, at, owner.actor)
		return err
	}
	named := namedDiagnosticUnits(units, result.Failing, owner.early.Adapter)
	if len(named) == 0 {
		return finding()
	}
	var decisions []ReturnDecision
	for _, unit := range units {
		if named[unit.GoalID] {
			decisions = append(decisions, ReturnDecision{GoalID: unit.GoalID, Outcome: UnitEjected,
				Reason: earlyEjection(id, unit.GoalID, early.Shape, result.Failing)})
		}
	}
	return ReassembleSurvivorsWithReturns(owner.store, id, owner.actor, at, decisions)
}

// earlyEjection is the return of a member a partial red names, headed as
// D1's return is and saying it came before the batch's proof.
func earlyEjection(id, goalID string, shape []string, failing []RedGroup) string {
	group := failing[0]
	test := group.ID
	if len(group.Failures) != 0 && group.Failures[0].Name != "" {
		test = group.Failures[0].Name + " (" + group.ID + ")"
	}
	return fmt.Sprintf("EJECTED from landing batch %s before its proof: %s failed on the batch's partial tip (%s) and passes on main; log %s. Fix it, then metasystem work land %s.",
		id, test, strings.Join(shape, ", "), group.LogPath, goalID)
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
		return fmt.Sprintf("partial red: %s on %s, nobody named; decided at the batch proof", early.Finding.Group, members)
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
