package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func homeOnly(home string) func(string) (string, bool) {
	return func(name string) (string, bool) {
		if name == "HOME" {
			return home, true
		}
		return "", false
	}
}

// evidence-gc with no configured root collects against the owner's default;
// a relative root refuses in the owner's sentence; an explicit root wins.
func TestEvidenceGCTargetResolvesThroughTheOwner(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	checkout := filepath.Join(t.TempDir(), "gc-seat")
	if err := os.MkdirAll(checkout, 0o755); err != nil {
		t.Fatal(err)
	}
	conf := filepath.Join(checkout, "metasystem.conf")
	if err := os.WriteFile(conf, []byte("metasystem.runtimes=fake\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	target, err := evidenceGCTarget(checkout, "", homeOnly(home))
	if err != nil || target != filepath.Join(home, "metasystem-evidence", "gc-seat") {
		t.Fatalf("default: got %q, %v", target, err)
	}
	if target, err := evidenceGCTarget(checkout, "/explicit", homeOnly(home)); err != nil || target != "/explicit" {
		t.Fatalf("explicit: got %q, %v", target, err)
	}
	if err := os.WriteFile(conf, []byte("evidence.root=relative\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := evidenceGCTarget(checkout, "", homeOnly(home)); err == nil ||
		!strings.HasPrefix(err.Error(), "evidence-gc refused: ") || !strings.Contains(err.Error(), `must be absolute (metasystem.conf reads "relative")`) {
		t.Fatalf("relative: got %v", err)
	}
}
