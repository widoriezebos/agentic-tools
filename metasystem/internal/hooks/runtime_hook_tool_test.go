package hooks

import (
	"os"
	"path/filepath"
	"testing"
)

// Claude's PreToolUse path: a delegate is fenced by its adapter; a host tool
// call execs the gate the last start recorded; missing or stale cache state
// leaves the call untouched and never exits 2. Ported from the former shell
// tool-branch tests.

func writeToolCache(t *testing.T, root, body string) {
	t.Helper()
	directory := filepath.Join(root, "artifacts", "agents", "context")
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "engine-path"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestHookToolGate(t *testing.T) {
	t.Parallel()
	type execCall struct {
		path string
		argv []string
	}
	gate := func(t *testing.T, root string, mode os.FileMode) string {
		t.Helper()
		path := filepath.Join(root, "gate engine")
		if err := os.WriteFile(path, []byte("#!/bin/sh\n"), mode); err != nil {
			t.Fatal(err)
		}
		return path
	}
	tests := []struct {
		name      string
		cache     func(t *testing.T, root string) string
		env       map[string]string
		wantExec  bool
		wantInstl string
	}{
		{name: "missing cache", cache: func(*testing.T, string) string { return "" }},
		{name: "one line", cache: func(t *testing.T, root string) string { return gate(t, root, 0o755) + "\n" }},
		{name: "unterminated second line", cache: func(t *testing.T, root string) string { return gate(t, root, 0o755) + "\n" + root }},
		{name: "engine not executable", cache: func(t *testing.T, root string) string { return gate(t, root, 0o644) + "\n" + root + "\n" }},
		{name: "delegate", cache: func(t *testing.T, root string) string { return gate(t, root, 0o755) + "\n" + root + "\n" },
			env: map[string]string{"METASYSTEM_HOOK_DELEGATE_JOB": "job-1"}},
		{name: "gate", cache: func(t *testing.T, root string) string {
			return gate(t, root, 0o755) + "\n" + filepath.Join(root, "installation root") + "\n"
		}, wantExec: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			installation := newHookInstallation(t)
			if body := test.cache(t, installation.root); body != "" {
				writeToolCache(t, installation.root, body)
			}
			var calls []execCall
			ops := newFakeOps(t, installation)
			run := runHook(t, installation, ops, hookCall{runtime: "claude", event: "tool", payload: "payload\n", env: test.env,
				exec: func(path string, argv, _ []string) error {
					calls = append(calls, execCall{path, argv})
					return nil
				}})
			if run.status != 0 || run.stdout != "" || run.stderr != "" {
				t.Fatalf("tool = status %d stdout %q stderr %q", run.status, run.stdout, run.stderr)
			}
			if !test.wantExec {
				if len(calls) != 0 {
					t.Fatalf("%s execed the gate: %+v", test.name, calls)
				}
				return
			}
			want := []string{filepath.Join(installation.root, "gate engine"), "adapter", "claude-tool-gate", "--root", filepath.Join(installation.root, "installation root")}
			if len(calls) != 1 || calls[0].path != want[0] || len(calls[0].argv) != len(want) {
				t.Fatalf("gate exec = %+v", calls)
			}
			for index := range want {
				if calls[0].argv[index] != want[index] {
					t.Fatalf("gate argv = %q, want %q", calls[0].argv, want)
				}
			}
			if ops.trace() != "\n" {
				t.Fatalf("the tool gate reached an owner: %s", ops.trace())
			}
		})
	}
}
