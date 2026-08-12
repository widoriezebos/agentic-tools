package main

import (
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestRunUtilHoldRequiresTag(t *testing.T) {
	if code := runUtilHold(nil); code != 2 {
		t.Fatalf("a missing tag must be a usage error, got %d", code)
	}
}

// The verb must survive until its termination signal, then acknowledge the
// orderly stop in the stopped file and exit 0.
func TestRunUtilHoldWritesStoppedFileOnTerm(t *testing.T) {
	// Registering our own handler first keeps an early SIGTERM from killing
	// the test process before the verb has installed its handler.
	guard := make(chan os.Signal, 1)
	signal.Notify(guard, syscall.SIGTERM)
	defer signal.Stop(guard)

	stopped := filepath.Join(t.TempDir(), "child.stopped")
	done := make(chan int, 1)
	go func() {
		done <- runUtilHold([]string{"--tag", "metasystem-job-hold-test", "--stopped-file", stopped})
	}()

	// The signal is re-sent until the verb reports it saw one, because there
	// is no ordering guarantee between this send and its handler install.
	deadline := time.After(10 * time.Second)
	tick := time.NewTicker(50 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case code := <-done:
			if code != 0 {
				t.Fatalf("hold must exit 0 on an orderly stop, got %d", code)
			}
			data, err := os.ReadFile(stopped)
			if err != nil || string(data) != "stopped\n" {
				t.Fatalf("stopped file wrong: %q err=%v", data, err)
			}
			return
		case <-tick.C:
			if err := syscall.Kill(os.Getpid(), syscall.SIGTERM); err != nil {
				t.Fatal(err)
			}
		case <-deadline:
			t.Fatal("hold never returned after SIGTERM")
		}
	}
}
