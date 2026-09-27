package proofrun

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// A custodian or worker start handshake ends on what the child did: its
// ready line, an invalid line, its exit (EOF on the pipe), or the caller's
// cancellation. The waiter has no clock, so a report delayed by a loaded host
// is still accepted whenever it comes.
func TestStartHandshakeDecidesOnReportExitOrCancellation(t *testing.T) {
	t.Parallel()
	start := func(t *testing.T, stop <-chan struct{}) (*os.File, <-chan error) {
		t.Helper()
		read, write, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = read.Close(); _ = write.Close() })
		result := make(chan error, 1)
		go func() { result <- awaitReadyLine(read, stop, "resource custodian") }()
		return write, result
	}
	t.Run("late report", func(t *testing.T) {
		write, result := start(t, nil)
		// Nothing was written and nothing ended; only a clock could decide.
		select {
		case err := <-result:
			t.Fatalf("handshake decided before any fact: %v", err)
		default:
		}
		if _, err := io.WriteString(write, "ready\n"); err != nil {
			t.Fatal(err)
		}
		if err := <-result; err != nil {
			t.Fatalf("late ready report refused: %v", err)
		}
	})
	t.Run("exit before report", func(t *testing.T) {
		write, result := start(t, nil)
		_ = write.Close()
		if err := <-result; err == nil || !strings.Contains(err.Error(), "exited before reporting ready") {
			t.Fatalf("exit before report = %v", err)
		}
	})
	t.Run("invalid report", func(t *testing.T) {
		write, result := start(t, nil)
		if _, err := io.WriteString(write, "later\n"); err != nil {
			t.Fatal(err)
		}
		if err := <-result; err == nil || !strings.Contains(err.Error(), "readiness is invalid") {
			t.Fatalf("invalid report = %v", err)
		}
	})
	t.Run("cancelled", func(t *testing.T) {
		stop := make(chan struct{})
		_, result := start(t, stop)
		close(stop)
		if err := <-result; err == nil || !strings.Contains(err.Error(), "cancelled") {
			t.Fatalf("cancelled start = %v", err)
		}
	})
}

// recordingTB stands in for a test whose barrier wait is under examination:
// Fatalf records and ends the waiting goroutine; Context stays live for the
// first live calls and is cancelled afterwards.
type recordingTB struct {
	testing.TB
	live    int
	calls   int
	stopped context.Context
	fatal   string
}

func (tb *recordingTB) Helper() {}

func (tb *recordingTB) Context() context.Context {
	tb.calls++
	if tb.calls <= tb.live {
		return context.Background()
	}
	return tb.stopped
}

func (tb *recordingTB) Fatalf(format string, args ...any) {
	tb.fatal = fmt.Sprintf(format, args...)
	runtime.Goexit()
}

// A custody barrier wait fails as soon as the command that would create the
// barrier has ended; without that it waited for the test's own deadline.
func TestCustodyBarrierWaitFailsWhenTheCommandEnds(t *testing.T) {
	t.Parallel()
	stopped, cancel := context.WithCancel(context.Background())
	cancel()
	tb := &recordingTB{TB: t, live: 3, stopped: stopped}
	ended := make(chan struct{})
	close(ended)
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		waitCustodyFileWhile(tb, filepath.Join(t.TempDir(), "never.ready"), 0, ended,
			func() error { return errors.New("custodian start refused") })
	}()
	<-finished
	if !strings.Contains(tb.fatal, "command ended before custody barrier never.ready appeared: custodian start refused") {
		t.Fatalf("barrier wait after the command ended = %q", tb.fatal)
	}
}

// A pid barrier is complete only when its line is: the writer creates the
// file before it writes the pid, so an existing empty file is still waited
// for. The command has ended here, so an incomplete record fails.
func TestCustodyPidBarrierWaitsForTheCompleteLine(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "worker.pid")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	stopped, cancel := context.WithCancel(context.Background())
	cancel()
	tb := &recordingTB{TB: t, live: 3, stopped: stopped}
	ended := make(chan struct{})
	close(ended)
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		waitCustodyBarrier(tb, path, 0, ended, func() error { return errors.New("worker exited") },
			func() bool { return completeCustodyRecord(path) })
	}()
	<-finished
	if !strings.Contains(tb.fatal, "command ended before custody barrier worker.pid appeared") {
		t.Fatalf("empty pid record = %q", tb.fatal)
	}
	if err := os.WriteFile(path, []byte("4242\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if !completeCustodyRecord(path) {
		t.Fatal("a complete pid line was not accepted")
	}
}
