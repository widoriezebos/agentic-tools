package acp

import (
	"os"
	"path/filepath"
	"testing"
)

// A symlinked root and a real path must land on the same canonical
// spelling, or containment is string luck.
func TestCanonicalizeSymlinks(t *testing.T) {
	dir := t.TempDir()
	real := filepath.Join(dir, "real")
	if err := os.Mkdir(real, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	viaLink, err := Canonicalize(filepath.Join(link, "file.txt"))
	if err != nil {
		t.Fatal(err)
	}
	base, err := Canonicalize(real)
	if err != nil {
		t.Fatal(err)
	}
	if !allInside([]string{viaLink}, []string{base}) {
		t.Fatalf("symlink resolution failed: %s not inside %s", viaLink, base)
	}
}
