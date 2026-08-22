package mission

import (
	"os"
	"path/filepath"
	"testing"
)

// A publication in durability doubt is proven by its bytes or refused:
// matching bytes pass, a mismatch or unreadable file names the doubt.
func TestVerifyPublishedBytes(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	if err := os.WriteFile(path, []byte("committed"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := verifyPublishedBytes(path, []byte("committed")); err != nil {
		t.Fatalf("matching bytes refused: %v", err)
	}
	if err := verifyPublishedBytes(path, []byte("different")); err == nil {
		t.Fatal("differing bytes passed verification")
	}
	if err := verifyPublishedBytes(filepath.Join(dir, "absent.json"), []byte("x")); err == nil {
		t.Fatal("an unreadable publication passed verification")
	}
}
