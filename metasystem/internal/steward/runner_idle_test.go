package steward

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func runnerVolume(t *testing.T, root string, alerts, payloads int) {
	t.Helper()
	for _, dir := range []string{alertDir(root), filepath.Join(root, recordStores[0])} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < alerts; i++ {
		body := fmt.Sprintf(`{"schema":1,"episodeId":"alert-%03d","digest":%q,"owner":"fixture","message":"fixture","openedAt":"2026-10-04T08:00:00Z","attempts":[],"transportResult":"TRANSPORT_SUBMITTED"}`, i, strings.Repeat("a", 64))
		if err := os.WriteFile(alertPath(root, fmt.Sprintf("alert-%03d", i)), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < payloads; i++ {
		body := `{"pid":0,"status":"green","exitCode":0,"payload":"` + strings.Repeat("x", 1<<20) + `"}`
		if err := os.WriteFile(filepath.Join(root, recordStores[0], fmt.Sprintf("proof-%03d.json", i)), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestRunnerPassCost(t *testing.T) {
	t.Parallel()
	for _, counts := range [][2]int{{200, 300}, {211, 267}} {
		t.Run(fmt.Sprintf("%d-alerts-%d-payloads", counts[0], counts[1]), func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			runnerVolume(t, root, counts[0], counts[1])
			start := time.Now()
			if _, err := loadAlertEpisodesUnlocked(root); err != nil {
				t.Fatal(err)
			}
			health := time.Since(start)
			start = time.Now()
			supplementWorkers(root)
			t.Logf("%d open alerts, %d proof payloads (1 MiB each): health alert read=%s census supplement=%s", counts[0], counts[1], health, time.Since(start))
		})
	}
}

func TestRunnerAuditsInPlaceEditsEveryTenTicks(t *testing.T) {
	t.Parallel()
	loop := newHelmLoop(t)
	runnerVolume(t, loop.root, 1, 0)
	now := time.Date(2026, 10, 4, 8, 0, 0, 0, time.UTC)
	deps := idleRunnerDependencies(&now)
	ticks := 0
	deps.Tick = func(root string, _ TickConfig, _ WorkerCensus) (TickResult, error) {
		ticks++
		alerts, err := loadAlertEpisodesUnlocked(root)
		if err != nil {
			t.Fatal(err)
		}
		want := "fixture"
		if ticks == runnerReadAuditTicks+1 {
			want = "changed"
			loop.stop(t)
		}
		if alerts[0].Message != want {
			t.Fatalf("tick %d: message=%q want=%q", ticks, alerts[0].Message, want)
		}
		if ticks == 1 {
			path := alertPath(root, alerts[0].EpisodeID)
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(strings.Replace(string(data), `"message":"fixture"`, `"message":"changed"`, 1)), 0o644); err != nil {
				t.Fatal(err)
			}
			// An explicit timestamp makes the same-size edit independent of
			// the filesystem's timestamp resolution.
			if err := os.Chtimes(path, now, now); err != nil {
				t.Fatal(err)
			}
		}
		return TickResult{}, nil
	}
	if err := runLoopWithDependencies(loop.root, fakeCensus{}, nil, time.Second, TickConfig{Now: now}, deps); err != nil {
		t.Fatal(err)
	}
	if ticks != runnerReadAuditTicks+1 {
		t.Fatalf("ticks=%d", ticks)
	}
}

type idleCensus func(string) (Workers, error)

func (c idleCensus) Workers(root string) (Workers, error) { return c(root) }

func idleRunnerDependencies(now *time.Time) runnerLoopDependencies {
	return runnerLoopDependencies{
		Now: func() time.Time { return *now }, Sleep: func(d time.Duration) { *now = now.Add(d) },
		Resumable:      func(string) (string, bool, error) { return "", false, nil },
		DeliverPending: func(string) (int, error) { return 0, nil },
		Channel:        func(context.Context, string) (int, error) { return 0, nil },
	}
}

func TestRunnerSleepsOnlyTheRemainder(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"normal", "helm", "failed", "overrun"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			loop := newHelmLoop(t)
			if mode == "helm" {
				takeHelmFixture(t, loop.root)
			}
			start := time.Date(2026, 10, 4, 8, 0, 0, 0, time.UTC)
			now, ticks := start, 0
			deps := idleRunnerDependencies(&now)
			deps.Tick = func(root string, _ TickConfig, census WorkerCensus) (TickResult, error) {
				ticks++
				if ticks > 1 {
					loop.stop(t)
				}
				_, _ = census.Workers(root)
				now = now.Add(400 * time.Millisecond)
				if mode == "overrun" {
					now = now.Add(time.Second)
					loop.stop(t)
				}
				result := TickResult{healthElapsed: 100 * time.Millisecond}
				if mode == "failed" {
					return result, errors.New("fixture tick failure")
				}
				return result, nil
			}
			var slept time.Duration
			deps.Sleep = func(d time.Duration) {
				slept += d
				now = now.Add(d)
				if !now.Before(start.Add(time.Second)) {
					loop.stop(t)
				}
			}
			census := idleCensus(func(string) (Workers, error) { now = now.Add(200 * time.Millisecond); return Workers{}, nil })
			if err := runLoopWithDependencies(loop.root, census, nil, time.Second, TickConfig{Now: start}, deps); err != nil {
				t.Fatal(err)
			}
			wantSleep, wantElapsed := 400*time.Millisecond, time.Second
			if mode == "overrun" {
				wantSleep, wantElapsed = 0, 1600*time.Millisecond
			}
			if ticks != 1 || slept != wantSleep || now.Sub(start) != wantElapsed {
				t.Fatalf("ticks=%d slept=%s elapsed=%s; want 1, %s, %s", ticks, slept, now.Sub(start), wantSleep, wantElapsed)
			}
			log, err := os.ReadFile(runnerLogPath(loop.root))
			want := fmt.Sprintf("tick 1: health 100.000ms census 200.000ms other %.3fms, slept %.3f s\n", float64(wantElapsed-wantSleep-300*time.Millisecond)/float64(time.Millisecond), wantSleep.Seconds())
			if err != nil || string(log) != want {
				t.Fatalf("timing log=%q err=%v; want %q", log, err, want)
			}
		})
	}
}

