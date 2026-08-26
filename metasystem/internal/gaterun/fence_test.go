package gaterun

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// A marker registered by the asking process itself, or by any of its
// ancestors, must not block the fence: that is the suite consulting the
// fence about its own run.
func TestFenceExemptsOwnChain(t *testing.T) {
	root := t.TempDir()
	self := int64(os.Getpid())
	if _, err := Register(root, self, "self-gate"); err != nil {
		t.Fatalf("register self: %v", err)
	}
	if _, err := Register(root, int64(os.Getppid()), "ancestor-gate"); err != nil {
		t.Fatalf("register ancestor: %v", err)
	}
	if holders := Fence(root, self); len(holders) != 0 {
		t.Fatalf("own chain blocked the fence: %+v", holders)
	}
}

// A live foreign process's marker blocks the fence and names its gate.
func TestFenceBlocksForeignLiveRun(t *testing.T) {
	root := t.TempDir()
	foreign := exec.Command("sleep", "60")
	if err := foreign.Start(); err != nil {
		t.Fatalf("start foreign process: %v", err)
	}
	defer func() {
		_ = foreign.Process.Kill()
		_, _ = foreign.Process.Wait()
	}()
	pid := int64(foreign.Process.Pid)
	if _, err := Register(root, pid, "foreign-gate"); err != nil {
		t.Fatalf("register foreign: %v", err)
	}
	holders := Fence(root, int64(os.Getpid()))
	if len(holders) != 1 || holders[0].Pid != pid || holders[0].Gate != "foreign-gate" {
		t.Fatalf("foreign live run did not block as itself: %+v", holders)
	}
}

// Dead and unparsable markers are pruned, exactly like Running prunes them:
// a marker only counts while its exact process is alive.
func TestFencePrunesDeadAndUnparsableMarkers(t *testing.T) {
	root := t.TempDir()
	foreign := exec.Command("sleep", "60")
	if err := foreign.Start(); err != nil {
		t.Fatalf("start foreign process: %v", err)
	}
	pid := int64(foreign.Process.Pid)
	if _, err := Register(root, pid, "dying-gate"); err != nil {
		t.Fatalf("register foreign: %v", err)
	}
	if err := foreign.Process.Kill(); err != nil {
		t.Fatalf("kill foreign process: %v", err)
	}
	if _, err := foreign.Process.Wait(); err != nil {
		t.Fatalf("wait foreign process: %v", err)
	}
	garbage := filepath.Join(markerDir(root), "garbage.json")
	if err := os.WriteFile(garbage, []byte("not json\n"), 0o644); err != nil {
		t.Fatalf("write garbage marker: %v", err)
	}
	if holders := Fence(root, int64(os.Getpid())); len(holders) != 0 {
		t.Fatalf("dead or garbage marker blocked the fence: %+v", holders)
	}
	if _, err := os.Stat(garbage); !os.IsNotExist(err) {
		t.Fatalf("garbage marker was not pruned")
	}
	markers, _ := filepath.Glob(filepath.Join(markerDir(root), "*.json"))
	if len(markers) != 0 {
		t.Fatalf("dead marker was not pruned: %v", markers)
	}
}

type ancestryProbeAnswer struct {
	exact identity.Exact
	live  identity.Liveness
	err   error
}

type ancestryProber map[int64]ancestryProbeAnswer

func (p ancestryProber) Probe(pid int64) (identity.Exact, identity.Liveness, error) {
	answer, ok := p[pid]
	if !ok {
		return identity.Exact{}, identity.Dead, nil
	}
	return answer.exact, answer.live, answer.err
}

func ancestryAlive(pid, startedAt, ticks int64, bootID string) ancestryProbeAnswer {
	return ancestryAliveAt(pid, time.Unix(startedAt, 0), ticks, bootID)
}

func ancestryAliveAt(pid int64, startedAt time.Time, ticks int64, bootID string) ancestryProbeAnswer {
	return ancestryProbeAnswer{
		exact: identity.Exact{Pid: pid, StartedAt: startedAt, StartTicks: ticks, BootID: bootID},
		live:  identity.Alive,
	}
}

