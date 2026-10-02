package proofrun

import (
	"os/exec"
	"runtime"
	"slices"
	"syscall"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// groupOfTwo starts a leader in a group of its own and a second member in
// that group; both end with the test.
func groupOfTwo(t *testing.T) (leader, member int64) {
	t.Helper()
	start := func(group int) *exec.Cmd {
		command := exec.Command("cat")
		command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pgid: group}
		stdin, err := command.StdinPipe()
		if err != nil {
			t.Fatal(err)
		}
		if err := command.Start(); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = stdin.Close(); _ = command.Wait() })
		return command
	}
	first := start(0)
	second := start(first.Process.Pid)
	return int64(first.Process.Pid), int64(second.Process.Pid)
}

// The group-member scans read only the process table they are given: a
// table of the test's own processes yields exactly its members, and a
// process outside the table is never a member, whatever runs on the host.
func TestGroupMemberScansReadOnlyTheirTable(t *testing.T) {
	t.Parallel()
	leader, member := groupOfTwo(t)
	table := identity.ListedProcessTable{leader, member}
	if members, err := liveGroupMembers(table, leader); err != nil || !slices.Equal(members, []int64{leader, member}) {
		t.Fatalf("live members = %v, %v; want [%d %d]", members, err, leader, member)
	}
	if members, err := liveGroupMembers(identity.ListedProcessTable{leader}, leader); err != nil || !slices.Equal(members, []int64{leader}) {
		t.Fatalf("live members of a table without the member = %v, %v; want only the leader", members, err)
	}
	if members, err := custodyGroupMembers(table, leader); err != nil || !slices.Equal(members, []int64{member}) {
		t.Fatalf("custody members = %v, %v; want [%d] (the leader is omitted)", members, err, member)
	}
	if members, err := custodyGroupMembers(identity.ListedProcessTable{leader}, leader); err != nil || len(members) != 0 {
		t.Fatalf("custody members of a table without the member = %v, %v; want none", members, err)
	}
	// The member's argv is readable once its exec completed; awaited on
	// the probe, never on a clock.
	for {
		exact, state, err := (identity.KernelProber{}).Probe(member)
		if err == nil && state == identity.Alive && exact.ArgvKnown {
			break
		}
		if t.Context().Err() != nil {
			t.Fatal("the member never became readable")
		}
		runtime.Gosched()
	}
	if live, err := processGroupLive(table, leader, identity.KernelProber{}); err != nil || !live {
		t.Fatalf("group with live members reads live=%v, %v", live, err)
	}
	if live, err := processGroupLive(identity.ListedProcessTable{}, leader, identity.KernelProber{}); err != nil || live {
		t.Fatalf("an empty table reads the group live=%v, %v; want not live", live, err)
	}
}
