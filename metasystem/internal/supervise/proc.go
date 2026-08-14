package supervise

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// ProcComponents implements Components against real processes: each
// component is launched detached in its own session (setsid, so it
// survives the owner's shell exactly as the shell system's
// launch_detached does), observed by heartbeat freshness plus kernel
// liveness, and stopped TERM-then-KILL by process group. Teardown
// works from HELD IDENTITY and system calls only (SLC-R7-005: it must
// run with the checkout gone), so nothing here reads a state file.
type ProcComponents struct {
	// SupervisionDir is where heartbeat files live.
	SupervisionDir string
	// Command builds the argv for one component; the binary supplies
	// its own `metasystem supervise component ...` invocation, tests
	// supply a heartbeat-writing stub.
	Command func(component Component, tag, heartbeatPath string) []string
	// Prober proves identities three-way; the kernel one in
	// production, a fake in tests.
	Prober identity.Prober
	// IntervalSec bounds heartbeat freshness (stale beyond 2×+2).
	IntervalSec int
	// StopCeiling is D-6's component stop wait.
	StopCeiling time.Duration
	// clock and signal are injectable; nil means real time and a real
	// group signal.
	clock  func() time.Time
	signal func(pid int64, sig syscall.Signal) error

	// children tracks processes this owner launched so it can reap
	// them (clear zombies) after a kill; guarded by mu.
	mu       sync.Mutex
	children map[int64]*os.Process
}

func (p *ProcComponents) now() time.Time {
	if p.clock != nil {
		return p.clock()
	}
	return time.Now()
}

func (p *ProcComponents) heartbeatPath(component Component) string {
	return filepath.Join(p.SupervisionDir, string(component)+".heartbeat.json")
}

// Launch starts one component in its own session and returns its
// identity. argv is exec'd directly with Setsid — the launched
// process IS the component, a session leader (pgid == pid) for group
// signalling. It stays a direct child of the owner on purpose: the
// owner reaps it when it dies (ReapDead's Wait), so no zombie
// lingers and Stop can prove it gone, and supervises it by identity
// (pid + start time), never by the child handle alone.
func (p *ProcComponents) Launch(component Component, tag string) (identity.Ref, error) {
	argv := p.Command(component, tag, p.heartbeatPath(component))
	if len(argv) == 0 {
		return identity.Ref{}, fmt.Errorf("component command for %s is empty", component)
	}
	launcher := exec.Command(argv[0], argv[1:]...)
	launcher.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := launcher.Start(); err != nil {
		return identity.Ref{}, fmt.Errorf("launch %s: %w", component, err)
	}
	pid := int64(launcher.Process.Pid)
	exact, state, err := p.Prober.Probe(pid)
	if err != nil || state != identity.Alive {
		_, _ = launcher.Process.Wait()
		return identity.Ref{Pid: pid}, fmt.Errorf("launched %s pid %d not observable: %v", component, pid, state)
	}
	// The launched process IS the component here (setsid session
	// leader). The owner reaps it when it dies via ReapDead below —
	// registered so a killed component leaves no zombie.
	p.track(pid, launcher.Process)
	return exact.Ref(), nil
}

func (p *ProcComponents) track(pid int64, process *os.Process) {
	p.mu.Lock()
	if p.children == nil {
		p.children = map[int64]*os.Process{}
	}
	p.children[pid] = process
	p.mu.Unlock()
}

// reap non-blockingly reaps a tracked child if it has become a zombie,
// so a killed component becomes definitively ABSENT rather than a
// defunct entry the prober still sees as present. A no-op for a pid
// this owner did not itself launch.
func (p *ProcComponents) reap(pid int64) {
	p.mu.Lock()
	process, tracked := p.children[pid]
	p.mu.Unlock()
	if !tracked {
		return
	}
	var status syscall.WaitStatus
	if reaped, err := syscall.Wait4(int(pid), &status, syscall.WNOHANG, nil); err == nil && reaped == int(pid) {
		_ = process.Release()
		p.mu.Lock()
		delete(p.children, pid)
		p.mu.Unlock()
	}
}

type heartbeatRecord struct {
	Pid             int64 `json:"pid"`
	PidStartedAt    int64 `json:"pidStartedAt"`
	ObservedAtEpoch int64 `json:"observedAtEpoch"`
}

// Observe reports one component's three-way health: identity alive
// AND heartbeat fresh is Healthy; a definitively dead identity is
// Failing; anything unreadable is Indeterminable (SLC-R3-004).
func (p *ProcComponents) Observe(held Held) Observation {
	// Reap first: a component that exited but is still our unreaped
	// child is a ZOMBIE the kernel still lists, which would read as
	// alive-with-no-heartbeat (Indeterminable) forever and stall the
	// breaker. Reaping makes its death visible as Dead → Failing.
	p.reap(held.Identity.Pid)
	switch identity.AliveRef(p.Prober, held.Identity) {
	case identity.Dead:
		return Failing
	case identity.Unknown:
		return Indeterminable
	}
	content, err := os.ReadFile(p.heartbeatPath(held.Component))
	if err != nil {
		return Indeterminable // alive but heartbeat unreadable: cannot decide fresh
	}
	var beat heartbeatRecord
	if json.Unmarshal(content, &beat) != nil {
		return Indeterminable
	}
	if beat.Pid != held.Identity.Pid {
		// The heartbeat belongs to a different generation's process:
		// this held component has no fresh beat of its own.
		return Failing
	}
	age := p.now().Unix() - beat.ObservedAtEpoch
	if age > int64(p.IntervalSec)*2+2 {
		return Failing
	}
	return Healthy
}

