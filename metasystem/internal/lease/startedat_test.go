package lease

import (
	"os"
	"os/exec"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// TestStartedAtNeedsNoCommandLine: a process's start time is readable before
// its command line is (Linux publishes a just-exec'd image's argv only after
// Start returns) and after it is gone (an unreaped child that exited). The
// start time comes from the kernel's start record alone, the same record
// ProcessIdentity reads it from, so a caller that wants only the start never
// fails on an unreadable command. The unreaped child stands for the exec
// window deterministically: its start is readable and its argv is not.
func TestStartedAtNeedsNoCommandLine(t *testing.T) {
	t.Parallel()
	read, write, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command("/bin/sh", "-c", "read value")
	command.Stdin = read
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	_ = read.Close()
	reaped := false
	t.Cleanup(func() {
		_ = write.Close()
		if !reaped {
			_ = command.Process.Kill()
			_ = command.Wait()
		}
	})
	pid := int64(command.Process.Pid)
	running, state, err := (identity.KernelProber{}).ReadStart(pid)
	if err != nil || state != identity.Alive {
		t.Fatalf("read the child's start: state=%s err=%v", state, err)
	}
	_ = write.Close()
	if err := awaitUnreapedExit(command.Process.Pid); err != nil {
		t.Fatalf("wait for the child's exit: %v", err)
	}
	if argv, known := (identity.KernelProber{}).ReadArgv(pid); known && len(argv) > 0 {
		t.Fatalf("the unreaped child's argv reads %q; this test needs a process whose command is unreadable", argv)
	}
	if _, ok := ProcessIdentity(pid, nil); ok {
		t.Fatal("ProcessIdentity read a command for a process without one")
	}
	started, ok := StartedAt(pid, nil)
	if !ok || started != running.StartedAt.Unix() {
		t.Fatalf("StartedAt = %d, %t; want the kernel start %d without a command line", started, ok, running.StartedAt.Unix())
	}
	if !Live(pid, running.StartedAt.Unix(), nil) || Live(pid, running.StartedAt.Unix()-61, nil) {
		t.Fatal("Live did not compare the readable start of a process without a command line")
	}
	reaped = true
	_ = command.Wait()
	if _, ok := StartedAt(pid, nil); ok {
		t.Fatal("StartedAt read a start for a reaped child")
	}
}

// TestStartedAtIsProcessIdentitysStart: where both read, the start time is
// the one ProcessIdentity authenticates with, from the kernel and from a
// fixture table alike (the single-source rule).
func TestStartedAtIsProcessIdentitysStart(t *testing.T) {
	t.Parallel()
	self := int64(os.Getpid())
	whole, ok := ProcessIdentity(self, nil)
	if !ok {
		t.Fatal("ProcessIdentity cannot read this test")
	}
	if started, ok := StartedAt(self, nil); !ok || started != whole.StartedAt {
		t.Fatalf("StartedAt(self) = %d, %t; ProcessIdentity says %d", started, ok, whole.StartedAt)
	}
	// A fixture row with a start and a command is the one source; a row
	// without a command leaves both readers on the kernel.
	fixture := startedAtFixture{
		self: {StartedAt: 42, HasStartedAt: true, Command: "fixture main", HasCommand: true},
	}
	if id, ok := ProcessIdentity(self, fixture); !ok || id.StartedAt != 42 {
		t.Fatalf("ProcessIdentity(fixture self) = %+v, %t", id, ok)
	}
	if started, ok := StartedAt(self, fixture); !ok || started != 42 {
		t.Fatalf("StartedAt(fixture self) = %d, %t; want the fixture's 42", started, ok)
	}
	commandless := startedAtFixture{self: {StartedAt: 43, HasStartedAt: true}}
	if started, ok := StartedAt(self, commandless); !ok || started != whole.StartedAt {
		t.Fatalf("StartedAt(commandless fixture self) = %d, %t; want the kernel's %d as ProcessIdentity reads it", started, ok, whole.StartedAt)
	}
}

type startedAtFixture map[int64]identity.FixtureEntry

func (f startedAtFixture) FixtureEntry(pid int64) (identity.FixtureEntry, bool) {
	entry, ok := f[pid]
	return entry, ok
}
