package delegation

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
)

// The seat's session is announced, and holds the lease, at its primary
// checkout; a goal worktree never has a main of its own. A delegate
// dispatched at an unarmed goal worktree judges holder-only authority by the
// primary's lease, while an armed linked worktree keeps its own, where the
// session holds nothing.
func TestHolderOnlyAtAnUnarmedGoalWorktreeReadsThePrimarysLease(t *testing.T) {
	t.Parallel()
	b := newCensusPrimaryBed(t)
	self := int64(os.Getpid())
	started, ok := lease.StartedAt(self, nil)
	if !ok {
		t.Fatal("could not read the test process's start")
	}
	if _, err := lease.AnnounceWithPair(b.primaryInstall, "seat-main", self, started, 0, "", "tag", "fake", ""); err != nil {
		t.Fatal(err)
	}
	authorize := func() error {
		ports, err := NewOwnerPorts(OwnerConfig{Root: b.linkedInstall, Host: stubHost{}})
		if err != nil {
			t.Fatal(err)
		}
		return ports.Lease.Authorize(Invocation{CallerPid: self}, AuthorityHolderOnly, "")
	}
	if err := authorize(); err != nil {
		t.Fatalf("the seat's session was refused holder-only authority at its unarmed goal worktree: %v", err)
	}

	// Armed, the worktree runs its own system and its own lease: the
	// session announced only at the primary holds nothing there.
	supervision := filepath.Join(b.linkedInstall, "artifacts", "agents", "supervision")
	if err := os.MkdirAll(supervision, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(supervision, "state.json"), []byte(`{"generation":1}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := authorize(); err == nil {
		t.Fatal("an armed linked worktree admitted a session announced only at the primary")
	}
}
