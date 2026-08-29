package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// withStdin runs fn with os.Stdin fed from content, so a verb that reads the
// hook payload or a --version stream can be driven from a test.
func withStdin(t *testing.T, content string, fn func()) {
	t.Helper()
	read, write, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	saved := os.Stdin
	os.Stdin = read
	go func() {
		write.WriteString(content)
		write.Close()
	}()
	fn()
	os.Stdin = saved
}

func TestRunAdapterVersionParse(t *testing.T) {
	var out string
	var code int
	withStdin(t, "codex-cli 4.2.0 (abc)\n", func() {
		out, code = captureStdout(t, func() int { return runAdapterVersionParse(nil) })
	})
	if code != 0 || strings.TrimSpace(out) != "4.2.0" {
		t.Fatalf("version-parse code=%d out=%q", code, out)
	}
}

func TestRunAdapterCodexCommandNULDelimited(t *testing.T) {
	dir := t.TempDir()
	out, code := captureStdout(t, func() int {
		return runAdapterCodexCommand([]string{
			"--verb", "dispatch", "--model", "gpt-5-sol", "--workspace", dir,
			"--schema", "/s.json", "--output", "/o.json",
			"--sandbox", "workspace-write", "--network", "true",
			"--instance-tag", "metasystem-job-t-abc",
		})
	})
	if code != 0 {
		t.Fatalf("codex-command code=%d", code)
	}
	tokens := strings.Split(strings.TrimRight(out, "\x00"), "\x00")
	if tokens[0] != "codex" || tokens[1] != "exec" || tokens[len(tokens)-1] != "-" {
		t.Fatalf("unexpected codex argv: %q", tokens)
	}
	if !strings.Contains(out, "sandbox_workspace_write.network_access=true\x00") {
		t.Fatalf("network access token missing: %q", out)
	}
}

func TestRunAdapterCodexUsageAndDevinSession(t *testing.T) {
	dir := t.TempDir()
	events := filepath.Join(dir, "events.jsonl")
	os.WriteFile(events, []byte(`{"type":"turn","usage":{"input_tokens":5,"output_tokens":2}}`+"\n"), 0o644)
	usage := filepath.Join(dir, "usage.json")
	if code := runAdapterCodexUsage([]string{"--events", events, "--output", usage}); code != 0 {
		t.Fatalf("codex-usage code=%d", code)
	}
	if _, err := os.Stat(usage); err != nil {
		t.Fatalf("codex-usage wrote no file: %v", err)
	}

	// Two new sessions in the same workspace is the ambiguous case: its own
	// exit 3, distinct from the package-wide 2 for usage.
	before := filepath.Join(dir, "before.json")
	current := filepath.Join(dir, "current.json")
	os.WriteFile(before, []byte(`[]`), 0o644)
	os.WriteFile(current, []byte(`[{"id":"a","working_directory":"`+dir+`"},{"id":"b","working_directory":"`+dir+`"}]`), 0o644)
	code := runAdapterDevinSession([]string{"--before", before, "--current", current, "--signal", "/none", "--workspace", dir})
	if code != 3 {
		t.Fatalf("ambiguous correlation should exit 3, got %d", code)
	}
}

func TestRunAdapterUsageUnavailable(t *testing.T) {
	out := filepath.Join(t.TempDir(), "usage.json")
	if code := runAdapterUsageUnavailable([]string{"--output", out}); code != 0 {
		t.Fatalf("usage-unavailable code=%d", code)
	}
	data, err := os.ReadFile(out)
	if err != nil || !strings.Contains(string(data), `"availability": "unavailable"`) {
		t.Fatalf("unexpected unavailable usage: %q err=%v", string(data), err)
	}
}
