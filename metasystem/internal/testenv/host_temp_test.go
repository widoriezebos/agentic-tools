package testenv

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The namespace pins the host temporary root the engine's shared host paths
// use to the registry home, which holds this binary's TMPDIR, so no test
// reaches the host's real temporary directory (Part B U1b-2).
func TestNamespacePinsTheHostTempRootToTheRegistryHome(t *testing.T) {
	t.Parallel()
	root := os.Getenv(hostTempRootEnv)
	if root == "" || root != os.Getenv(supervisionRegistryHome) {
		t.Fatalf("%s = %q, want the registry home %q", hostTempRootEnv, root, os.Getenv(supervisionRegistryHome))
	}
	relative, err := filepath.Rel(root, os.TempDir())
	if err != nil || strings.HasPrefix(relative, "..") {
		t.Fatalf("TMPDIR %s is not inside the pinned host temporary root %s", os.TempDir(), root)
	}
}

// MkdirTemp makes test support's directories in the namespace Main removes.
func TestMkdirTempLandsInTheNamespace(t *testing.T) {
	t.Parallel()
	dir, err := MkdirTemp("metasystem-witness-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(dir) })
	if filepath.Dir(dir) != filepath.Clean(os.TempDir()) || !strings.HasPrefix(filepath.Base(dir), "metasystem-witness-") {
		t.Fatalf("MkdirTemp made %s, want a new directory in the namespace's TMPDIR %s", dir, os.TempDir())
	}
}
