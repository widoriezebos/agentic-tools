package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/applaunch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot/stateroottest"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/lifecycle"
)

// A goal run's evidence lands under the owner's default root when the
// installation configures none; a relative root keeps the record, prefixed
// by the owner's sentence.
func TestPreserveRunEvidenceUsesTheResolvedRoot(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	checkout := filepath.Join(t.TempDir(), "app-seat")
	installation := filepath.Join(checkout, "metasystem")
	if err := os.MkdirAll(installation, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(checkout, ".git"), []byte("gitdir: /elsewhere\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	conf := filepath.Join(installation, "metasystem.conf")
	if err := os.WriteFile(conf, []byte("metasystem.runtimes=fake\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run := appRun{roots: lifecycle.Roots{Checkout: checkout, Installation: stateroottest.Installation(t, installation)}, key: "k1", lookupEnv: homeOnly(home)}
	record := &applaunch.Record{Key: "k1", Goal: "g1"}
	if err := run.preserveRunEvidence(record, io.Discard); err != nil {
		t.Fatal(err)
	}
	runs, err := filepath.Glob(filepath.Join(home, "metasystem-evidence", "app-seat", "goals", "g1", "app", "k1", "*", "*-run.txt"))
	if err != nil || len(runs) != 1 {
		t.Fatalf("run.txt under the default root: %v, %v", runs, err)
	}
	if err := os.WriteFile(conf, []byte("evidence.root=relative\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	err = run.preserveRunEvidence(record, io.Discard)
	if err == nil || !strings.Contains(err.Error(), `evidence.root must be absolute (metasystem.conf reads "relative")`) || !strings.Contains(err.Error(), "its run stays open") {
		t.Fatalf("relative root: got %v", err)
	}
}
