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
	return runCensus(metasystemRoot, repo, fingerprint, interval, now,
		func(string) ([]Process, error) { return EnumerateProcesses() }, ResolveCwds)
}
