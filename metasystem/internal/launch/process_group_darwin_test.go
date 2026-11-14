package launch

import (
	"path/filepath"
	"syscall"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

func TestGroupListedReadsTheKernelsGroup(t *testing.T) {
	t.Parallel()
	processes := OSProcesses{Prober: identity.KernelProber{}}
	child, ref, err := processes.StartChild(Command{
		Program: "/bin/sh", Args: []string{"-c", "exec sleep 600"},
		LogPath: filepath.Join(t.TempDir(), "child.log"),
	})
	if err != nil {
		t.Fatal(err)
	}
	reaped := false
	t.Cleanup(func() {
		_ = processes.SignalGroup(ref.Pid, syscall.SIGKILL)
		if !reaped {
			_, _ = child.Wait()
		}
	})
	if listed, err := groupListed(ref.Pid); err != nil || !listed {
		t.Fatalf("live group listed = %v: %v", listed, err)
	}
	if err := processes.SignalGroup(ref.Pid, syscall.SIGKILL); err != nil {
		t.Fatal(err)
	}
	code, err := child.Wait()
	reaped = true
	if code != 128+int(syscall.SIGKILL) {
		t.Fatalf("killed child exit = %d: %v", code, err)
	}
	if listed, err := groupListed(ref.Pid); err != nil || listed {
		t.Fatalf("reaped group listed = %v: %v", listed, err)
	}
	if alive, err := processes.GroupAlive(ref.Pid); err != nil || alive {
		t.Fatalf("reaped group alive = %v: %v", alive, err)
	}
}
