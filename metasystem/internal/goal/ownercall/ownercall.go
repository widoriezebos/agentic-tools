// Package ownercall is the explicit context of one in-process owner call
// (plans/designs/verbs-object-action.md 6.2, VOA-02). An owner function
// called in its caller's process would otherwise read that caller's parent
// and a process-global lineage variable, so each call carries its context:
// the process identity that classification and human proof start from, and
// the lineage the request carries. Nothing on these paths reads
// os.Getppid() or the lineage variable, and nothing sets process-global
// environment to imitate a child.
//
// The supplied identity is fixed per call edge (VOA-02-R2): an edge that
// replaced a child supplies the current process, because that is the parent
// the child observed; a process entry supplies its own caller, as it always
// did; the landing owner supplies itself.
package ownercall

import (
	"fmt"
	"os"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// Process is one process as a classification starting point: its pid and,
// when known, its kernel start time (Unix seconds), which guards the pid
// against reuse between the moment it is supplied and the moment it is read.
type Process struct {
	Pid       int64
	StartedAt int64
}

// CurrentProcess is the identity an edge that replaced a child supplies:
// this process, the parent the child used to observe.
func CurrentProcess() Process {
	pid := int64(os.Getpid())
	supplied := Process{Pid: pid}
	if exact, state, err := (identity.KernelProber{}).Probe(pid); err == nil && state == identity.Alive {
		supplied.StartedAt = exact.StartedAt.Unix()
	}
	return supplied
}

// EntryCaller is the identity a process entry supplies: the process that
// started this one, read once at the entry's boundary.
func EntryCaller() Process {
	return Process{Pid: int64(os.Getppid())}
}

// ClassifiablePid returns the supplied pid, refusing when a recorded start
// time no longer matches the live process (the pid was reused).
func (p Process) ClassifiablePid(prober identity.Prober) (int64, error) {
	if p.StartedAt == 0 || prober == nil {
		return p.Pid, nil
	}
	exact, state, err := prober.Probe(p.Pid)
	if err != nil || state != identity.Alive || exact.StartedAt.Unix() != p.StartedAt {
		return 0, fmt.Errorf("the process that asked (pid %d, started %d) is gone or replaced (%s): %v", p.Pid, p.StartedAt, state, err)
	}
	return p.Pid, nil
}

// Invocation is the explicit context of one owner call.
type Invocation struct {
	// Caller is the process identity classification and human proof start
	// from.
	Caller Process
	// Lineage is the owner lineage the request carries; it replaces the
	// child's inherited METASYSTEM_OWNER_LINEAGE.
	Lineage string
	// LaneEpoch, when set, is the landing lane's custody epoch read from the
	// host's lane record: the call acts as the lane's stable claim identity
	// at that epoch (lane design r10 K7). Only the lane's kernel sets it.
	LaneEpoch int64
}

// FromThisProcess is the context of an edge that replaced a child run with
// lineage: the current process is the supplied identity.
func FromThisProcess(lineage string) Invocation {
	return Invocation{Caller: CurrentProcess(), Lineage: lineage}
}

// HandoverRequest is one claim transfer to a named target pair.
type HandoverRequest struct {
	Root, GoalID, TargetMachine, TargetLineage string
	TargetEpoch                                int64
	Batch, TargetRoot                          string
	// LaneHome is the host home whose lane record authenticates a target
	// that is the landing lane's claim identity (lineage landing-lane):
	// the lane is live when that record registers it at TargetEpoch on
	// TargetMachine, whether or not any agent runs.
	LaneHome string
}

// Usage names the missing parts of a handover, or "" when it is complete.
func (r HandoverRequest) Usage() string {
	if r.GoalID == "" || r.TargetMachine == "" || r.TargetLineage == "" || r.TargetEpoch < 1 || r.Batch == "" {
		return "goal handover needs --id, --target-machine, --target-lineage, --target-claim-epoch, and --batch"
	}
	return ""
}

// PublishError is a published result as the error a former child's nonzero
// exit became: the refusal, or the unconfirmed outcome it printed.
func PublishError(verb string, res goal.PublishResult, err error) error {
	if err != nil {
		return fmt.Errorf("goal %s: %w", verb, err)
	}
	if res.Outcome != goal.OutcomeConfirmed {
		return fmt.Errorf("goal %s: outcome=%s tip=%s detail=%s", verb, res.Outcome, res.Tip, res.Detail)
	}
	return nil
}
