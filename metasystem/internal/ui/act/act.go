// Package act is the interface server's human hand on the ledger.
//
// Ten verbs reach it, and every one of them is a verb a human performs on
// their own backlog: goal approve and goal unapprove admit and withdraw work,
// goal park and goal unpark pause it and let it go again, goal set-priority
// places a goal in a band and orders it there, goal open is
// the intake act that creates one, goal block and goal unblock write and
// remove the edges that say which goal waits for which, goal edit
// rewrites the three fields of a goal nobody has approved yet, and goal
// abandon records that a goal will never be worked and why. Nothing here
// shells out. A
// child of this server has no terminal in its ancestry and would be refused
// by the very check that makes these acts a human's, so the engine is called
// in-process, through the same internal/goal request path the command edge
// builds, with the same proof object the command edge carries. The goal's
// History line is therefore the line a terminal approval writes.
//
// The proof is taken ONCE, at boot, and kept for the server's life. That is
// not a shortcut: a proof is an in-process observation of a live ancestry,
// and the only moment this process has an ancestry reaching the human is the
// moment it is started — by `metasystem ui start`, from the human's own
// terminal, which waits for the readiness line and is therefore still this
// process's parent while the observation is made. A parsed proof has no
// authority and none is ever read from disk here.
package act

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/brain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/counselor"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/fixtureauth"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	goalbranch "github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goalbudget"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ledgerfence"
)

// Restart is what a human does about an interface that cannot act as them.
// Every refusal ends with it, because it is the whole remedy.
const Restart = "start it from your own terminal with bin/metasystem ui restart to act as yourself"

// Authority is one boot-time observation of who started this server: the
// human proof, the caller classification the command edge takes at the same
// moment, and the name and lineage of the enrolled terminal. An Authority
// that is not proven carries the reason instead, and refuses both verbs.
type Authority struct {
	proven         bool
	human          string
	lineage        string
	reason         string
	root           string
	proof          humanauthority.Proof
	classification lease.ClassifyResult
	reads          reads
}

// reads are the four readings a verb request is assembled from that stand on
// Git: the ledger fence's enrollment probe, the endpoint and the machine name
// the checkout's configuration answers, and the branch-safety check a park
// runs.
//
// They are fields rather than direct calls so that a test in this package can
// hand in an in-memory ledger through goal.Endpoint's own Repository seam. This
// repository's rules ask that a behaviour test stub Git, fixture setup
// included, and every one of these four runs it. The zero value is the
// production reading — each accessor falls back to the package it names — so
// nothing outside a test ever chooses, and nothing here is settable from
// another package.
type reads struct {
	fence     func(root string) error
	endpoint  func(root string) (goal.Endpoint, error)
	machine   func(root string) (string, error)
	parkCheck func(root string, endpoint goal.Endpoint) func(goalID, next string) (string, error)
}

func (r reads) ensureFence(root string) error {
	if r.fence != nil {
		return r.fence(root)
	}
	return ledgerfence.Ensure(root)
}

func (r reads) resolveEndpoint(root string) (goal.Endpoint, error) {
	if r.endpoint != nil {
		return r.endpoint(root)
	}
	return goal.ResolveEndpoint(root)
}

func (r reads) resolveMachine(root string) (string, error) {
	if r.machine != nil {
		return r.machine(root)
	}
	return goal.ResolveMachine(root)
}

func (r reads) parkBranchCheck(root string, endpoint goal.Endpoint) func(goalID, next string) (string, error) {
	if r.parkCheck != nil {
		return r.parkCheck(root, endpoint)
	}
	return goalbranch.ParkCheck(root, endpoint)
}

// Prove takes the one observation. invokerPID is the process whose ancestry
// is walked: the launcher, which is this server's parent while it boots, and
// whose own ancestry reaches the enrolled terminal. The server's own pid
// cannot be walked, because it is started with setsid and so has no
// controlling terminal at all; the command edge proves over its parent for
// the same reason, and the relation "this pid is my parent" is established by
// os.Getppid rather than asserted by a caller.
func Prove(root, installation string, invokerPID int64, now time.Time) Authority {
	unproven := func(reason string) Authority {
		return Authority{root: root, reason: reason + "; " + Restart}
	}
	proof, err := humanauthority.Prove(root, invokerPID, nil, now)
	if err != nil || !proof.ValidFor(root) {
		return unproven(proofReason(proof, err))
	}
	// A helm proof lasts one act; the interface's proof outlives the helm.
	if proof.Helm != nil {
		return unproven("the process that started the interface was admitted only by the person at the helm, not proven at the enrolled terminal")
	}
	classification, classifyErr := lease.ClassifyVerbAt(root, installation, invokerPID)
	if classifyErr != nil {
		return unproven("the process that started the interface could not be classified (" + classifyErr.Error() + ")")
	}
	// The brain never carries a human's word. A checkout that declares
	// itself the brain refuses a named human from any caller the lease does
	// not see as a person, exactly as the command edge refuses it.
	if state := brain.Read(root, goal.ExistingLedgerIdentity(root)); state.State != brain.Undeclared {
		if state.State == brain.Corrupt {
			return unproven("this checkout's brain declaration is corrupt (" + state.Reason + ")")
		}
		if classification.Class != lease.ClassHuman {
			return unproven("this checkout is declared the brain, and the brain never carries a human's word into a goal verb")
		}
	}
	human, lineage, err := enrolledHuman(root, proof)
	if err != nil {
		return unproven(err.Error())
	}
	return Authority{
		proven: true, human: human, lineage: lineage, root: root,
		proof: proof, classification: classification,
	}
}

