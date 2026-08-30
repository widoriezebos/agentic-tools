package census

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"golang.org/x/sys/unix"
)

// EnumerateConfiguredProcesses uses the same authorized process source as a
// census pass: the explicit fixture table when configured, otherwise the live
// kernel table.
func EnumerateConfiguredProcesses(metasystemRoot string) ([]Process, error) {
	if processes, configured, err := ConfiguredProcessFixture(metasystemRoot); configured || err != nil {
		return processes, err
	}
	return EnumerateProcesses()
}

// ConfiguredProcessFixture returns the authorized synthetic process universe
// without falling through to the host process table when that universe is
// empty. The boolean distinguishes an empty configured table from no table.
func ConfiguredProcessFixture(metasystemRoot string) ([]Process, bool, error) {
	processFile := os.Getenv("METASYSTEM_CENSUS_PROCESS_FILE")
	if processFile == "" {
		return nil, false, nil
	}
	processes, err := enumerateFixture(metasystemRoot, processFile)
	return processes, true, err
}

// The production enumeration path reads the real process table and resolves
// working directories per matched process. It feeds the same classification
// core as the fixture path while retaining both the whole-second join key and
// the platform's exact process identity.

// EnumerateProcesses returns the live process table NATIVELY: sysctl
// kern.proc.all for the pid list, the kernel prober for each pid's start
// time and argv, and getpgid for the group. No `ps` subprocess. A pid that
// vanishes between enumeration and
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
			Started: exact.StartedAt.Unix(), StartedExactMicro: exact.StartedAt.UnixMicro(),
			StartTicks: exact.StartTicks, BootID: exact.BootID, Argv: argv,
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
// production counterpart of RunFixtureCensus over the same runCensus core.
func RunProductionCensus(metasystemRoot, repo, fingerprint string, interval int, now time.Time) (Verdict, error) {
	return RunProductionCensusAt(metasystemRoot, metasystemRoot, repo, fingerprint, interval, now)
}

// RunProductionCensusAt separates installed configuration from repository
// state while keeping one census scope.
func RunProductionCensusAt(metasystemRoot, stateRoot, repo, fingerprint string, interval int, now time.Time) (Verdict, error) {
	return runCensus(metasystemRoot, stateRoot, repo, fingerprint, interval, now,
		func(string) ([]Process, error) { return EnumerateProcesses() }, ResolveCwds)
}
