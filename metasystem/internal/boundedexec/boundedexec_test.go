package boundedexec

import (
	"bufio"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestTimeoutResolution(t *testing.T) {
	dir := t.TempDir()
	conf := filepath.Join(dir, "metasystem.conf")
	// Absent file: the stated defaults.
	if got := Timeout(conf, Local); got.Limit != 300*time.Second || got.Key != "exec.local-timeout-sec" {
		t.Fatalf("local default: %v", got)
	}
	if got := Timeout(conf, Network); got.Limit != 120*time.Second || got.Key != "exec.network-timeout-sec" {
		t.Fatalf("network default: %v", got)
	}
	os.WriteFile(conf, []byte("exec.local-timeout-sec=42\nexec.network-timeout-sec=7\n"), 0o644)
	if got := Timeout(conf, Local); got.Limit != 42*time.Second {
		t.Fatalf("configured local: %v", got)
	}
	if got := Timeout(conf, Network); got.Limit != 7*time.Second {
		t.Fatalf("configured network: %v", got)
	}
	// A malformed or non-positive bound must not disable bounding.
	os.WriteFile(conf, []byte("exec.local-timeout-sec=nonsense\n"), 0o644)
	if got := Timeout(conf, Local); got.Limit != 300*time.Second {
		t.Fatalf("malformed bound did not fall back: %v", got)
	}
	os.WriteFile(conf, []byte("exec.local-timeout-sec=0\n"), 0o644)
	if got := Timeout(conf, Local); got.Limit != 300*time.Second {
		t.Fatalf("zero bound did not fall back: %v", got)
	}
}

func TestRunReturnsPromptlyOnSuccess(t *testing.T) {
	if err := Run(exec.Command("true"), FixedBound(5*time.Second, "exec.local-timeout-sec"), "the true command"); err != nil {
		t.Fatalf("a fast command failed: %v", err)
	}
}

func TestRunPropagatesCommandFailure(t *testing.T) {
	err := Run(exec.Command("false"), FixedBound(5*time.Second, "exec.local-timeout-sec"), "the false command")
	if err == nil {
		t.Fatal("a failing command reported success")
	}
	if strings.Contains(err.Error(), "timed out") {
		t.Fatalf("a failure was misreported as a timeout: %v", err)
	}
}

// The hang test (B4's proof): a command that never returns is killed at the
// bound and named, instead of hanging its caller forever.
func TestRunKillsAHangingCommand(t *testing.T) {
	bound := FixedBound(300*time.Millisecond, "exec.local-timeout-sec")
	expiry := make(chan time.Time, 1)
	expiry <- time.Time{}
	var waits []time.Duration
	command := exec.Command("sleep", "60")
	err := runWithDeadline(command, bound, "the sleeping command", func(wait time.Duration) <-chan time.Time {
		waits = append(waits, wait)
		return expiry
	})
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("a hanging command was not bounded: %v", err)
	}
	if !strings.Contains(err.Error(), "the sleeping command") {
		t.Fatalf("the failure does not name the operation: %v", err)
	}
	if command.ProcessState == nil || command.ProcessState.Success() {
		t.Fatalf("the expired command was not reaped after being killed: %v", command.ProcessState)
	}
	assertDeadlineWaits(t, waits, bound.Limit)
}

// A script's children must die with it: the group is signalled, not just the
// direct child.
func TestRunKillsTheWholeProcessGroup(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "spawn.sh")
	if err := os.WriteFile(script, []byte(
		"#!/bin/sh\nsleep 60 &\necho ready\nwait\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	readPipe, writePipe, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = readPipe.Close()
		_ = writePipe.Close()
	})
	deadline := make(chan time.Time)
	command := exec.Command("/bin/sh", script)
	command.Stdout = writePipe
	done := make(chan error, 1)
	var waits []time.Duration
	go func() {
		defer writePipe.Close()
		done <- runWithDeadline(command, FixedBound(300*time.Millisecond, "exec.local-timeout-sec"), "the spawning script", func(duration time.Duration) <-chan time.Time {
			waits = append(waits, duration)
			return deadline
		})
	}()
	reader := bufio.NewReader(readPipe)
	ready, err := reader.ReadString('\n')
	if err != nil || ready != "ready\n" {
		t.Fatalf("the spawning script did not report its child ready: line %q, error %v", ready, err)
	}
	if err := writePipe.Close(); err != nil {
		t.Fatalf("release parent pipe writer: %v", err)
	}
	deadline <- time.Time{}
	remaining, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read process-group completion signal: %v", err)
	}
	if len(remaining) != 0 {
		t.Fatalf("unexpected process-group output after ready: %q", remaining)
	}
	err = <-done
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("the spawning script was not bounded: %v", err)
	}
	assertDeadlineWaits(t, waits, 300*time.Millisecond)
}

// Callers for whom a timeout is an ANSWER (a ceiling verdict, not a failure
// to run) rely on the sentinel surviving the wrap.
func TestRunTimeoutMatchesTheSentinel(t *testing.T) {
	bound := FixedBound(300*time.Millisecond, "exec.local-timeout-sec")
	expiry := make(chan time.Time, 1)
	expiry <- time.Time{}
	var waits []time.Duration
	err := runWithDeadline(exec.Command("sleep", "60"), bound, "the sleeping command", func(wait time.Duration) <-chan time.Time {
		waits = append(waits, wait)
		return expiry
	})
	if !errors.Is(err, ErrTimedOut) {
		t.Fatalf("expiry must match ErrTimedOut: %v", err)
	}
	assertDeadlineWaits(t, waits, bound.Limit)
	exit := exec.Command("false")
	if failure := Run(exit, FixedBound(time.Minute, "exec.local-timeout-sec"), "false"); errors.Is(failure, ErrTimedOut) {
		t.Fatalf("a plain failure must not match ErrTimedOut: %v", failure)
	}
}

// foundations-5: expiry names the key that actually produced the bound —
// never a magnitude-based guess that misdirects once a bound is tuned.
func TestTimeoutErrorNamesItsOwnKey(t *testing.T) {
	dir := t.TempDir()
	conf := filepath.Join(dir, "metasystem.conf")
	// A LOCAL bound tuned down to 1s sits below the network default — the
	// old guess would have named the network key.
	if err := os.WriteFile(conf, []byte("exec.local-timeout-sec=1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	bound := Timeout(conf, Local)
	if bound.Key != "exec.local-timeout-sec" || bound.Limit != time.Second {
		t.Fatalf("bound = %+v", bound)
	}
	expiry := make(chan time.Time, 1)
	expiry <- time.Time{}
	var waits []time.Duration
	err := runWithDeadline(exec.Command("sleep", "60"), bound, "the tuned command", func(wait time.Duration) <-chan time.Time {
		waits = append(waits, wait)
		return expiry
	})
	if err == nil || !strings.Contains(err.Error(), "exec.local-timeout-sec") {
		t.Fatalf("expiry must name the key that produced the bound: %v", err)
	}
	if strings.Contains(err.Error(), "network") {
		t.Fatalf("the old magnitude guess resurfaced: %v", err)
	}
	assertDeadlineWaits(t, waits, bound.Limit)
}

func assertDeadlineWaits(t *testing.T, got []time.Duration, bound time.Duration) {
	t.Helper()
	if len(got) != 2 || got[0] != bound || got[1] != killGraceWindow {
		t.Fatalf("deadline waits = %v, want [%s %s]", got, bound, killGraceWindow)
	}
}
