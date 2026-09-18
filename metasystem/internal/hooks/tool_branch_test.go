package hooks

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func copyToolHook(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	scriptDir := filepath.Join(root, "scripts", "agents")
	if err := os.MkdirAll(scriptDir, 0o755); err != nil {
		t.Fatal(err)
	}
	source, err := os.ReadFile(filepath.Join("..", "..", "scripts", "agents", "supervision-hook.sh"))
	if err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(scriptDir, "supervision-hook.sh")
	if err := os.WriteFile(script, source, 0o755); err != nil {
		t.Fatal(err)
	}
	return root, script
}

func writeToolEngine(t *testing.T, root, body string, mode os.FileMode) string {
	t.Helper()
	path := filepath.Join(root, "stub-engine")
	if err := os.WriteFile(path, []byte(body), mode); err != nil {
		t.Fatal(err)
	}
	return path
}

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

func cleanToolHookEnvironment(extra ...string) []string {
	environment := make([]string, 0, len(os.Environ())+len(extra))
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "METASYSTEM_") {
			environment = append(environment, entry)
		}
	}
	return append(environment, extra...)
}

func runToolHook(t *testing.T, script, runtime, event, input string, environment []string) (int, string, string) {
	t.Helper()
	command := exec.Command("bash", script, runtime, event)
	command.Env = environment
	command.Stdin = strings.NewReader(input)
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	err := command.Run()
	if err == nil {
		return 0, stdout.String(), stderr.String()
	}
	var exitError *exec.ExitError
	if !errors.As(err, &exitError) {
		t.Fatalf("run tool hook: %v", err)
	}
	return exitError.ExitCode(), stdout.String(), stderr.String()
}

func TestHookToolBranchNeverExitsTwo(t *testing.T) {
	t.Run("missing cache", func(t *testing.T) {
		_, script := copyToolHook(t)
		exit, stdout, stderr := runToolHook(t, script, "claude", "tool", "payload\n", cleanToolHookEnvironment())
		if exit != 0 || stdout != "" || stderr != "" {
			t.Fatalf("missing cache result = exit %d, stdout %q, stderr %q", exit, stdout, stderr)
		}
	})

	t.Run("malformed cache", func(t *testing.T) {
		root, script := copyToolHook(t)
		writeToolCache(t, root, "/missing/second/line\n")
		exit, stdout, stderr := runToolHook(t, script, "claude", "tool", "payload\n", cleanToolHookEnvironment())
		if exit != 0 || stdout != "" || stderr != "" {
			t.Fatalf("malformed cache result = exit %d, stdout %q, stderr %q", exit, stdout, stderr)
		}
	})

	t.Run("engine is not executable", func(t *testing.T) {
		root, script := copyToolHook(t)
		record := filepath.Join(root, "engine-record")
		engine := writeToolEngine(t, root, "#!/usr/bin/env bash\nprintf 'ran\\n' >\"${HOOK_TOOL_RECORD:?}\"\nexit 19\n", 0o644)
		writeToolCache(t, root, engine+"\n"+root+"\n")
		exit, stdout, stderr := runToolHook(t, script, "claude", "tool", "payload\n", cleanToolHookEnvironment("HOOK_TOOL_RECORD="+record))
		if exit != 0 || stdout != "" || stderr != "" {
			t.Fatalf("non-executable engine result = exit %d, stdout %q, stderr %q", exit, stdout, stderr)
		}
		if _, err := os.Stat(record); !os.IsNotExist(err) {
			t.Fatalf("non-executable engine ran: %v", err)
		}
	})

	t.Run("engine owns its nonzero exit", func(t *testing.T) {
		root, script := copyToolHook(t)
		record := filepath.Join(root, "engine-record")
		engine := writeToolEngine(t, root, "#!/usr/bin/env bash\nprintf 'ran\\n' >\"${HOOK_TOOL_RECORD:?}\"\nexit 19\n", 0o755)
		writeToolCache(t, root, engine+"\n"+root+"\n")
		exit, stdout, stderr := runToolHook(t, script, "claude", "tool", "payload\n", cleanToolHookEnvironment("HOOK_TOOL_RECORD="+record))
		if exit != 19 || stdout != "" || stderr != "" {
			t.Fatalf("engine failure result = exit %d, stdout %q, stderr %q", exit, stdout, stderr)
		}
		recorded, err := os.ReadFile(record)
		if err != nil || string(recorded) != "ran\n" {
			t.Fatalf("engine failure record = %q, %v", recorded, err)
		}
	})

	t.Run("other runtime retains malformed-event refusal", func(t *testing.T) {
		_, script := copyToolHook(t)
		exit, stdout, stderr := runToolHook(t, script, "foo", "tool", "payload\n", cleanToolHookEnvironment())
		if exit != 2 || stdout != "" || stderr != "" {
			t.Fatalf("foreign runtime result = exit %d, stdout %q, stderr %q", exit, stdout, stderr)
		}
	})
}

func TestHookToolBranchSkipsTheEngineForADelegate(t *testing.T) {
	root, script := copyToolHook(t)
	record := filepath.Join(root, "engine-record")
	engine := writeToolEngine(t, root, "#!/usr/bin/env bash\nprintf 'ran\\n' >\"${HOOK_TOOL_RECORD:?}\"\nexit 19\n", 0o755)
	writeToolCache(t, root, engine+"\n"+root+"\n")
	exit, stdout, stderr := runToolHook(t, script, "claude", "tool", "payload\n", cleanToolHookEnvironment(
		"METASYSTEM_HOOK_DELEGATE_JOB=job-1",
		"HOOK_TOOL_RECORD="+record,
	))
	if exit != 0 || stdout != "" || stderr != "" {
		t.Fatalf("delegate result = exit %d, stdout %q, stderr %q", exit, stdout, stderr)
	}
	if _, err := os.Stat(record); !os.IsNotExist(err) {
		t.Fatalf("delegate tool call ran the engine: %v", err)
	}
}

func TestHookToolBranchExecsTheGateWithTheRoot(t *testing.T) {
	root, script := copyToolHook(t)
	record := filepath.Join(root, "engine-record")
	installation := filepath.Join(root, "installation root")
	engine := writeToolEngine(t, root, `#!/usr/bin/env bash
{
  printf 'argc=%s\n' "$#"
  for argument in "$@"; do
    printf 'arg=%s\n' "$argument"
  done
  while IFS= read -r line || [[ -n "$line" ]]; do
    printf 'stdin=%s\n' "$line"
  done
} >"${HOOK_TOOL_RECORD:?}"
printf 'gate stdout\n'
exit 23
`, 0o755)
	writeToolCache(t, root, engine+"\n"+installation+"\n")
	exit, stdout, stderr := runToolHook(t, script, "claude", "tool", "payload one\npayload two\n", cleanToolHookEnvironment(
		"HOOK_TOOL_RECORD="+record,
	))
	if exit != 23 || stdout != "gate stdout\n" || stderr != "" {
		t.Fatalf("gate result = exit %d, stdout %q, stderr %q", exit, stdout, stderr)
	}
	recorded, err := os.ReadFile(record)
	if err != nil {
		t.Fatal(err)
	}
	want := "argc=4\narg=adapter\narg=claude-tool-gate\narg=--root\narg=" + installation + "\nstdin=payload one\nstdin=payload two\n"
	if string(recorded) != want {
		t.Fatalf("engine record:\n got %q\nwant %q", recorded, want)
	}
}
