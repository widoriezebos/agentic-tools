package dispatch

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
)

func writeLaunchSlotWaits(t *testing.T, bed *goalAdmissionBed, id string, lines ...proofrun.SlotWaitLine) string {
	t.Helper()
	dir, err := (launch.Store{Root: filepath.Join(bed.reads.BudgetHome, "launch")}).StateDir(id)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	var data []byte
	for _, line := range lines {
		raw, err := json.Marshal(line)
		if err != nil {
			t.Fatal(err)
		}
		data = append(data, append(raw, '\n')...)
	}
	path := filepath.Join(dir, "waits.jsonl")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func pairedSlotWait(start, end time.Time) []proofrun.SlotWaitLine {
	return []proofrun.SlotWaitLine{{Wait: "start", Marker: "wait-run-fixture", At: start.Format(time.RFC3339Nano)}, {Wait: "end", Marker: "wait-run-fixture", At: end.Format(time.RFC3339Nano)}}
}

func heldSlotWaitMarker(t *testing.T) (string, string) {
	t.Helper()
	directory, marker := t.TempDir(), "wait-run-held"
	f, err := os.OpenFile(filepath.Join(directory, marker), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		f.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN); _ = f.Close() })
	return directory, marker
}

func slotWaitLaunch(t *testing.T, bed *goalAdmissionBed, id string, start, end time.Time) {
	t.Helper()
	record := launch.Record{ID: id, Goal: "bounded", Kind: "proof", Adapter: "local", State: launch.Running, StartedAt: start.Format(time.RFC3339Nano)}
	if !end.IsZero() {
		record.State, record.FinishedAt = launch.Completed, end.Format(time.RFC3339Nano)
	}
	if err := (launch.Store{Root: filepath.Join(bed.reads.BudgetHome, "launch")}).Create(record); err != nil {
		t.Fatal(err)
	}
}

func TestOwnWorkSlotWaits(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name string
		want time.Duration
	}{
		{"paired", time.Hour}, {"clipped", 6 * time.Hour}, {"build inside wait", 2 * time.Hour}, {"two waiters", time.Hour}, {"person beside wait", time.Hour},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			bed, file, start := ownWorkBed(t, 10)
			end := start.Add(7 * time.Hour)
			slotWaitLaunch(t, bed, "check", start, end)
			from, until := start.Add(5*time.Minute), start.Add(6*time.Hour+5*time.Minute)
			if row.name == "clipped" {
				from, until = start.Add(-time.Hour), start.Add(time.Hour)
			}
			writeLaunchSlotWaits(t, bed, "check", pairedSlotWait(from, until)...)
			switch row.name {
			case "build inside wait":
				ownWorkLaunch(t, bed, file.Id, "build", start.Add(time.Hour), start.Add(2*time.Hour))
			case "two waiters":
				slotWaitLaunch(t, bed, "second-check", start, end)
				writeLaunchSlotWaits(t, bed, "second-check", pairedSlotWait(from, until)...)
			case "person beside wait":
				writePersonQuestion(t, bed.root, "person", file.Id, start, nil)
			}
			p := assertOwnElapsed(t, bed, file, end, row.want)
			if len(p.Problems) != 0 || !p.SlotWaitSince.IsZero() {
				t.Fatalf("paired wait: %+v", p)
			}
			if row.name == "person beside wait" {
				if p.PersonPaused != 6*time.Hour {
					t.Fatalf("person pause = %s", p.PersonPaused)
				}
				consumption := ProjectConsumption(bed.root, file, end, bed.reads.BudgetHome)
				if consumption.PersonPaused != p.PersonPaused {
					t.Fatalf("consumption pause = %s, authority = %s", consumption.PersonPaused, p.PersonPaused)
				}
			}
		})
	}
}

