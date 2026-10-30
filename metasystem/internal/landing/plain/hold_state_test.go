package plain

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
)

// holdTick is one lane tick in production's order, called directly: the
// keeper clock, the keeper's contention hold, the steward step, then health.
type holdTick struct {
	clockCalls int
	hold       string
	run        StewardRun
	health     *Health
}

func TestTheLanesHoldEndsWithItsCause(t *testing.T) {
	t.Parallel()
	until := bedNow.Add(30 * time.Minute).Local().Format("15:04 MST")
	bed := func(t *testing.T) (*stewardBed, launch.Store) {
		b, path := contentionStewardBed(t, "build")
		return b, launch.Store{Root: filepath.Dir(filepath.Dir(path))}
	}
	lines := func(t *testing.T, b *stewardBed, prefix string) []Intervention {
		t.Helper()
		all, err := readLines[Intervention](interventionsPath(b.install))
		if err != nil {
			t.Fatal(err)
		}
		var found []Intervention
		for _, p := range all {
			if p.Row == "proof-beside-builds" && strings.HasPrefix(p.Outcome, prefix) {
				found = append(found, p)
			}
		}
		return found
	}
	health := func(t *testing.T, b *stewardBed, now time.Time) *Health {
		t.Helper()
		b.seams.Now = func() time.Time { return now }
		status := ReadStatus(b.home, b.record, b.view, b.seams)
		if status.Health == nil || len(status.Problems) > 0 {
			t.Fatalf("status problems=%v health=%+v", status.Problems, status.Health)
		}
		return status.Health
	}
	heldRow := func(h *Health) *Fired {
		for _, r := range h.Rows() {
			if r.Row == "proof-beside-builds" && strings.HasPrefix(r.Act, "held:") {
				return &r
			}
		}
		return nil
	}
	keeperHold := func(t *testing.T, b *stewardBed, now time.Time) string {
		t.Helper()
		b.seams.Now = func() time.Time { return now }
		hold, err := KeeperContentionHold(b.install, b.seams)
		if err != nil {
			t.Fatal(err)
		}
		return hold
	}
	tick := func(t *testing.T, b *stewardBed, now time.Time) holdTick {
		t.Helper()
		b.seams.Now = func() time.Time { return now }
		before := len(b.calls)
		if _, err := KeeperClock(b.record, func(lane.Record) error { return nil }, b.seams); err != nil {
			t.Fatal(err)
		}
		result := holdTick{clockCalls: len(b.calls) - before, hold: keeperHold(t, b, now)}
		result.run = b.step(now)
		result.health = health(t, b, now)
		return result
	}
	end := func(t *testing.T, store launch.Store, id string) {
		t.Helper()
		if _, err := store.Update(id, func(r *launch.Record) error { r.State = launch.Completed; return nil }); err != nil {
			t.Fatal(err)
		}
	}

	t.Run("the build ends", func(t *testing.T) {
		t.Parallel()
		b, store := bed(t)
		first := tick(t, b, bedNow)
		held := lines(t, b, "held:")
		clock, err := readClock(b.install)
		if err != nil || len(held) != 1 || len(b.calls) != 0 || !clock.RetryAt.Equal(bedNow.Add(30*time.Minute)) || !strings.Contains(first.hold, until) {
			t.Fatalf("first tick: held=%+v calls=%v retry=%v hold=%q err=%v", held, b.calls, clock.RetryAt, first.hold, err)
		}
		now := bedNow.Add(10 * time.Minute)
		end(t, store, "host-work")
		if hold := keeperHold(t, b, now); hold != "" {
			t.Fatalf("the keeper holds after the build ended: %q", hold)
		}
		if row := heldRow(health(t, b, now)); row != nil {
			t.Fatalf("health shows an ended hold: %+v", row)
		}
		second := tick(t, b, now)
		released := lines(t, b, "released: cause ended")
		if second.clockCalls != 1 || len(b.calls) != 1 {
			t.Fatalf("the clock did not start main's check once the build ended: clock=%d calls=%v", second.clockCalls, b.calls)
		}
		if len(released) != 1 || released[0].Subject != held[0].Subject {
			t.Fatalf("release lines=%+v held=%+v", released, held)
		}
		if second.hold != "" || strings.Contains(second.run.Line, "held beside builds") || healthRow(second.health, "proof-beside-builds") != nil {
			t.Fatalf("an ended hold is still read: hold=%q line=%q row=%+v", second.hold, second.run.Line, healthRow(second.health, "proof-beside-builds"))
		}
		if err := store.Create(launch.Record{ID: "host-work-2", Kind: "build", State: launch.Running}); err != nil {
			t.Fatal(err)
		}
		third := tick(t, b, bedNow.Add(12*time.Minute))
		if third.hold != "" || heldRow(third.health) != nil || len(lines(t, b, "held:")) != 1 || len(b.calls) != 1 {
			t.Fatalf("a new build revived the ended hold: hold=%q row=%+v held=%+v calls=%v", third.hold, heldRow(third.health), lines(t, b, "held:"), b.calls)
		}
	})

	t.Run("the steward's act starts the proof", func(t *testing.T) {
		t.Parallel()
		b, store := bed(t)
		b.seams.TimerHeld = func() bool { return true }
		tick(t, b, bedNow)
		if len(lines(t, b, "held:")) != 1 || len(b.calls) != 0 {
			t.Fatalf("the steward's start was not held: %+v calls=%v", lines(t, b, "held:"), b.calls)
		}
		end(t, store, "host-work")
		second := tick(t, b, bedNow.Add(10*time.Minute))
		if second.clockCalls != 0 || len(b.calls) != 1 || strings.Contains(second.run.Line, "held beside builds") {
			t.Fatalf("the steward's act did not start the proof: clock=%d calls=%v line=%q", second.clockCalls, b.calls, second.run.Line)
		}
		if released := lines(t, b, "released: cause ended"); len(released) != 1 {
			t.Fatalf("release lines=%+v", released)
		}
	})

	t.Run("a build still runs", func(t *testing.T) {
		t.Parallel()
		b, _ := bed(t)
		tick(t, b, bedNow)
		second := tick(t, b, bedNow.Add(10*time.Minute))
		if !strings.Contains(second.hold, until) || !strings.Contains(second.run.Line, "bounded: the proof start is held beside builds until "+until) || healthRow(second.health, "proof-beside-builds") == nil {
			t.Fatalf("a live hold was not read: hold=%q line=%q", second.hold, second.run.Line)
		}
		if len(b.calls) != 0 || len(lines(t, b, "released")) != 0 {
			t.Fatalf("a held start launched or released: calls=%v released=%+v", b.calls, lines(t, b, "released"))
		}
	})

	t.Run("a released hold names nothing", func(t *testing.T) {
		t.Parallel()
		b, _ := bed(t)
		for _, entry := range []Intervention{
			{At: bedNow.Add(-48 * time.Hour).Format(time.RFC3339Nano), Row: "proof-beside-builds", Subject: "old-main", Outcome: "held: 1 builds and 0 unit checks run; load 2"},
			{At: bedNow.Add(-47*time.Hour - 30*time.Minute).Format(time.RFC3339Nano), Row: "proof-beside-builds", Subject: "old-main", Outcome: "released after the 30-minute hold; 1 builds and 0 unit checks run; load 2"},
		} {
			if err := RecordIntervention(b.install, entry); err != nil {
				t.Fatal(err)
			}
		}
		if row := healthRow(health(t, b, bedNow), "proof-beside-builds"); row != nil {
			t.Fatalf("a days-old released hold is shown: %+v", row)
		}
	})
}
