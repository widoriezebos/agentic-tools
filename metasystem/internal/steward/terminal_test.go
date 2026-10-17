package steward

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

func TestPersonTerminalRecordsAreNotSupplementaryWorkers(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	pid := int64(os.Getpid())
	exact, state, err := (identity.KernelProber{}).Probe(pid)
	if err != nil || state != identity.Alive {
		t.Fatalf("test process is not kernel-live: %v %v", state, err)
	}
	dir := filepath.Join(root, "artifacts/agents/terminals")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	record := fmt.Sprintf(`{"kind":"person-terminal","pid":%d,"pidStartedAt":%d,"callerPid":%d,"callerPidStartedAt":%d}`, pid, exact.StartedAt.Unix(), pid, exact.StartedAt.Unix())
	if err := os.WriteFile(filepath.Join(dir, "live.json"), []byte(record), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "malformed.json"), []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	live, mains, unknown := supplementWorkers(root, identity.KernelProber{})
	if live != 0 || mains != 0 || unknown != 0 {
		t.Fatalf("terminals entered the worker universe: live=%d mains=%d unprovable=%d", live, mains, unknown)
	}
}
