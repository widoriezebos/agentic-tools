package supervise

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/registry"
)

// scheduleProber answers liveness from a programmable table so tests
// can force Dead/Unknown without real processes.
type procProber map[int64]identity.Liveness

func (p procProber) Probe(pid int64) (identity.Exact, identity.Liveness, error) {
	state, known := p[pid]
	if !known {
		return identity.Exact{}, identity.Dead, nil
	}
	if state == identity.Alive {
		return identity.Exact{Pid: pid, StartedAt: time.Unix(200, 0)}, identity.Alive, nil
	}
	if state == identity.Unknown {
		return identity.Exact{}, identity.Unknown, os.ErrPermission
	}
	return identity.Exact{}, identity.Dead, nil
}

func TestProcObserveThreeWay(t *testing.T) {
	dir := t.TempDir()
	now := time.Unix(1000, 0)
	comps := &ProcComponents{
		SupervisionDir: dir, IntervalSec: 5,
		Prober: procProber{50: identity.Alive},
		clock:  func() time.Time { return now },
	}
	held := Held{Component: Watcher, Identity: identity.Ref{Pid: 50, StartedAtSec: 200}}

	// Alive but no heartbeat file: indeterminable, never a verdict.
	if got := comps.Observe(held); got != Indeterminable {
		t.Fatalf("missing heartbeat must be indeterminable, got %v", got)
	}
	writeBeat := func(pid, observed int64) {
		record, _ := json.Marshal(heartbeatRecord{Pid: pid, PidStartedAt: 200, ObservedAtEpoch: observed})
		os.WriteFile(filepath.Join(dir, "watcher.heartbeat.json"), record, 0o644)
	}
	writeBeat(50, now.Unix()) // fresh
	if got := comps.Observe(held); got != Healthy {
		t.Fatalf("fresh beat + alive must be healthy, got %v", got)
	}
	writeBeat(50, now.Unix()-100) // stale
	if got := comps.Observe(held); got != Failing {
		t.Fatalf("stale beat must be failing, got %v", got)
	}
	writeBeat(99, now.Unix()) // wrong generation's pid
	if got := comps.Observe(held); got != Failing {
		t.Fatalf("foreign heartbeat pid must be failing, got %v", got)
	}

	comps.Prober = procProber{50: identity.Dead}
	if got := comps.Observe(held); got != Failing {
		t.Fatalf("dead identity must be failing, got %v", got)
	}
	comps.Prober = procProber{50: identity.Unknown}
	if got := comps.Observe(held); got != Indeterminable {
		t.Fatalf("unknown liveness must be indeterminable, got %v", got)
	}
}

// Stop is TERM-then-KILL and reports proven-gone honestly: a probeable
// death is proven; an unknown is NOT (teardownComplete stays honest).
func TestProcStopProvenGone(t *testing.T) {
	var signals []syscall.Signal
	var mu sync.Mutex
	prober := procProber{50: identity.Alive}
	comps := &ProcComponents{
		Prober:      prober,
		StopCeiling: 50 * time.Millisecond,
		clock:       time.Now,
		signal: func(pid int64, sig syscall.Signal) error {
			mu.Lock()
			signals = append(signals, sig)
			// The process dies on TERM in this scenario.
			prober[50] = identity.Dead
			mu.Unlock()
			return nil
		},
	}
	held := Held{Component: Watcher, Identity: identity.Ref{Pid: 50, StartedAtSec: 200}}
	if !comps.Stop(held) {
		t.Fatal("a process that dies on TERM must be proven gone")
	}
	if len(signals) == 0 || signals[0] != syscall.SIGTERM {
		t.Fatalf("stop must TERM first: %v", signals)
	}

	// An already-dead identity is proven without a signal.
	comps.Prober = procProber{51: identity.Dead}
	if !comps.Stop(Held{Component: Reaper, Identity: identity.Ref{Pid: 51, StartedAtSec: 200}}) {
		t.Fatal("an already-dead identity is proven gone")
	}

	// A survivor whose liveness stays UNKNOWN is NOT proven gone.
	comps.Prober = procProber{52: identity.Unknown}
	comps.signal = func(int64, syscall.Signal) error { return nil }
	if comps.Stop(Held{Component: Watcher, Identity: identity.Ref{Pid: 52, StartedAtSec: 200}}) {
		t.Fatal("an unknown-liveness survivor must NOT read as proven gone")
	}
}

// A real detached child: launch it, observe its heartbeat, stop it.
func TestProcLaunchRealChild(t *testing.T) {
	dir := t.TempDir()
	comps := &ProcComponents{
		SupervisionDir: dir, IntervalSec: 60,
		Prober:      identity.KernelProber{},
		StopCeiling: 2 * time.Second,
		Command: func(component Component, tag, heartbeatPath string) []string {
			// A stub component: write a heartbeat, then sleep.
			script := `printf '{"pid":'"$$"',"pidStartedAt":0,"observedAtEpoch":'"$(date +%s)"'}' > "$1"; sleep 30`
			return []string{"/bin/sh", "-c", script, "sh", heartbeatPath}
		},
	}
	ref, err := comps.Launch(Watcher, "w1")
	if err != nil {
		t.Fatalf("launch: %v", err)
	}
	if ref.Pid <= 0 {
		t.Fatalf("no pid captured: %+v", ref)
	}
	// The child is detached and alive.
	if identity.AliveRef(identity.KernelProber{}, ref) != identity.Alive {
		t.Fatal("launched child not alive")
	}
	// Group count sees the leader.
	count, err := comps.GroupCount([]Held{{Identity: ref, Component: Watcher}})
	if err != nil || count < 1 {
		t.Fatalf("group count: %d %v", count, err)
	}
	// Stop it for real.
	if !comps.Stop(Held{Component: Watcher, Identity: ref}) {
		t.Fatal("real child not proven stopped")
	}
}

