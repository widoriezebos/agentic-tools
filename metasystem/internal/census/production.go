package census

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// The PRODUCTION enumeration path (process-census.py enumerate_ps +
// resolve_cwds): the real process table via ps, and cwd resolution via lsof.
// It feeds the SAME classification core the fixture path proves, so the
// verdict logic is conformance-covered; this file is the process-touching
// binding. Start times come from ps lstart (whole-second local epoch), NOT
// the kernel-exact prober, because the rest of the system's (pid, started)
// join keys — announcements, custody — are lstart-derived, and a
// microsecond-exact census would fail to match them.

var lstartLine = regexp.MustCompile(
	`^\s*(\d+)\s+(\d+)\s+(\d+)\s+` +
		`([A-Z][a-z]{2}\s+[A-Z][a-z]{2}\s+\d{1,2}\s+\d{2}:\d{2}:\d{2}\s+\d{4})\s+(.*)$`)

// EnumerateProcesses returns the live process table, ports enumerate_ps.
func EnumerateProcesses() ([]Process, error) {
	cmd := exec.Command("ps", "-axo", "pid=,ppid=,pgid=,lstart=,command=")
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("process enumeration failed: %w", err)
	}
	return parseProcessTable(string(out), time.Now().Location()), nil
}

// parseProcessTable parses ps -axo pid,ppid,pgid,lstart,command output into
// Process rows — extracted so the lstart parsing is deterministically
// testable. A malformed lstart yields started=-1 (an impossible time), so an
// agent-shaped row with a bad time is failed by the classifier rather than
// falsely joining custody.
func parseProcessTable(output string, local *time.Location) []Process {
	var processes []Process
	for _, raw := range strings.Split(output, "\n") {
		m := lstartLine.FindStringSubmatch(raw)
		if m == nil {
			continue
		}
		argv := m[5]
		if argv == "" {
			continue
		}
		pid, _ := strconv.ParseInt(m[1], 10, 64)
		ppid, _ := strconv.ParseInt(m[2], 10, 64)
		pgid, _ := strconv.ParseInt(m[3], 10, 64)
		var started int64 = -1
		if parsed, err := time.ParseInLocation("Mon Jan 2 15:04:05 2006", normalizeLstart(m[4]), local); err == nil {
			started = parsed.Unix()
		}
		processes = append(processes, Process{
			Pid: pid, PPID: ppid, PGID: pgid, Started: started,
			Argv: argv, Cwd: "", CwdError: false, Alive: true,
		})
	}
	return processes
}

// normalizeLstart collapses the multiple spaces ps uses to pad the day
// field ("Aug  1") into single spaces so Go's reference-time parser accepts
// it. python's strptime %a %b %d tolerates the padding; Go's does not.
func normalizeLstart(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// ResolveCwds resolves the cwd of each pid via one batched lsof, ports
// resolve_cwds_batch. A pid lsof cannot resolve keeps cwd empty with
// cwdError true when the pid is still alive (an lsof denial), or empty with
// cwdError false when the pid is gone.
func ResolveCwds(pids []int64) map[int64]cwdResult {
	found := map[int64]cwdResult{}
	if len(pids) == 0 {
		return found
	}
	if _, err := exec.LookPath("lsof"); err != nil {
		for _, pid := range pids {
			found[pid] = resolveCwdSingle(pid)
		}
		return found
	}
	args := []string{"-a", "-p", joinPids(pids), "-d", "cwd", "-Fpn"}
	out, _ := exec.Command("lsof", args...).Output()
	var current int64 = -1
	for _, raw := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(raw, "p") {
			if v, err := strconv.ParseInt(raw[1:], 10, 64); err == nil {
				current = v
			} else {
				current = -1
			}
		} else if strings.HasPrefix(raw, "n") && current >= 0 {
			if _, seen := found[current]; !seen {
				found[current] = cwdResult{Cwd: realpath(raw[1:]), CwdError: false}
			}
		}
	}
	// A pid lsof said nothing about: resolve singly, preserving the
	// alive-versus-denied distinction.
	for _, pid := range pids {
		if _, seen := found[pid]; !seen {
			found[pid] = resolveCwdSingle(pid)
		}
	}
	return found
}

type cwdResult struct {
	Cwd      string
	CwdError bool
}

func resolveCwdSingle(pid int64) cwdResult {
	if _, err := exec.LookPath("lsof"); err == nil {
		out, _ := exec.Command("lsof", "-a", "-p", fmt.Sprint(pid), "-d", "cwd", "-Fn").Output()
		for _, raw := range strings.Split(string(out), "\n") {
			if strings.HasPrefix(raw, "n") && len(raw) > 1 {
				return cwdResult{Cwd: realpath(raw[1:]), CwdError: false}
			}
		}
	}
	// Alive but unresolved -> cwdError; gone -> not an error.
	if _, state, _ := kernelProbe(pid); state == probeDead {
		return cwdResult{Cwd: "", CwdError: false}
	}
	return cwdResult{Cwd: "", CwdError: true}
}

func joinPids(pids []int64) string {
	parts := make([]string, len(pids))
	for i, pid := range pids {
		parts[i] = strconv.FormatInt(pid, 10)
	}
	return strings.Join(parts, ",")
}

// RunProductionCensus computes the verdict from the LIVE process table (no
// fixture), resolving cwds only for the signature-matched processes — the
// production counterpart of RunFixtureCensus over the same classification.
func RunProductionCensus(metasystemRoot, repo, fingerprint string, interval int, now time.Time) (Verdict, error) {
	metasystemRoot = realpath(metasystemRoot)
	repoReal := realpath(repo)
	var errors, diagnostics []string
	counts := map[string]int{"CUSTODY": 0, "ANNOUNCED": 0, "UNTRACKED": 0}
	var generation *int64
	var stateDigest *string

	if ids, gen, digest, err := readSupervisionSnapshot(metasystemRoot); err != nil {
		errors = append(errors, "supervision-state:"+err.Error())
	} else {
		generation, stateDigest = &gen, &digest
		verifySupervisionSnapshot(ids, &errors)
	}

	processes, enumErr := EnumerateProcesses()
	var signatures []Signature
	if enumErr != nil {
		errors = append(errors, "enumeration:"+enumErr.Error())
		processes = nil
	} else if sigs, err := configuredSignatures(metasystemRoot); err != nil {
		errors = append(errors, "enumeration:"+err.Error())
		processes = nil
	} else {
		signatures = sigs
	}

	argvs := make([]string, len(processes))
	for i, p := range processes {
		argvs[i] = p.Argv
	}
	matched := Classify(argvs, signatures)
	// Resolve cwds only for matched processes (the python's
	// resolve_signature_cwds), the lsof cost the census is careful about.
	var matchedPids []int64
	for _, a := range matched {
		matchedPids = append(matchedPids, processes[a.Index].Pid)
	}
	cwds := ResolveCwds(matchedPids)

	custody := liveCustody(metasystemRoot)
	announced := announcementsList(metasystemRoot, &errors)
	var inventory []InventoryItem
	for _, assignment := range matched {
		process := processes[assignment.Index]
		if resolved, ok := cwds[process.Pid]; ok {
			process.Cwd, process.CwdError = resolved.Cwd, resolved.CwdError
		}
		item, ok := classifyProcess(process, assignment.Runtime, repoReal, custody, announced, &errors, &diagnostics)
		if !ok {
			continue
		}
		counts[item.Class]++
		inventory = append(inventory, item)
	}

	return assembleVerdict(verdictLabelFor(errors), fingerprint, interval, generation, stateDigest,
		counts, inventory, diagnostics, errors, now), nil
}
