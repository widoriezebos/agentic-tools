package main

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/output"
)

func TestLandingOutputSpillReturnsTheTextLine(t *testing.T) {
	root := t.TempDir()
	data := []byte("complete landing output\n")
	input := filepath.Join(t.TempDir(), "step.log")
	if err := os.WriteFile(input, data, 0o600); err != nil {
		t.Fatal(err)
	}
	line, err := landingPathSpill(root, "land", "log", data)
	if err != nil {
		t.Fatalf("output spill: %v", err)
	}
	stdout := line + "\n"
	files, err := filepath.Glob(filepath.Join(root, filepath.FromSlash(output.Dir), "land-*.log"))
	if err != nil || len(files) != 1 {
		t.Fatalf("output spill files = %v, err=%v", files, err)
	}
	digest := sha256.Sum256(data)
	want := fmt.Sprintf("output-reference verb=land path=%s bytes=%d sha256=%x format=text\n", files[0], len(data), digest)
	if stdout != want {
		t.Fatalf("output spill stdout = %q, want %q", stdout, want)
	}
	if got, err := os.ReadFile(files[0]); err != nil || string(got) != string(data) {
		t.Fatalf("retained output = %q, err=%v", got, err)
	}
}
