package atomicfile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteTextReplacesContent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "file.txt")
	if err := WriteText(path, "first\n"); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(path); string(data) != "first\n" {
		t.Fatalf("got %q", data)
	}
	if err := WriteText(path, "second\n"); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(path); string(data) != "second\n" {
		t.Fatalf("replacement got %q", data)
	}
}

// A failed write must leave the original intact and drop no temp files.
func TestWriteTextLeavesNoResidue(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.txt")
	WriteText(path, "original\n")
	// A directory where the file should be makes the rename fail.
	blocked := filepath.Join(dir, "blocked")
	os.MkdirAll(blocked, 0o755)
	if err := WriteText(blocked, "x"); err == nil {
		t.Fatal("writing over a directory must fail")
	}
	entries, _ := os.ReadDir(dir)
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) == ".tmp" {
			t.Fatalf("temp residue left behind: %s", entry.Name())
		}
	}
	if data, _ := os.ReadFile(path); string(data) != "original\n" {
		t.Fatalf("original disturbed: %q", data)
	}
}
