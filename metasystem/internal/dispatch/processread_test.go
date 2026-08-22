package dispatch

import (
	"os"
	"strings"
	"testing"
)

// The process-command read behind the mission-lease identity proof.
// These tests hold for a `ps` subprocess and for the identity owner's
// native read alike, so the contract is what is verified, not the
// mechanism.

func TestProcessCommandReadsOwnCommandLine(t *testing.T) {
	command, err := processCommand(int64(os.Getpid()))
	if err != nil {
		t.Fatalf("reading our own command line failed: %v", err)
	}
	if strings.TrimSpace(command) == "" {
		t.Fatal("our own command line came back empty")
	}
	// The test binary's own path is in argv[0] under every runner.
	if !strings.Contains(command, ".test") && !strings.Contains(command, "dispatch") {
		t.Fatalf("command line does not look like this test process: %q", command)
	}
}

// A pid that cannot exist is a read failure, which is what sends the caller
// to its fixture fallback — never a silent empty string that would fail a
// tag check as though the process were a stranger.
func TestProcessCommandImpossiblePidFails(t *testing.T) {
	command, err := processCommand(1 << 30)
	if err == nil {
		t.Fatalf("impossible pid returned a command line: %q", command)
	}
}
