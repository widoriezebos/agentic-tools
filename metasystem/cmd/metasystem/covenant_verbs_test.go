package main

// The covenant family's one verb: a thin structural check whose
// success line must carry the honesty distinction — shape validity is
// not adequacy, and the interview repeats that to the human.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCovenantValidateVerb(t *testing.T) {
	root := t.TempDir()
	source, err := os.ReadFile(filepath.Join("..", "..", "internal", "covenant", "testdata", "taskrun-covenant.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "covenant.json"), source, 0o644); err != nil {
		t.Fatal(err)
	}
	out, code := captureStdout(t, func() int {
		return runCovenantValidate([]string{"--root", root})
	})
	if code != 0 {
		t.Fatalf("the kit-extracted covenant must validate at the one home: code=%d out=%q", code, out)
	}
	if !strings.Contains(out, "covenant shape valid") ||
		!strings.Contains(out, "adequacy not established") {
		t.Fatalf("the success line must carry the honesty distinction: %q", out)
	}

	// A stray positional argument is usage, never a silent success.
	_, code = captureStdout(t, func() int {
		return runCovenantValidate([]string{"--root", root, "garbage"})
	})
	if code != 2 {
		t.Fatalf("a positional argument must refuse with usage exit 2: code=%d", code)
	}

	// A missing covenant at the one home refuses.
	_, code = captureStdout(t, func() int {
		return runCovenantValidate([]string{"--root", t.TempDir()})
	})
	if code != 1 {
		t.Fatalf("a missing covenant must refuse with exit 1: code=%d", code)
	}

	// A broken covenant refuses.
	brokenRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(brokenRoot, "covenant.json"), []byte(`{"schemaVersion": 2}`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, code = captureStdout(t, func() int {
		return runCovenantValidate([]string{"--root", brokenRoot})
	})
	if code != 1 {
		t.Fatalf("a broken covenant must refuse with exit 1: code=%d", code)
	}
}
