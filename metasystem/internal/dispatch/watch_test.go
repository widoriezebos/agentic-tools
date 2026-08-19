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
	deadline := time.Now().Add(5 * time.Second)
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
	case <-time.After(5 * time.Second):
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
