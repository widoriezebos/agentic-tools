//go:build linux

package diskstore

import (
	"os/exec"
	"strings"
	"syscall"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"golang.org/x/sys/unix"
)

// Batch 28 (VM): TestProcessScratchOfAKilledOwnerIsKeptWhileItsChildLives
// saw its reparented child release the writer lock and then still counted
// as a live member of the owner's session until init reaped it. An exited
// member awaiting collection holds nothing, so it keeps no root: the group
// verdict counts only a live, non-zombie member, as the proof lease reclaim
// does. The member here is this test's own unreaped child, made a zombie
// deterministically: waitid(WEXITED|WNOWAIT) returns once it has exited and
// leaves it uncollected.
func TestOwnerGroupVerdictIgnoresAZombieMember(t *testing.T) {
	t.Parallel()
	member := exec.Command("true")
	member.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := member.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = member.Wait() })
	pid := member.Process.Pid
	var info unix.Siginfo
	if err := unix.Waitid(unix.P_PID, pid, &info, unix.WEXITED|unix.WNOWAIT, nil); err != nil {
		t.Fatal(err)
	}
	if group, err := unix.Getpgid(pid); err != nil || group != pid {
		t.Fatalf("the zombie member's group reads %d, %v; want %d", group, err, pid)
	}
	record := Record{OwnerGroup: int64(pid), OwnerSession: int64(pid)}
	if verdict := ownerGroupVerdict(record, identity.ListedProcessTable{int64(pid)}); verdict.Decision != Release {
		t.Fatalf("a zombie in the owner's session keeps the root: %+v", verdict)
	}
}

// A running member still keeps the root.
func TestOwnerGroupVerdictKeepsForARunningMember(t *testing.T) {
	t.Parallel()
	member := exec.Command("cat")
	member.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	stdin, err := member.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := member.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = stdin.Close(); _ = member.Wait() })
	pid := member.Process.Pid
	record := Record{OwnerGroup: int64(pid), OwnerSession: int64(pid)}
	if verdict := ownerGroupVerdict(record, identity.ListedProcessTable{int64(pid)}); verdict.Decision != Keep || !strings.Contains(verdict.Reason, "is alive") {
		t.Fatalf("a running member in the owner's session does not keep the root: %+v", verdict)
	}
}
