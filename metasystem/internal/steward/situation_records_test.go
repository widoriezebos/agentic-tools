package steward

import (
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/enginepin"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/hostload"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/registry"
)

// situationRegistry is a host registry naming each checkout armed, written
// through the registry's own writer with the owner rows its tests write.
func situationRegistry(t *testing.T, checkouts ...string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "armed-checkouts.jsonl")
	for i, checkout := range checkouts {
		tag := fmt.Sprintf("tag-%d", i)
		payload, err := json.Marshal(map[string]any{"schemaVersion": 1, "event": registry.EventRelaunched, "checkoutPath": checkout, "ownerTag": tag,
			"at": "2026-09-29T08:00:00Z", "generation": 1, "watcherTag": tag + "-w", "reaperTag": tag + "-r", "retiredThrough": 0})
		if err != nil {
			t.Fatal(err)
		}
		if err := registry.AppendFrame(path, payload); err != nil {
			t.Fatal(err)
		}
	}
	return path
}

// situationProber answers each pid's start and counts its probes.
type situationProber struct {
	starts map[int64]time.Time
	calls  map[int64]int
}

func (p *situationProber) Probe(pid int64) (identity.Exact, identity.Liveness, error) {
	p.calls[pid]++
	return identity.Exact{Pid: pid, StartedAt: p.starts[pid]}, identity.Alive, nil
}

// deadProber finds every process dead.
type deadProber struct{}

func (deadProber) Probe(int64) (identity.Exact, identity.Liveness, error) {
	return identity.Exact{}, identity.Dead, nil
}

// situationHostSources are injected host reads: no kernel, no host registry.
func situationHostSources(t *testing.T, registryPath string) SituationSources {
	t.Helper()
	return SituationSources{Now: func() time.Time { return situationNow }, LaneHome: func() (string, error) { return t.TempDir(), nil },
		Load: func(now time.Time) hostload.Sample {
			return hostload.Sample{At: now.Format(time.RFC3339), Load1m: 3, Available: true}
		},
		Processes: identity.FixedProcessTable{}, Argv: func(int64) ([]string, bool) { return nil, false },
		Cwd: func(int64) (string, bool) { return "", false }, Prober: deadProber{},
		Registry:   func() (string, error) { return registryPath, nil },
		Executions: func(string) map[string]enginepin.Execution { return map[string]enginepin.Execution{} }}
}

func TestTheSituationReadsTheHostByCheckout(t *testing.T) {
	t.Parallel()
	top := t.TempDir()
	a, b := filepath.Join(top, "A"), filepath.Join(top, "A", "nested", "B")
	src := situationHostSources(t, situationRegistry(t, a, b))
	argv := map[int64][]string{101: {"go", "test", "./..."}, 102: {"/var/folders/x/go-build123/b001/plain.test", "-test.run=X"},
		103: {"vim"}, 104: {"steward.test"}, 106: {"/usr/local/go/bin/go", "test", "./internal/steward"}}
	cwds := map[int64]string{101: a, 102: filepath.Join(a, "artifacts", "agents", "workspaces", "goal-x", "build", "metasystem"),
		103: a, 104: "/elsewhere", 105: a, 106: filepath.Join(b, "internal")}
	cwdCalls := map[int64]int{}
	prober := &situationProber{starts: map[int64]time.Time{}, calls: map[int64]int{}}
	table := identity.FixedProcessTable{}
	for pid := int64(101); pid <= 106; pid++ {
		table = append(table, identity.ProcessRow{Pid: pid, Group: pid, Session: pid, Parent: 1})
	}
	src.Processes = table
	src.Argv = func(pid int64) ([]string, bool) { words, ok := argv[pid]; return words, ok }
	src.Cwd = func(pid int64) (string, bool) { cwdCalls[pid]++; return cwds[pid], true }
	src.Prober = prober
	read := func(load float64, start time.Time) map[int64]HostProcess {
		t.Helper()
		for pid := int64(101); pid <= 106; pid++ {
			prober.starts[pid] = start.Add(time.Duration(pid) * time.Second)
		}
		src.Load = func(now time.Time) hostload.Sample {
			return hostload.Sample{At: now.Format(time.RFC3339), Load1m: load, Available: true}
		}
		s := ReadSituation(t.TempDir(), src)
		if s.Load.Load1m != load || s.Unavailable["host"] != "" {
			t.Fatalf("the load = %+v, unavailable %q", s.Load, s.Unavailable["host"])
		}
		byPid := map[int64]HostProcess{}
		for _, process := range s.Processes {
			byPid[process.Pid] = process
		}
		if !slices.IsSortedFunc(s.Processes, func(x, y HostProcess) int {
			return cmp.Or(strings.Compare(x.Checkout, y.Checkout), cmp.Compare(x.Pid, y.Pid))
		}) {
			t.Fatalf("processes are not sorted by checkout, then pid: %+v", s.Processes)
		}
		for pid, process := range byPid {
			if !process.Started.Equal(prober.starts[pid]) {
				t.Fatalf("pid %d started %v, want the prober's %v", pid, process.Started, prober.starts[pid])
			}
		}
		return byPid
	}
	facts := func(processes map[int64]HostProcess) map[int64][3]string {
		out := map[int64][3]string{}
		for pid, process := range processes {
			out[pid] = [3]string{process.Kind, process.Checkout, process.Workspace}
		}
		return out
	}

	first := read(3, situationNow.Add(-time.Hour))
	want := map[int64][3]string{101: {"go test", a, ""}, 102: {"test binary", a, "goal-x/build"}, 104: {"test binary", "elsewhere", ""}, 106: {"go test", b, ""}}
	if got := facts(first); !maps.Equal(got, want) {
		t.Fatalf("processes = %v, want %v", got, want)
	}
	for _, pid := range []int64{103, 105} {
		if cwdCalls[pid] != 0 || prober.calls[pid] != 0 {
			t.Fatalf("pid %d is no test process but its cwd was read %d and its start %d times", pid, cwdCalls[pid], prober.calls[pid])
		}
	}
	if second := read(9, situationNow.Add(-2*time.Hour)); !maps.Equal(facts(second), want) {
		t.Fatalf("another load and other starts changed the facts: %v", facts(second))
	}
}