// Fixture is the headless equivalent of Prove for a fake-runtime checkout: it
// carries the same explicit fixture grant internal/goal's own tests carry, so
// a test can exercise the whole publication path without an enrolled
// terminal. It is bound to the exact root that declared metasystem.runtimes
// fake and cannot be obtained for a production checkout.
func Fixture(root, human, lineage string, now time.Time) (Authority, error) {
	authorization, err := fixtureauth.New(root)
	if err != nil {
		return Authority{}, err
	}
	proof, err := humanauthority.FixtureGoalProof(root, authorization.GoalHumanAuthority(), now)
	if err != nil {
		return Authority{}, err
	}
	if human == "" || lineage == "" {
		return Authority{}, fmt.Errorf("a fixture authority carries the human and the lineage it acts under")
	}
	return Authority{
		proven: true, human: human, lineage: lineage, root: root, proof: proof,
		classification: lease.ClassifyResult{Class: lease.ClassHuman},
	}, nil
}

// SessionLineage is the lineage every signed-in browser act carries. It is
// deliberately not the enrolled terminal's: a session is a different hand, and
// a hand that shared the terminal's lineage would be the terminal's claim
// holder for every verb that compares the pair.
const SessionLineage = "browser-session"

// SignedIn is the acting authority for one browser session a human signed
// into with the seat's one-time code.
//
// It carries the session's own proof rather than the boot proof, so the ledger
// records the session as the hand that acted: its History line names the
// issuer, the handle and the opaque session reference, and its approval record
// reads authority=session. The classification is the human class, as the
// headless fixture authority's is: there is no ancestry to classify, and the
// thing that was proven is that a human answered a one-time code.
func SignedIn(root, human, sessionRef string, proof humanauthority.Proof) (Authority, error) {
	if strings.TrimSpace(human) == "" || strings.TrimSpace(sessionRef) == "" {
		return Authority{}, fmt.Errorf("a signed-in session authority names its human and its session")
	}
	if !proof.SessionValidFor(root) {
		return Authority{}, fmt.Errorf("a signed-in session authority requires a freshly minted session proof for this checkout")
	}
	// The name and the session are not the caller's to assert. They are what
	// the proof was minted for, and they are what the ledger records, so a
	// caller that supplied either of them differently would be publishing
	// under a name the proof does not carry.
	if proof.ChannelUser != human || proof.ChannelRef != sessionRef {
		return Authority{}, fmt.Errorf("a signed-in session authority must name the human and the session its proof was minted for")
	}
	return Authority{
		proven: true, human: human, lineage: SessionLineage, root: root, proof: proof,
		classification: lease.ClassifyResult{Class: lease.ClassHuman},
	}, nil
}

// Unproven builds the refusing authority a server records when its boot
// observation found no human. The reason is what every route answers with.
func Unproven(reason string) Authority { return Authority{reason: reason} }

func (a Authority) Proven() bool { return a.proven }

// Human is the enrolled terminal's recorded name, which is the actor every
// act publishes under. It is empty when nothing was proven.
func (a Authority) Human() string { return a.human }

// Reason is why this server cannot act, in one sentence ending in the remedy.
func (a Authority) Reason() string { return a.reason }

// Line is what `metasystem ui status` prints and the server's record keeps.
func (a Authority) Line() string {
	if a.proven {
		return "acting as human:" + a.human + " — proven at the enrolled terminal"
	}
	return "not proven: " + a.reason
}

// Refusal is an answer a human acts on: what the act would have been, and why
// it was not made. Kind routes it to a status; Message is the engine's own
// words wherever the engine supplied them.
type Refusal struct {
	Kind    string
	Code    string
	Message string
}

func (r *Refusal) Error() string { return r.Message }

