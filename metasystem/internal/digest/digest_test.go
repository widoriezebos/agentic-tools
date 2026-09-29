package digest

import (
	"os"
	"path/filepath"
	"testing"
)

// TestDigestSpellsSHA256InLowercaseHex: bytes and a file of the same bytes
// have the one known digest; a missing file answers its read error.
func TestDigestSpellsSHA256InLowercaseHex(t *testing.T) {
	t.Parallel()
	const abc = "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"
	if got := SHA256([]byte("abc")); got != abc {
		t.Fatalf("SHA256(abc) = %s, want %s", got, abc)
	}
	path := filepath.Join(t.TempDir(), "abc")
	if err := os.WriteFile(path, []byte("abc"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got, err := FileSHA256(path); err != nil || got != abc {
		t.Fatalf("FileSHA256 = %s, %v; want %s", got, err, abc)
	}
	if _, err := FileSHA256(filepath.Join(t.TempDir(), "absent")); !os.IsNotExist(err) {
		t.Fatalf("FileSHA256 of a missing file = %v, want its not-exist error", err)
	}
}