// The ledger frames each record with the owner's identity and the
// engine stamp, and gating vs best-effort semantics hold.
func TestRegistryLedgerRecords(t *testing.T) {
	var records []map[string]any
	fail := false
	ledger := &RegistryLedger{
		CheckoutPath: "/repo", OwnerTag: "tag-a",
		now: func() time.Time { return time.Unix(1000, 0).UTC() },
		Append: func(record map[string]any) error {
			if fail {
				return os.ErrPermission
			}
			records = append(records, record)
			return nil
		},
	}
	if err := ledger.AppendRelaunched(2, "w2", "r2", 1); err != nil {
		t.Fatal(err)
	}
	if err := ledger.AppendLaunched(Held{Component: Watcher, Tag: "w2", Generation: 2,
		Identity: identity.Ref{Pid: 60, StartedAtSec: 300}}); err != nil {
		t.Fatal(err)
	}
	ledger.AppendExited("purpose-gone", "root gone", true)
	if len(records) != 3 {
		t.Fatalf("want 3 records, got %d", len(records))
	}
	relaunched := records[0]
	if relaunched["event"] != registry.EventRelaunched || relaunched["engine"] != "go" ||
		relaunched["ownerTag"] != "tag-a" || relaunched["retiredThrough"] != int64(1) {
		t.Fatalf("relaunched record malformed: %+v", relaunched)
	}

	// Write-ahead is GATING: a failed relaunched append is an error.
	fail = true
	if err := ledger.AppendRelaunched(3, "w3", "r3", 2); err == nil {
		t.Fatal("a failed write-ahead must return an error (SLC-R6-006)")
	}
	// Exited is best-effort: a failed append is swallowed.
	ledger.AppendExited("giving-up", "diag", false) // must not panic or block
}

// dispatch-supervise-6: the group count is real and its error paths are
// verdicts, not silence.
func TestProcessGroupMembersErrorPaths(t *testing.T) {
	savedPids, savedPgid := groupAllPids, groupGetpgid
	defer func() { groupAllPids, groupGetpgid = savedPids, savedPgid }()

	// Real members are counted by pgid.
	groupAllPids = func() ([]int64, error) { return []int64{10, 11, 12}, nil }
	groupGetpgid = func(pid int64) (int64, error) {
		if pid == 12 {
			return 99, nil
		}
		return 42, nil
	}
	if n, err := processGroupMembers(42); err != nil || n != 2 {
		t.Fatalf("count = %d, %v; want 2 members", n, err)
	}

	// ESRCH is genuine absence, not an error.
	groupGetpgid = func(pid int64) (int64, error) {
		if pid == 11 {
			return 0, syscall.ESRCH
		}
		return 42, nil
	}
	if n, err := processGroupMembers(42); err != nil || n != 2 {
		t.Fatalf("ESRCH member: count = %d, %v; want 2", n, err)
	}

	// Any other Getpgid failure is indeterminable.
	groupGetpgid = func(pid int64) (int64, error) { return 0, syscall.EPERM }
	if _, err := processGroupMembers(42); err == nil {
		t.Fatal("a denied probe must be indeterminable, not an undercount")
	}

	// An unreadable process table is indeterminable.
	groupAllPids = func() ([]int64, error) { return nil, syscall.EIO }
	if _, err := processGroupMembers(42); err == nil {
		t.Fatal("an unreadable table must be indeterminable")
	}
}

// GroupMemberPids: the F4 kill domain (own group minus self), same
// indeterminability contract as the ceiling count.
func TestGroupMemberPids(t *testing.T) {
	restoreAll, restoreGet := groupAllPids, groupGetpgid
	defer func() { groupAllPids, groupGetpgid = restoreAll, restoreGet }()
	groupAllPids = func() ([]int64, error) { return []int64{10, 11, 12, 13}, nil }
	groupGetpgid = func(pid int64) (int64, error) {
		switch pid {
		case 10, 11, 12:
			return 42, nil
		case 13:
			return 0, syscall.ESRCH // gone between enumeration and probe
		}
		return 0, syscall.EPERM
	}
	members, err := GroupMemberPids(42, 11)
	if err != nil || len(members) != 2 || members[0] != 10 || members[1] != 12 {
		t.Fatalf("members = %v, %v", members, err)
	}
	groupGetpgid = func(pid int64) (int64, error) { return 0, syscall.EPERM }
	if _, err := GroupMemberPids(42, 0); err == nil {
		t.Fatal("an unreadable probe must refuse, not undercount")
	}
}
