package lane

import (
	"errors"
	"os/exec"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
)

func nameMachine(t *testing.T, dir, machine string) {
	t.Helper()
	command := exec.Command("git", "-C", dir, "config", "metasystem.goal.machine", machine)
	command.Env = gittree.ScrubbedEnviron()
	if out, err := command.CombinedOutput(); err != nil {
		t.Fatalf("name machine: %v %s", err, out)
	}
}

// The lane's claim identity is the registered lane's machine, the stable
// lineage landing-lane and the custody epoch of the host record (design r10
// K7): no session, lease or process takes part, and a new registration
// takes a new epoch.
func TestClaimIdentityIsTheRegisteredLaneAtItsCustodyEpoch(t *testing.T) {
	t.Parallel()
	home, first, second := laneDirs(t)
	if _, err := Claim(home); err == nil {
		t.Fatal("a claim identity was named with no lane registered")
	}
	nameMachine(t, first, "lane-host")
	nameMachine(t, second, "lane-host")
	record := register(t, home, first)
	identity, err := Claim(home)
	if err != nil {
		t.Fatal(err)
	}
	want := ClaimIdentity{Root: record.Root, Machine: "lane-host", Lineage: ClaimLineage, Epoch: record.CustodyEpoch}
	if identity != want || identity.Epoch != 1 || identity.Actor() != AccountID(record.Root) {
		t.Fatalf("claim identity = %+v (actor %s), want %+v", identity, identity.Actor(), want)
	}
	next := register(t, home, second)
	moved, err := Claim(home)
	if err != nil || moved.Epoch != 2 || moved.Root != next.Root {
		t.Fatalf("after a new registration = %+v, %v; want epoch 2 at %s", moved, err, second)
	}
	if ClaimLineage != "landing-lane" || AgentLineage != "landing-agent" {
		t.Fatalf("lineages = %s, %s", ClaimLineage, AgentLineage)
	}
}

// A lane whose checkout has no machine nickname has no claim identity: a
// claim is never handed to a lane that cannot be named on the ledger.
func TestClaimIdentityNeedsTheLanesMachine(t *testing.T) {
	t.Parallel()
	home, first, _ := laneDirs(t)
	register(t, home, first)
	if identity, err := Claim(home); err == nil {
		t.Fatalf("claim identity without a machine nickname = %+v", identity)
	}
	var refusal *Refusal
	if _, err := Claim(home); !errors.As(err, &refusal) || refusal.Code != CodeClaimUnnamed {
		t.Fatalf("refusal = %v, want %s", err, CodeClaimUnnamed)
	}
}
