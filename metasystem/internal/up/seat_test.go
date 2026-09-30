package up

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/supervise"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
)

// seatLineage is the lineage a steward's seat launch names in
// METASYSTEM_OWNER_LINEAGE (launch.SeatOwnerLineage); up reads only the
// environment, so the bed spells the value.
const seatLineage = "steward-seat"

// seatUpOptions arms a checkout for the test process as the session main,
// reported by pid and start time as the SessionStart hook reports it, with
// the supervision and steward-runner owners recorded instead of started.
func seatUpOptions(t *testing.T) Options {
	t.Helper()
	root := canonicalRuntimePath(t.TempDir())
	binary := filepath.Join(root, "metasystem")
	if err := testexec.WriteFile(binary, []byte("engine\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	stageEnrollment(t, root, binary, 1)
	pid, started := liveSelf(t)
	return Options{
		Root: root, MetasystemRoot: syntheticFingerprintRoot(t), Scope: root, Binary: binary, WaitScaleMilli: 1,
		Pid: pid, StartTime: started, CallerPid: pid, Runtime: "claude",
		EnsureArmed: func(armed supervise.EnsureOptions) (supervise.EnsureResult, error) {
			return supervise.EnsureResult{Action: "verified", Owner: supervise.ArmingOwner{Pid: 6161}, Generation: armed.FenceGeneration}, nil
		},
		EnsureStewardRunner: func(string, *steward.EnrolledBinary, int) (steward.EnsureRunnerResult, error) {
			return steward.EnsureRunnerResult{Action: "verified", Pid: 5151, Generation: 1}, nil
		},
	}
}

func component(result Result, name string) ComponentOutcome {
	for _, item := range result.Components {
		if item.Component == name {
			return item
		}
	}
	return ComponentOutcome{}
}

// TestUpAdoptsTheLaunchersLineageForAMain (authority, D-seat): a session
// reported by pid whose launcher named METASYSTEM_OWNER_LINEAGE=steward-seat,
// with the seat's empty session id, is announced under that lineage, is
// classified MAIN, and takes the unheld lease fresh at epoch 1 under the
// same lineage: up answers "holder". up proves the pair and the caller's
// descent only; the steward plumbing above the session is never examined.
func TestUpAdoptsTheLaunchersLineageForAMain(t *testing.T) {
	t.Setenv("METASYSTEM_OWNER_LINEAGE", seatLineage)
	t.Setenv("METASYSTEM_SESSION_ID", "")
	t.Setenv("METASYSTEM_DELEGATE_ROOT", "")
	options := seatUpOptions(t)
	result := ordinary(options)
	if result.Outcome != "armed" || result.Authority != "writer" || component(result, "checkout-lease").Outcome != "holder" {
		t.Fatalf("a steward-started main was not the holder: %#v", result)
	}
	view, err := lease.ClassifyVerb(options.Root, options.Pid)
	if err != nil {
		t.Fatal(err)
	}
	if view.Class != "MAIN" || !view.Holder || view.ClaimEpoch == nil || *view.ClaimEpoch != 1 {
		t.Fatalf("classification = %+v", view)
	}
	announcements := lease.AnnouncementsForOwnerLineage(options.Root, seatLineage)
	if len(announcements) != 1 || announcements[0].Pid != options.Pid {
		t.Fatalf("announcements under %s = %+v", seatLineage, announcements)
	}
	current := readLease(t, options.Root)
	if current.OwnerLineage != seatLineage || current.ClaimEpoch != 1 || current.Pid != options.Pid || len(current.Takeovers) != 0 {
		t.Fatalf("lease = %+v", current)
	}
}

// heldSeat starts a live stand-in for a seat main and announces it under the
// seat lineage through the lease verb up calls; it returns the process.
func heldSeat(t *testing.T, root, session string) *testutil.HeldProcess {
	t.Helper()
	held := testutil.StartHeldProcess(t, exec.Command("/bin/sh", "-c", "printf x >&3; IFS= read -r _ || :"))
	t.Cleanup(func() { _ = held.Kill() })
	exact, state, err := (identity.KernelProber{}).Probe(int64(held.Command.Process.Pid))
	if err != nil || state != identity.Alive {
		t.Fatalf("read seat stand-in: %v %s", err, state)
	}
	if _, err := lease.Announce(root, session, exact.Pid, exact.StartedAt.Unix(), "metasystem-main-claude-"+session, "claude", seatLineage); err != nil {
		t.Fatalf("announce %s: %v", session, err)
	}
	return held
}

func readLease(t *testing.T, root string) lease.Lease {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, "artifacts", "agents", "mains", "worktree-lease.json"))
	if err != nil {
		t.Fatal(err)
	}
	var value lease.Lease
	if err := json.Unmarshal(data, &value); err != nil {
		t.Fatal(err)
	}
	return value
}

