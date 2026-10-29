package lease

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"errors"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/delegatecustody"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
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
		return HookDelegateResult{}, errors.New("the hook question needs the state root, the installation root and the asking process")
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
	return hookDelegateWith(owned, installationRoot, jobID, callerPID, probe, launch.Store{})
}

func hookDelegateWith(owned []delegatecustody.Owned, installationRoot, jobID string, callerPID int64, probe identity.FixtureProbe, store launch.Store) (HookDelegateResult, error) {
	if jobID == "" {
		paths, err := filepath.Glob(filepath.Join(installationRoot, "artifacts", "agents", "jobs", "*.json"))
		if err != nil {
			return HookDelegateResult{}, err
		}
		for _, path := range paths {
			data, err := hookDelegateReadFile(path)
			if os.IsNotExist(err) {
				continue
			}
			var record map[string]any
			if err == nil {
				err = json.Unmarshal(data, &record)
			}
			if err != nil {
				return HookDelegateResult{}, fmt.Errorf("hook delegate query could not read job record %s: %w", filepath.Base(path), err)
			}
			id, _ := record["jobId"].(string)
			if record["unitRun"] == nil || record["unitRun"] == "" || record["operationId"] != "unit-launch:"+id || dispatch.TerminalStatus(fmt.Sprint(record["status"])) {
				continue
			}
			execution, err := store.Read(id)
			if os.IsNotExist(err) || (err == nil && execution.State.Terminal()) {
				continue
			}
			if err != nil {
				return HookDelegateResult{}, fmt.Errorf("hook delegate query could not read unit launch %s: %w", id, err)
			}
			for _, ref := range []*identity.Ref{execution.Supervisor, execution.Child} {
				if ref != nil {
					owned = append(owned, delegatecustody.Owned{JobID: id, Ref: *ref})
				}
			}
		}
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