func TestTheSituationReadsEngineAndSeatHolders(t *testing.T) {
	t.Parallel()
	top := t.TempDir()
	a, b, c := filepath.Join(top, "A"), filepath.Join(top, "B"), filepath.Join(top, "C")
	src := situationHostSources(t, situationRegistry(t, a, b, c))
	executionReads := 0
	src.Executions = func(string) map[string]enginepin.Execution {
		executionReads++
		return map[string]enginepin.Execution{"runner": {Generation: 7, Digest: "sha256:runner"}, "supervisor": {Absent: true, Cause: "no record"}}
	}
	src.Holder = func(checkout string) (SeatHolder, error) {
		switch checkout {
		case a:
			return SeatHolder{MainId: "m-1", SessionId: "s-1", Lineage: "lin-a", Pid: 4242}, nil
		case b:
			return SeatHolder{}, nil
		}
		return SeatHolder{}, errors.New("the lease cannot be read")
	}
	s := ReadSituation(t.TempDir(), src)
	if s.Executions["runner"].Generation != 7 || !s.Executions["supervisor"].Absent || s.Unavailable["engine"] != "" {
		t.Fatalf("executions = %+v, unavailable %q", s.Executions, s.Unavailable["engine"])
	}
	holders := map[string]SeatHolder{}
	for _, holder := range s.Holders {
		holders[holder.Checkout] = holder
	}
	if holder := holders[a]; holder.MainId != "m-1" || holder.Lineage != "lin-a" || holder.Unavailable != "" {
		t.Fatalf("A's holder = %+v, want m-1 of lin-a", holder)
	}
	if holder, ok := holders[b]; !ok || holder.MainId != "" || holder.Unavailable != "" {
		t.Fatalf("B's holder = %+v (listed %v), want no holder and no error", holder, ok)
	}
	if holder := holders[c]; holder.Unavailable != "the lease cannot be read" || len(s.Holders) != 3 {
		t.Fatalf("C's holder = %+v of %d seats, want its error on its row", holder, len(s.Holders))
	}

	src.Holder = nil
	if s = ReadSituation(t.TempDir(), src); s.Unavailable["engine"] != "no reader" || s.Executions["runner"].Generation != 7 || executionReads != 2 {
		t.Fatalf("no holder reader: unavailable %q, executions %+v (%d reads)", s.Unavailable["engine"], s.Executions, executionReads)
	}
}

func TestTheSituationPartsFailAlone(t *testing.T) {
	t.Parallel()
	src := situationHostSources(t, "")
	src.Registry = func() (string, error) { return "", errors.New("no host home") }
	src.Executions = func(string) map[string]enginepin.Execution {
		return map[string]enginepin.Execution{"runner": {Generation: 7}}
	}
	src.Holder = func(string) (SeatHolder, error) {
		t.Error("a holder was read without the registry")
		return SeatHolder{}, nil
	}
	s := ReadSituation(t.TempDir(), src)
	if !strings.Contains(s.Unavailable["host"], "no host home") || !strings.Contains(s.Unavailable["engine"], "no host home") ||
		!s.Load.Available || s.Executions["runner"].Generation != 7 || len(s.Processes) != 0 || len(s.Holders) != 0 {
		t.Fatalf("a failed registry read: unavailable %v, load %+v, executions %+v, %d processes, %d holders",
			s.Unavailable, s.Load, s.Executions, len(s.Processes), len(s.Holders))
	}

	seat := filepath.Join(t.TempDir(), "A")
	src.Registry = func() (string, error) { return situationRegistry(t, seat), nil }
	src.Processes = identity.ScriptedProcessTable{PidsErr: errors.New("the process table cannot be read")}
	src.Holder = func(string) (SeatHolder, error) { return SeatHolder{MainId: "m-1"}, nil }
	s = ReadSituation(t.TempDir(), src)
	if !strings.Contains(s.Unavailable["host"], "the process table cannot be read") || len(s.Holders) != 1 || s.Holders[0].MainId != "m-1" || s.Unavailable["engine"] != "" {
		t.Fatalf("a failed process table: unavailable %v, holders %+v, want the seats still read", s.Unavailable, s.Holders)
	}
}
