package steward

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gocache"
)

// The trim step runs as the cycle's last step, after Tick returned and the
// channel phase ran, and holds no arbitration: a concurrent
// AcquireArbitration during it succeeds (disk-lifetimes A12, DL2-17).
func TestRunLoopTrimsTheCachesLastWithoutArbitration(t *testing.T) {
	t.Parallel()
	repository := newDecisionTickRepository(t)
	root := repository.root
	census := fakeCensus{workers: Workers{Live: 1, CensusComplete: true}}
	var mu sync.Mutex
	var order []string
	record := func(step string) {
		mu.Lock()
		order = append(order, step)
		mu.Unlock()
	}
	tick := repository.runnerTick()
	deps := runnerLoopDependencies{
		Tick: func(top string, cfg TickConfig, census WorkerCensus) (TickResult, error) {
			result, err := tick(top, cfg, census)
			record("tick")
			return result, err
		},
		DeliverPending: runnerPendingDelivery(root, "true"),
		Channel:        func(context.Context, string) (int, error) { record("channel"); return 0, nil },
		TrimCaches: func(ctx context.Context, top string, cfg TickConfig) error {
			record("trim")
			held, err := AcquireArbitration(top)
			if err != nil {
				t.Errorf("arbitration during the trim: %v", err)
			} else {
				held.Release()
			}
			return stopRunnerLoop(top)
		},
		Now: time.Now, Sleep: func(time.Duration) {},
	}
	if err := runLoopWithDependencies(root, census, nil, time.Hour, TickConfig{}, deps); err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(order) != "[tick channel trim]" {
		t.Fatalf("cycle order = %v", order)
	}
}

// A stop file during a trim ends the trim at its next batch and the runner
// within the budget, leaving the trim's checkpoint for the next pass.
func TestAStopFileDuringATrimEndsTheTrimAndTheRunner(t *testing.T) {
	t.Parallel()
	repository := newDecisionTickRepository(t)
	root := repository.root
	userCache := t.TempDir()
	state := filepath.Join(t.TempDir(), "cache-trim")
	for index := 0; index < 4*256; index++ {
		path := filepath.Join(userCache, "go-build", "7f", fmt.Sprintf("%06d-a", index))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	tick := repository.runnerTick()
	deps := runnerLoopDependencies{
		Tick: func(top string, cfg TickConfig, census WorkerCensus) (TickResult, error) {
			result, err := tick(top, cfg, census)
			if stopErr := stopRunnerLoop(top); stopErr != nil {
				t.Error(stopErr)
			}
			return result, err
		},
		DeliverPending: runnerPendingDelivery(root, "true"),
		Channel:        func(context.Context, string) (int, error) { return 0, nil },
		TrimCaches:     machineCacheTrimmer(func() (string, error) { return userCache, nil }, state),
		Now:            time.Now, Sleep: func(time.Duration) { t.Error("the runner waited after its stop file") },
	}
	// The runner's Sleep seam fails the test, so a runner that waited after
	// the stop file instead of ending is red without timing it.
	if err := runLoopWithDependencies(root, fakeCensus{workers: Workers{Live: 1, CensusComplete: true}}, nil, time.Hour, TickConfig{}, deps); err != nil {
		t.Fatal(err)
	}
	var report gocache.TrimReport
	data, err := os.ReadFile(filepath.Join(state, "engine-go-build.json"))
	if err != nil || json.Unmarshal(data, &report) != nil {
		t.Fatalf("no engine trim report: %v %s", err, data)
	}
	if report.EndedBy != "cancelled" || report.Checkpoint.Shard != "7f" || report.Checkpoint.BytesSoFar != 256 {
		t.Fatalf("the stop file did not end the trim at its first batch: %+v", report)
	}
}

// The steward's pass carries the floor and its budget's deadline to the
// trimmer: an engine Go cache over its cap with everything used within the
// keep window starts evicting in the pass that has counted more than the
// cap, before the measurement is whole, and keeps what was used within
// disk.cache-min-keep-minutes. The pass's clock moves a minute a reading,
// so its ten-second budget is spent at once; the stop question ends the
// pass at its second batch boundary, which is after that eviction.
func TestTheStewardPassYieldsTheKeepWindowOverTheCap(t *testing.T) {
	t.Parallel()
	top := t.TempDir()
	if err := os.WriteFile(filepath.Join(top, "metasystem.conf"), []byte("disk.go-cache-cap-gib=1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	userCache := t.TempDir()
	state := filepath.Join(t.TempDir(), "cache-trim")
	now := time.Date(2026, 9, 29, 16, 0, 0, 0, time.UTC)
	entry := func(name string, size int64, age time.Duration) string {
		path := filepath.Join(userCache, "go-build", name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		file, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := file.Truncate(size); err != nil {
			t.Fatal(err)
		}
		file.Close()
		when := now.Add(-age)
		if err := os.Chtimes(path, when, when); err != nil {
			t.Fatal(err)
		}
		return path
	}
	const gib = int64(1) << 30
	older := entry("00/aaaa-d", gib, 3*time.Hour)
	newer := entry("00/bbbb-d", gib, 2*time.Hour)
	recent := entry("01/cccc-a", 1024, 30*time.Minute)
	readings := 0
	clock := func() time.Time { readings++; return now.Add(time.Duration(readings) * time.Minute) }
	questions := 0
	stopped := func() bool { questions++; return questions >= 2 }
	reports, err := TrimMachineCaches(context.Background(), top, func() (string, error) { return userCache, nil }, state, now, clock, stopped)
	if err != nil {
		t.Fatal(err)
	}
	if len(reports) == 0 || reports[0].Cache != "engine-go-build" {
		t.Fatalf("reports = %+v", reports)
	}
	report := reports[0]
	if report.Phase != "measure" || report.EntriesRemoved != 1 || !report.OverCap || report.MinKeepMinutes != 120 {
		t.Fatalf("the pass did not evict from its partial measure: %+v", report)
	}
	if _, err := os.Stat(older); !os.IsNotExist(err) {
		t.Errorf("the oldest entry inside the keep window stayed over the cap: %v", err)
	}
	for _, path := range []string{newer, recent} {
		if _, err := os.Stat(path); err != nil {
			t.Errorf("%s went: %v", path, err)
		}
	}
}