// The kinds a route maps to a status.
const (
	// KindUnproven is the server that cannot act as anybody.
	KindUnproven = "unproven"
	// KindRequest is the request's own fault: a budget that is not a budget,
	// an id that is not one.
	KindRequest = "request"
	// KindEngine is the ledger refusing the act in the state it is in.
	KindEngine = "engine"
	// KindFailed is the engine unable to answer at all.
	KindFailed = "failed"
)

func refuse(kind, code, message string) *Refusal {
	return &Refusal{Kind: kind, Code: code, Message: message}
}

// Approve publishes goal approve for one goal with the complete budget the
// human confirmed. The budget is never invented here: an incomplete tuple is
// refused rather than filled in.
func (a Authority) Approve(id string, budget goalbudget.Budget) error {
	if strings.TrimSpace(id) == "" {
		return refuse(KindRequest, "no-goal", "approval names one live goal")
	}
	if err := budget.Validate(); err != nil {
		return refuse(KindRequest, "budget", "the budget is not complete: "+err.Error())
	}
	request, done, err := a.request(id, "goal approve")
	if err != nil {
		return err
	}
	defer done()
	result, publishErr := goal.Approve(request, []string{id}, &budget, &a.proof)
	return a.settle(request, result, publishErr, "goal approve")
}

// Withdraw publishes goal unapprove for one goal. The engine decides what a
// withdrawal means in the goal's current state, including refusing one whose
// work has begun.
func (a Authority) Withdraw(id, because string) error {
	if strings.TrimSpace(id) == "" {
		return refuse(KindRequest, "no-goal", "a withdrawal names one live goal")
	}
	if strings.TrimSpace(because) == "" {
		because = "withdrawn from the board"
	}
	request, done, err := a.request(id, "goal unapprove")
	if err != nil {
		return err
	}
	defer done()
	result, publishErr := goal.Unapprove(request, id, because, &a.proof)
	return a.settle(request, result, publishErr, "goal unapprove")
}

// Park pauses one goal with the reason a human gave, and Unpark lifts a
// park. They are the interface's "Pause" and its undo, admitted under
// R-125-m1u: the engine takes this hand's session proof at the three rows the
// ruling names and at no others, so a goal another pair claimed between the
// page's read and this act is refused rather than displaced.
//
// The reason is required, exactly as the engine requires it: a pause without
// a why is a stall in disguise, and a default invented here would be a why
// nobody wrote.
func (a Authority) Park(id, because string) error {
	if strings.TrimSpace(id) == "" {
		return refuse(KindRequest, "no-goal", "a park names one live goal")
	}
	if strings.TrimSpace(because) == "" {
		return refuse(KindRequest, "no-reason", "park needs its reason — a pause without a why is a stall in disguise")
	}
	request, done, err := a.request(id, "goal park")
	if err != nil {
		return err
	}
	defer done()
	result, publishErr := goal.Park(request, id, because)
	return a.settle(request, result, publishErr, "goal park")
}

// Unpark returns a parked goal to the state it rests in: approved where its
// approval still stands, queued otherwise. The engine decides which, and
// refuses a park this hand may not lift.
func (a Authority) Unpark(id string) error {
	if strings.TrimSpace(id) == "" {
		return refuse(KindRequest, "no-goal", "an unpark names one live goal")
	}
	request, done, err := a.request(id, "goal unpark")
	if err != nil {
		return err
	}
	defer done()
	result, publishErr := goal.Unpark(request, id)
	return a.settle(request, result, publishErr, "goal unpark")
}

// Abandon publishes goal abandon for one goal: the reason it will never be
// worked, and the live goal carrying its work where there is one.
//
// It is admitted from a browser under R-128-ui, which extends R-125-m1u by
// this one verb: the engine takes this hand's session proof at abandon's own
// human row, and records the session on the History line it appends.
//
// The reason is required, exactly as the engine requires it: a goal that will
// never be worked owes the reader why, and a sentence invented here would be a
// why nobody wrote. The successor is optional and travels as the spec's
// Carried, which is what the engine calls it; where the goal has live
// dependents and no successor is named, the engine refuses in its own words,
// which name the terminal forms, and the card shows them. Neither --waive nor
// --also travels from here at all: releasing or abandoning somebody else's
// dependent is a separate judgement, made at a terminal, and an empty Waive
// and Also is how this hand says it is not making it.
func (a Authority) Abandon(id, because, successor string) error {
	if strings.TrimSpace(id) == "" {
		return refuse(KindRequest, "no-goal", "an abandon names one live goal")
	}
	if strings.TrimSpace(because) == "" {
		return refuse(KindRequest, "no-reason",
			"abandon needs its reason — a goal that will never be worked owes the reader why")
	}
	request, done, err := a.request(id, "goal abandon")
	if err != nil {
		return err
	}
	defer done()
	result, publishErr := goal.Abandon(request, id, goal.AbandonSpec{
		Because: strings.TrimSpace(because),
		Carried: strings.TrimSpace(successor),
	}, &a.proof)
	return a.settle(request, result, publishErr, "goal abandon")
}

