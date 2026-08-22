package lease

import (
	"os"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// The clock-step-immune pair outranks a drifted second in holder liveness
// and same-process reconciliation; wrong pairs still read dead.
func TestPairAwareHolderIdentity(t *testing.T) {
	self := int64(os.Getpid())
	exact, state, err := (identity.KernelProber{}).Probe(self)
	if err != nil || state != identity.Alive {
		t.Fatalf("cannot probe self: %v %v", state, err)
	}
	if exact.StartTicks > 0 && exact.BootID != "" {
		if !LiveRef(self, exact.StartedAt.Unix()+3, exact.StartTicks, exact.BootID, nil) {
			t.Fatal("pair-bearing holder with drifted second read dead")
		}
		if LiveRef(self, exact.StartedAt.Unix(), exact.StartTicks+1, exact.BootID, nil) {
			t.Fatal("wrong ticks read alive")
		}
	} else if LiveRef(self, exact.StartedAt.Unix(), 0, "", nil) != Live(self, exact.StartedAt.Unix(), nil) {
		t.Fatal("pairless LiveRef diverges from Live")
	}

	current := &Lease{Pid: 7, PidStartedAt: 100, PidStartTicks: 55, BootID: "boot-a"}
	drifted := &Announcement{Pid: 7, PidStartedAt: 104, PidStartTicks: 55, BootID: "boot-a"}
	if !sameLeaseProcess(current, drifted) {
		t.Fatal("KI-33 reconciliation failed on a drifted second despite a matching pair")
	}
	rebooted := &Announcement{Pid: 7, PidStartedAt: 100, PidStartTicks: 55, BootID: "boot-b"}
	if sameLeaseProcess(current, rebooted) {
		t.Fatal("cross-boot announcement reconciled")
	}
	legacy := &Announcement{Pid: 7, PidStartedAt: 100}
	if !sameLeaseProcess(current, legacy) {
		t.Fatal("legacy announcement with matching seconds refused")
	}
}
