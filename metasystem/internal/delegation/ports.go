package delegation

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/census"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gaterun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/wallclock"
)

// Invocation is the explicit invocation context of design 6.2: the supplied
// process identity classification starts from, and the authority the entry
// check established for it. A call edge that replaces a dispatch.sh child
// supplies the current process, because that is the parent the child
// classified (VOA-02-R2). Nothing below reads os.Getppid or inherited
// environment to rediscover it.
type Invocation struct {
	// CallerPid is the supplied identity (dispatch.sh's entry_caller_pid).
	CallerPid int64
	// ClaimEpoch is the lease claim epoch the entry check read; nil for a
	// HUMAN or STEWARD caller, which hold no epoch (current_claim_epoch).
	ClaimEpoch *int64
	// MainID is the holder's main id (current_main_id); empty when none.
	MainID string
	// CallerClass is the entry classification (current_caller_class).
	CallerClass string
}

// AuthorityMode is a control-plane write mode of the authority matrix
// (internal/authority): the modes dispatch.sh's internal entries check.
type AuthorityMode string

const (
	AuthorityHolderOnly    AuthorityMode = "holder-only"
	AuthorityRecordWriter  AuthorityMode = "record-writer"
	AuthorityAdapterWriter AuthorityMode = "adapter-writer"
	AuthorityStopCustodian AuthorityMode = "stop-custodian"
)

// LeaseOps is the checkout-lease owner as the lifecycle uses it:
// lease_entry_check (RequireHolder), lease_run_held (Held), internal_authority
// (Classify + Authorize), and the holder's renewal.
type LeaseOps interface {
	// Classify reports who the supplied caller is (lease classify).
	Classify(inv Invocation) (lease.ClassifyResult, error)
	// RequireHolder gates a write on the authenticated holder (lease
	// require-holder); expectedEpoch nil means any epoch.
	RequireHolder(inv Invocation, expectedEpoch *int64) (lease.HolderView, error)
	// Renew bumps the holder's lease revision (lease renew).
	Renew(inv Invocation) (lease.RenewResult, error)
	// Held runs fn under the lease lock, gated as run-held gates its child.
	Held(inv Invocation, expectedEpoch *int64, fn func() error) error
	// Authorize checks the caller against the authority matrix for mode;
	// job names the record a record-mutating write touches (may be empty).
	Authorize(inv Invocation, mode AuthorityMode, job string) error
}

// StewardOps is the steward owner: the unattended continuation's
// authorization (dispatch --steward-intent, steward authorize-dispatch).
type StewardOps interface {
	AuthorizeDispatch(inv Invocation, intent string) (steward.DispatchAuthorization, error)
}

// ExecutionGuard names the checkout execution guard a launched adapter joins
// (the retired checkout-execution-guard.sh run-member wrapper plus supervise
// launch-detached --execution-guard-root/--execution-guard-owner).
type ExecutionGuard struct {
	Root  string
	Owner string
}

// AdapterLaunch is one adapter process start (launch_adapter's argv).
type AdapterLaunch struct {
	Runtime          string
	Verb             string // dispatch or follow-up
	Job              string
	StartGate        string
	InstanceTag      string
	LaunchCapability string
	ExecutionGuard   *ExecutionGuard
}

// AdapterOps is the runtime adapter owner as the dispatcher drives it: the
// engine's delegate-supervisor entry (internal/adapter/supervisor), whose
// verbs are the ones the retired dispatch.sh invoked on it.
type AdapterOps interface {
	// Installed reports whether the installation resolves the runtime to an
	// adapter at all (the entry's signature read succeeds).
	Installed(runtime string) bool
	ConfigIdentity(ctx context.Context, runtime string) (string, error)
	Probe(ctx context.Context, runtime string) error
	OutputStream(ctx context.Context, runtime, roundDir string) (string, error)
	// Launch starts the adapter detached in its own session and returns the
	// supervisor pid (which is also its process group id).
	Launch(ctx context.Context, request AdapterLaunch) (int64, error)
	// Cancel asks the runtime's adapter to cancel a launched job.
	Cancel(ctx context.Context, runtime, job string) error
	// ResultPatch writes an {error,phase,usage} record patch (adapter
	// result-patch); failure "null" writes a JSON null error.
	ResultPatch(outputPath, failure, phase, usagePath string) error
}

