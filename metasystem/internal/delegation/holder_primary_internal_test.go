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

// At an unarmed goal worktree the lease is the primary's, where a job's
// adapter supervisor whose dispatcher has gone (reparented to launchd) is
// unrecognized; its custody record, kept with the job's records in the
// worktree, names it. It may write exactly its own job's record, and never
// what needs the holder; a holder is never read at the worktree.
func TestAWorktreeJobsSupervisorIsKnownByItsCustody(t *testing.T) {
	t.Parallel()
	const supervisor, session = int64(42), int64(43)
	classify := func(root string, pid int64) (lease.ClassifyResult, error) {
		switch {
		case root == "/worktree" && pid == supervisor:
			return lease.ClassifyResult{Class: lease.ClassAdapterSupervisor, JobId: "crit-1", Pid: 7}, nil
		case root == "/worktree" && pid == session:
			return lease.ClassifyResult{Class: lease.ClassMain, Holder: true}, nil
		}
		return lease.ClassifyResult{Class: lease.ClassUntrusted}, nil
	}
	owner := ownerLease{root: "/primary", custody: "/worktree", classify: classify}
	if err := owner.Authorize(Invocation{CallerPid: supervisor}, AuthorityRecordWriter, "crit-1"); err != nil {
		t.Fatalf("the worktree job's supervisor was refused its own record: %v", err)
	}
	if err := owner.Authorize(Invocation{CallerPid: supervisor}, AuthorityRecordWriter, "crit-2"); err == nil {
		t.Fatal("the supervisor of crit-1 may write another job's record")
	}
	if err := owner.Authorize(Invocation{CallerPid: supervisor}, AuthorityHolderOnly, ""); err == nil {
		t.Fatal("custody at the worktree granted holder-only authority")
	}
	if err := owner.Authorize(Invocation{CallerPid: session}, AuthorityRecordWriter, "crit-1"); err == nil {
		t.Fatal("a holder was read at the worktree instead of the primary's lease")
	}
	if err := (ownerLease{root: "/primary", classify: classify}).Authorize(Invocation{CallerPid: supervisor}, AuthorityRecordWriter, "crit-1"); err == nil {
		t.Fatal("without a custody root the primary alone recognized the worktree's supervisor")
	}
}
