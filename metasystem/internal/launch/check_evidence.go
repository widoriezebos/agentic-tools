package launch

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/loopstop"
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
			record.Directory = filepath.Dir(path)
			records = append(records, record)
		}
		return nil
	})
	if err != nil {
		unknown = append(unknown, "check evidence unavailable: "+root+": "+err.Error())
	}
	return records, unknown
}

func newestCheckExecution(round, launchID string) (*CheckExecution, []string) {
	executions, unknown := ReadCheckExecutions(round)
	var latest *CheckExecution
	var newest time.Time
	for i, execution := range executions {
		if execution.Steps[0].Parent != launchID {
			continue
		}
		started, err := time.Parse(time.RFC3339Nano, execution.Steps[0].Start)
		if err != nil {
			unknown = append(unknown, "the check's start time cannot be read")
			continue
		}
		if started.After(newest) {
			latest, newest = &executions[i], started
		}
	}
	return latest, unknown
}

// checkCause makes a failed step's cause environment when the newest frozen
// check of its own launch stopped at proof.deadline and nothing in it failed.
// Unreadable evidence or any other outcome leaves the step as it is.
func checkCause(step *UnitStep, round string) bool {
	execution, unknown := newestCheckExecution(round, step.LaunchID)
	if len(unknown) > 0 || execution == nil || step.LaunchID == "" {
		return false
	}
	deadline := false
	for _, check := range execution.Steps {
		if check.Outcome != "passed" && check.Outcome != "deadline" {
			return false
		}
		deadline = deadline || check.Outcome == "deadline"
	}
	if deadline {
		step.Cause, step.Deadline = "environment", true
	}
	return deadline
}

func (execution CheckExecution) sectionUnits(id, directory string) []loopstop.RedUnit {
	var result struct{ Exits []CheckExit }
	data, err := os.ReadFile(filepath.Join(execution.Directory, "result.json"))
	if err != nil || json.Unmarshal(data, &result) != nil {
		return nil
	}
	mod, _ := os.ReadFile(filepath.Join(directory, "go.mod"))
	module := regexp.MustCompile(`(?m)^[ \t]*module[ \t]+(\S+)`).FindSubmatch(mod)
	if len(module) != 2 {
		return nil
	}
	prefix := ""
	if config.TemplateMode(directory) {
		prefix = "metasystem/"
	}
	for _, exit := range result.Exits {
		if exit.Name == id[strings.LastIndex(id, ":")+1:] {
			return sectionRedUnits(exit.Output, strings.Trim(string(module[1]), "\"`"), prefix)
		}
	}
	return nil
}

// sectionRedUnits names tests only when every package failure is accounted for.
func sectionRedUnits(output, module, prefix string) []loopstop.RedUnit {
	var units []loopstop.RedUnit
	var pending []string
	finished := false
	failure := regexp.MustCompile(`^--- FAIL: ([^\s/]+) \([^)]+\)$`)
	for _, line := range strings.Split(output, "\n") {
		if match := failure.FindStringSubmatch(line); len(match) > 0 {
			pending = append(pending, match[1])
			finished = false
		}
		if line == "FAIL" {
			finished = true
		}
		if strings.HasPrefix(line, "FAIL\t") {
			fields := strings.Fields(line)
			if len(fields) < 2 || len(pending) == 0 || !finished || strings.Contains(line, "[build failed]") || fields[1] != module && !strings.HasPrefix(fields[1], module+"/") {
				return nil
			}
			pkg := strings.TrimPrefix(strings.TrimPrefix(fields[1], module), "/")
			units = append(units, loopstop.RedUnit{Unit: prefix + filepath.Join(".", pkg), Tests: pending})
			pending = nil
			finished = false
		}
	}
	if len(pending) > 0 {
		return nil
	}
	return units
}

func mergeRedUnits(units []loopstop.RedUnit) []loopstop.RedUnit {
	var merged []loopstop.RedUnit
	for _, unit := range units {
		i := slices.IndexFunc(merged, func(u loopstop.RedUnit) bool { return u.Unit == unit.Unit })
		if i >= 0 && (len(merged[i].Tests) == 0 || len(unit.Tests) == 0) {
			merged[i].Tests, merged[i].Registered = nil, nil
			continue
		}
		if i < 0 {
			merged = append(merged, loopstop.RedUnit{Unit: unit.Unit})
			i = len(merged) - 1
		}
		for j, test := range unit.Tests {
			if !slices.Contains(merged[i].Tests, test) {
				registration := ""
				if j < len(unit.Registered) {
					registration = unit.Registered[j]
				}
				merged[i].Tests = append(merged[i].Tests, test)
				merged[i].Registered = append(merged[i].Registered, registration)
			}
		}
	}
	return merged
}
