package custody

import (
	"os/exec"
	"slices"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// TestStopEndsLiveWorkAndSettles (K10 deadline, K9): the keeper's stop of
// the work a cancelled session left: a live execution's group and child
// are signalled and custody settles; an execution that already ended, and
// a group whose leader is gone, are never signalled (a reused pid is never
// hit); work whose state can't be read stays for a person.
func TestStopEndsLiveWorkAndSettles(t *testing.T) {
	t.Parallel()
	home := testHome(t)
	command := exec.Command("sleep", "120")
	if _, err := Start(home, KindProve, "batch b attempt live", custodyNow, command); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() { _ = command.Wait(); close(done) }()
	t.Cleanup(func() { _ = command.Process.Kill(); <-done })
	ended, err := Open(home, KindProve, "batch b attempt ended", custodyNow)
	if err != nil {
		t.Fatal(err)
	}
	gone := deadRef(t)
	reused, _ := identity.ParseRef(gone)
	if err := BindChild(home, ended.ID, reused); err != nil {
		t.Fatal(err)
	}
	var signalled []int
	signal := func(pid int, sig syscall.Signal) error {
		signalled = append(signalled, pid)
		return syscall.Kill(pid, sig)
	}
	settlement, err := Stop(home, Probes{}, signal, 50*time.Millisecond, time.Sleep)
	if err != nil || !settlement.Settled(false) {
		t.Fatalf("after the stop: %+v %v; want custody settled", settlement, err)
	}
	pid := command.Process.Pid
	for _, target := range signalled {
		if target != pid && target != -pid {
			t.Fatalf("signalled %d; only the live execution's group %d and child may be", target, pid)
		}
	}
	if !slices.Contains(signalled, -pid) {
		t.Fatalf("signalled %v; want the live execution's group -%d", signalled, pid)
	}
	<-done
}
