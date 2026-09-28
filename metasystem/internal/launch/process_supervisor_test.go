package launch

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

// TestSupervisorStartsAsItsOwnSessionLeaderWithoutAnEngineHop is the witness
// that the launch supervisor is started directly as the leader of a new
// session: the program runs `launch supervise --id ID` itself, with no
// `proc setsid` engine child in between (verbs-object-action U9a), and its
// process group is its own pid, so the launch outlives the request's terminal
// session.
func TestSupervisorStartsAsItsOwnSessionLeaderWithoutAnEngineHop(t *testing.T) {
	dir := t.TempDir()
	// The stand-in engine writes the words it was started with into a FIFO
	// the test reads (the read waits for the writer, no clock), then holds
	// until the test stops its session.
	words := filepath.Join(dir, "words")
	if err := syscall.Mkfifo(words, 0o600); err != nil {
		t.Fatal(err)
	}
	engine := filepath.Join(dir, "engine")
	script := "#!/bin/sh\nprintf '%s\\n' \"$*\" > \"" + words + "\"\nexec sleep 600\n"
	if err := testexec.WriteFile(engine, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	ref, err := OSSupervisorStarter{Executable: engine}.StartSupervisor("run-1", dir)
	if err != nil {
		t.Fatal(err)
	}
	pid := int(ref.Pid)
	t.Cleanup(func() { _ = syscall.Kill(-pid, syscall.SIGKILL); _ = syscall.Kill(pid, syscall.SIGKILL) })
	argv, err := os.ReadFile(words)
	if err != nil {
		t.Fatalf("the started program never recorded its words: %v", err)
	}
	if got, want := strings.TrimSpace(string(argv)), "launch supervise --id run-1"; got != want {
		t.Fatalf("the supervisor ran %q, want %q (no engine hop)", got, want)
	}
	pgid, err := syscall.Getpgid(pid)
	if err != nil {
		t.Fatal(err)
	}
	if pgid != pid {
		t.Fatalf("the supervisor's process group is %d, want its own pid %d (a new session's leader)", pgid, pid)
	}
}