// TestPersonBesideASeatMainIsAdvisor (coexistence): while the seat main
// holds the lease, a person's session announced beside it under another
// lineage is "advisor", and the lease stays the seat's.
func TestPersonBesideASeatMainIsAdvisor(t *testing.T) {
	t.Setenv("METASYSTEM_OWNER_LINEAGE", "person-terminal")
	t.Setenv("METASYSTEM_SESSION_ID", "")
	options := seatUpOptions(t)
	seat := heldSeat(t, options.Root, "seat-one")
	before := readLease(t, options.Root)
	if before.OwnerLineage != seatLineage || before.Pid != int64(seat.Command.Process.Pid) {
		t.Fatalf("the seat does not hold the lease: %+v", before)
	}
	result := ordinary(options)
	if result.Outcome != "advisor" || result.Authority != "read-only" || component(result, "checkout-lease").Outcome != "advisor" {
		t.Fatalf("a person beside a live seat main = %#v", result)
	}
	after := readLease(t, options.Root)
	if after.HolderMainId != before.HolderMainId || after.ClaimEpoch != before.ClaimEpoch || len(after.Takeovers) != 0 {
		t.Fatalf("the person's session moved the seat's lease: %+v -> %+v", before, after)
	}
}

// TestSuccessorSeatInheritsADeadPredecessorsLease (SW-14, the lease leg of
// section 3's test 11): seat one holds the lease and dies; the successor's
// up under the same lineage answers "holder" with the epoch preserved and no
// takeover recorded. A person's session under another lineage, after the
// successor dies too, takes the lease over with the epoch plus one.
func TestSuccessorSeatInheritsADeadPredecessorsLease(t *testing.T) {
	t.Setenv("METASYSTEM_SESSION_ID", "")
	t.Run("successor by up", func(t *testing.T) {
		t.Setenv("METASYSTEM_OWNER_LINEAGE", seatLineage)
		options := seatUpOptions(t)
		one := heldSeat(t, options.Root, "seat-one")
		first := readLease(t, options.Root)
		_ = one.Kill()
		result := ordinary(options)
		if result.Outcome != "armed" || component(result, "checkout-lease").Outcome != "holder" {
			t.Fatalf("the successor seat = %#v", result)
		}
		next := readLease(t, options.Root)
		if next.ClaimEpoch != first.ClaimEpoch || len(next.Takeovers) != 0 || next.OwnerLineage != seatLineage || next.Pid != options.Pid {
			t.Fatalf("the successor did not succeed the lease: %+v -> %+v", first, next)
		}
	})
	t.Run("a person takes over", func(t *testing.T) {
		t.Setenv("METASYSTEM_OWNER_LINEAGE", "person-terminal")
		options := seatUpOptions(t)
		one := heldSeat(t, options.Root, "seat-one")
		first := readLease(t, options.Root)
		_ = one.Kill()
		two := heldSeat(t, options.Root, "seat-two")
		second := readLease(t, options.Root)
		if second.ClaimEpoch != first.ClaimEpoch || len(second.Takeovers) != 0 || second.Pid != int64(two.Command.Process.Pid) {
			t.Fatalf("seat two did not succeed seat one: %+v -> %+v", first, second)
		}
		_ = two.Kill()
		result := ordinary(options)
		if result.Outcome != "armed" || component(result, "checkout-lease").Outcome != "holder" {
			t.Fatalf("the person's session = %#v", result)
		}
		third := readLease(t, options.Root)
		if third.ClaimEpoch != second.ClaimEpoch+1 || len(third.Takeovers) != 1 || third.OwnerLineage != "person-terminal" {
			t.Fatalf("the person did not take over: %+v -> %+v", second, third)
		}
	})
}