// SetPriority publishes goal set-priority for one goal: the band it is to be
// in, and where in that band it stands.
//
// The sequence is a position within the destination band, one-based, and the
// engine refuses one outside `1..len(band)+1` rather than clamping it — which
// is why nothing is clamped here either. Inserting renumbers every goal at or
// after that position, in the band the goal leaves as well as the one it
// joins, so this verb is never about one record. A nil sequence appends,
// which is what the engine does when the command edge is given no --sequence.
//
// Nothing here reports where the goal landed: PublishResult carries the tip
// and the outcome and no rank. The caller reads the board again, which the
// act routes answer with, and finds the goal at whatever the ledger made of
// the request.
func (a Authority) SetPriority(id string, priority uint8, sequence *uint64) error {
	if strings.TrimSpace(id) == "" {
		return refuse(KindRequest, "no-goal", "a re-rank names one live goal")
	}
	if priority < 1 || priority > 3 {
		return refuse(KindRequest, "priority", "a priority is 1, 2, or 3")
	}
	if sequence != nil && *sequence < 1 {
		return refuse(KindRequest, "sequence", "a sequence is a one-based position within the priority")
	}
	request, done, err := a.request(id, "goal set-priority")
	if err != nil {
		return err
	}
	defer done()
	result, publishErr := goal.SetPriority(request, id, priority, sequence, &a.proof)
	return a.settle(request, result, publishErr, "goal set-priority")
}

// Opened is one goal as a human stated it at intake.
//
// Every field is one the verb takes, and there is no field here the verb does
// not take. In particular there is no priority and no arc: `goal open` has no
// flag for either (cmd/metasystem/goalsync_mutations.go:1716), and a sheet
// that offered them would be promising a record this act cannot write. The
// risk answers and their basis are not optional decoration — the engine
// derives the rigor tier from them and refuses an open without them.
type Opened struct {
	ID       string
	Intent   string
	NextStep string
	// Tier overrides the tier the risk answers derive. Zero takes the derived
	// one, which is the usual case; anything else needs Why.
	Tier uint8
	Why  string
	// Blocks names the goals that will wait for this one, and BlockedBy the
	// goals this one will wait for. Both are lists because the relation is
	// one: a goal can be the blocker of several and can wait for several, and
	// a sheet that took one id each way would be offering half of what the
	// verb takes.
	Blocks    []string
	BlockedBy []string
	Labels    []string
	Risk      goal.RiskRecord
}

// Open publishes goal open for one new goal, under origin `human`.
//
// The origin is not the caller's to choose. This server acts as the enrolled
// human and as nobody else, so every goal it opens is the human's own intake
// act and carries the origin that says so; a seat's open is confined to the
// blocker of its claimed goal and is not something a browser performs.
func (a Authority) Open(opened Opened) error {
	if strings.TrimSpace(opened.ID) == "" {
		return refuse(KindRequest, "no-goal", "a new goal is named by one id")
	}
	if strings.TrimSpace(opened.Intent) == "" {
		return refuse(KindRequest, "no-intent", "a goal's intent says what done looks like, in one line")
	}
	if strings.TrimSpace(opened.NextStep) == "" {
		return refuse(KindRequest, "no-next-step",
			"a goal's next step states intent, constraints and freedoms, never a script of the how")
	}
	if err := opened.Risk.Validate(); err != nil {
		return refuse(KindRequest, "risk", "the risk answers are not complete: "+err.Error())
	}
	if opened.Tier > 3 {
		return refuse(KindRequest, "tier", "a rigor tier is 1, 2, or 3")
	}
	request, done, err := a.request(opened.ID, "goal open")
	if err != nil {
		return err
	}
	defer done()
	result, publishErr := goal.OpenRisked(request, opened.ID, opened.Intent, goal.OriginHuman,
		opened.NextStep, opened.Blocks, opened.BlockedBy, opened.Risk, opened.Tier, opened.Why, nil, &a.proof, opened.Labels...)
	return a.settle(request, result, publishErr, "goal open")
}

// Edited is what a browser may change on a goal nobody has approved yet: its
// intent, its next step, and its labels.
//
// Every field is a pointer because absence and emptiness are two different
// statements. A field nobody touched in the sheet is not sent at all, so a
// terminal edit of that same field survives this save; a label list a human
// emptied is sent as an empty list, and clears the labels. There is no tier
// here and no risk, no dependency, no why and no evidence: tier and risk
// enter the approval digest and dependencies change other goals' readiness,
// and neither is a direct edit the master design admits.
type Edited struct {
	Intent   *string
	NextStep *string
	Labels   *[]string
}