// GoalBinding is a claimed goal's stop-capability binding (job goal-binding).
type GoalBinding struct {
	GoalID     string
	Revision   uint64
	Tier       uint8
	GateWidth  string
	Machine    string
	Lineage    string
	Capability goal.StopCapability
	Fenced     bool
}

// GoalOps is the goal owner as the lifecycle binds a delegate operation to
// an accepted, claimed goal revision.
type GoalOps interface {
	Binding(goalID string) (GoalBinding, error)
	// LedgerIdentity is the accepted goal ledger's identity, empty when the
	// checkout has none; the brain fence judges a declaration against it.
	LedgerIdentity() string
	// BreachStop closes the exact breached revision's fence and creates its
	// resumable batch (dispatch.EnsureBreachStop). orderedBy names the person
	// a human-ordered stop records as its actor (rule H1); empty records the
	// stop custodian.
	BreachStop(goalID string, revision uint64, now time.Time, orderedBy string) (goal.StopBatch, error)
}

// RecordOps is the job-record owner: reservation, setup, and the one
// compare-and-swap every lifecycle transition goes through.
type RecordOps interface {
	Create(job, sourcePath string) error
	Setup(job, sourcePath string) error
	// CAS returns the observed status on a lost compare.
	CAS(job, expect, target, patchPath string) (observed string, err error)
	Read(job string) (map[string]any, error)
}

// EventOps is the flight recorder. Emission never fails its caller.
type EventOps interface {
	Emit(event, summary string, fields map[string]string)
}

// ProcessOps is the kernel as the lifecycle observes and signals it: the
// liveness ladder of `proc classify`, existence probes, custody groups and
// signals. Tests supply a scripted process table.
type ProcessOps interface {
	// TagState answers live, stale, dead, or unknown for a recorded pid and
	// instance tag (proc classify).
	TagState(pid int64, tag string) string
	// Exists reports a pid exists (proc exists; permission denial counts).
	Exists(pid int64) bool
	// GroupExists reports a process group exists (proc group-exists).
	GroupExists(pgid int64) bool
	// GroupOwned reports a group member carries the tag, or the record
	// carries an exact trusted-launcher proof (proc group-owned).
	GroupOwned(recordPath string, pgid int64, tag string) bool
	// SignalGroup sends sig to the process group.
	SignalGroup(pgid int64, sig Signal) error
	// CustodyGroups lists a record's custody process-group kill targets.
	CustodyGroups(record map[string]any) ([]int64, error)
	// StartedAt is a live pid's kernel start second (proc started-at).
	StartedAt(pid int64) (int64, error)
	// ClaimProcesses is the process-table reading the claim state machine
	// and the reservation reconciliation judge by.
	ClaimProcesses() (ClaimProcesses, error)
}

// ClaimProcesses reads process identity for the claim owner: kernel start
// identities, the tagged-process census, and the tagged-argv proof.
type ClaimProcesses struct {
	Reader   identity.StartReader
	Scanner  census.TaggedProcessScanner
	Verifier dispatch.ClaimProcessVerifier
}

// Signal is a signal the lifecycle sends to a custody group.
type Signal int

const (
	SignalTerm Signal = 15
	SignalKill Signal = 9
)

// GitOps runs Git for the lifecycle: worktree creation, the quarantine
// object store, commit subjects of code-critic reviews, the follow-up
// rebase, and the engine skew preflight. Behavior tests stub it per test.
type GitOps interface {
	// Run runs git with args in dir and returns its standard output and
	// standard error. A non-zero exit is an error carrying the exit code.
	Run(ctx context.Context, dir string, args ...string) (stdout, stderr []byte, err error)
}

// GitExitError is a Git run that exited non-zero.
type GitExitError struct {
	Code   int
	Stderr string
}