func TestOwnWorkOpenSlotWait(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"held", "lost end", "two lost ends", "terminal", "probe error"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			bed, file, start := ownWorkBed(t, 10)
			now, since := start.Add(2*time.Hour), start.Add(5*time.Minute)
			end := time.Time{}
			if mode == "terminal" {
				end = now
			}
			slotWaitLaunch(t, bed, "check", start, end)
			directory, marker := heldSlotWaitMarker(t)
			if mode == "lost end" || mode == "two lost ends" {
				directory = t.TempDir()
			}
			if mode == "probe error" {
				marker = "../invalid"
			}
			lines := []proofrun.SlotWaitLine{{Wait: "start", Marker: marker, Directory: directory, At: since.Format(time.RFC3339Nano)}}
			if mode == "two lost ends" {
				lines = append(lines, proofrun.SlotWaitLine{Wait: "start", Marker: "wait-run-earlier", Directory: directory, At: since.Add(-time.Minute).Format(time.RFC3339Nano)})
			}
			writeLaunchSlotWaits(t, bed, "check", lines...)
			want := 2 * time.Hour
			if mode == "held" {
				want = 5 * time.Minute
			}
			p := assertOwnElapsed(t, bed, file, now, want)
			if mode == "two lost ends" {
				for range 128 {
					p = assertOwnElapsed(t, bed, file, now, want)
					if len(p.Problems) != 2 || !strings.Contains(p.Problems[0], "test slot wait "+marker+" of launch check") || !strings.Contains(p.Problems[1], "test slot wait wait-run-earlier of launch check") || !p.SlotWaitSince.IsZero() || p.SlotWords() != "" {
						t.Fatalf("uncredited waits out of file order: %+v", p)
					}
				}
				return
			}
			if mode == "held" {
				words := "waiting for a test slot since " + since.Local().Format("15:04") + ", clock paused (1.9h so far)"
				if !p.SlotWaitSince.Equal(since) || p.SlotWaited != now.Sub(since) || p.SlotWords() != words || len(p.Problems) != 0 {
					t.Fatalf("open wait: %+v words=%q", p, p.SlotWords())
				}
			} else if len(p.Problems) != 1 || !strings.Contains(p.Problems[0], "check") || !strings.Contains(p.Problems[0], marker) || !p.SlotWaitSince.IsZero() || p.SlotWords() != "" {
				t.Fatalf("uncredited wait: %+v", p)
			}
		})
	}
}

func TestOwnWorkSeatSlotWait(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"seat wait", "build inside wait", "goal wait", "open seat"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			bed, file, start := ownWorkBed(t, 9)
			now := start.Add(8 * time.Hour)
			writeBudgetSeat(t, bed.root, file.Id, start.Format(time.RFC3339Nano))
			if mode == "open seat" {
				if err := (launch.Store{Root: filepath.Join(bed.reads.BudgetHome, "launch")}).Create(launch.Record{ID: "budget-seat", Kind: "seat", Adapter: "local", State: launch.Running, StartedAt: start.Format(time.RFC3339Nano)}); err != nil {
					t.Fatal(err)
				}
			} else {
				ownWorkLaunch(t, bed, "", "budget-seat", start, now)
			}
			from, until := start.Add(time.Hour), start.Add(7*time.Hour)
			want := 2 * time.Hour
			if mode == "goal wait" {
				slotWaitLaunch(t, bed, "check", from, until)
				writeLaunchSlotWaits(t, bed, "check", pairedSlotWait(from, until)...)
			} else if mode == "open seat" {
				directory, marker := heldSlotWaitMarker(t)
				writeLaunchSlotWaits(t, bed, "budget-seat", proofrun.SlotWaitLine{Wait: "start", Marker: marker, Directory: directory, At: from.Format(time.RFC3339Nano)})
				want = time.Hour
			} else {
				writeLaunchSlotWaits(t, bed, "budget-seat", pairedSlotWait(from, until)...)
				if mode == "build inside wait" {
					ownWorkLaunch(t, bed, file.Id, "build", start.Add(2*time.Hour), start.Add(3*time.Hour))
					want += time.Hour
				}
			}
			if err := os.MkdirAll(filepath.Join(bed.root, "artifacts/agents/steward/seats/directory.json"), 0o700); err != nil {
				t.Fatal(err)
			}
			p := assertOwnElapsed(t, bed, file, now, want)
			if len(p.Problems) != 0 {
				t.Fatalf("seat wait: %+v", p)
			}
			if mode == "open seat" && !p.SlotWaitSince.Equal(from) {
				t.Fatalf("seat open wait missing: %+v", p)
			}
		})
	}
}

func TestOwnWorkBrokenSlotWait(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"broken line", "duplicate start", "end before start", "bad time", "bad kind", "empty marker", "read error"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			bed, file, start := ownWorkBed(t, 10)
			now := start.Add(7 * time.Hour)
			slotWaitLaunch(t, bed, "check", start, now)
			lines := pairedSlotWait(start.Add(5*time.Minute), start.Add(6*time.Hour))
			switch mode {
			case "duplicate start":
				lines = append(lines[:1], lines...)
			case "end before start":
				lines[1].At = start.Format(time.RFC3339Nano)
			case "bad time":
				lines[0].At = "unknown"
			case "bad kind":
				lines[0].Wait = "other"
			case "empty marker":
				lines[0].Marker = ""
			case "broken line", "read error":
				lines = lines[1:]
			}
			path := writeLaunchSlotWaits(t, bed, "check", lines...)
			if mode == "broken line" {
				f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600)
				if err != nil {
					t.Fatal(err)
				}
				_, err = f.WriteString("{\n")
				_ = f.Close()
				if err != nil {
					t.Fatal(err)
				}
			}
			if mode == "read error" {
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(path, 0o700); err != nil {
					t.Fatal(err)
				}
			}
			p := assertOwnElapsed(t, bed, file, now, 7*time.Hour)
			if len(p.Problems) != 1 || !strings.Contains(p.Problems[0], "check") {
				t.Fatalf("bad wait problem: %+v", p)
			}
		})
	}
}