func TestRunnerSecondTickDoesNotScanUnchangedRecords(t *testing.T) {
	t.Parallel()
	loop := newHelmLoop(t)
	runnerVolume(t, loop.root, 211, 267)
	proofs := filepath.Join(loop.root, "artifacts", "agents", "proof-runs", "attempts")
	if err := os.MkdirAll(proofs, 0o755); err != nil {
		t.Fatal(err)
	}
	writeAttemptRecord(t, proofs, "proof-1", `{"terminal":{"result":"success"},"payload":"`+strings.Repeat("x", 1<<20))
	now := time.Date(2026, 10, 4, 8, 0, 0, 0, time.UTC)
	deps := idleRunnerDependencies(&now)
	ticks, scans, reads := 0, 0, 0
	deps.Tick = func(root string, _ TickConfig, census WorkerCensus) (TickResult, error) {
		ticks++
		if ticks > 2 {
			loop.stop(t)
			return TickResult{}, nil
		}
		value, _ := runnerReads.Load(root)
		state := value.(*runnerReadState)
		beforeScans, beforeReads := state.scans, state.reads
		start := time.Now()
		alerts, err := loadAlertEpisodesUnlocked(root)
		if err != nil || len(alerts) != 211 {
			t.Fatalf("alerts=%d err=%v", len(alerts), err)
		}
		if role := checkProofAttempts(root, attemptProbe{}); role.Status != HealthAlive {
			t.Fatal(role)
		}
		health := time.Since(start)
		start = time.Now()
		_, _ = census.Workers(root)
		t.Logf("tick %d: health records=%s census supplement=%s scans=%d reads=%d", ticks, health, time.Since(start), state.scans-beforeScans, state.reads-beforeReads)
		if ticks == 1 {
			scans, reads = state.scans, state.reads
		} else if state.scans != scans || state.reads != reads {
			t.Fatalf("unchanged tick scanned: scans %d -> %d reads %d -> %d", scans, state.scans, reads, state.reads)
		}
		if ticks == 2 {
			loop.stop(t)
		}
		return TickResult{}, nil
	}
	if err := runLoopWithDependencies(loop.root, idleCensus(func(root string) (Workers, error) {
		live, mains, unknown := supplementWorkers(root)
		return Workers{Live: live, LiveSeatMains: mains, Unprovable: unknown}, nil
	}), nil, time.Second, TickConfig{Now: now}, deps); err != nil {
		t.Fatal(err)
	}
	if ticks != 2 {
		t.Fatalf("ticks=%d, want 2", ticks)
	}
	if _, ok := runnerReads.Load(loop.root); ok {
		t.Fatal("runner retained its snapshots after releasing its lock")
	}
}

func TestRunnerRefreshesRecordsAndProtectsCachedAlerts(t *testing.T) {
	t.Parallel()
	loop := newHelmLoop(t)
	runnerVolume(t, loop.root, 1, 1)
	now := time.Date(2026, 10, 4, 8, 0, 0, 0, time.UTC)
	deps := idleRunnerDependencies(&now)
	ticks := 0
	deps.Tick = func(root string, _ TickConfig, _ WorkerCensus) (TickResult, error) {
		ticks++
		alerts, err := loadAlertEpisodesUnlocked(root)
		if err != nil {
			t.Fatal(err)
		}
		if ticks == 1 {
			alerts[0].Message = "unpublished change"
			again, err := loadAlertEpisodesUnlocked(root)
			if err != nil || again[0].Message != "fixture" {
				t.Fatalf("caller changed cached alert: %+v %v", again, err)
			}
			again[0].Message = "replacement"
			if err := saveAlertEpisode(root, again[0]); err != nil {
				t.Fatal(err)
			}
		} else if ticks == 2 {
			if alerts[0].Message != "replacement" {
				t.Fatalf("replacement was stale: %+v", alerts)
			}
			if err := os.Remove(alertPath(root, alerts[0].EpisodeID)); err != nil {
				t.Fatal(err)
			}
		} else {
			if len(alerts) != 0 {
				t.Fatalf("removed alert was retained: %+v", alerts)
			}
			loop.stop(t)
		}
		return TickResult{}, nil
	}
	if err := runLoopWithDependencies(loop.root, fakeCensus{}, nil, time.Second, TickConfig{Now: now}, deps); err != nil {
		t.Fatal(err)
	}
}