// GroupCount counts live members of the held components' process
// groups — the ceiling's input. Under setsid a component's pgid is
// its pid, so the group is enumerated by that pgid.
func (p *ProcComponents) GroupCount(held []Held) (int, error) {
	total := 0
	for _, member := range held {
		members, err := processGroupMembers(member.Identity.Pid)
		if err != nil {
			return 0, err
		}
		total += members
	}
	return total, nil
}

// Stop signals one held identity's group TERM-then-KILL within the
// stop ceiling and reports whether it is PROVEN gone (three-way:
// false on definitive survival OR unknown — teardownComplete stays
// honest, SLC-R4-012).
func (p *ProcComponents) Stop(held Held) (proven bool) {
	// Reap first: a component already killed but not yet reaped is a
	// zombie the prober still sees; reaping makes its death provable.
	p.reap(held.Identity.Pid)
	if identity.AliveRef(p.Prober, held.Identity) == identity.Dead {
		return true
	}
	p.signalGroup(held.Identity.Pid, syscall.SIGTERM)
	deadline := p.now().Add(p.StopCeiling)
	for p.now().Before(deadline) {
		p.reap(held.Identity.Pid)
		if identity.AliveRef(p.Prober, held.Identity) == identity.Dead {
			return true
		}
		time.Sleep(20 * time.Millisecond)
	}
	p.signalGroup(held.Identity.Pid, syscall.SIGKILL)
	// Give the kill a moment to land, reaping the zombie so death is
	// provable; only a definitive absence is proven.
	for i := 0; i < 25; i++ {
		p.reap(held.Identity.Pid)
		if identity.AliveRef(p.Prober, held.Identity) == identity.Dead {
			return true
		}
		time.Sleep(20 * time.Millisecond)
	}
	return false
}

func (p *ProcComponents) signalGroup(pid int64, sig syscall.Signal) {
	if p.signal != nil {
		_ = p.signal(pid, sig)
		return
	}
	// Negative pid signals the whole process group (setsid: pgid=pid).
	_ = syscall.Kill(int(-pid), sig)
}

// Group enumeration goes through seams so the ceiling's error paths are
// testable without a process-table fixture (dispatch-supervise-6).
var (
	groupAllPids = identity.AllPids
	groupGetpgid = func(pid int64) (int64, error) {
		pg, err := syscall.Getpgid(int(pid))
		return int64(pg), err
	}
)

// processGroupMembers counts live processes in the group led by pgid — the
// REAL enumeration the ceiling verdict needs (review dispatch-supervise-6:
// the old leader-only floor could never exceed len(held), so the SLC-R4-010
// duration property was vacuous outside test injection). Only ESRCH counts
// as an absent member; any other failure makes the count indeterminable —
// a process-table denial must not undercount while the breaker resets.
func processGroupMembers(pgid int64) (int, error) {
	pids, err := groupAllPids()
	if err != nil {
		return 0, fmt.Errorf("group ceiling count is indeterminable: %w", err)
	}
	count := 0
	for _, pid := range pids {
		pg, err := groupGetpgid(pid)
		if err != nil {
			if err == syscall.ESRCH {
				continue // genuinely gone between enumeration and probe
			}
			return 0, fmt.Errorf("group ceiling count is indeterminable: getpgid %d: %w", pid, err)
		}
		if pg == pgid {
			count++
		}
	}
	return count, nil
}

// GroupMemberPids lists the live members of the group led by pgid, minus an
// excluded pid — the F4 kill domain "my own group except me" (D32). The
// same indeterminability rule as the ceiling count: only ESRCH is an absent
// member; any other probe failure refuses, because a sweep that silently
// undercounts would prove a death that was not proven.
func GroupMemberPids(pgid int64, except ...int64) ([]int64, error) {
	excluded := map[int64]bool{}
	for _, pid := range except {
		excluded[pid] = true
	}
	pids, err := groupAllPids()
	if err != nil {
		return nil, fmt.Errorf("group membership is indeterminable: %w", err)
	}
	var members []int64
	for _, pid := range pids {
		if excluded[pid] {
			continue
		}
		pg, err := groupGetpgid(pid)
		if err != nil {
			if err == syscall.ESRCH {
				continue
			}
			return nil, fmt.Errorf("group membership is indeterminable: getpgid %d: %w", pid, err)
		}
		if pg == pgid {
			members = append(members, pid)
		}
	}
	return members, nil
}
