package report

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// The watcher's job-file classification engine (review
// script-orchestration-06 = script-misc-3, relocated from
// watch-background-jobs.sh; the REPORT family per the recorded
// r3/KS-R3-009 ruling). Every report line and the seen-state file format
// are wire: humans and fixtures grep them. The census half of the watcher
// already runs through `supervise watcher-pass`; this is the other half.

// terminalScanStatuses is the watcher's terminal vocabulary — wider than
// dispatch's, deliberately: it classifies FOREIGN runners' records too.
var terminalScanStatuses = map[string]bool{
	"completed": true, "complete": true, "success": true, "succeeded": true,
	"failed": true, "failure": true, "error": true, "errored": true,
	"cancelled": true, "canceled": true, "killed": true,
	"timeout": true, "timed_out": true,
}

// ScanJobsParams is one classification pass.
type ScanJobsParams struct {
	Dirs           []string // glob patterns; each is scanned as <pattern>/*
	StateFile      string   // seen ids, one per line (the greppable contract)
	RunningFile    string   // running set carried between passes, "id\tpath" lines; the caller owns its lifetime so a watcher restart still resets it
	Scope          string   // resolved scope root; empty scans everything
	ScopeField     string   // record field naming the job's workspace
	StaleMin       int64
	CapMin         int64
	StartVerifyMin int64
	Baseline       bool // adopt history: mark without reporting (STALE stays unmarked)
	Now            time.Time
}

// scanRecordField reads one top-level field from a JSON record, empty when
// absent or unparsable — the json-get semantics the shell consumed.
func scanRecordField(path, field string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var value map[string]any
	if json.Unmarshal(data, &value) != nil {
		return ""
	}
	switch typed := value[field].(type) {
	case string:
		return typed
	case float64:
		if typed == float64(int64(typed)) {
			return fmt.Sprintf("%d", int64(typed))
		}
		return fmt.Sprintf("%v", typed)
	case bool:
		return fmt.Sprintf("%v", typed)
	default:
		return ""
	}
}

func scanRecordParses(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	var value any
	return json.Unmarshal(data, &value) == nil
}

func scanMtime(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return info.ModTime().Unix()
}

func readLineSet(path string) map[string]bool {
	set := map[string]bool{}
	data, err := os.ReadFile(path)
	if err != nil {
		return set
	}
	for _, line := range strings.Split(string(data), "\n") {
		if line != "" {
			set[line] = true
		}
	}
	return set
}

