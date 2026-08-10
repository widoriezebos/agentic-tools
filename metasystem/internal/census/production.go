package census

import (
	"fmt"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"golang.org/x/sys/unix"
)

// The PRODUCTION enumeration path: the real process table, and cwd resolution
// per matched process. It feeds the SAME classification core the fixture path
// exercises, so the verdict logic is shared; this file is the process-touching
// binding. Start times are whole-second local epoch, NOT microsecond-exact,
// because the rest of the system's (pid, started) join keys — announcements,
// custody — are whole-second, and a microsecond-exact census would fail to
// match them.

// EnumerateProcesses returns the live process table NATIVELY: sysctl
// kern.proc.all for the pid list, the kernel prober for each pid's start
// time (whole seconds, join-key consistent) and argv, and getpgid for the
// group. No `ps` subprocess. A pid that vanishes between enumeration and
// probing is simply dropped (it was not live).
func EnumerateProcesses() ([]Process, error) {
	pids, err := identity.AllPids()
	if err != nil {
		return nil, fmt.Errorf("process enumeration failed: %w", err)
	}
	prober := identity.KernelProber{}
	var processes []Process
	for _, pid := range pids {
		exact, state, err := prober.Probe(pid)
		if err != nil || state != identity.Alive {
			continue
		}
		argv := strings.Join(exact.Argv, " ")
		if argv == "" {
			continue
		}
		pgid, perr := unix.Getpgid(int(pid))
		if perr != nil {
			pgid = 0
		}
		processes = append(processes, Process{
			Pid: pid, PPID: 0, PGID: int64(pgid),
			Started: exact.StartedAt.Unix(), Argv: argv,
			Cwd: "", CwdError: false, Alive: true,
		})
	}
	return processes, nil
}

// ResolveCwds resolves each pid's cwd NATIVELY via the proc_info syscall
// (the call lsof itself makes). A cwd that cannot be read is an error only
// when the pid is still alive (a permission denial); a gone pid is not an
// error. No `lsof` subprocess.
func ResolveCwds(pids []int64) map[int64]cwdResult {
	found := map[int64]cwdResult{}
	for _, pid := range pids {
		if cwd, ok := identity.ProcessCwd(pid); ok {
			found[pid] = cwdResult{Cwd: realpath(cwd), CwdError: false}
			continue
		}
		if _, state, _ := kernelProbe(pid); state == probeDead {
			found[pid] = cwdResult{Cwd: "", CwdError: false}
		} else {
			found[pid] = cwdResult{Cwd: "", CwdError: true}
		}
	}
	return found
}

type cwdResult struct {
	Cwd      string
	CwdError bool
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
	// Resolve cwds only for matched processes — the cwd-resolution cost the
	// census is careful about.
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