// Edit publishes goal edit for one queued goal, under the allowlist the
// mutation itself holds.
//
// The flag is set here and always: this hand edits a queued unapproved goal
// or it edits nothing, and a goal approved or claimed between the page's read
// and this act is refused at the tip in the engine's own words rather than
// being displaced. Only the three fields travel, and only the ones the sheet
// says changed.
func (a Authority) Edit(id string, edited Edited) error {
	if strings.TrimSpace(id) == "" {
		return refuse(KindRequest, "no-goal", "an edit names one live goal")
	}
	if edited.Intent == nil && edited.NextStep == nil && edited.Labels == nil {
		return refuse(KindRequest, "no-change",
			"an edit changes at least one of the intent, the next step or the labels")
	}
	if edited.Intent != nil && strings.TrimSpace(*edited.Intent) == "" {
		return refuse(KindRequest, "no-intent", "a goal's intent says what done looks like, in one line")
	}
	if edited.NextStep != nil && strings.TrimSpace(*edited.NextStep) == "" {
		return refuse(KindRequest, "no-next-step",
			"a goal's next step states intent, constraints and freedoms, never a script of the how")
	}
	request, done, err := a.request(id, "goal edit")
	if err != nil {
		return err
	}
	defer done()
	result, publishErr := goal.Edit(request, id, goal.EditFields{
		QueuedOnly: true,
		Intent:     edited.Intent,
		NextStep:   edited.NextStep,
		Labels:     edited.Labels,
	})
	return a.settle(request, result, publishErr, "goal edit")
}

// Block publishes goal block: the dependent waits for the blocker from now on.
//
// The path the route reads names the dependent both ways round, because the
// goal page shows the relation from both ends and only one of the two ends
// owns the edge: a row in "Holds" on G's page acts on X's route with G as the
// blocker, so there is one mutation and not a second, mirrored one.
func (a Authority) Block(dependent, blocker string) error {
	if strings.TrimSpace(dependent) == "" || strings.TrimSpace(blocker) == "" {
		return refuse(KindRequest, "no-goal", "an edge names the goal that waits and the goal it waits for")
	}
	if strings.TrimSpace(dependent) == strings.TrimSpace(blocker) {
		return refuse(KindRequest, "self-edge", "a goal cannot wait for itself")
	}
	request, done, err := a.request(dependent, "goal block")
	if err != nil {
		return err
	}
	defer done()
	result, publishErr := goal.Block(request, dependent, blocker, &a.proof)
	return a.settle(request, result, publishErr, "goal block")
}

// Unblock publishes goal unblock. Removing an edge whose blocker is not done
// is an early lift, which the engine admits under this hand's own proof — the
// browser session where a human signed in, the boot proof otherwise — and
// refuses outright where neither is a person's.
func (a Authority) Unblock(dependent, blocker string) error {
	if strings.TrimSpace(dependent) == "" || strings.TrimSpace(blocker) == "" {
		return refuse(KindRequest, "no-goal", "an edge names the goal that waits and the goal it waits for")
	}
	request, done, err := a.request(dependent, "goal unblock")
	if err != nil {
		return err
	}
	defer done()
	result, publishErr := goal.Unblock(request, dependent, blocker, &a.proof)
	return a.settle(request, result, publishErr, "goal unblock")
}