type ancestrySequenceProber map[int64][]ancestryProbeAnswer

func (p ancestrySequenceProber) Probe(pid int64) (identity.Exact, identity.Liveness, error) {
	answers := p[pid]
	if len(answers) == 0 {
		return identity.Exact{}, identity.Dead, nil
	}
	answer := answers[0]
	if len(answers) > 1 {
		p[pid] = answers[1:]
	}
	return answer.exact, answer.live, answer.err
}

func ancestryParents(values map[int64]int64) parentPIDLookup {
	return func(pid int64) (int64, bool) {
		parent, ok := values[pid]
		return parent, ok
	}
}

func TestControllerDescendantRequiresExactLiveAncestry(t *testing.T) {
	controller := identity.Ref{Pid: 10, StartedAtSec: 100, StartTicks: 7, BootID: "boot"}
	prober := ancestryProber{
		10: ancestryAlive(10, 100, 7, "boot"),
		20: ancestryAlive(20, 200, 8, "boot"),
		30: ancestryAlive(30, 300, 9, "boot"),
	}
	if err := controllerDescendant(prober, ancestryParents(map[int64]int64{30: 20, 20: 10}), 30, controller); err != nil {
		t.Fatalf("exact descendant refused: %v", err)
	}
	if err := controllerDescendant(prober, ancestryParents(map[int64]int64{30: 20, 20: 10}), 10, controller); err == nil {
		t.Fatal("the controller itself was accepted as its own descendant")
	}
	if err := controllerDescendant(prober, ancestryParents(map[int64]int64{30: 20}), 30, controller); err == nil {
		t.Fatal("a broken ancestry walk was accepted")
	}
	if err := controllerDescendant(prober, ancestryParents(map[int64]int64{30: 20, 20: 30}), 30, controller); err == nil {
		t.Fatal("a cyclic ancestry walk was accepted")
	}
}

func TestControllerDescendantRefusesReusedParentDuringEdgeConfirmation(t *testing.T) {
	controller := identity.Ref{Pid: 10, StartedAtSec: 100}
	prober := ancestrySequenceProber{
		10: {ancestryAlive(10, 100, 0, "")},
		20: {
			ancestryAliveAt(20, time.Unix(200, 1_000), 0, ""),
			ancestryAliveAt(20, time.Unix(200, 2_000), 0, ""),
		},
		30: {ancestryAlive(30, 300, 0, "")},
	}
	parents := ancestryParents(map[int64]int64{30: 20, 20: 10})
	if err := controllerDescendant(prober, parents, 30, controller); err == nil {
		t.Fatal("a parent pid reused within the recorded start second was accepted")
	}
}

func TestControllerDescendantRefusesDeadReusedAndUnreadableIdentity(t *testing.T) {
	controller := identity.Ref{Pid: 10, StartedAtSec: 100, StartTicks: 7, BootID: "boot"}
	parents := ancestryParents(map[int64]int64{30: 10})
	tests := []struct {
		name   string
		answer ancestryProbeAnswer
	}{
		{name: "dead", answer: ancestryProbeAnswer{live: identity.Dead}},
		{name: "reused", answer: ancestryAlive(10, 101, 8, "boot")},
		{name: "unreadable", answer: ancestryProbeAnswer{live: identity.Unknown, err: errors.New("permission denied")}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			prober := ancestryProber{10: test.answer, 30: ancestryAlive(30, 300, 9, "boot")}
			if err := controllerDescendant(prober, parents, 30, controller); err == nil {
				t.Fatalf("%s controller identity was accepted", test.name)
			}
		})
	}
}

func TestControllerDescendantRefusesUnreadableConsumerChain(t *testing.T) {
	controller := identity.Ref{Pid: 10, StartedAtSec: 100}
	prober := ancestryProber{
		10: ancestryAlive(10, 100, 0, ""),
		30: {live: identity.Unknown, err: errors.New("consumer unreadable")},
	}
	if err := controllerDescendant(prober, ancestryParents(map[int64]int64{30: 10}), 30, controller); err == nil {
		t.Fatal("an unreadable consuming pid was accepted")
	}
}
