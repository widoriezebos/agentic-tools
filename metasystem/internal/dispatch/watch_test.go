package dispatch

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/run"
)

// MON-04: the job waiter blocks to terminal with pinned codes, holds a
// waiter record while waiting, and removes it on exit.
func TestJobWatchRoundTrip(t *testing.T) {
	root := t.TempDir()
	jobs := filepath.Join(root, "artifacts", "agents", "jobs")
	os.MkdirAll(jobs, 0o755)
	record := filepath.Join(jobs, "j-watch.json")
	os.WriteFile(record, []byte(`{"jobId":"j-watch","status":"running","startedAt":"2026-08-15T10:00:00Z"}`), 0o644)

	caller := run.Caller{Class: "MAIN", MainId: "main-w", SessionId: "s"}
	done := make(chan int, 1)
	go func() { done <- JobWatch(root, "j-watch", caller, 20*time.Millisecond) }()

	// While waiting, the waiter record is live (our own process, so the
	// kernel prober verifies it) and owner-correlated.
	time.Sleep(80 * time.Millisecond)
	target := run.WaiterTarget{StartedAt: "2026-08-15T10:00:00Z"}
	if !run.LiveWaiter(root, identity.KernelProber{}, "job", "j-watch", "main-w", target) {
		t.Fatal("the waiting watch holds no live waiter record")
	}
	if run.LiveWaiter(root, identity.KernelProber{}, "job", "j-watch", "main-other", target) {
		t.Fatal("a foreign owner saw the waiter as its own")
	}

	os.WriteFile(record, []byte(`{"jobId":"j-watch","status":"completed","startedAt":"2026-08-15T10:00:00Z"}`), 0o644)
	select {
	case code := <-done:
		if code != 0 {
			t.Fatalf("completed job watch exit %d", code)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("watch did not return")
	}

	// Failed maps to 1; missing maps to 4.
	os.WriteFile(record, []byte(`{"jobId":"j-watch","status":"failed","startedAt":"2026-08-15T10:00:00Z"}`), 0o644)
	if code := JobWatch(root, "j-watch", caller, time.Millisecond); code != 1 {
		t.Fatalf("failed job watch exit %d", code)
	}
	if code := JobWatch(root, "ghost", caller, time.Millisecond); code != run.ExitNoRecord {
		t.Fatalf("missing job watch exit %d", code)
	}
}
