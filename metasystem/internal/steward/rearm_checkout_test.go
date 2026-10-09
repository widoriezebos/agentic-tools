package steward

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/enginebuild"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
)

func TestRearmBehindCheckoutDefersOncePerPush(t *testing.T) {
	t.Parallel()
	root := canonicalPath(t.TempDir())
	engine := filepath.Join(root, "metasystem")
	head, main := rearmID(1), rearmID(2)
	writeRearmFile(t, engine, enginebuild.StampRecord(main))
	if err := os.Chmod(engine, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := MintIdentity(RepoIdentityPath(root), InstallIdentity{RepoIdentity: root, Generation: 1, InstallPath: engine, InstallDigest: "sha256:" + strings.Repeat("0", 64), MintedAt: "2026-10-04T08:00:00Z", EngineBuild: head}); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(RepoIdentityPath(root))
	for range 2 {
		deps := rearmTestDeps(t, append(append(rearmOwnedRef(root, "refs/remotes/origin/main", main), rearmBuild(root, main, main, 0), rearmAncestor(root, main, "refs/remotes/origin/main", 0)), rearmExpected(root, head, nil, "rev-parse", "--verify", "HEAD^{commit}"))...)
		out, err := reArmRebuiltEngineWithDeps(deps, root, root, engine, func() time.Time { return time.Date(2026, 10, 4, 8, 0, 0, 0, time.UTC) })
		if !errors.Is(err, ErrRearmDeferred) || out.Stage != StageBeforeMint || out.StoppedRunnerPid != 0 {
			t.Fatalf("re-arm did not wait for checkout: %+v %v", out, err)
		}
	}
	after, _ := os.ReadFile(RepoIdentityPath(root))
	log, err := os.ReadFile(runnerLogPath(root))
	if string(before) != string(after) || err != nil || strings.Count(string(log), "re-arm deferred: checkout at "+head+", main at "+main) != 1 {
		t.Fatalf("deferral changed enrollment or repeated its log: %s %v", log, err)
	}
	if line := RearmDeferredLine(root); line != "engine "+main+", checkout "+head+", re-arm deferred" {
		t.Fatalf("deferred health line = %q", line)
	}
}

func TestRunnerRearmsAtNextUnitBoundary(t *testing.T) {
	t.Parallel()
	loop := newHelmLoop(t)
	now := time.Date(2026, 10, 4, 8, 0, 0, 0, time.UTC)
	deps := idleRunnerDependencies(&now)
	ticks, refreshes := 0, 0
	deps.Tick = func(string, TickConfig, WorkerCensus) (TickResult, error) { ticks++; return TickResult{}, nil }
	cfg := TickConfig{RearmAtBoundary: func() (bool, error) { refreshes++; return ticks == 2, nil }}
	deps.Sleep = func(d time.Duration) {
		now = now.Add(d)
		if ticks > 2 {
			loop.stop(t)
		}
	}
	if err := runLoopWithDependencies(loop.root, fakeCensus{}, nil, time.Second, cfg, deps); err != nil || ticks != 2 || refreshes != 2 {
		t.Fatalf("boundary re-arm: ticks=%d refreshes=%d err=%v", ticks, refreshes, err)
	}
}

func TestUnitBoundaryHoldsALiveStepPastItsBudget(t *testing.T) {
	t.Parallel()
	for _, scenario := range []string{"live", "orphan", "starting", "capacity-held"} {
		t.Run(scenario, func(t *testing.T) {
			t.Parallel()
			bed, units, work, _ := seatBusyFixture(t)
			seatBusyRun(t, units, bed.now.Add(-24*time.Hour), 41)
			store := launch.Store{Root: filepath.Join(filepath.Dir(units), "launch")}
			record, err := store.Read("launch-1")
			if err != nil {
				t.Fatal(err)
			}
			if scenario == "live" {
				exact, state, err := (identity.KernelProber{}).Probe(int64(os.Getpid()))
				if err != nil || state != identity.Alive {
					t.Fatalf("fixture identity: %s %v", state, err)
				}
				ref := exact.Ref()
				record.Supervisor = &ref
			} else if scenario == "orphan" {
				record.Supervisor = &identity.Ref{Pid: int64(os.Getpid()), StartedAtSec: 1}
			} else {
				record.Supervisor = nil
			}
			if scenario == "starting" {
				record.State = launch.Starting
			}
			if _, err := store.Update(record.ID, func(saved *launch.Record) error { *saved = record; return nil }); err != nil {
				t.Fatal(err)
			}
			if scenario == "starting" || scenario == "capacity-held" {
				step := launch.UnitStep{Name: "build", State: launch.StepStarting, LaunchID: "not-started", StartedAt: bed.now.Format(time.RFC3339Nano)}
				if scenario == "starting" {
					step.LaunchID = record.ID
				}
				writeStewardRecord(t, filepath.Join(units, "run-1", "run.json"), map[string]any{"id": "run-1", "goal": "held", "state": "running", "rounds": []launch.UnitRound{{Steps: []launch.UnitStep{step}}}})
			}
			ready, err := SeatAtUnitBoundary(bed.root, filepath.Dir(units), bed.now, work)
			if err != nil || ready != (scenario == "orphan") {
				t.Fatalf("%s boundary: ready=%t error=%v", scenario, ready, err)
			}
		})
	}
}
