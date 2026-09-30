package proofrun

import (
	"os"
	"path/filepath"
	"testing"
)

// A fixture's admission directory is checked against the host's temporary
// roots, never the checking process's TMPDIR (Part B U1b-2, DL2-15): an
// engine child started with a process-scratch TMPDIR accepts the directory
// its parent selected, and a path outside every host temporary root is
// still refused.
func TestAdmissionTestDirectoryIsJudgedAgainstTheHostTempRoot(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.runtimes=fake\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	selected := filepath.Join(root, "admission")
	scratch := filepath.Join(t.TempDir(), "metasystem", "01K6CHILDSCRATCH0000000000")
	if err := os.MkdirAll(scratch, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TMPDIR", scratch)
	got, err := hostAdmissionDirectoryForRequest(root, selected)
	if err != nil || got != selected {
		t.Fatalf("a child with a process-scratch TMPDIR judged its parent's admission directory %s: %q, %v", selected, got, err)
	}
	if _, err := hostAdmissionDirectoryForRequest(root, filepath.Join(string(filepath.Separator), "outside-proof-admission")); err == nil {
		t.Fatal("a path outside every host temporary root was accepted")
	}
}
