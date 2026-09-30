package steward

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// Round D3 N10: a starting session's worktree whose announcements
// directory exists but holds no main announcement (empty, or only the
// worktree lease) has announced no main: its reserved, starting state keeps
// it, never a dead verdict read from an absence.
func TestAnEmptyAnnouncementsDirectoryIsNoMain(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mains := filepath.Join(root, "artifacts", "agents", "mains")
	if err := os.MkdirAll(mains, 0o700); err != nil {
		t.Fatal(err)
	}
	liveness := sessionMainLiveness(identity.KernelProber{})
	if got, why := liveness(root); got != diskstore.MainNone {
		t.Fatalf("an empty announcements directory = %v (%s), want no main", got, why)
	}
	if err := os.WriteFile(filepath.Join(mains, "worktree-lease.json"), []byte(`{"holder":"x"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if got, why := liveness(root); got != diskstore.MainNone {
		t.Fatalf("only a worktree lease = %v (%s), want no main", got, why)
	}
}
