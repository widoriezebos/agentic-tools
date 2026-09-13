package main

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/output"
)

func TestOutputSpillVerbPrintsTheTextLine(t *testing.T) {
	root := t.TempDir()
	data := []byte("complete landing output\n")
	input := filepath.Join(t.TempDir(), "step.log")
	if err := os.WriteFile(input, data, 0o600); err != nil {
		t.Fatal(err)
	}
	stdout, code := captureStdout(t, func() int {
		return runOutputSpill([]string{"--root", root, "--verb", "land", "--ext", "log", "--file", input})
	})
	if code != 0 {
		t.Fatalf("output spill exit = %d", code)
	}
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

func TestOutputPruneVerbReportsEachRemoval(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, filepath.FromSlash(output.Dir))
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	files := []string{filepath.Join(dir, "b.log"), filepath.Join(dir, "a.log")}
	old := time.Now().Add(-8 * 24 * time.Hour)
	for _, path := range files {
		if err := os.WriteFile(path, []byte(path), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(path, old, old); err != nil {
			t.Fatal(err)
		}
	}
	stdout, code := captureStdout(t, func() int {
		return runOutputPrune([]string{"--root", root, "--older-than", "7d"})
	})
	if code != 0 {
		t.Fatalf("output prune exit = %d", code)
	}
	want := "pruned " + filepath.Join(dir, "a.log") + "\npruned " + filepath.Join(dir, "b.log") + "\n"
	if stdout != want {
		t.Fatalf("output prune stdout = %q, want %q", stdout, want)
	}
	for _, path := range files {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("pruned file %s still exists: %v", path, err)
		}
	}
	if strings.Count(stdout, "pruned ") != 2 {
		t.Fatalf("output prune did not report each removal: %q", stdout)
	}

	fresh := filepath.Join(dir, "fresh.log")
	if err := os.WriteFile(fresh, []byte("fresh\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	stderr, code := captureStderr(t, func() int {
		return runOutputPrune([]string{"--root", root, "--older-than", "-1h"})
	})
	if code != 2 || !strings.Contains(stderr, "must not be negative") {
		t.Fatalf("negative retention window: exit=%d stderr=%q", code, stderr)
	}
	if got, err := os.ReadFile(fresh); err != nil || string(got) != "fresh\n" {
		t.Fatalf("negative retention window changed a fresh file: bytes=%q err=%v", got, err)
	}
}
