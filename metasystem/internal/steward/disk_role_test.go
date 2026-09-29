package steward

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
)

// The runner sweeps after the tick returned and released arbitration, and
// at the helm it reports without acting (3.3; the 2026-09-29 amendment).
func TestRunnerSweepsDiskAfterTheTickReleasedArbitration(t *testing.T) {
	t.Parallel()
	loop := newHelmLoop(t)
	type sweep struct {
		free, helm bool
	}
	var sweeps []sweep
	now := time.Date(2026, 9, 28, 19, 0, 0, 0, time.UTC)
	deps := runnerLoopDependencies{
		Tick: func(root string, _ TickConfig, _ WorkerCensus) (TickResult, error) {
			lock, err := AcquireArbitration(root)
			if err != nil {
				return TickResult{}, err
			}
			defer lock.Release()
			return TickResult{}, nil
		},
		Resumable:      func(string) (string, bool, error) { return "", false, nil },
		DeliverPending: func(string) (int, error) { return 0, nil },
		Channel:        func(context.Context, string) (int, error) { return 0, nil },
		Now:            func() time.Time { return now },
		Sleep: func(d time.Duration) {
			now = now.Add(d)
			if len(sweeps) == 1 {
				takeHelmFixture(t, loop.root)
				return
			}
			loop.stop(t)
		},
		SweepDisk: func(top string, _ time.Time, helm bool) {
			free, err := ProbeArbitration(top)
			if err != nil {
				t.Error(err)
			}
			sweeps = append(sweeps, sweep{free: free, helm: helm})
		},
	}
	if err := runLoopWithDependencies(loop.root, fakeCensus{}, nil, 200*time.Millisecond, TickConfig{Now: now}, deps); err != nil {
		t.Fatal(err)
	}
	if len(sweeps) != 2 || !sweeps[0].free || sweeps[0].helm || !sweeps[1].helm {
		t.Fatalf("sweeps = %+v; want one after a released tick, then one at the helm", sweeps)
	}
}

// oldHandoffs makes n complete, cancelled handoffs aged a month.
func oldHandoffs(t *testing.T, n int) (*handoffGoalFixture, []string) {
	t.Helper()
	fixture := newHandoffGoalFixture(t, "claimed")
	var nonces []string
	for index := range n {
		nonces = append(nonces, fmt.Sprintf("69%014d", index))
	}
	useHandoffNonces(t, append(append([]string(nil), nonces...), "6999999999999999")...)
	for range nonces {
		handoff, err := fixture.handoff(fixture.root, handoffMainCaller(), handoffTestRecord(t, fixture.root, nil), handoffCaptureNow, filepath.Join(fixture.root, "memory", "receipts.log"))
		if err != nil {
			t.Fatal(err)
		}
		if err := CancelHandoff(fixture.root, handoff.Nonce, HandoffCanceller{Caller: handoffMainCaller()}); err != nil {
			t.Fatal(err)
		}
		ageHandoffState(t, fixture.root, handoff.Nonce, handoffCaptureNow.AddDate(0, 0, -30))
	}
	return fixture, nonces
}

func handoffPass(root string, clock func() time.Time) diskstore.PassOptions {
	registry := diskstore.CheckoutRegistry(root)
	return diskstore.PassOptions{Kind: "checkout", Name: root, Registry: registry, LockPath: filepath.Join(registry.Dir, ".sweep.flock"),
		ReportPath: diskstore.CheckoutReportPath(root), Mode: diskstore.ModeApply, Now: handoffCaptureNow, Clock: clock, Entropy: rand.Reader,
		Classes: []diskstore.Class{HandoffClass{Root: root, Keep: 14 * 24 * time.Hour}}}
}

// A concurrent AcquireArbitration started during a sweep completes within
// one nonce's critical section: the sweeper releases between nonces, and a
// nonce whose turn meets the other holder is pending, never waited on
// (3.3, R14).
func TestArbitrationDuringASweepCompletesWithinOneNonce(t *testing.T) {
	fixture, nonces := oldHandoffs(t, 6)
	waiting, acquired := make(chan struct{}), make(chan struct{})
	previousWait, previousRemove := beforeArbitrationWait, beforeHandoffPruneRemove
	t.Cleanup(func() { beforeArbitrationWait, beforeHandoffPruneRemove = previousWait, previousRemove })
	beforeArbitrationWait = func() { close(waiting) }
	removals := 0
	beforeHandoffPruneRemove = func(string) {
		removals++
		if removals == 1 {
			// A hook starts waiting while the sweeper holds nonce one.
			go func() {
				lock, err := AcquireArbitration(fixture.root)
				if err != nil {
					t.Error(err)
					return
				}
				close(acquired)
				lock.Release()
			}()
			<-waiting
			return
		}
		select {
		case <-acquired:
		default:
			t.Errorf("the sweeper took arbitration for a second nonce while a hook was queued")
		}
	}
	report, err := diskstore.RunPass(context.Background(), handoffPass(fixture.root, time.Now))
	if err != nil {
		t.Fatal(err)
	}
	<-acquired
	if len(report.Actions)+len(report.Pending) != len(nonces) || len(report.Actions) < 1 {
		t.Fatalf("sweep acted on %d and left %d pending of %d", len(report.Actions), len(report.Pending), len(nonces))
	}
}

