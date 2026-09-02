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
	announcement := liveProcessRecord(t)
	announcement["mainId"] = "main-fixture"
	writeStewardRecord(t, filepath.Join(root, "artifacts", "agents", "mains", "fixture.json"), announcement)
	w, reason, err := LegacyOpenWork(root)
	if err != nil || w != WorkInFlight {
		t.Fatalf("a legacy current goal counts only with a live process: %v %q %v", w, reason, err)
	}
}

func TestMissingLedgerIsDegradedNeverNoWork(t *testing.T) {
	w, reason, err := LegacyOpenWork(t.TempDir())
	if err != nil || w != WorkDegraded {
		t.Fatalf("absence proves nothing — a half-deleted checkout must not read as no-work: %v %q %v", w, reason, err)
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

func TestQueuedWithoutClaimIsActionableBacklog(t *testing.T) {
	root := t.TempDir()
	writeLedger(t, root, "# Goals\n\n## Queued goal: later-thing \u2014 Someday\n- Origin: main\n- Next step: Start it.\n")
	w, reason, err := LegacyOpenWork(root)
	if err != nil || w != WorkClaimable {
		t.Fatalf("unclaimed legacy backlog must be action-worthy: %v %q %v", w, reason, err)
	}
}
