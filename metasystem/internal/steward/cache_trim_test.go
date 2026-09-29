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
	started := time.Now()
	if err := runLoopWithDependencies(root, fakeCensus{workers: Workers{Live: 1, CensusComplete: true}}, nil, time.Hour, TickConfig{}, deps); err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(started); elapsed > 10*time.Second {
		t.Fatalf("the runner took %v to stop", elapsed)
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
