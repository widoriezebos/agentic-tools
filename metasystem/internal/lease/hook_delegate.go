package lease

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"time"

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

type hookProcessRecord struct {
	Pid               int64  `json:"pid"`
	PidStartedAt      int64  `json:"pidStartedAt"`
	PidStartedAtMicro int64  `json:"pidStartedAtExactMicro,omitempty"`
	PidStartTicks     int64  `json:"pidStartTicks,omitempty"`
	BootID            string `json:"bootId,omitempty"`
}

type hookJobRecord struct {
	JobID            string              `json:"jobId"`
	Pid              *int64              `json:"pid"`
	PidStartedAt     *int64              `json:"pidStartedAt"`
	PidStartedMicro  *int64              `json:"pidStartedAtExactMicro"`
	PidStartTicks    *int64              `json:"pidStartTicks"`
	BootID           *string             `json:"bootId"`
	CustodyProcesses []hookProcessRecord `json:"custodyProcesses"`
}

type ownedHookProcess struct {
	jobID string
	ref   identity.Ref
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
	owned, err := readHookJobCustody(stateRoot, jobID)
	if err != nil {
		return HookDelegateResult{}, err
	}
	seen := map[int64]bool{}
	current := callerPID
	for current > 0 && !seen[current] {
		seen[current] = true
		exact, ok := hookLiveIdentity(current, probe)
		if !ok {
			return HookDelegateResult{}, fmt.Errorf("hook delegate query could not authenticate process %d", current)
		}
		for _, candidate := range owned {
			comparison := identity.Compare(exact, candidate.ref)
			if comparison.Matches {
				return HookDelegateResult{Delegate: true, JobID: candidate.jobID, MatchedPID: current, ComparisonMode: string(comparison.Mode)}, nil
			}
		}
		parent, present := ParentPid(current)
		if !present || parent == current {
			break
		}
		current = parent
	}
	return HookDelegateResult{Delegate: false}, nil
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

func readHookJobCustody(stateRoot, onlyJob string) ([]ownedHookProcess, error) {
	jobsDir := filepath.Join(stateRoot, "artifacts", "agents", "jobs")
	var paths []string
	if onlyJob != "" {
		paths = []string{filepath.Join(jobsDir, onlyJob+".json")}
	} else {
		var err error
		paths, err = filepath.Glob(filepath.Join(jobsDir, "*.json"))
		if err != nil {
			return nil, fmt.Errorf("hook delegate query could not list job records: %w", err)
		}
		sort.Strings(paths)
	}
	var out []ownedHookProcess
	for _, path := range paths {
		data, err := hookDelegateReadFile(path)
		if err != nil {
			if onlyJob == "" && os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("hook delegate query could not read job record %s: %w", filepath.Base(path), err)
		}
		var record hookJobRecord
		if json.Unmarshal(data, &record) != nil || record.JobID == "" || (onlyJob != "" && record.JobID != onlyJob) {
			return nil, fmt.Errorf("hook delegate query found a corrupt or mismatched job record: %s", filepath.Base(path))
		}
		if record.Pid != nil || record.PidStartedAt != nil {
			if record.Pid == nil || record.PidStartedAt == nil {
				return nil, fmt.Errorf("hook delegate query found a partial job identity: %s", filepath.Base(path))
			}
			process := hookProcessRecord{Pid: *record.Pid, PidStartedAt: *record.PidStartedAt}
			if record.PidStartedMicro != nil {
				process.PidStartedAtMicro = *record.PidStartedMicro
			}
			if record.PidStartTicks != nil {
				process.PidStartTicks = *record.PidStartTicks
			}
			if record.BootID != nil {
				process.BootID = *record.BootID
			}
			ref, err := process.hookRef()
			if err != nil {
				return nil, fmt.Errorf("hook delegate query found an invalid job identity in %s: %w", filepath.Base(path), err)
			}
			out = append(out, ownedHookProcess{jobID: record.JobID, ref: ref})
		}
		for _, process := range record.CustodyProcesses {
			ref, err := process.hookRef()
			if err != nil {
				return nil, fmt.Errorf("hook delegate query found invalid custody in %s: %w", filepath.Base(path), err)
			}
			out = append(out, ownedHookProcess{jobID: record.JobID, ref: ref})
		}
	}
	return out, nil
}

func (p hookProcessRecord) hookRef() (identity.Ref, error) {
	ref := identity.Ref{Pid: p.Pid, StartedAtSec: p.PidStartedAt, StartedAtUnixMicro: p.PidStartedAtMicro, StartTicks: p.PidStartTicks, BootID: p.BootID}
	if p.Pid < 1 || p.PidStartedAt < 1 || ref.Mode() == identity.CompareInvalid {
		return identity.Ref{}, fmt.Errorf("invalid process identity for pid %d", p.Pid)
	}
	return ref, nil
}
