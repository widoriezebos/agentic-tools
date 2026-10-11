package rehearsal

import (
	"bufio"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/enginebuild"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

func TestRehearsalFinishWaitsForDetachedChildBeforeCleanup(t *testing.T) {
	t.Parallel()
	path := recordScratch(t)
	engine := filepath.Join(path, "engine")
	if err := os.WriteFile(engine, []byte(enginebuild.StampRecord("tree")), 0644); err != nil {
		t.Fatal(err)
	}
	s := &Scratch{Record: Record{Tree: "tree", Engine: engine, Scratch: path}, base: filepath.Dir(filepath.Dir(path)), Registry: filepath.Join(path, "registry/.metasystem"), ctx: context.Background()}
	child := exec.Command("sh", "-c", "echo ready; read release")
	child.Dir = path
	child.Env = append(os.Environ(), "METASYSTEM_SUPERVISION_REGISTRY_HOME="+filepath.Dir(s.Registry))
	child.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	input, err := child.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	output, err := child.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = input.Close(); _ = child.Process.Kill(); _ = child.Wait() })
	if text, err := bufio.NewReader(output).ReadString('\n'); err != nil || text != "ready\n" {
		t.Fatalf("child readiness: %q: %v", text, err)
	}
	s.processes = identity.ListedProcessTable{int64(child.Process.Pid)}
	children, err := s.liveChildren()
	if err != nil || len(children) != 1 || children[0].Pid != int64(child.Process.Pid) {
		t.Fatalf("owned detached children: %v: %v", children, err)
	}
	table := &rehearsalChildTable{ProcessTable: s.processes, observed: make(chan struct{}), release: make(chan struct{})}
	s.processes = table
	done := make(chan error, 1)
	module := t.TempDir()
	go func() { done <- s.Finish(module, time.Now(), false, nil) }()
	select {
	case err := <-done:
		t.Fatalf("cleanup returned with live child: %v", err)
	case <-table.observed:
	}
	if _, err := os.Stat(engine); err != nil {
		t.Fatalf("cleanup removed live child's scratch: %v", err)
	}
	if _, err := input.Write([]byte("release\n")); err != nil {
		t.Fatal(err)
	}
	if err := child.Wait(); err != nil {
		t.Fatal(err)
	}
	close(table.release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("success did not remove scratch: %v", err)
	}
}

// The second scan proves Finish observed the live child and waited. Its
// release lets the next scan observe the child's actual exit without sleeps.
type rehearsalChildTable struct {
	identity.ProcessTable
	scans             int
	observed, release chan struct{}
}

func (table *rehearsalChildTable) Pids() ([]int64, error) {
	table.scans++
	if table.scans == 2 {
		close(table.observed)
		<-table.release
	}
	return table.ProcessTable.Pids()
}
