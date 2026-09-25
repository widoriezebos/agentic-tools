package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

// goalAuthorityRefusalRoot supplies only the files read before these commands
// reject authority. No accepted goal tree is needed before the refusal.
func goalAuthorityRefusalRoot(t *testing.T) (string, goalAuthorityReadFacts) {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.runtimes=fake\nmetasystem.governance.correlation-policy=A\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(root, "plans", "goals", "backlog.md")
	if err := os.MkdirAll(filepath.Dir(marker), 0o755); err != nil {
		t.Fatal(err)
	}
	const ledgerID = "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	rootRecord := &goal.RootRecord{Identity: ledgerID, FormatVersion: "1", SyncMode: goal.SyncLocal, Revision: 1}
	if err := os.WriteFile(marker, goal.RenderRoot(rootRecord), 0o644); err != nil {
		t.Fatal(err)
	}
	ledgerReads := 0
	facts := goalAuthorityReadFacts{
		repositoryTop: func(got string) (string, error) {
			if got != root {
				t.Fatalf("repository top requested for %q, want %q", got, root)
			}
			return root, nil
		},
		ledgerIdentity: func(got string) string {
			if got != root {
				t.Fatalf("ledger identity requested for %q, want %q", got, root)
			}
			ledgerReads++
			return ledgerID
		},
	}
	t.Cleanup(func() {
		if ledgerReads != 1 {
			t.Errorf("ledger identity read %d times, want one eager read", ledgerReads)
		}
	})
	return root, facts
}