// alreadyCarried names the acts whose repeat the engine answers with an
// explicit no-op — the one outcome that is abandoned with no error, which is
// NothingToDo (internal/goal/txn.go terminalFromMutate).
//
// It is R-129-ui's list, and every one of them was read in the engine before it
// was named here:
//
//   - approve: the goal already carries this exact budget under this exact
//     authority, unexpired (approval.go);
//   - edit: the intent, the next step and the labels already read exactly this
//     way, under this signed-in session (verbs.go);
//   - set-priority: the requested priority and sequence already hold
//     (order.go);
//   - block and unblock: the edge is already there, or already gone
//     (verbs.go);
//   - park: the goal is already parked, whatever reason the repeat carries,
//     because the pause is the effect and the reason is not (verbs.go);
//   - unpark: the goal is not parked, which is the state a resume asks for
//     (verbs.go);
//   - abandon: the goal is already abandoned — never merely archived, because a
//     done goal is archived too — and to the successor this act names
//     (abandon.go);
//   - unapprove: the goal is LIVE and already carries no approval, which is the
//     state a withdrawal asks for; a goal that has left the live tree keeps the
//     engine's own refusal (approval.go);
//   - open: the goal the id already names reads exactly what this open states —
//     the same intent, next step, tier and labels, the same goals it waits for,
//     and the edge on every goal it names with --blocks (verbs.go).
//   - a sitting's hold and release: this human's sitting already holds the
//     goal on this record, or no sitting of this human's stands (landgate.go);
//   - land without a sitting: this human's decision already stands at this
//     tip (landgate.go).
//
// Park, unpark and abandon used to be excluded: park and abandon answered
// LostToCompetitor, naming the operation that got there first, and unpark
// refused a goal that is not parked in its own words, so all three reached the
// page as refusals of an act the human had already made. R-129-ui closed that —
// an act whose effect already holds is success — and the engine now answers
// each of them NothingToDo under a browser session, which is the one hand that
// can press one card twice.
//
// Withdraw and open were excluded for one round longer, on a reading of the
// ruling's examples as its whole list. They are the two acts whose repeat is
// decided by a COMPARISON rather than by the target's mere existence, and the
// engine now makes it: unapprove reads the live goal's own approval, and open
// reads the record the id already names against what the request states. A
// request that asks for anything else is a different act and still loses, so
// nothing here swallows it — this map only turns the engine's no-op into
// applied, and OutcomeLost and OutcomeRejected still reach the page as
// refusals.
var alreadyCarried = map[string]bool{
	"goal approve":      true,
	"goal edit":         true,
	"goal set-priority": true,
	"goal block":        true,
	"goal unblock":      true,
	"goal park":         true,
	"goal unpark":       true,
	"goal abandon":      true,
	"goal unapprove":    true,
	"goal open":         true,
	"goal review":       true,
	// The landing gate's acts (g1-s70).
	"goal review --hold":        true,
	"goal review --release":     true,
	"goal land-without-sitting": true,
}

// settle turns one publication into the answer a route gives, and records the
// authority proof beside the act exactly as the command edge does. A proof
// that cannot be written after the act landed is reported as such: the act is
// not undone and must not be retried.
func (a Authority) settle(request goal.VerbRequest, result goal.PublishResult, publishErr error, action string) error {
	operation := goal.Opid(request.Ulid, request.Actor.Machine, request.Actor.Lineage)
	if publishErr != nil {
		return a.unsettled(operation, publishErr)
	}
	if result.Outcome != goal.OutcomeConfirmed {
		// An act the engine found nothing to do for is an act whose effect
		// already stands, and that is the act HAVING its effect: the second
		// press of one proposal's Apply asked for a state the ledger is
		// already in, so the page is told applied rather than refused.
		// Nothing new landed, so no second proof is recorded beside it.
		if alreadyCarried[action] && result.Outcome == goal.OutcomeAbandoned {
			return nil
		}
		detail := result.Detail
		if detail == "" {
			detail = "the ledger did not confirm " + action
		}
		return refuse(KindEngine, string(result.Outcome), detail)
	}
	// The evidence beside the act is the proof that carried it, which for a
	// browser session is a session proof rather than an ancestry.
	record := humanauthority.RecordProof
	if a.proof.Outcome == humanauthority.OutcomeSession {
		record = humanauthority.RecordSessionProof
	}
	if err := record(a.root, operation, action, a.proof); err != nil {
		return refuse(KindFailed, "proof-not-recorded",
			"the act landed at tip "+result.Tip+", but its authority proof did not: "+err.Error()+"; do not run it again")
	}
	return nil
}

// unsettled is what a publication error means, read from this operation's own
// journal entry rather than assumed.
//
// It used to be assumed, and the assumption was wrong in the one case that
// matters. A push can LAND and then fail its confirmation — the refetch fails,
// the opid is not visible on the refetched tip, or a confirmed follow-on effect
// fails — and the transaction returns an empty outcome with an error whose
// detail begins "pushed;" (internal/goal/txn.go runTransaction). Calling that
// refused told a human, and the Partner, that an approval had not been made when
// it had, and offered them the one press that would make a second one.
//
// The journal knows the difference, because it is written before the push leaves
// this process: an entry still at pushed with no terminal outcome is an act
// whose fate nobody here can state. So:
//
//   - pushed and not terminal is UNRESOLVED, under `pushed-unknown`. The page
//     stops the run there and reads the ledger; nothing is sent again.
//   - an entry that cannot be read AT ALL is unresolved too, under
//     `journal-unreadable`, because one persistent journal fault fails
//     MarkTerminal and this read alike, and nothing then proves the push did not
//     land. Assuming it did not is the one assumption that invites a repeat.
//   - an ABSENT entry, one still at created, and a terminal one are refused, as
//     they always were: the push never left, or it was refused, and the outcome
//     the entry carries says which.
//
// Both new codes are failures rather than engine refusals, so the routes answer
// 500 and the page's mapping calls them unresolved.
func (a Authority) unsettled(operation string, publishErr error) error {
	entry, err := goal.ReadEntry(a.root, operation)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return refuse(KindEngine, "refused", publishErr.Error())
	case err != nil:
		return refuse(KindFailed, "journal-unreadable",
			"whether it landed is unresolved: this act's own journal entry could not be read ("+
				err.Error()+"); check the goal before acting again")
	case entry.Phase == goal.PhasePushed && entry.Outcome == "":
		return refuse(KindFailed, "pushed-unknown",
			"pushed; whether it landed is unresolved: "+publishErr.Error()+
				"; the page's next read says what the ledger did")
	default:
		return refuse(KindEngine, "refused", publishErr.Error())
	}
}

