package delegation

import (
	"context"
	"errors"
	"fmt"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
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
// (checkout-execution-guard.sh run-member, supervise launch-detached
// --execution-guard-root/--execution-guard-owner).
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

// AdapterOps is the runtime adapter owner as the dispatcher drives it. Until
// U6a ports the adapters into Go, the real owner is the runtime's adapter
// script; the operations are the verbs dispatch.sh invokes on it.
type AdapterOps interface {
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
	return errors.Join(missing...)
}
