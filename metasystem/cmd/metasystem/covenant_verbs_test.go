package main

// The covenant family's one verb: a thin structural check whose
// success line must carry the honesty distinction — shape validity is
// not adequacy, and the interview repeats that to the human.

import (
	"os"
	"path/filepath"
	"testing"
)

// TestSystemCheckReadsTheCovenantShape: system check reports the shape of
// the app covenant at its one home (the internal covenant validate it
// replaced read the same file through the same owner): a valid covenant
// names its path and no problem, an absent one reports nothing, and a broken
// one names its path and the parse problem.
func TestSystemCheckReadsTheCovenantShape(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	source, err := os.ReadFile(filepath.Join("..", "..", "internal", "covenant", "testdata", "taskrun-covenant.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "covenant.json"), source, 0o644); err != nil {
		t.Fatal(err)
	}
	if path, problem := checkCovenantShape(processScope{Installation: root}); path != filepath.Join(root, "covenant.json") || problem != nil {
		t.Fatalf("the kit-extracted covenant must validate at the one home: path=%q problem=%v", path, problem)
	}
	if path, problem := checkCovenantShape(processScope{Installation: t.TempDir()}); path != "" || problem != nil {
		t.Fatalf("an absent covenant must report nothing: path=%q problem=%v", path, problem)
	}
	brokenRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(brokenRoot, "covenant.json"), []byte(`{"schemaVersion": 2}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if path, problem := checkCovenantShape(processScope{Installation: brokenRoot}); path == "" || problem == nil {
		t.Fatalf("a broken covenant must name its path and its problem: path=%q problem=%v", path, problem)
	}
}
