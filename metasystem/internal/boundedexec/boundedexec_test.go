package boundedexec

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
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
	started := time.Now()
	err := Run(exec.Command("sleep", "60"), FixedBound(300*time.Millisecond, "exec.local-timeout-sec"), "the sleeping command")
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("a hanging command was not bounded: %v", err)
	}
	if !strings.Contains(err.Error(), "the sleeping command") {
		t.Fatalf("the failure does not name the operation: %v", err)
	}
	if elapsed := time.Since(started); elapsed > 10*time.Second {
		t.Fatalf("the bound did not release the caller promptly: %v", elapsed)
	}
}

// A script's children must die with it: the group is signalled, not just the
// direct child.
func TestRunKillsTheWholeProcessGroup(t *testing.T) {
	dir := t.TempDir()
	ready := filepath.Join(dir, "grandchild-ready")
	script := filepath.Join(dir, "spawn.sh")
	os.WriteFile(script, []byte(
		"#!/bin/sh\nsleep 60 &\necho ready > "+ready+"\nwait\n"), 0o755)
	deadline := make(chan time.Time)
	command := exec.Command("/bin/sh", script)
	done := make(chan error, 1)
	go func() {
		done <- runWithDeadline(command, FixedBound(300*time.Millisecond, "exec.local-timeout-sec"), "the spawning script", func(duration time.Duration) <-chan time.Time {
			if duration == 300*time.Millisecond {
				return deadline
			}
			return time.After(duration)
		})
	}()
	readyBy := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(ready); err == nil {
			break
		}
		if time.Now().After(readyBy) {
			deadline <- time.Time{}
			<-done
			t.Fatal("the spawning script did not start its child")
		}
		time.Sleep(10 * time.Millisecond)
	}
	deadline <- time.Time{}
	err := <-done
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("the spawning script was not bounded: %v", err)
	}
	groupGoneBy := time.Now().Add(5 * time.Second)
	for {
		probeErr := syscall.Kill(-command.Process.Pid, 0)
		if errors.Is(probeErr, syscall.ESRCH) {
			break
		}
		if time.Now().After(groupGoneBy) {
			t.Fatalf("a grandchild outlived the bound: group probe: %v", probeErr)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// Callers for whom a timeout is an ANSWER (a ceiling verdict, not a failure
// to run) rely on the sentinel surviving the wrap.
func TestRunTimeoutMatchesTheSentinel(t *testing.T) {
	err := Run(exec.Command("sleep", "60"), FixedBound(300*time.Millisecond, "exec.local-timeout-sec"), "the sleeping command")
	if !errors.Is(err, ErrTimedOut) {
		t.Fatalf("expiry must match ErrTimedOut: %v", err)
	}
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
	err := Run(exec.Command("sleep", "60"), bound, "the tuned command")
	if err == nil || !strings.Contains(err.Error(), "exec.local-timeout-sec") {
		t.Fatalf("expiry must name the key that produced the bound: %v", err)
	}
	if strings.Contains(err.Error(), "network") {
		t.Fatalf("the old magnitude guess resurfaced: %v", err)
	}
}
