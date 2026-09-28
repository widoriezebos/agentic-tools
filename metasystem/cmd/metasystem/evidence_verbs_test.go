package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/seat/launch"
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

// A machine launch's preflight fact is this seat's resolved root; a resolver
// refusal is the launch's evidence-root refusal before the lock.
func TestSeatLaunchEvidenceRootResolvesThroughTheOwner(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	installation := filepath.Join(t.TempDir(), "seat")
	if err := os.MkdirAll(installation, 0o755); err != nil {
		t.Fatal(err)
	}
	conf := filepath.Join(installation, "metasystem.conf")
	if err := os.WriteFile(conf, []byte("x=1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if root, err := seatLaunchEvidenceRoot(conf, homeOnly(home)); err != nil || root != filepath.Join(home, "metasystem-evidence", "seat") {
		t.Fatalf("default: %q, %v", root, err)
	}
	if err := os.WriteFile(conf, []byte("evidence.root=relative\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := seatLaunchEvidenceRoot(conf, homeOnly(home))
	refusal, named := err.(*launch.Refusal)
	if !named || refusal.Code != launch.CodeEvidenceRootUnsafe || !strings.Contains(refusal.Message, `metasystem.conf reads "relative"`) {
		t.Fatalf("relative: %v", err)
	}
}
