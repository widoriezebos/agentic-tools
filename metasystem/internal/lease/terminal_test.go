package lease

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

type terminalReader struct {
	snapshot humanauthority.Snapshot
}

func (r terminalReader) Read(int64) (humanauthority.Snapshot, error) { return r.snapshot, nil }
func (r terminalReader) SessionLeader(pid int64) (int64, error)      { return pid, nil }

func TestPersonTerminalIdentityPairs(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name        string
		left, right humanauthority.ProcessRef
		matches     bool
	}{
		{"legacy", humanauthority.ProcessRef{PID: 42, PIDStartedAt: 100}, humanauthority.ProcessRef{PID: 42, PIDStartedAt: 100}, true},
		{"other-pid", humanauthority.ProcessRef{PID: 42, PIDStartedAt: 100}, humanauthority.ProcessRef{PID: 43, PIDStartedAt: 100}, false},
		{"other-start", humanauthority.ProcessRef{PID: 42, PIDStartedAt: 100}, humanauthority.ProcessRef{PID: 42, PIDStartedAt: 101}, false},
		{"ticks-prefer-pair", humanauthority.ProcessRef{PID: 42, PIDStartedAt: 100, StartTicks: 5, BootID: "a"}, humanauthority.ProcessRef{PID: 42, PIDStartedAt: 101, StartTicks: 5, BootID: "a"}, true},
		{"ticks-differ", humanauthority.ProcessRef{PID: 42, PIDStartedAt: 100, StartTicks: 5, BootID: "a"}, humanauthority.ProcessRef{PID: 42, PIDStartedAt: 100, StartTicks: 6, BootID: "a"}, false},
		{"boot-differs", humanauthority.ProcessRef{PID: 42, PIDStartedAt: 100, StartTicks: 5, BootID: "a"}, humanauthority.ProcessRef{PID: 42, PIDStartedAt: 100, StartTicks: 5, BootID: "b"}, false},
		{"one-legacy", humanauthority.ProcessRef{PID: 42, PIDStartedAt: 100, StartTicks: 5, BootID: "a"}, humanauthority.ProcessRef{PID: 42, PIDStartedAt: 100}, true},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			if got := terminalRefMatches(row.left, row.right); got != row.matches {
				t.Fatalf("identity match = %v, want %v", got, row.matches)
			}
		})
	}
}

func TestPersonTerminalBindsTheResolvedPair(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.runtimes=fake\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	reader := terminalReader{snapshot: humanauthority.Snapshot{
		Exact:      identity.Exact{Pid: 42, StartedAt: time.Unix(100, 0), StartTicks: 5, BootID: "a", Argv: []string{"/bin/zsh"}, ArgvKnown: true},
		Executable: "/bin/zsh", ExecutableKnown: true, OwnerUID: 1000, OwnerKnown: true,
		ParentKnown: true, TerminalID: "tty-1", TerminalKnown: true,
	}}
	for _, row := range []struct {
		name string
		ref  humanauthority.ProcessRef
		want string
	}{
		{"same", humanauthority.ProcessRef{PID: 42, PIDStartedAt: 100, StartTicks: 5, BootID: "a"}, PersonTerminal},
		{"changed-second", humanauthority.ProcessRef{PID: 42, PIDStartedAt: 101, StartTicks: 5, BootID: "a"}, TerminalUnknown},
		{"changed-ticks", humanauthority.ProcessRef{PID: 42, PIDStartedAt: 100, StartTicks: 6, BootID: "a"}, TerminalUnknown},
		{"changed-boot", humanauthority.ProcessRef{PID: 42, PIDStartedAt: 100, StartTicks: 5, BootID: "b"}, TerminalUnknown},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			if got := PersonTerminalAt(root, root, row.ref, reader); got != row.want {
				t.Fatalf("predicate = %s, want %s", got, row.want)
			}
		})
	}
}