// request takes this act for this process and assembles the engine request it
// will publish.
//
// Taking it is the first thing that happens, before any read: while an act on
// one goal is executing here, a second request for that same goal and that
// same act is refused at once (owner.begin). The one lock over this clone's
// ledger is taken with it and held until the act answers, so the publication,
// its settlement, and the journal recovery this request runs before it
// publishes are one critical section that a concurrent recovery cannot enter.
// The returned release clears both, whatever the act answered.
func (a Authority) request(id, action string) (goal.VerbRequest, func(), error) {
	if !a.proven {
		return goal.VerbRequest{}, nil, refuse(KindUnproven, "unproven", a.reason)
	}
	held := ownerOf(a.root)
	endHold, err := held.begin(id, action)
	if err != nil {
		return goal.VerbRequest{}, nil, err
	}
	held.publications.Lock()
	release := func() {
		held.publications.Unlock()
		endHold()
	}
	// Both holds are cleared here until the caller has them, because between
	// taking them and returning them there is no `defer done()` anywhere: the
	// caller installs its own only once this function has returned. An assembly
	// reader that UNWINDS rather than answering would otherwise strand the
	// registration and this clone's one lock until the process restarted, and
	// every act on that goal and every publication on that clone with them
	// (Astra E-04). The flag is what transfers ownership: on any error, and on
	// any panic, this releases; on the one path that hands the release back, it
	// does not.
	transferred := false
	defer func() {
		if !transferred {
			release()
		}
	}()
	request, err := a.assemble()
	if err != nil {
		return goal.VerbRequest{}, nil, err
	}
	transferred = true
	return request, release, nil
}

// assemble builds the same goal.VerbRequest the command edge assembles for
// a human verb: this checkout's endpoint and machine, the enrolled terminal's
// lineage and name, the boot-time caller classification, a fresh operation
// identifier and the checkout's clock.
func (a Authority) assemble() (goal.VerbRequest, error) {
	if err := a.reads.ensureFence(a.root); err != nil {
		return goal.VerbRequest{}, refuse(KindFailed, "no-fence", err.Error())
	}
	endpoint, err := a.reads.resolveEndpoint(a.root)
	if err != nil {
		return goal.VerbRequest{}, refuse(KindFailed, "no-endpoint", err.Error())
	}
	endpoint.ConfigureCarriedCounselorAppend(func(root, _ string, row goal.HistoryLine, _ time.Time) error {
		line, lineErr := counselor.CarriedLandingLine(row)
		if lineErr != nil {
			return lineErr
		}
		return counselor.AppendCarriedLanding(root, line)
	})
	// An unclassified push wedges this clone: the engine refuses every
	// mutation while one stands, and nothing in the browser could reach the
	// classification. So this press makes it, through the engine's own
	// recovery rule, before it publishes. It is the rule Reconcile runs, run
	// here inside the lock this request already holds rather than through
	// Reconcile, which takes that same lock for a Refresh.
	if err := a.reads.reconcile(a.root, endpoint); err != nil {
		return goal.VerbRequest{}, err
	}
	machine, err := a.reads.resolveMachine(a.root)
	if err != nil {
		return goal.VerbRequest{}, refuse(KindFailed, "no-machine", err.Error())
	}
	ulid, err := goal.NewOperationULID()
	if err != nil {
		return goal.VerbRequest{}, refuse(KindFailed, "no-operation-id", err.Error())
	}
	now, err := Now(a.root)
	if err != nil {
		return goal.VerbRequest{}, refuse(KindFailed, "no-clock", err.Error())
	}
	request := goal.VerbRequest{
		Endpoint: endpoint,
		// The branch-safety check a park runs, which is the command edge's
		// own: a goal whose branch is not on origin is a goal a park would
		// strand. It is carried on every request because it costs nothing
		// until a park calls it, and only a park does.
		ParkBranchCheck: a.reads.parkBranchCheck(a.root, endpoint),
		Actor:           goal.Actor{Machine: machine, Lineage: a.lineage, Human: a.human},
		// The proof travels as the request's authority, never as a name: a
		// human name without this object authorizes nothing.
		Authority:   &a.proof,
		Ulid:        ulid,
		Now:         now,
		CallerClass: a.classification.Class,
	}
	// The epoch is the checkout lease's as the boot observation found it.
	// Neither of these two verbs creates a claimed revision — approve admits
	// queued or parked work and unapprove withdraws an approval — so the
	// epoch is carried for the record rather than consumed.
	if a.classification.Holder && a.classification.ClaimEpoch != nil {
		request.EpochAuthority = goal.EpochAuthorityHolder
	}
	switch {
	case a.classification.ClaimEpoch != nil:
		request.ClaimEpoch = *a.classification.ClaimEpoch
	case a.classification.Class == lease.ClassHuman:
		request.ClaimEpoch = 1
	}
	return request, nil
}

