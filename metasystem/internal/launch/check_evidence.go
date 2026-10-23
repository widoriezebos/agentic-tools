package launch

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// CheckParent joins live process ancestry to the round's retained child custody.
func CheckParent(round UnitRound, store Store, prober identity.Prober) (string, string, string) {
	if prober == nil {
		prober = identity.KernelProber{}
	}
	for pid, seen := int64(os.Getpid()), map[int64]bool{}; pid > 0 && !seen[pid]; {
		seen[pid] = true
		exact, state, err := prober.Probe(pid)
		if err != nil || state != identity.Alive {
			break
		}
		for _, step := range round.Steps {
			record, err := store.Read(step.LaunchID)
			if err == nil && !record.State.Terminal() && (record.Kind == "build" || record.Kind == "proof") && record.Child != nil && record.Child.NativeExact() && identity.SameIdentity(exact, *record.Child) {
				role := "builder"
				if record.Kind == "proof" {
					role = "attestation"
				}
				var brief string
				_ = json.Unmarshal(record.AdapterData["brief"], &brief)
				return record.ID, role, brief
			}
		}
		parent, known := identity.ParentPid(pid)
		if !known {
			break
		}
		pid = parent
	}
	return "", "unknown", ""
}

// ReadCheckExecutions includes partial observations; unreadable evidence stays explicit.
func ReadCheckExecutions(root string) ([]CheckExecution, []string) {
	var records []CheckExecution
	var unknown []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if os.IsNotExist(err) && path == root {
			return nil
		}
		if err != nil {
			return err
		}
		if entry.IsDir() && strings.HasPrefix(strings.TrimPrefix(entry.Name(), "builder-"), "check-") {
			if _, err := os.Stat(filepath.Join(path, "observation.json")); err != nil {
				unknown = append(unknown, "check observation unavailable: "+path)
			}
		}
		if entry.IsDir() || entry.Name() != "observation.json" {
			return nil
		}
		data, err := os.ReadFile(path)
		var record CheckExecution
		if err != nil || json.Unmarshal(data, &record) != nil || record.ExecutionID != strings.TrimPrefix(filepath.Base(filepath.Dir(path)), "builder-") || len(record.Steps) == 0 {
			unknown = append(unknown, "check observation unavailable: "+path)
		} else {
			records = append(records, record)
		}
		return nil
	})
	if err != nil {
		unknown = append(unknown, "check evidence unavailable: "+root+": "+err.Error())
	}
	return records, unknown
}

// checkCause makes a failed step's cause environment when the newest frozen
// check of its own launch stopped at proof.deadline and nothing in it failed.
// Unreadable evidence or any other outcome leaves the step as it is.
func checkCause(step *UnitStep, round string) bool {
	executions, unknown := ReadCheckExecutions(round)
	if len(unknown) > 0 || step.LaunchID == "" {
		return false
	}
	var newest time.Time
	deadline := false
	for _, execution := range executions {
		started, err := time.Parse(time.RFC3339Nano, execution.Steps[0].Start)
		if err != nil && execution.Steps[0].Parent == step.LaunchID {
			return false
		}
		if execution.Steps[0].Parent != step.LaunchID || !started.After(newest) {
			continue
		}
		newest, deadline = started, false
		for _, check := range execution.Steps {
			if check.Outcome != "passed" && check.Outcome != "deadline" {
				deadline = false
				break
			}
			deadline = deadline || check.Outcome == "deadline"
		}
	}
	if deadline {
		step.Cause, step.Deadline = "environment", true
	}
	return deadline
}
