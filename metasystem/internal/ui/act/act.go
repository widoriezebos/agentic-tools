// Package act is the interface server's human hand on the ledger.
//
// Four verbs reach it, and every one of them is a verb a human performs on
// their own backlog: goal approve and goal unapprove admit and withdraw work,
// goal set-priority places a goal in a band and orders it there, and goal
// open is the intake act that creates one. Nothing here shells out. A
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
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/brain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/counselor"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/fixtureauth"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
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
	request, err := a.request()
	if err != nil {
		return err
	}
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
	request, err := a.request()
	if err != nil {
		return err
	}
	result, publishErr := goal.Unapprove(request, id, because, &a.proof)
	return a.settle(request, result, publishErr, "goal unapprove")
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
	request, err := a.request()
	if err != nil {
		return err
	}
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
	Tier   uint8
	Why    string
	Blocks string
	Labels []string
	Risk   goal.RiskRecord
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
	request, err := a.request()
	if err != nil {
		return err
	}
	result, publishErr := goal.OpenRisked(request, opened.ID, opened.Intent, goal.OriginHuman,
		opened.NextStep, opened.Blocks, opened.Risk, opened.Tier, opened.Why, nil, &a.proof, opened.Labels...)
	return a.settle(request, result, publishErr, "goal open")
}

// settle turns one publication into the answer a route gives, and records the
// authority proof beside the act exactly as the command edge does. A proof
// that cannot be written after the act landed is reported as such: the act is
// not undone and must not be retried.
func (a Authority) settle(request goal.VerbRequest, result goal.PublishResult, publishErr error, action string) error {
	if publishErr != nil {
		return refuse(KindEngine, "refused", publishErr.Error())
	}
	if result.Outcome != goal.OutcomeConfirmed {
		detail := result.Detail
		if detail == "" {
			detail = "the ledger did not confirm " + action
		}
		return refuse(KindEngine, string(result.Outcome), detail)
	}
	operation := goal.Opid(request.Ulid, request.Actor.Machine, request.Actor.Lineage)
	if err := humanauthority.RecordProof(a.root, operation, action, a.proof); err != nil {
		return refuse(KindFailed, "proof-not-recorded",
			"the act landed at tip "+result.Tip+", but its authority proof did not: "+err.Error()+"; do not run it again")
	}
	return nil
}

// request assembles the same goal.VerbRequest the command edge assembles for
// a human verb: this checkout's endpoint and machine, the enrolled terminal's
// lineage and name, the boot-time caller classification, a fresh operation
// identifier and the checkout's clock.
func (a Authority) request() (goal.VerbRequest, error) {
	if !a.proven {
		return goal.VerbRequest{}, refuse(KindUnproven, "unproven", a.reason)
	}
	if err := ledgerfence.Ensure(a.root); err != nil {
		return goal.VerbRequest{}, refuse(KindFailed, "no-fence", err.Error())
	}
	endpoint, err := goal.ResolveEndpoint(a.root)
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
	machine, err := goal.ResolveMachine(a.root)
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
		Actor:    goal.Actor{Machine: machine, Lineage: a.lineage, Human: a.human},
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
		return "this checkout has no enrolled terminal (run: bin/metasystem goal enroll-terminal --by <your name>)"
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
