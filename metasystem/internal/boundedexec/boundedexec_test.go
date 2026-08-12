package boundedexec

import (
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
	if got := Timeout(conf, Local); got != 300*time.Second {
		t.Fatalf("local default: %v", got)
	}
	if got := Timeout(conf, Network); got != 120*time.Second {
		t.Fatalf("network default: %v", got)
	}
	os.WriteFile(conf, []byte("exec.local-timeout-sec=42\nexec.network-timeout-sec=7\n"), 0o644)
	if got := Timeout(conf, Local); got != 42*time.Second {
		t.Fatalf("configured local: %v", got)
	}
	if got := Timeout(conf, Network); got != 7*time.Second {
		t.Fatalf("configured network: %v", got)
	}
	// A malformed or non-positive bound must not disable bounding.
	os.WriteFile(conf, []byte("exec.local-timeout-sec=nonsense\n"), 0o644)
	if got := Timeout(conf, Local); got != 300*time.Second {
		t.Fatalf("malformed bound did not fall back: %v", got)
	}
	os.WriteFile(conf, []byte("exec.local-timeout-sec=0\n"), 0o644)
	if got := Timeout(conf, Local); got != 300*time.Second {
		t.Fatalf("zero bound did not fall back: %v", got)
	}
}

func TestRunReturnsPromptlyOnSuccess(t *testing.T) {
	if err := Run(exec.Command("true"), 5*time.Second, "the true command"); err != nil {
		t.Fatalf("a fast command failed: %v", err)
	}
}

func TestRunPropagatesCommandFailure(t *testing.T) {
	err := Run(exec.Command("false"), 5*time.Second, "the false command")
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
	err := Run(exec.Command("sleep", "60"), 300*time.Millisecond, "the sleeping command")
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

// A script's CHILDREN must die with it: the group is signalled, not just the
// direct child. The grandchild writes to a file after a delay; if the group
// kill worked, that file never appears.
func TestRunKillsTheWholeProcessGroup(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "grandchild-survived")
	script := filepath.Join(dir, "spawn.sh")
	os.WriteFile(script, []byte(
		"#!/bin/sh\n( sleep 3; echo alive > "+marker+" ) &\nsleep 60\n"), 0o755)
	err := Run(exec.Command("/bin/sh", script), 300*time.Millisecond, "the spawning script")
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("the spawning script was not bounded: %v", err)
	}
	time.Sleep(4 * time.Second)
	if _, statErr := os.Stat(marker); statErr == nil {
		t.Fatal("a grandchild outlived the bound: the group was not killed")
	}
}
