package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAuditDependencyRatchetRelayReportsPathLineAndExit(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "scripts", "fixture.sh")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("node -v\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, code := captureRelay(t, func() int {
		return runAuditDependencyRatchet([]string{"--root", root})
	})
	if code != 1 || stdout != "" || !strings.Contains(stderr, "scripts/fixture.sh:1") {
		t.Fatalf("dependency audit relay = code %d, stdout %q, stderr %q", code, stdout, stderr)
	}
	if _, _, code = captureRelay(t, func() int {
		return runAuditDependencyRatchet([]string{"--root", root, "extra"})
	}); code != 2 {
		t.Fatalf("dependency audit relay accepted an extra argument with code %d", code)
	}
	if err := os.WriteFile(path, []byte("printf '%s' node\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, code = captureRelay(t, func() int {
		return runAuditDependencyRatchet([]string{"--root", root})
	})
	if code != 0 || stdout != "dependency ratchet passed\n" || stderr != "" {
		t.Fatalf("clean dependency audit relay = code %d, stdout %q, stderr %q", code, stdout, stderr)
	}
}
