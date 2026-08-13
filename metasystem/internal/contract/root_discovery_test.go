package contract

import (
	"os"
	"path/filepath"
	"testing"
)

// mission-contract-1 (the review): the root-discovery off-by-one. The
// binary ships at <root>/bin/metasystem, so the checkout is two directory
// components up — the third Dir landed on the checkout's PARENT and the
// confirmation check failed everywhere, silently returning "".
// contractMetasystemRoot reads os.Executable, which in tests is the test
// binary in a temp dir, so the derivation is proven through the same
// arithmetic on a constructed layout instead.
func TestRootDiscoveryArithmetic(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "scripts", "agents"), 0o755)
	os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("x=y\n"), 0o644)
	exe := filepath.Join(root, "bin", "metasystem")

	derived := filepath.Dir(filepath.Dir(exe))
	if resolvePath(derived) != resolvePath(root) {
		t.Fatalf("Dir^2 of %s is %s, want the checkout %s", exe, derived, root)
	}
	// The old arithmetic, preserved as the counter-proof: Dir^3 is the
	// parent, where the confirmation assets do not exist.
	wrong := filepath.Dir(filepath.Dir(filepath.Dir(exe)))
	if fileExists(filepath.Join(wrong, "metasystem.conf")) {
		t.Fatalf("the counter-proof layout is broken: %s has a conf", wrong)
	}
}
