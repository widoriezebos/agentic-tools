package steward

import (
	"os"
	"path/filepath"
	"testing"
)

func writeLedger(t *testing.T, root, content string) {
	t.Helper()
	dir := filepath.Join(root, "plans")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "goals.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCurrentGoalIsOwnedWork(t *testing.T) {
	root := t.TempDir()
	writeLedger(t, root, "# Goals\n\n## Current goal: fix-the-thing \u2014 Repair it\n- Origin: main\n- Next step: Do the repair.\n")
	w, reason, err := LegacyOpenWork(root)
	if err != nil || w != WorkOwned {
		t.Fatalf("a current goal is owned work: %v %q %v", w, reason, err)
	}
}

func TestMissingLedgerIsNoWork(t *testing.T) {
	w, _, err := LegacyOpenWork(t.TempDir())
	if err != nil || w != WorkNone {
		t.Fatalf("no ledger means nothing delegated: %v %v", w, err)
	}
}

func TestUnparseableLedgerIsDegradedNeverNoWork(t *testing.T) {
	root := t.TempDir()
	writeLedger(t, root, "# Goals\n\n## Current goal: broken \u2014 X\n- Unknown field: boom\n")
	w, reason, _ := LegacyOpenWork(root)
	if w != WorkDegraded {
		t.Fatalf("parse problems must degrade, not clear: %v %q", w, reason)
	}
}

func TestQueuedWithoutClaimIsVisibleNotRevivable(t *testing.T) {
	root := t.TempDir()
	writeLedger(t, root, "# Goals\n\n## Queued goal: later-thing \u2014 Someday\n- Origin: main\n- Next step: Start it.\n")
	w, reason, err := LegacyOpenWork(root)
	if err != nil || w != WorkNone {
		t.Fatalf("unowned queued work has no worker to revive: %v %q %v", w, reason, err)
	}
}