// Now is the checkout's clock: the wall clock, unless the root explicitly
// declares the fake runtime and stages an instant, which is how a fixture
// ledger publishes at a fixed time.
func Now(root string) (time.Time, error) {
	authorization, err := fixtureauth.New(root)
	if err != nil {
		return time.Time{}, err
	}
	staged, ok, err := authorization.Clock().GoalNow()
	if err != nil {
		return time.Time{}, err
	}
	if ok {
		return staged, nil
	}
	return time.Now().UTC(), nil
}

// enrolledHuman derives the acting name exactly as the command edge derives
// it when no --by is given: from the enrolled terminal's own record, checked
// against the proof that was just observed. The lineage comes from the same
// enrollment, so the act's actor and its lineage name one terminal.
func enrolledHuman(root string, proof humanauthority.Proof) (human, lineage string, err error) {
	const missing = "the enrolled terminal has no recorded name"
	if !proof.EnrolledTerminalFor(root) && !(proof.FixtureOnly && proof.ValidFor(root)) {
		return "", "", fmt.Errorf("%s", missing)
	}
	enrollment, readErr := humanauthority.ReadEnrollment(root)
	if readErr != nil || strings.TrimSpace(enrollment.Human) == "" {
		return "", "", fmt.Errorf("%s", missing)
	}
	if proof.EnrolledTerminalFor(root) &&
		(proof.TerminalGeneration != enrollment.Generation || proof.TerminalRef != enrollment.TerminalRef) {
		return "", "", fmt.Errorf("%s", missing)
	}
	return enrollment.Human, terminalLineage(enrollment), nil
}

// terminalLineage renders the enrolled terminal's lineage the way the command
// edge renders it, so an act from the browser and an act from the terminal
// carry one lineage.
func terminalLineage(enrollment humanauthority.Enrollment) string {
	identifier := []byte(enrollment.TerminalID)
	for index, character := range identifier {
		if !('A' <= character && character <= 'Z') && !('a' <= character && character <= 'z') &&
			!('0' <= character && character <= '9') && character != '-' {
			identifier[index] = '-'
		}
	}
	return fmt.Sprintf("terminal-%s-%d", identifier, enrollment.Generation)
}

// proofReason says what the walk found, in the words a human acts on. The
// agent case names the runtime the census recognized, because that is the one
// fact that tells a human which window they are in.
func proofReason(proof humanauthority.Proof, err error) string {
	switch proof.Outcome {
	case humanauthority.OutcomeAgent:
		return "the interface was started by an agent process (" + agentRuntime(proof) + ")"
	case humanauthority.OutcomeNotEnrolled:
		return "this checkout has no enrolled terminal (run: bin/metasystem system enroll --name <your name>)"
	case humanauthority.OutcomeTerminalMissing:
		return "the process that started the interface does not descend from the enrolled terminal"
	case humanauthority.OutcomeChanged, humanauthority.OutcomeReused:
		return "the ancestry of the process that started the interface changed while it was read"
	case humanauthority.OutcomeUnreadable, humanauthority.OutcomeArgvUnreadable:
		return "the ancestry of the process that started the interface could not be read"
	case humanauthority.OutcomeCycle:
		return "the process ancestry of the interface is a cycle"
	}
	if err != nil {
		return "human authority could not be observed at boot: " + err.Error()
	}
	return "human authority could not be observed at boot"
}

func agentRuntime(proof humanauthority.Proof) string {
	for _, node := range proof.Nodes {
		if node.AgentRuntime != nil {
			return *node.AgentRuntime
		}
	}
	return "unnamed runtime"
}

// ParentPID is the pid Prove walks from in production: this process's parent,
// which while the server boots is the launcher the human ran.
func ParentPID() int64 { return int64(os.Getppid()) }
