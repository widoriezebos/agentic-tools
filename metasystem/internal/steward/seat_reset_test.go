package steward

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/outage"
)

func TestSeatProviderLimitHoldEndsAtCappedReset(t *testing.T) {
	t.Parallel()
	zone, err := time.LoadLocation("Europe/Amsterdam")
	if err != nil {
		t.Fatal(err)
	}
	at := func(hour, minute int) time.Time {
		return time.Date(2026, 10, 4, hour, minute, 0, 0, zone)
	}
	for _, c := range []struct {
		name  string
		line  string
		seen  time.Time
		reset time.Time
	}{
		{"session-before-reset", "You've hit your session limit · resets 8:50pm (Europe/Amsterdam)", at(20, 49), at(20, 50)},
		{"session-after-reset", "You've hit your session limit · resets 8:50pm (Europe/Amsterdam)", at(20, 51), at(20, 51).Add(5 * time.Hour)},
		{"5-hour-after-reset", "Claude AI 5-hour limit reached ∙ resets 3pm (Europe/Amsterdam)", at(15, 1), at(20, 1)},
		{"usage-after-reset", "You've hit your usage limit · resets 3pm (Europe/Amsterdam)", at(15, 1), at(20, 1)},
		{"usage-before-reset", "You've hit your usage limit · resets 3pm (Europe/Amsterdam)", at(14, 1), at(15, 0)},
		{"weekly-after-reset", "You've hit your weekly limit · resets 3pm (Europe/Amsterdam)", at(15, 1), at(20, 1)},
		{"rate-after-reset", "HTTP 429 Too Many Requests · resets 3pm (Europe/Amsterdam)", at(15, 1), at(20, 1)},
		{"usage-epoch", fmt.Sprintf("Claude AI usage limit reached|%d", at(15, 1).Add(24*time.Hour).Unix()), at(15, 1), at(20, 1)},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			bed := newSeatBed(t, seatReadyGoal("alpha", "Build it."))
			bed.now = c.seen
			first := bed.start(bed.tick(deadWorkers).Seat)
			// The provider still limits the one allowed reset retry.
			first.ResetRetry = true
			if err := writeSeatRecord(bed.root, first); err != nil {
				t.Fatal(err)
			}
			data, err := json.Marshal(map[string]any{"is_error": true, "result": c.line})
			if err != nil {
				t.Fatal(err)
			}
			bed.end(first.LaunchID, "failed", string(data))
			// Reaping later must retain the message's reset and cap.
			bed.now = bed.now.Add(5 * time.Second)
			for _, tickAt := range []time.Time{bed.now, c.reset.Add(-time.Second)} {
				bed.now = tickAt
				result := bed.tick(deadWorkers)
				mark, standing := outage.StandingAt(bed.root, bed.now)
				if !standing || mark.LastClass != outage.ProviderLimit || mark.LastAt != c.seen.UTC().Format(time.RFC3339) || mark.ResetAt != c.reset.UTC().Format(time.RFC3339) {
					t.Fatalf("message %q at %s: mark=%+v, standing=%v at %s, want reset %s", c.line, c.seen, mark, standing, bed.now, c.reset)
				}
				if !result.ProviderOutage || result.Seat != nil || len(bed.launcher.starts) != 1 {
					t.Fatalf("a limit holds the seat until reset: %+v, starts=%d", result, len(bed.launcher.starts))
				}
			}
			bed.now = c.reset
			result := bed.tick(deadWorkers)
			if result.ProviderOutage || result.Seat == nil {
				t.Fatalf("reset releases the seat: %+v", result)
			}
			bed.start(result.Seat)
			if len(bed.launcher.starts) != 2 {
				t.Fatalf("reset must permit the next seat start, got %d", len(bed.launcher.starts))
			}
		})
	}
}

