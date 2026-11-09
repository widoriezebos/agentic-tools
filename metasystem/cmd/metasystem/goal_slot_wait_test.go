package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testprovider"
)

func TestGoalStatusSaysItWaitsForATestSlot(t *testing.T) {
	t.Parallel()
	bed := newWorkBedWith(t, workApprovedBox)
	clock := &workClock{now: bed.manager.Now()}
	bed.manager.Now = clock.Now
	owners := bed.workOwners()
	owners.commandNow = func(string) (time.Time, error) { return clock.Now(), nil }
	owners.lookupEnv = func(key string) (string, bool) { return bed.root(), key == "METASYSTEM_SUPERVISION_REGISTRY_HOME" }
	bed.manager.Store.Root = filepath.Join(testprovider.Register(t, bed.root()), "launch")
	start := clock.Now()
	if err := bed.manager.Store.Create(launch.Record{ID: "slot-check", Kind: "proof", Adapter: "local", Goal: bed.id, State: launch.Running, StartedAt: start.Format(time.RFC3339Nano)}); err != nil {
		t.Fatal(err)
	}
	directory, marker := t.TempDir(), "wait-run-command"
	held, err := os.OpenFile(filepath.Join(directory, marker), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer held.Close()
	if err := syscall.Flock(int(held.Fd()), syscall.LOCK_EX); err != nil {
		t.Fatal(err)
	}
	defer syscall.Flock(int(held.Fd()), syscall.LOCK_UN)
	state, err := bed.manager.Store.StateDir("slot-check")
	if err != nil {
		t.Fatal(err)
	}
	since := start.Add(5 * time.Minute)
	appendWait := func(kind string, at time.Time) {
		t.Helper()
		data, err := json.Marshal(proofrun.SlotWaitLine{Wait: kind, Marker: marker, Directory: directory, At: at.Format(time.RFC3339Nano)})
		if err != nil {
			t.Fatal(err)
		}
		f, err := os.OpenFile(filepath.Join(state, "waits.jsonl"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
		if err != nil {
			t.Fatal(err)
		}
		_, err = f.Write(append(data, '\n'))
		_ = f.Close()
		if err != nil {
			t.Fatal(err)
		}
	}
	appendWait("start", since)
	move := func(duration time.Duration) { clock.mu.Lock(); clock.now = clock.now.Add(duration); clock.mu.Unlock() }
	move(2 * time.Hour)
	assertView := func(waiting bool, elapsed time.Duration) {
		t.Helper()
		projection := dispatchcore.ProjectBudget(bed.root(), bed.goalFile(bed.id), clock.Now(), filepath.Dir(bed.manager.Store.Root))
		if projection.Status != dispatchcore.BudgetKnown || projection.ElapsedLegacy || projection.Elapsed != elapsed {
			t.Fatalf("command clock: %+v", projection)
		}
		for _, verb := range []string{"status", "show"} {
			code, text, stderr := bed.run(owners, "goal", verb, bed.id)
			if code != 0 {
				t.Fatalf("goal %s: %d %s %s", verb, code, text, stderr)
			}
			words := "waiting for a test slot since " + since.Local().Format("15:04") + ", clock paused (1.9h so far)"
			if waiting && !strings.Contains(text, words) || !waiting && strings.Contains(text, "waiting for a test slot") {
				t.Fatalf("goal %s wait words: %s", verb, text)
			}
			if verb == "status" && !strings.Contains(text, "spent: elapsed "+elapsed.String()+" of") {
				t.Fatalf("goal status elapsed: %s", text)
			}
			t.Logf("goal %s (waiting=%t):\n%s", verb, waiting, text)
		}
	}
	assertView(true, 5*time.Minute)
	appendWait("end", clock.Now())
	if err := syscall.Flock(int(held.Fd()), syscall.LOCK_UN); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(directory, marker)); err != nil {
		t.Fatal(err)
	}
	move(time.Hour)
	if _, err := bed.manager.Store.Update("slot-check", func(record *launch.Record) error {
		record.State, record.FinishedAt = launch.Completed, clock.Now().Format(time.RFC3339Nano)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	assertView(false, time.Hour+5*time.Minute)
}
