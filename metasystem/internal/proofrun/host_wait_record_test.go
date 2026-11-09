package proofrun

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func readSlotWaitLines(t *testing.T, path string) []SlotWaitLine {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var lines []SlotWaitLine
	for _, raw := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		var line SlotWaitLine
		if err := json.Unmarshal([]byte(raw), &line); err != nil {
			t.Fatal(err)
		}
		lines = append(lines, line)
	}
	return lines
}

func TestHostSlotWaitRecorded(t *testing.T) {
	t.Parallel()
	for _, outcome := range []string{"success", "give-up", "end write fails", "no destination"} {
		t.Run(outcome, func(t *testing.T) {
			t.Parallel()
			directory, conf := isolatedHostResources(t)
			readers := deterministicTestLoadReaders()
			holder, err := acquireHostResourcesWith(t.Context(), readers, directory, directory, conf, "heavy", nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			release := func(lease *HostResourceLease) {
				t.Helper()
				if err := MarkHostResourcesClean(lease.Files()); err != nil {
					t.Fatal(err)
				}
				if err := lease.Close(); err != nil {
					t.Fatal(err)
				}
			}
			defer holder.Close()
			destination := t.TempDir()
			path := filepath.Join(destination, "waits.jsonl")
			start := time.Date(2026, 8, 28, 10, 5, 0, 0, time.UTC)
			tick := 0
			now := func() time.Time { at := start.Add(time.Duration(tick) * time.Hour); tick++; return at }
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if outcome != "no destination" {
				ctx = withSlotWaitRecord(ctx, destination, now)
			} else {
				ctx = withSlotWaitRecord(ctx, "", now)
			}
			observed := 0
			ctx = WithHostResourceWaitObserver(ctx, func() {
				observed++
				if outcome != "no destination" {
					lines := readSlotWaitLines(t, path)
					if len(lines) != 1 {
						t.Fatalf("before retry: %+v", lines)
					}
					line := lines[0]
					if line.Wait != "start" || !strings.HasPrefix(line.Marker, "wait-run-") || line.Directory != directory || line.Pid != os.Getpid() || line.At != start.Format(time.RFC3339Nano) {
						t.Fatalf("start: %+v", line)
					}
					if _, err := os.Stat(filepath.Join(directory, line.Marker)); err != nil {
						t.Fatal(err)
					}
				}
				if outcome == "give-up" {
					cancel()
					return
				}
				if outcome == "end write fails" {
					if err := os.Chmod(path, 0o400); err != nil {
						t.Fatal(err)
					}
				}
				release(holder)
			})
			lease, err := acquireHostResourcesWith(ctx, readers, directory, directory, conf, "heavy", nil, nil)
			if outcome == "give-up" {
				if !errors.Is(err, context.Canceled) || lease != nil {
					t.Fatalf("give-up: %v %v", lease, err)
				}
				release(holder)
			} else {
				if err != nil {
					t.Fatal(err)
				}
				release(lease)
			}
			if observed != 1 {
				t.Fatalf("wait observations = %d", observed)
			}
			if outcome == "no destination" {
				entries, err := os.ReadDir(destination)
				if err != nil || len(entries) != 0 {
					t.Fatalf("unrequested records: %v %v", entries, err)
				}
				return
			}
			lines := readSlotWaitLines(t, path)
			want := 2
			if outcome == "end write fails" {
				want = 1
			}
			if len(lines) != want {
				t.Fatalf("after acquisition: %+v", lines)
			}
			if want == 2 && (lines[1].Wait != "end" || lines[1].Marker != lines[0].Marker || lines[1].At != start.Add(time.Hour).Format(time.RFC3339Nano)) {
				t.Fatalf("end: %+v", lines)
			}
			if _, err := os.Stat(filepath.Join(directory, lines[0].Marker)); !os.IsNotExist(err) {
				t.Fatalf("marker survived: %v", err)
			}
		})
	}
}

func TestSlotWaitHeld(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	marker := "wait-run-fixture"
	file, available, err := tryHostFile(filepath.Join(directory, marker))
	if err != nil || !available {
		t.Fatalf("hold: %t %v", available, err)
	}
	defer file.Close()
	if held, err := SlotWaitHeld(directory, marker); err != nil || !held {
		t.Fatalf("held: %t %v", held, err)
	}
	if err := releaseHostProbe(file); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(directory, marker)); err != nil {
		t.Fatal(err)
	}
	if held, err := SlotWaitHeld(directory, marker); err != nil || held {
		t.Fatalf("released: %t %v", held, err)
	}
	for _, marker := range []string{"../x", "ordinary"} {
		if _, err := SlotWaitHeld(directory, marker); err == nil {
			t.Fatalf("accepted %q", marker)
		}
	}
}
