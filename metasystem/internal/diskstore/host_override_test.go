package diskstore

import (
	"os"
	"path/filepath"
	"testing"
)

// Round D3 N3: the host temporary root's override is a test binary's
// alone; a production engine resolves the host's own root whatever the
// environment says.
func TestTheHostTempRootOverrideIsHonouredOnlyInATestBinary(t *testing.T) {
	t.Parallel()
	override := os.Getenv(HostTempRootEnv)
	if override == "" {
		t.Fatal("testenv did not pin the override; the witness proves nothing")
	}
	if !hostTempOverrideAllowed() {
		t.Fatal("a test binary does not honour the override")
	}
	if root, err := resolveHostTempRoot(true); err != nil || root != override {
		t.Fatalf("a test binary's root = %q, %v; want %q", root, err, override)
	}
	system, err := readHostTempRoot()
	if err != nil {
		t.Fatal(err)
	}
	if root, err := resolveHostTempRoot(false); err != nil || root == override || root != filepath.Clean(system) {
		t.Fatalf("a production engine's root = %q, %v; want the host's own %q", root, err, system)
	}
}