func TestSeatResetRetryBeforeMark(t *testing.T) {
	t.Parallel()
	for _, minute := range []int{49, 50, 51, 53, 54} {
		t.Run(strconv.Itoa(minute), func(t *testing.T) {
			t.Parallel()
			bed := newSeatBed(t, seatReadyGoal("alpha", "Build it."))
			seenMinute := minute
			if minute == 54 {
				seenMinute = 49
			}
			bed.now = time.Date(2026, 10, 4, 18, seenMinute, 0, 0, time.UTC)
			first := bed.start(bed.tick(deadWorkers).Seat)
			end := func(id string) {
				bed.end(id, "failed", `{"is_error":true,"result":"You've hit your session limit · resets 8:50pm (Europe/Amsterdam)"}`)
				state := bed.launcher.states[id]
				state.FinishedAt = bed.now.Format(time.RFC3339)
				bed.launcher.states[id] = state
			}
			end(first.LaunchID)
			// Reaping occurs after reset; parsing must still use the ending.
			if minute == 53 {
				bed.now = bed.now.Add(time.Minute)
			} else if minute == 54 {
				bed.now = bed.now.Add(5 * time.Minute)
			}
			deps := bed.dependencies()
			waited := time.Duration(0)
			deps.Seat.Sleep = func(wait time.Duration) {
				if _, err := os.Stat(outage.Path(bed.root)); !os.IsNotExist(err) {
					t.Fatalf("mark exists before retry: %v", err)
				}
				waited += wait
				bed.now = bed.now.Add(wait)
			}
			result, err := decideTickWithDependencies(bed.root, TickConfig{Now: bed.now}, fakeCensus{workers: deadWorkers}, Evidence{}, Marks{}, deps)
			if err != nil {
				t.Fatal(err)
			}
			if minute == 53 {
				mark, _ := outage.Read(bed.root)
				if result.Seat != nil || mark.LastAt != "2026-10-04T18:53:00Z" || mark.ResetAt != "2026-10-04T23:53:00Z" || waited != 0 {
					t.Fatalf("non-edge failure: result=%+v, mark=%+v, wait=%v", result, mark, waited)
				}
				return
			}
			if _, err := os.Stat(outage.Path(bed.root)); !os.IsNotExist(err) {
				t.Fatalf("mark exists before retry launch: %v", err)
			}
			if minute == 49 && (waited != time.Minute || bed.now.Minute() != 50) || minute >= 50 && waited != 0 {
				t.Fatalf("wait=%v, retry time=%s", waited, bed.now)
			}
			retry := bed.start(result.Seat)
			if !retry.ResetRetry || len(bed.launcher.starts) != 2 {
				t.Fatalf("one reset retry: %+v, starts=%d", retry, len(bed.launcher.starts))
			}
			end(retry.LaunchID)
			result = bed.tick(deadWorkers)
			if _, ok := outage.Read(bed.root); !ok || result.Seat != nil || len(bed.launcher.starts) != 2 {
				t.Fatalf("failed retry must mark and hold: %+v", result)
			}
		})
	}
}

func TestSeatIgnoresAndLogsDistantReset(t *testing.T) {
	t.Parallel()
	bed := newSeatBed(t, seatReadyGoal("alpha", "Build it."))
	bed.now = time.Date(2026, 10, 4, 18, 49, 0, 0, time.UTC)
	mark, err := outage.Record(bed.root, outage.ProviderLimit, "limited", "fixture", bed.now)
	if err != nil {
		t.Fatal(err)
	}
	mark.ResetAt = bed.now.Add(24 * time.Hour).Format(time.RFC3339)
	data, err := json.Marshal(mark)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(outage.Path(bed.root), data, 0o644); err != nil {
		t.Fatal(err)
	}
	deps := bed.dependencies()
	var lines []string
	deps.Seat.Log = func(line string) { lines = append(lines, line) }
	result, err := decideTickWithDependencies(bed.root, TickConfig{Now: bed.now}, fakeCensus{workers: deadWorkers}, Evidence{}, Marks{}, deps)
	if err != nil || result.ProviderOutage || result.Seat == nil || len(lines) != 1 || !strings.Contains(lines[0], "mark resetAt 2026-10-05T18:49:00Z is more than 5h ahead of now; ignored") {
		t.Fatalf("distant mark must be ignored and logged: %+v, log=%v, error=%v", result, lines, err)
	}
}

func TestSeatResetRetryCanRecover(t *testing.T) {
	t.Parallel()
	bed := newSeatBed(t, seatReadyGoal("alpha", "Build it."))
	bed.now = time.Date(2026, 10, 4, 18, 51, 0, 0, time.UTC)
	first := bed.start(bed.tick(deadWorkers).Seat)
	bed.end(first.LaunchID, "failed", `{"is_error":true,"result":"You've hit your session limit · resets 8:50pm (Europe/Amsterdam)"}`)
	selection := bed.tick(deadWorkers).Seat
	bed.now = bed.now.Add(time.Second)
	retry := bed.start(selection)
	bed.tips["alpha"] = "advanced"
	bed.end(retry.LaunchID, "completed", `{"is_error":false,"result":"done"}`)
	bed.tick(deadWorkers)
	if _, err := os.Stat(outage.Path(bed.root)); !os.IsNotExist(err) {
		t.Fatalf("a successful retry wrote an outage mark: %v", err)
	}
	if records := bed.records(); len(records) != 2 || !records[1].ResetRetry || records[1].Outcome != SeatProgress {
		t.Fatalf("the retry did not retain progress: %+v", records)
	}
}
