package lease

import (
	"fmt"
	"os"
	"regexp"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/delegatecustody"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

var hookDelegateJobID = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
var hookDelegateReadFile = os.ReadFile

// HookDelegateResult is positive only when the hook caller or one of its live
// ancestors is owned by a delegate job record. Runtime signatures, mission
// custody, and environment hints are deliberately not authority.
type HookDelegateResult struct {
	Delegate       bool   `json:"delegate"`
	JobID          string `json:"jobId,omitempty"`
	MatchedPID     int64  `json:"matchedPid,omitempty"`
	ComparisonMode string `json:"comparisonMode,omitempty"`
}

// HookDelegate verifies delegate custody for a hook invocation. jobID narrows
// the evidence to the adapter-supplied record when present; an empty jobID
// scans local delegate jobs so older launchers still isolate their children.
func HookDelegate(stateRoot, installationRoot, jobID string, callerPID int64) (HookDelegateResult, error) {
	if stateRoot == "" || installationRoot == "" || callerPID < 1 {
		return HookDelegateResult{}, fmt.Errorf("hook delegate query requires state root, installation root, and caller pid")
	}
	if jobID != "" && !hookDelegateJobID.MatchString(jobID) {
		return HookDelegateResult{}, fmt.Errorf("hook delegate query received an invalid job identifier")
	}
	probe, err := fixtureProbe(installationRoot)
	if err != nil {
		return HookDelegateResult{}, err
	}
	owned, err := delegatecustody.Read(stateRoot, jobID, hookDelegateReadFile)
	if err != nil {
		return HookDelegateResult{}, err
	}
	candidate, matched, mode, found, err := delegatecustody.Walk(owned, callerPID,
		func(pid int64) (identity.Exact, bool) { return hookLiveIdentity(pid, probe) }, ParentPid)
	if err != nil || !found {
		return HookDelegateResult{Delegate: false}, err
	}
	return HookDelegateResult{Delegate: true, JobID: candidate.JobID, MatchedPID: matched, ComparisonMode: string(mode)}, nil
}

func hookLiveIdentity(pid int64, probe identity.FixtureProbe) (identity.Exact, bool) {
	if probe != nil {
		if _, present := probe.FixtureEntry(pid); present {
			observed, ok := ProcessIdentity(pid, probe)
			if !ok {
				return identity.Exact{}, false
			}
			return identity.Exact{Pid: pid, StartedAt: time.Unix(observed.StartedAt, 0), StartTicks: observed.StartTicks, BootID: observed.BootID, ArgvKnown: true}, true
		}
	}
	exact, state, err := (identity.KernelProber{}).Probe(pid)
	return exact, err == nil && state == identity.Alive
}
