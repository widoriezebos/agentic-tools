package census

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

type productionProbeResult struct {
	exact identity.Exact
	state identity.Liveness
	err   error
}

type productionProbe map[int64]productionProbeResult

func (probe productionProbe) Probe(pid int64) (identity.Exact, identity.Liveness, error) {
	result, ok := probe[pid]
	if !ok {
		return identity.Exact{}, identity.Dead, nil
	}
	return result.exact, result.state, result.err
}

// RunProductionCensus runs against the live machine: assert it yields a valid
// schemaVersion-2 verdict (classification of real processes is
// non-deterministic, so only the envelope is asserted here; the fixture-path
// tests cover the classification).
func TestRunProductionCensusEnvelope(t *testing.T) {
	root := t.TempDir()
	// No supervision state, no runtimes -> errors, but a well-formed verdict.
	v, err := RunProductionCensus(root, root, "fp", 60, time.Unix(1786000000, 0))
	if err != nil {
		t.Fatal(err)
	}
	if v.SchemaVersion != 2 || v.Writer != "watch-background-jobs.sh" {
		t.Fatalf("bad verdict envelope: %+v", v)
	}
	if v.Counts == nil || v.Inventory == nil || v.Errors == nil {
		t.Fatal("verdict collections must be non-nil")
	}
}

func TestFixtureSurvivorSourceNamesLiveUnreadableProcess(t *testing.T) {
	t.Setenv("METASYSTEM_CENSUS_PROCESS_FILE", "")
	base := int64(os.Getpid()) + 100_000
	livePID, deadPID := base+1, base+2
	live := identity.Exact{
		Pid: livePID, StartedAt: time.Unix(1_786_000_001, 123_000_000),
		Exe: filepath.Join(t.TempDir(), "go-tmp", "TestUnreadable", "child"), ExeKnown: true,
	}
	if runtime.GOOS == "linux" {
		live.StartTicks, live.BootID = 12345, "fixture-boot"
	}
	probe := productionProbe{
		livePID: {exact: live, state: identity.Alive},
		deadPID: {state: identity.Dead},
	}
	source := productionProcessSource{
		pids:   func() ([]int64, error) { return []int64{livePID, deadPID}, nil },
		prober: probe,
		processGroup: func(pid int) (int, error) {
			return pid, nil
		},
		parentProcess: func(pid int64) (int64, bool) {
			return 1, true
		},
	}
	inventory, err := source.enumerate()
	if err != nil {
		t.Fatal(err)
	}
	if len(inventory) != 0 {
		t.Fatalf("ordinary process inventory retained empty argv rows: %#v", inventory)
	}

	prober, processes, configured, err := fixtureSurvivorSource(t.TempDir(), source)
	if err != nil {
		t.Fatal(err)
	}
	if configured {
		t.Fatal("injected production source was reported as a configured fixture")
	}
	if len(processes) != 1 || processes[0].Pid != livePID || !processes[0].Unreadable || processes[0].Argv != "" {
		t.Fatalf("production survivor rows = %#v, want only live unreadable pid %d", processes, livePID)
	}

	lines, certain, err := FixtureSurvivorLines(prober, processes)
	if err != nil {
		t.Fatal(err)
	}
	want := fmt.Sprintf("fixture-survivor? pid=%d", livePID)
	if certain || len(lines) != 1 || !strings.Contains(lines[0], want) {
		t.Fatalf("fixture-survivor failure lines = %q, certain=%t; want one line containing %q", lines, certain, want)
	}
}
