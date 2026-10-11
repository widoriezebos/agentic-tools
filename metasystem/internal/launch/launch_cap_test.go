package launch

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

func TestLaneFixLaunchCapIsRetained(t *testing.T) {
	t.Parallel()
	m, _, _, _ := manager(t)
	m.Supervisor = childStarter(m)
	m.LaunchCap = func(StartSpec) (time.Duration, error) { return 7 * time.Minute, nil }
	record, err := m.Start(StartSpec{ID: "bounded", Kind: "build", WorkingDirectory: t.TempDir(), Brief: brief(t)})
	if err != nil {
		t.Fatal(err)
	}
	deadline, err := time.Parse(time.RFC3339Nano, readString(record.AdapterData, "deadline"))
	if err != nil || deadline.Sub(m.Now()) != 7*time.Minute {
		t.Fatalf("deadline %s: %v", deadline, err)
	}
}

// A real child proves expiry terminates the process group and survives a manager restart.
func TestLaneFixLaunchDeadlineRealProcess(t *testing.T) {
	t.Parallel()
	m, _, _, _ := manager(t)
	reaped := make(chan struct{})
	m.Processes, m.Prober = reapingProcesses{OSProcesses{Prober: identity.KernelProber{}}, reaped}, identity.KernelProber{}
	m.Sleep = func(time.Duration) { <-reaped }
	m.Adapters = map[string]Adapter{"plain-exec": PlainExec{}}
	record := seed(t, m, "deadline-child", Starting)
	body, _ := json.Marshal(PlainBrief{Argv: []string{"/bin/sh", "-c", "exec sleep 600"}, Dir: record.WorkingDirectory, Env: os.Environ()})
	path := filepath.Join(t.TempDir(), "plain.json")
	if err := os.WriteFile(path, body, 0600); err != nil {
		t.Fatal(err)
	}
	_, err := m.Store.Update(record.ID, func(r *Record) error {
		r.Adapter = "plain-exec"
		setString(r.AdapterData, "brief", path)
		setString(r.AdapterData, "deadline", m.Now().Add(-time.Second).Format(time.RFC3339Nano))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	record, err = m.Supervise(record.ID)
	if err != nil || record.State != Failed || record.Cause != "deadline" || record.ProcessGroup == nil {
		t.Fatalf("deadline outcome %+v: %v", record, err)
	}
	alive, err := m.Processes.GroupAlive(record.ProcessGroup.Pid)
	if err != nil || alive {
		t.Fatalf("expired group remains alive=%v: %v", alive, err)
	}
}

type reapingProcesses struct {
	OSProcesses
	reaped chan struct{}
}

func (p reapingProcesses) StartChild(command Command) (Child, identity.Ref, error) {
	child, ref, err := p.OSProcesses.StartChild(command)
	if err != nil {
		return child, ref, err
	}
	return reapingChild{child, p.reaped}, ref, nil
}

type reapingChild struct {
	Child
	reaped chan struct{}
}

func (child reapingChild) Wait() (int, error) {
	code, err := child.Child.Wait()
	close(child.reaped)
	return code, err
}
