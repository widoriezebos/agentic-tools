package main

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeTemp writes a JSON file and returns its path.
func writeTemp(t *testing.T, dir, name string, value any) string {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}

// captureStdout runs fn with stdout redirected and returns what it printed.
func captureStdout(t *testing.T, fn func() int) (string, int) {
	t.Helper()
	original := os.Stdout
	read, write, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = write
	code := fn()
	write.Close()
	os.Stdout = original
	out, _ := io.ReadAll(read)
	return string(out), code
}

// TestDispatchRecordVerbsPath drives the whole record lifecycle through the CLI
// verbs the shell invokes, proving the flag parsing, exit-code mapping, and the
// lost-compare stdout witness all work end to end.
func TestDispatchRecordVerbsPath(t *testing.T) {
	root := t.TempDir()
	tmp := t.TempDir()
	job := "job-cli"

	create := writeTemp(t, tmp, "create.json", map[string]any{
		"jobId": job, "status": "pending-setup", "mainId": "main-1", "claimEpoch": 7,
	})
	if code := runDispatchRecordCreate([]string{"--root", root, "--job", job, "--source", create}); code != 0 {
		t.Fatalf("record-create exit = %d, want 0", code)
	}
	// A second create on the same id is a collision (exit 1).
	if code := runDispatchRecordCreate([]string{"--root", root, "--job", job, "--source", create}); code != 1 {
		t.Fatalf("record-create collision exit = %d, want 1", code)
	}

	setup := writeTemp(t, tmp, "setup.json", map[string]any{
		"jobId": job, "status": "pending", "mainId": "main-1", "claimEpoch": 7,
		"startedAt": "2026-08-10T00:00:00Z",
	})
	if code := runDispatchRecordSetup([]string{"--root", root, "--job", job, "--source", setup}); code != 0 {
		t.Fatalf("record-setup exit = %d, want 0", code)
	}

	run := writeTemp(t, tmp, "run.json", map[string]any{"sessionId": "s"})
	if code := runDispatchRecordCAS([]string{"--root", root, "--job", job, "--expect", "pending", "--status", "running", "--patch", run}); code != 0 {
		t.Fatalf("record-cas pending->running exit = %d, want 0", code)
	}

	// A lost compare prints the observed status on stdout and exits 3.
	stale := writeTemp(t, tmp, "stale.json", map[string]any{"note": "x"})
	out, code := captureStdout(t, func() int {
		return runDispatchRecordCAS([]string{"--root", root, "--job", job, "--expect", "pending", "--status", "failed", "--patch", stale})
	})
	if code != 3 {
		t.Fatalf("stale record-cas exit = %d, want 3", code)
	}
	if strings.TrimSpace(out) != "observed=running" {
		t.Fatalf("stale record-cas stdout = %q, want observed=running", out)
	}

	// A missing required flag is a usage error (exit 2).
	if code := runDispatchRecordCAS([]string{"--root", root, "--job", job, "--expect", "running"}); code != 2 {
		t.Fatalf("record-cas missing-flags exit = %d, want 2", code)
	}
}