// The stop-path composition (DL3B-04): a hook capturing a handoff (which
// takes arbitration) against a sweep over two hundred old handoffs: the
// capture completes, serialized per nonce, and when the budget ends (the
// cancel stands in for the pass deadline) the sweep returns.
func TestHookCaptureCompletesDuringALargeHandoffSweep(t *testing.T) {
	fixture, _ := oldHandoffs(t, 200)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	swept := make(chan diskstore.Report, 1)
	go func() {
		report, err := diskstore.RunPass(ctx, handoffPass(fixture.root, time.Now))
		if err != nil {
			t.Error(err)
		}
		swept <- report
	}()
	captured, err := fixture.handoff(fixture.root, handoffMainCaller(), handoffTestRecord(t, fixture.root, nil), handoffCaptureNow, filepath.Join(fixture.root, "memory", "receipts.log"))
	if err != nil {
		t.Fatalf("the hook's capture failed during the sweep: %v", err)
	}
	if _, err := os.Stat(captured.StatePath); err != nil {
		t.Fatalf("the capture's state is missing: %v", err)
	}
	cancel()
	report := <-swept
	if len(report.Actions) != 200 && len(report.Backlog) == 0 {
		t.Fatalf("a cut-short sweep left no backlog: %d actions, pending %v", len(report.Actions), report.Pending)
	}
	// The next pass resumes and finishes; the capture's fresh handoff stays.
	next, err := diskstore.RunPass(context.Background(), handoffPass(fixture.root, time.Now))
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Actions)+len(next.Actions) != 200 {
		t.Fatalf("two passes removed %d+%d of 200 old handoffs", len(report.Actions), len(next.Actions))
	}
	if _, err := os.Stat(captured.StatePath); err != nil {
		t.Fatalf("the sweep removed the capture's fresh handoff: %v", err)
	}
}

// The disk role reads the last reports: none is alive, a floor breach raises
// it with the report's remedy, and an unreadable report is unknown.
func TestDiskRoleFollowsTheLastReport(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if role := checkDiskAt(root, filepath.Join(root, "home")); role.Status != HealthAlive || !strings.Contains(role.Reason, "no disk pass") {
		t.Fatalf("no report = %+v", role)
	}
	path := diskstore.CheckoutReportPath(root)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(health diskstore.Health) {
		report := diskstore.Report{Schema: diskstore.ReportSchema, Kind: "checkout", Name: root, Health: health}
		data := fmt.Sprintf(`{"schema":%q,"kind":"checkout","name":%q,"at":"2026-09-28T19:00:00Z","mode":"apply","health":{"status":%q,"reason":%q,"remedy":%q}}`,
			report.Schema, root, health.Status, health.Reason, health.Remedy)
		if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(diskstore.Health{Status: diskstore.HealthOK, Reason: "above"})
	if role := checkDiskAt(root, filepath.Join(root, "home")); role.Status != HealthAlive {
		t.Fatalf("an ok report = %+v", role)
	}
	write(diskstore.Health{Status: diskstore.HealthAttention, Reason: "free space is below the floor", Remedy: "metasystem disk clean --preview"})
	if role := checkDiskAt(root, filepath.Join(root, "home")); role.Status != HealthDead || role.Remedy != "metasystem disk clean --preview" {
		t.Fatalf("a floor breach = %+v", role)
	}
	if err := os.WriteFile(path, []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	if role := checkDiskAt(root, filepath.Join(root, "home")); role.Status != HealthUnknown || role.Remedy != "metasystem disk show" {
		t.Fatalf("an unreadable report = %+v", role)
	}
}

// The sweeper yields to a queued blocking acquirer: with arbitration free
// and a waiter queued, the nonblocking acquisition answers held, so the
// waiter is the next holder (Part B 3.3).
func TestSweeperYieldsArbitrationToAQueuedWaiter(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	lock, err := TryAcquireArbitration(root)
	if err != nil {
		t.Fatal(err)
	}
	lock.Release()
	want, err := os.OpenFile(arbitrationWantPath(root), os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if err := flockWaiting(want, 1); err != nil { // LOCK_SH: a waiter is queued
		t.Fatal(err)
	}
	if _, err := TryAcquireArbitration(root); !errors.Is(err, ErrArbitrationHeld) {
		t.Fatalf("the sweeper's acquisition with a waiter queued = %v; want held", err)
	}
	want.Close()
	lock, err = TryAcquireArbitration(root)
	if err != nil {
		t.Fatalf("with no waiter the sweeper's acquisition = %v", err)
	}
	lock.Release()
}