func (e *GitExitError) Error() string {
	return fmt.Sprintf("git exited %d: %s", e.Code, e.Stderr)
}

// Clock is the lifecycle's time: every deadline, poll and stamp reads it.
type Clock = wallclock.Clock

// WaitOutcome is the job waiter's answer (metasystem internal wait): its
// exit code and its combined output.
type WaitOutcome struct {
	Code   int
	Output string
}

// HostOps are the operations whose owners still live in the engine's command
// layer: the job waiter and watcher, the consumption-earned budget
// extension, and the snapshot selection that probes the runtime.
type HostOps interface {
	// WaitJob blocks on a job to terminal as `internal wait --job` does.
	WaitJob(ctx context.Context, root, job string, callerPid int64) WaitOutcome
	// WatchJob is `job watch`: block to terminal, tailing the watched
	// workspace's suite journal onto stderr; returns the pinned exit code.
	WatchJob(ctx context.Context, root, job string, callerPid int64, progressRoot string) int
	// ExtendBudget is `goal extend-budget` for the supplied caller; it
	// returns the verb's combined output and exit code.
	ExtendBudget(ctx context.Context, request ExtendBudgetRequest) (string, int)
	// BreachStopOrderingHuman is the enrolled person a HUMAN caller's breach
	// stop records as its actor (rule H1), proven from the supplied caller as
	// the human verbs prove it; an unproven terminal is an error that guides
	// to enrollment.
	BreachStopOrderingHuman(ctx context.Context, root string, callerPid int64, now time.Time) (string, error)
}

// ExtendBudgetRequest is goal extend-budget's selection.
type ExtendBudgetRequest struct {
	Root             string
	GoalID           string
	Revision         uint64
	ProposedCap      uint64
	Role             string
	DispatchMode     string
	DestructiveReach string
	CallerPid        int64
	OwnerLineage     string
}

// GuardOps is the checkout execution guard owner (internal/gaterun): the
// dispatcher waits for exclusive checkout execution, or joins the chain that
// owns it, before any launch; its launched adapter joins as a member.
type GuardOps interface {
	Acquire(root string, pid int64, owner string, wait, progress time.Duration, notes io.Writer) (gaterun.GuardResult, error)
	Release(root string, pid int64) error
}

// Ports is every owner operation the lifecycle reaches. Each field is
// required; New refuses a partial set so a missing wire is a construction
// error, never a nil dereference in the middle of a launch.
type Ports struct {
	Lease   LeaseOps
	Steward StewardOps
	Adapter AdapterOps
	Goal    GoalOps
	Records RecordOps
	Events  EventOps
	Process ProcessOps
	Git     GitOps
	Clock   Clock
	Host    HostOps
	Guard   GuardOps
}

// Validate names every missing port.
func (p Ports) Validate() error {
	var missing []error
	if p.Lease == nil {
		missing = append(missing, fmt.Errorf("lease operations are not wired"))
	}
	if p.Steward == nil {
		missing = append(missing, fmt.Errorf("steward operations are not wired"))
	}
	if p.Adapter == nil {
		missing = append(missing, fmt.Errorf("adapter operations are not wired"))
	}
	if p.Goal == nil {
		missing = append(missing, fmt.Errorf("goal operations are not wired"))
	}
	if p.Records == nil {
		missing = append(missing, fmt.Errorf("record operations are not wired"))
	}
	if p.Events == nil {
		missing = append(missing, fmt.Errorf("event operations are not wired"))
	}
	if p.Process == nil {
		missing = append(missing, fmt.Errorf("process operations are not wired"))
	}
	if p.Git == nil {
		missing = append(missing, fmt.Errorf("git operations are not wired"))
	}
	if p.Clock == nil {
		missing = append(missing, fmt.Errorf("clock operations are not wired"))
	}
	if p.Host == nil {
		missing = append(missing, fmt.Errorf("host operations are not wired"))
	}
	if p.Guard == nil {
		missing = append(missing, fmt.Errorf("guard operations are not wired"))
	}
	return errors.Join(missing...)
}
