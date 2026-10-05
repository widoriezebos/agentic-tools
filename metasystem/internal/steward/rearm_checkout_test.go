package steward

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/enginebuild"
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
	bed, units, work, options := seatBusyFixture(t)
	seatBusyRun(t, units, bed.now.Add(-24*time.Hour), 41)
	options.AtBoundary = true
	busy, _, skipped := SeatBusyAt(bed.root, units, work, options)
	if !busy || skipped != 0 {
		t.Fatalf("boundary allowed a running step: busy=%t skipped=%d", busy, skipped)
	}
}
