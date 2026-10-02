package census

import (
	"os"
	"path/filepath"
	"runtime"
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

func TestProductionInventoryDropsUnreadableProcesses(t *testing.T) {
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
		table: identity.FixedProcessTable{
			{Pid: livePID, Group: livePID, Session: livePID, Parent: 1},
			{Pid: deadPID, Group: deadPID, Session: deadPID, Parent: 1},
		},
		prober: probe,
	}
	inventory, err := source.enumerate()
	if err != nil {
		t.Fatal(err)
	}
	if len(inventory) != 0 {
		t.Fatalf("ordinary process inventory retained empty argv rows: %#v", inventory)
	}
}
