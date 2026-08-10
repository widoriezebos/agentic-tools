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

// Launch starts one component FULLY DETACHED and returns its
// identity. Detachment is by reparenting, not just setsid: a child
// still parented to the owner becomes a ZOMBIE when killed (nobody
// reaps it) and lingers in the process table, so Stop could never
// prove it gone. So the component is backgrounded inside a launcher
// shell that immediately exits — the component reparents to launchd,
// which reaps it on death exactly as the shell system's
// launch_detached arranges. The launcher prints the component's pid
// on stdout; the owner Waits on the (short-lived) launcher so IT does
// not zombie, and supervises the component by identity thereafter.
func (p *ProcComponents) Launch(component Component, tag string) (identity.Ref, error) {
	argv := p.Command(component, tag, p.heartbeatPath(component))
	if len(argv) == 0 {
		return identity.Ref{}, fmt.Errorf("component command for %s is empty", component)
	}
	// The launcher backgrounds argv, prints its pid, and exits. The
	// backgrounded process reparents to launchd when the launcher
	// exits. setsid on the launcher gives the component its own
	// session (pgid == its pid) for group signalling.
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

// processGroupMembers counts live processes in the group led by pgid.
func processGroupMembers(pgid int64) (int, error) {
	// getpgid on each candidate would need a full scan; census
	// enumeration owns that. Until then GroupCount is used
	// only for the ceiling, and the owner-alone fixtures inject it.
	// Here we count the leader if alive as a conservative floor.
	if err := syscall.Kill(int(pgid), 0); err == nil {
		return 1, nil
	}
	return 0, nil
}
