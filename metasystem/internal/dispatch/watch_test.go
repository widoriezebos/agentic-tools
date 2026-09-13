package dispatch

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/run"
)

// The job waiter blocks to terminal with pinned codes, holds a
// waiter record while waiting, and removes it on exit.
func TestJobWatchRoundTrip(t *testing.T) {
	root := t.TempDir()
	jobs := filepath.Join(root, "artifacts", "agents", "jobs")
	os.MkdirAll(jobs, 0o755)
	record := filepath.Join(jobs, "j-watch.json")
	// Records are rewritten the way the engine writes them: whole, by
	// rename. os.WriteFile truncates first, and a poll landing in that gap
	// reads an empty file, which is "no record" (exit 4), not a status.
	writeRecord := func(status string) {
		t.Helper()
		temp := record + ".tmp"
		if err := os.WriteFile(temp, []byte(`{"jobId":"j-watch","status":"`+status+`","startedAt":"2026-08-15T10:00:00Z"}`), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Rename(temp, record); err != nil {
			t.Fatal(err)
		}
	}
	writeRecord("running")

	caller := run.Caller{Class: "MAIN", MainId: "main-w", SessionId: "s"}
	done := make(chan int, 1)
	go func() { done <- JobWatch(root, "j-watch", caller, 20*time.Millisecond) }()

	// While waiting, the waiter record is live (our own process, so the
	// kernel prober verifies it) and owner-correlated. Wait for it rather
	// than sleeping a fixed slice: under a loaded -race suite the watcher
	// goroutine can take longer than any constant to write its record.
	target := run.WaiterTarget{StartedAt: "2026-08-15T10:00:00Z"}
	deadline := time.Now().Add(wiringBound)
	for !run.LiveWaiter(root, identity.KernelProber{}, "job", "j-watch", "main-w", target) {
		if time.Now().After(deadline) {
			t.Fatal("the waiting watch holds no live waiter record")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if run.LiveWaiter(root, identity.KernelProber{}, "job", "j-watch", "main-other", target) {
		t.Fatal("a foreign owner saw the waiter as its own")
	}

	writeRecord("completed")
	select {
	case code := <-done:
		if code != 0 {
			t.Fatalf("completed job watch exit %d", code)
		}
	case <-time.After(wiringBound):
		t.Fatal("watch did not return")
	}

	// Failed maps to 1; missing maps to 4.
	writeRecord("failed")
	if code := JobWatch(root, "j-watch", caller, time.Millisecond); code != 1 {
		t.Fatalf("failed job watch exit %d", code)
	}
	if code := JobWatch(root, "ghost", caller, time.Millisecond); code != run.ExitNoRecord {
		t.Fatalf("missing job watch exit %d", code)
	}
}

func TestWaitJobTerminals(t *testing.T) {
	root := t.TempDir()
	jobs := filepath.Join(root, "artifacts", "agents", "jobs")
	if err := os.MkdirAll(jobs, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(jobs, "job-a.json")
	if err := os.WriteFile(path, []byte(`{"jobId":"job-a","operationId":"reserve-a","status":"pending-setup","createdAt":"2026-09-13T09:59:00Z"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	setup, setupErr := ObserveJob(context.Background(), root, "job-a", run.WaiterTarget{}, "")
	if setupErr != nil || !setup.Pending || setup.Outcome != "pending-setup" || setup.ExitCode != 0 || setup.Incarnation.OperationID != "reserve-a" || setup.Incarnation.Round != 0 {
		t.Fatalf("pending-setup observation = %+v err=%v", setup, setupErr)
	}
	write := func(status, operationID, started string, round int) {
		t.Helper()
		body := fmt.Sprintf(`{"jobId":"job-a","operationId":"%s","round":%d,"status":"%s","startedAt":"%s"}`, operationID, round, status, started)
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("pending", "reserve-a", "2026-09-13T10:00:00Z", 2)
	pending, pendingErr := ObserveJob(context.Background(), root, "job-a", setup.Incarnation, "")
	if pendingErr != nil || !pending.Pending || pending.Incarnation != setup.Incarnation {
		t.Fatalf("pre-running observation adopted round or start early: %+v err=%v", pending, pendingErr)
	}
	for status, code := range map[string]int{"completed": 0, "failed": 1, "timeout": 2, "cancelled": 3} {
		write(status, "reserve-a", "2026-09-13T10:00:00Z", 2)
		observation, err := ObserveJob(context.Background(), root, "job-a", run.WaiterTarget{}, "")
		if err != nil || observation.Pending || observation.ExitCode != code || observation.Incarnation.Round != 2 || observation.Incarnation.OperationID != "reserve-a" {
			t.Fatalf("%s observation = %+v err=%v", status, observation, err)
		}
	}
	write("running", "reserve-a", "2026-09-13T10:00:00Z", 2)
	observation, _ := ObserveJob(context.Background(), root, "job-a", setup.Incarnation, "")
	if !observation.Pending || observation.Incarnation.Round != 2 {
		t.Fatalf("running observation = %+v", observation)
	}
	write("completed", "reserve-b", "2026-09-13T11:00:00Z", 3)
	reused, _ := ObserveJob(context.Background(), root, "job-a", observation.Incarnation, "")
	if reused.Incarnation == observation.Incarnation || reused.Incarnation.OperationID != "reserve-b" {
		t.Fatal("job reservation operation identifier reuse did not change the pinned incarnation")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	missing, _ := ObserveJob(context.Background(), root, "job-a", run.WaiterTarget{}, "")
	if missing.ExitCode != run.ExitNoRecord {
		t.Fatalf("missing observation = %+v", missing)
	}
}

func TestWaitJobPendingSetupAdoptsRunningOnce(t *testing.T) {
	root := t.TempDir()
	jobs := filepath.Join(root, "artifacts", "agents", "jobs")
	if err := os.MkdirAll(jobs, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(jobs, "job-adopt.json")
	write := func(status string) {
		t.Helper()
		body := fmt.Sprintf(`{"jobId":"job-adopt","operationId":"reserve-a","round":1,"status":"%s","startedAt":"2026-09-13T10:00:00Z"}`, status)
		if status == "pending-setup" {
			body = `{"jobId":"job-adopt","operationId":"reserve-a","status":"pending-setup"}`
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("pending-setup")
	now := time.Date(2026, 9, 13, 10, 0, 0, 0, time.UTC)
	boot := time.Hour
	step := 0
	result := (&run.Store{Root: root}).Wait(context.Background(), run.WaitRequest{
		Selector:       run.WaitSelector{Kind: "job", TargetID: "job-adopt"},
		Owner:          run.Caller{Class: "MAIN", MainId: "main-adopt", OwnerLineage: "lineage-adopt", SessionId: "session-adopt"},
		RuntimeSession: "session-adopt", Timeout: time.Hour,
	}, run.WaitOptions{
		Now:       func() time.Time { return now },
		BootClock: func() (string, time.Duration, error) { return "boot-adopt", boot, nil },
		Deliver: func(context.Context, string, string, time.Time, string) (string, bool, error) {
			return "blocking", false, nil
		},
		Observe: func(ctx context.Context, selector run.WaitSelector, pinned run.WaiterTarget, tip string) (run.SourceObservation, error) {
			return ObserveJob(ctx, root, selector.TargetID, pinned, tip)
		},
		Sleep: func(context.Context, time.Duration) error {
			step++
			now = now.Add(time.Second)
			boot += time.Second
			switch step {
			case 1:
				write("pending")
			case 2:
				write("running")
			case 3:
				rows, failures := run.PendingWaiters(root, "lineage-adopt")
				if len(failures) != 0 || len(rows) != 1 || rows[0].Target.OperationID != "reserve-a" || rows[0].Target.Round != 1 || rows[0].Target.StartedAt == "" {
					t.Fatalf("running job incarnation was not durably adopted: rows=%+v failures=%v", rows, failures)
				}
				write("completed")
			default:
				t.Fatal("job adoption wait did not terminate")
			}
			return nil
		},
	})
	if result.ExitCode != run.ExitGreen || result.TargetIncarnation.OperationID != "reserve-a" || result.TargetIncarnation.Round != 1 || result.TargetIncarnation.StartedAt == "" {
		t.Fatalf("pending-setup adoption result = %+v", result)
	}
}
