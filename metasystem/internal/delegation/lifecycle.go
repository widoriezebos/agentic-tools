package delegation

import (
	"context"
	"errors"
	"fmt"
)

// Phase is one public command of the delegate lifecycle. The set mirrors
// scripts/agents/dispatch.sh's command router one for one.
type Phase string

const (
	PhaseDispatch Phase = "dispatch"
	PhaseFollowUp Phase = "follow-up"
	PhaseWatch    Phase = "watch"
	PhaseStatus   Phase = "status"
	PhaseCancel   Phase = "cancel"
	PhaseClose    Phase = "close"
	PhaseReap     Phase = "reap"
)

// Phases lists the lifecycle's phases in dispatch.sh's router order.
func Phases() []Phase {
	return []Phase{PhaseDispatch, PhaseWatch, PhaseFollowUp, PhaseStatus, PhaseCancel, PhaseClose, PhaseReap}
}

// ErrNotPorted is every phase's answer until U6b ports it. A caller wired to
// the skeleton early refuses instead of reporting a launch that never ran.
var ErrNotPorted = errors.New("the delegate lifecycle is not ported yet (verbs-object-action U6b)")

// OutcomeNotPorted is the outcome word a skeleton phase reports.
const OutcomeNotPorted = "NOT-PORTED"

// Outcome is a phase's typed result, the shape dispatch.sh writes to
// METASYSTEM_DELEGATE_OUTCOME_FILE plus the exit code it returns.
type Outcome struct {
	Phase    Phase
	Outcome  string
	Headline string
	Detail   string
	JobID    string
	ExitCode int
}

// DispatchRequest is dispatch_job's selection (its flags).
type DispatchRequest struct {
	Invocation        Invocation
	Role              string
	Brief             string
	Mode              string
	Runtime           string
	Model             string
	JobID             string
	Reviews           string
	Outputs           string
	Design            string
	Workspace         string
	Worktree          bool
	Permissions       string
	Mission           string
	Stream            string
	Goal              string
	DestructiveReach  string
	CapMin            string
	ApprovedRef       string
	Sources           []string
	ApproveEscalation bool
	ServingGoal       bool
	StewardIntent     string
	Wait              bool
}

// FollowUpRequest is follow_up's selection.
type FollowUpRequest struct {
	Invocation  Invocation
	Job         string
	Message     string
	OperationID string
	ApprovedRef string
	Wait        bool
}

// CloseRequest is close_chain's selection.
type CloseRequest struct {
	Invocation        Invocation
	Job               string
	RunnerClosed      bool
	ReconcileEvidence string
}

// ReapRequest is reap_jobs's selection; an empty Job sweeps every record.
type ReapRequest struct {
	Invocation Invocation
	Job        string
	PostWait   bool
}

// Lifecycle is the composed delegate lifecycle. It owns no decision of its
// own that an owner already makes; it sequences owner operations in
// dispatch.sh's order.
type Lifecycle struct {
	ports Ports
}

// New composes a lifecycle over a complete port set.
func New(ports Ports) (*Lifecycle, error) {
	if err := ports.Validate(); err != nil {
		return nil, fmt.Errorf("delegation lifecycle refused construction: %w", err)
	}
	return &Lifecycle{ports: ports}, nil
}

// Ports returns the wired operations (for a caller composing a sibling
// action over the same owners).
func (l *Lifecycle) Ports() Ports { return l.ports }

func notPorted(phase Phase, job string) (Outcome, error) {
	return Outcome{
		Phase: phase, Outcome: OutcomeNotPorted, Headline: "refused",
		Detail: ErrNotPorted.Error(), JobID: job, ExitCode: 2,
	}, fmt.Errorf("%s: %w", phase, ErrNotPorted)
}

// Dispatch is dispatch_job then finalize_and_launch. Its steps, in order:
// brain fence; engine skew preflight; steward authorization (steward intent)
// or selection; open process-creation fence; source validate-only compose;
// checkout execution guard; review-subject and design-critic checks; brief
// mode; serving goal; lease entry check; roster, mission and escalation;
// permission preset and worktree choice; goal binding and tier ladder;
// operation id; brief authority; goal-free admission; census freshness; plan
// drift report; launch chain lock; goal revision lock; worktree and
// quarantine; read subject and critique read admission; testing requirement;
// permission expansion; capability snapshot; brief appendices; cap authority
// lock and cap authorization; role packet composition; inline input limit;
// output stream; claim-launch preflight; goal revision, goal and slice
// admission; lifecycle lock; claim occupancy and claim-launch; round
// payload; record build; record setup; adapter launch with ownership CAS and
// start gate; lock release; fence after launch; creation claim close;
// handshake wait; optional wait.
func (l *Lifecycle) Dispatch(ctx context.Context, request DispatchRequest) (Outcome, error) {
	return notPorted(PhaseDispatch, request.JobID)
}

// FollowUp is follow_up then finalize_and_launch: the chain's newest record
// decides between a fresh round, a repeated wrapper, a continuation after a
// cap, and an examination retry; a worktree chain may be rebased onto trunk;
// critic chains fold their register and carry open finding ids; then the
// same authorize, compose, claim, record and launch tail as Dispatch.
func (l *Lifecycle) FollowUp(ctx context.Context, request FollowUpRequest) (Outcome, error) {
	return notPorted(PhaseFollowUp, request.Job)
}

// Watch is watch_job: block on a job to terminal through the job watch owner.
func (l *Lifecycle) Watch(ctx context.Context, job string) (Outcome, error) {
	return notPorted(PhaseWatch, job)
}

// Status is status_job: the record's status and the census verdict.
func (l *Lifecycle) Status(ctx context.Context, job string) (Outcome, error) {
	return notPorted(PhaseStatus, job)
}

// Cancel is cancel_job and internal_cancel: a record that never published a
// process is concluded directly; a launched one is cancelled through its
// runtime adapter, marking the record before winding down its group.
func (l *Lifecycle) Cancel(ctx context.Context, invocation Invocation, job string) (Outcome, error) {
	return notPorted(PhaseCancel, job)
}

// Close is close_chain, the chain's finish: mirror every terminal member,
// fold and close a critic register, close-check, then mark the root closed
// and remove the chain's build cache.
func (l *Lifecycle) Close(ctx context.Context, request CloseRequest) (Outcome, error) {
	return notPorted(PhaseClose, request.Job)
}

// Reap is reap_jobs and reap_one_locked: under the lease, conclude records
// whose process is lost or whose budget expired, recollect a lost round's
// complete return, aggregate usage, mirror evidence and remove terminal
// chains' build caches.
func (l *Lifecycle) Reap(ctx context.Context, request ReapRequest) (Outcome, error) {
	return notPorted(PhaseReap, request.Job)
}
