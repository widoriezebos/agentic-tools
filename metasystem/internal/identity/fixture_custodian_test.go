package identity

import (
	"io"
	"strings"
	"syscall"
	"testing"
	"time"
)

type custodianTable struct {
	fixtureTable
	ownerUntil time.Time
	ownerState Liveness
}

func (table *custodianTable) Probe(pid int64) (Exact, Liveness, error) {
	if pid == 700 {
		if time.Now().Before(table.ownerUntil) {
			if table.ownerState == Alive {
				return table.fixtureTable.Probe(pid)
			}
			return Exact{}, table.ownerState, nil
		}
		return Exact{}, Dead, nil
	}
	return table.fixtureTable.Probe(pid)
}

func TestCustodianWaitsForDeadOwnerAndExcludesItself(t *testing.T) {
	owner, self := fixtureExact(700, 70).Ref(), fixtureExact(701, 71)
	environmentChild, argvChild := fixtureExact(702, 72), fixtureExact(703, 73)
	key := FixtureKey{Owner: owner, Test: t.Name(), Nonce: "00000001"}
	self.Argv, self.ArgvKnown = []string{fixtureWord(t, key)}, true
	environmentChild.Environ, environmentChild.EnvironKnown = []string{fixtureWord(t, key)}, true
	argvChild.Argv, argvChild.ArgvKnown = []string{fixtureWord(t, key)}, true
	processes := fixtureTable{700: fixtureExact(700, 70), 701: self, 702: environmentChild, 703: argvChild}
	installFixtureScanTable(t, processes)
	var signaled []int64
	var log strings.Builder
	runtime := custodianRuntime{prober: processes, self: self.Ref(), poll: time.Millisecond, bound: 2 * time.Millisecond,
		sender: func(pid int, _ syscall.Signal) error {
			signaled = append(signaled, int64(pid))
			delete(processes, int64(pid))
			return nil
		}}
	if err := reapDeadOwner(owner, &log, runtime); err == nil || len(signaled) != 0 {
		t.Fatalf("live owner allowed reap: signaled=%v err=%v", signaled, err)
	}
	table := &custodianTable{fixtureTable: processes, ownerUntil: time.Now().Add(40 * time.Millisecond), ownerState: Alive}
	runtime.prober, runtime.bound = table, 30*time.Millisecond
	if err := runCustodian(owner, strings.NewReader(""), &log, runtime); err != nil || len(signaled) != 2 || signaled[0] != 702 || signaled[1] != 703 {
		t.Fatalf("custodian signaled=%v err=%v; want children 702 and 703 after owner death, never self 701", signaled, err)
	}
	if !strings.Contains(log.String(), "pid=702 carrier=environment") || !strings.Contains(log.String(), "pid=703 carrier=argv-word") {
		t.Fatalf("custodian log %q does not name each child's carrier", log.String())
	}
}

func TestCustodianHardHaltBoundsBlockedScan(t *testing.T) {
	release, halted, done := make(chan struct{}), make(chan int, 1), make(chan error, 1)
	oldPids := survivorPids
	survivorPids = func() ([]int64, error) { <-release; return nil, nil }
	runtime := custodianRuntime{prober: fixtureTable{}, poll: time.Millisecond, bound: 5 * time.Millisecond,
		halt: func(code int) { halted <- code }}
	go func() { done <- reapDeadOwner(fixtureExact(700, 70).Ref(), io.Discard, runtime) }()
	t.Cleanup(func() { close(release); <-done; survivorPids = oldPids })
	select {
	case code := <-halted:
		if code != 2 {
			t.Fatalf("hard halt exit=%d, want 2", code)
		}
	case <-time.After(runtime.bound + custodianHaltMargin + 250*time.Millisecond):
		t.Fatal("blocked fixture scan was not hard-halted")
	}
}

func TestCustodianKeepsSeparateProofAndCleanupBudgets(t *testing.T) {
	owner, child := fixtureExact(700, 70).Ref(), fixtureExact(702, 72)
	key := FixtureKey{Owner: owner, Test: t.Name(), Nonce: "00000001"}
	child.Environ, child.EnvironKnown = []string{fixtureWord(t, key)}, true
	processes := fixtureTable{702: child}
	installFixtureScanTable(t, processes)
	prober := &custodianTable{fixtureTable: processes, ownerUntil: time.Now().Add(45 * time.Millisecond), ownerState: Unknown}
	var signaled []int64
	runtime := custodianRuntime{prober: prober, poll: time.Millisecond, bound: 60 * time.Millisecond,
		sender: func(pid int, _ syscall.Signal) error {
			signaled = append(signaled, int64(pid))
			delete(processes, int64(pid))
			return nil
		}}
	var log strings.Builder
	if err := runCustodian(owner, strings.NewReader(""), &log, runtime); err != nil || len(signaled) != 1 || signaled[0] != 702 ||
		!strings.Contains(log.String(), "action=complete") {
		t.Fatalf("custodian signaled=%v log=%q err=%v; want a complete cleanup after proof", signaled, log.String(), err)
	}
}
