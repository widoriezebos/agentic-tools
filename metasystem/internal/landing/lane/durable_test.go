package lane

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
)

// doubted are the homes whose lane writes commit but are not confirmed on
// disk, each with the anchors its writes were given.
var doubted sync.Map

type doubt struct {
	mu      sync.Mutex
	anchors []string
}

// writeFile is installed once, before any test runs, so parallel tests
// doubt only their own home's writes.
func init() {
	writeFile = func(path string, data []byte, mode os.FileMode, anchor string) (bool, error) {
		durable, err := atomicfile.WriteFile(path, data, mode, anchor)
		if value, ok := doubted.Load(filepath.Dir(filepath.Dir(path))); ok {
			d := value.(*doubt)
			d.mu.Lock()
			d.anchors = append(d.anchors, anchor)
			d.mu.Unlock()
			return false, err
		}
		return durable, err
	}
}

// doubtWrites makes every lane write under home commit without durability.
func doubtWrites(t *testing.T, home string) *doubt {
	t.Helper()
	d := &doubt{}
	doubted.Store(home, d)
	t.Cleanup(func() { doubted.Delete(home) })
	return d
}

// anchoredAtTheHomesParent fails unless every doubted write was anchored at
// home's parent, which pre-exists where the host directory may be new.
func (d *doubt) anchoredAtTheHomesParent(t *testing.T, home string) {
	t.Helper()
	d.mu.Lock()
	defer d.mu.Unlock()
	if len(d.anchors) == 0 {
		t.Fatal("no lane write was made")
	}
	for _, anchor := range d.anchors {
		if anchor != filepath.Dir(home) {
			t.Fatalf("a lane write was anchored at %q; want the home's parent %q", anchor, filepath.Dir(home))
		}
	}
}

// A person's register and stop on a fresh computer, whose writes commit but
// are not confirmed on disk, return ErrNotDurable with changed true, and the
// files hold what was written.
func TestLaneWriteNotConfirmedOnDiskIsWritten(t *testing.T) {
	t.Parallel()
	home, root, _ := laneDirs(t)
	if err := os.Remove(home); err != nil {
		t.Fatal(err)
	}
	d := doubtWrites(t, home)
	layout, err := NewLayout(root)
	if err != nil {
		t.Fatal(err)
	}
	_, changed, err := Register(home, layout, "Wido", laneNow)
	if !changed || !errors.Is(err, ErrNotDurable) || !strings.Contains(err.Error(), RecordPath(home)) {
		t.Fatalf("Register = %v, %v; want changed and ErrNotDurable naming %s", changed, err, RecordPath(home))
	}
	if record, ok, err := Read(home); err != nil || !ok || record.Root != string(layout.Checkout) || record.CustodyEpoch != 1 {
		t.Fatalf("the record written = %+v, %v, %v", record, ok, err)
	}
	if epoch, err := readEpoch(home); err != nil || epoch != 1 {
		t.Fatalf("the custody epoch written = %d, %v", epoch, err)
	}
	if changed, err := SetPause(home, "Wido", laneNow); !changed || !errors.Is(err, ErrNotDurable) {
		t.Fatalf("SetPause = %v, %v; want changed and ErrNotDurable", changed, err)
	}
	if pause, paused := ReadPause(home); !paused || pause.By != "Wido" {
		t.Fatalf("the pause written = %+v, %v", pause, paused)
	}
	d.anchoredAtTheHomesParent(t, home)
}

// The keeper's step goes on after a write of its state that is not
// confirmed on disk: the claim and the launch are recorded and the agent is
// started, not failed.
func TestKeeperGoesOnAfterAnUnconfirmedWrite(t *testing.T) {
	t.Parallel()
	home, _, module := nestedLaneDirs(t)
	d := doubtWrites(t, home)
	clock := laneNow
	reasons := []string{queuedReason}
	agent := &fakeAgent{}
	run := agent.keeper(home, module, &clock, wakeFor(&reasons)).Run()
	if run.Outcome != AgentStarted || len(agent.starts) != 1 {
		t.Fatalf("a step with unconfirmed writes = %+v, starts %d; want started once", run, len(agent.starts))
	}
	if state, err := ReadAgentState(home); err != nil || state.Launch != run.Launch || state.StartingAt != "" {
		t.Fatalf("the keeper's state written = %+v, %v; want launch %s, no claim left", state, err, run.Launch)
	}
	clock = clock.Add(time.Minute)
	agent.running = ""
	if line := agent.keeper(home, module, &clock, wakeFor(&reasons)).Step(); len(agent.reaped) != 1 || len(agent.starts) != 2 {
		t.Fatalf("the next step = %q, reaped %v, starts %d; want the ended launch reaped and a new start", line, agent.reaped, len(agent.starts))
	}
	d.anchoredAtTheHomesParent(t, home)
}