// ScanJobs runs one classification pass, writing the historical report
// lines ("STATE id status=S age=Nm record=PATH") to out, appending marked
// ids to the state file, and carrying the running set in RunningFile for
// VANISHED detection. Verdict precedence is the shipped one: terminal
// beats cap beats never-started beats stale.
func ScanJobs(p ScanJobsParams, out io.Writer) error {
	if p.StaleMin < 1 || p.CapMin < 1 || p.StartVerifyMin < 0 {
		// The shell's concatenated digit check let an EMPTY --stale-min
		// through, after which `[ age -ge "" ]` failed and STALE silently
		// never fired (the review's verified defect). The engine refuses.
		return fmt.Errorf("scan-jobs thresholds must be positive integers (stale-min, cap-min) and start-verify-min non-negative")
	}
	seen := readLineSet(p.StateFile)
	stateHandle, err := os.OpenFile(p.StateFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("scan-jobs cannot append to the state file: %v", err)
	}
	defer stateHandle.Close()
	mark := func(id string) {
		fmt.Fprintln(stateHandle, id)
		seen[id] = true
	}

	runningIDs := []string{}
	runningPaths := map[string]string{}
	if data, err := os.ReadFile(p.RunningFile); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			id, path, ok := strings.Cut(line, "\t")
			if ok && id != "" && runningPaths[id] == "" {
				runningIDs = append(runningIDs, id)
				runningPaths[id] = path
			}
		}
	}
	remember := func(id, path string) {
		if runningPaths[id] == "" {
			runningIDs = append(runningIDs, id)
			runningPaths[id] = path
		}
	}
	forget := func(id string) { delete(runningPaths, id) }

	report := func(state, id, status string, age int64, record string) {
		if status == "" {
			status = "unknown"
		}
		fmt.Fprintf(out, "%s %s status=%s age=%dm record=%s\n", state, id, status, age, record)
	}

	now := p.Now.Unix()
	for _, pattern := range p.Dirs {
		paths, _ := filepath.Glob(pattern + "/*")
		sort.Strings(paths)
		for _, path := range paths {
			info, err := os.Stat(path)
			if err != nil || info.IsDir() {
				continue
			}
			base := filepath.Base(path)
			id := base
			if i := strings.LastIndexByte(base, '.'); i >= 0 {
				id = base[:i]
			}
			if seen[id] {
				continue
			}
			// Prefer the record that actually carries fields; skip sidecars
			// of an id whose fields-carrying record exists, so one job
			// reports once and scope holds.
			siblings, _ := filepath.Glob(filepath.Join(filepath.Dir(path), id+".*"))
			sort.Strings(siblings)
			primary := path
			for _, sibling := range siblings {
				if info, err := os.Stat(sibling); err != nil || info.IsDir() {
					continue
				}
				if scanRecordField(sibling, "status") != "" ||
					(p.ScopeField != "" && scanRecordField(sibling, p.ScopeField) != "") {
					primary = sibling
					break
				}
			}
			if primary != path {
				continue
			}
			if p.Scope != "" {
				workspace := scanRecordField(path, p.ScopeField)
				// No workspace field: report it — unknown ownership beats
				// an unobserved job.
				if workspace != "" &&
					!strings.HasPrefix(strings.TrimRight(workspace, "/")+"/", p.Scope+"/") {
					continue
				}
			}

			status := scanRecordField(path, "status")
			// Liveness is the NEWEST mtime across every file the runner
			// keeps for this job: runners write the record once and stream
			// progress to sibling logs.
			mtime := scanMtime(path)
			for _, sibling := range siblings {
				if m := scanMtime(sibling); m > mtime {
					mtime = m
				}
			}
			age := (now - mtime) / 60

			switch {
			case status != "" && terminalScanStatuses[status]:
				if p.Baseline {
					mark(id)
					continue
				}
				report("DONE", id, status, age, path)
				mark(id)
				forget(id)
			case age >= p.CapMin:
				if p.Baseline {
					mark(id)
					continue
				}
				if status == "" {
					status = "running"
				}
				report("CAPPED", id, status, age, path)
				mark(id)
				forget(id)
			case p.StartVerifyMin > 0 && age >= p.StartVerifyMin &&
				((status == "" && scanRecordParses(path)) ||
					status == "queued" || status == "pending" || status == "starting" || status == "created"):
				// A dispatch that never left the queue is a silent failure
				// long before STALE fires; report it early, by its real name.
				if p.Baseline {
					mark(id)
					continue
				}
				if status == "" {
					status = "absent"
				}
				report("NEVER-STARTED", id, status, age, path)
				mark(id)
				forget(id)
			case age >= p.StaleMin:
				if p.Baseline {
					continue
				}
				if status == "" {
					status = "running"
				}
				report("STALE", id, status, age, path)
				mark(id)
				forget(id)
			default:
				remember(id, path)
			}
		}
	}

	// Records that were running and are now gone: the runner lost the job.
	kept := []string{}
	for _, id := range runningIDs {
		path := runningPaths[id]
		if path == "" {
			continue
		}
		// Mirror the shell's [ ! -f ]: anything that is not a readable
		// regular file counts as vanished.
		if info, err := os.Stat(path); err != nil || !info.Mode().IsRegular() {
			if !p.Baseline {
				report("VANISHED", id, "running", 0, path)
				mark(id)
			}
			continue
		}
		kept = append(kept, id)
	}
	var buffer strings.Builder
	for _, id := range kept {
		fmt.Fprintf(&buffer, "%s\t%s\n", id, runningPaths[id])
	}
	if err := os.WriteFile(p.RunningFile, []byte(buffer.String()), 0o644); err != nil {
		return fmt.Errorf("scan-jobs cannot persist the running set: %v", err)
	}
	return nil
}
