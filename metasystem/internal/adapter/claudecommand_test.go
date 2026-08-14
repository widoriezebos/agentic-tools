package adapter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fakeEnv(pairs map[string]string) func(string) (string, bool) {
	return func(name string) (string, bool) {
		value, ok := pairs[name]
		return value, ok
	}
}

func TestClaudeBudgetPolicy(t *testing.T) {
	budget, turns, err := ClaudeBudget(fakeEnv(nil))
	if err != nil || budget != "5.00" || turns != "50" {
		t.Fatalf("defaults = (%s,%s,%v)", budget, turns, err)
	}
	budget, turns, err = ClaudeBudget(fakeEnv(map[string]string{
		"METASYSTEM_CLAUDE_MAX_BUDGET_USD": "12.5", "METASYSTEM_CLAUDE_MAX_TURNS": "9"}))
	if err != nil || budget != "12.5" || turns != "9" {
		t.Fatalf("overrides = (%s,%s,%v)", budget, turns, err)
	}
	if _, _, err := ClaudeBudget(fakeEnv(map[string]string{"METASYSTEM_CLAUDE_MAX_BUDGET_USD": "free"})); err == nil ||
		err.Error() != "invalid_native_budget" {
		t.Fatalf("budget refusal = %v", err)
	}
	if _, _, err := ClaudeBudget(fakeEnv(map[string]string{"METASYSTEM_CLAUDE_MAX_BUDGET_USD": "0"})); err == nil ||
		err.Error() != "invalid_native_budget" {
		t.Fatalf("zero budget refusal = %v", err)
	}
	if _, _, err := ClaudeBudget(fakeEnv(map[string]string{"METASYSTEM_CLAUDE_MAX_TURNS": "0"})); err == nil ||
		err.Error() != "invalid_native_turn_limit" {
		t.Fatalf("turns refusal = %v", err)
	}
}

func writeClaudeRecord(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "job.json")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestBuildClaudeCommandArgv pins the argv byte order both shells read
// back NUL by NUL — the wording of every flag is wire.
func TestBuildClaudeCommandArgv(t *testing.T) {
	t.Run("read-only envelope narrows and adds read roots", func(t *testing.T) {
		record := writeClaudeRecord(t, `{
			"workspaceRoot": "/ws",
			"permissions": {"requested": {"writeRoots": [], "readRoots": ["/ws", "/extra/docs"]}}
		}`)
		command, err := BuildClaudeCommand(record, "sonnet", `{"s":1}`, "/tmp/settings.json", "", "5.00", "50")
		if err != nil {
			t.Fatal(err)
		}
		want := []string{
			"claude", "-p", "--output-format", "json", "--model", "sonnet",
			"--json-schema", `{"s":1}`,
			"--permission-mode", "dontAsk",
			"--tools", "Read,Glob,Grep",
			"--allowedTools", "Read,Glob,Grep",
			"--settings", "/tmp/settings.json",
			"--max-budget-usd", "5.00", "--max-turns", "50",
			"--add-dir", "/extra/docs",
		}
		if strings.Join(command, "\x00") != strings.Join(want, "\x00") {
			t.Fatalf("argv:\n got %q\nwant %q", command, want)
		}
	})
	t.Run("write envelope keeps full tools and resumes", func(t *testing.T) {
		record := writeClaudeRecord(t, `{
			"workspaceRoot": "/ws",
			"permissions": {"requested": {"writeRoots": ["/ws"]}}
		}`)
		command, err := BuildClaudeCommand(record, "sonnet", "{}", "/tmp/settings.json", "sess-9", "5.00", "50")
		if err != nil {
			t.Fatal(err)
		}
		joined := strings.Join(command, " ")
		for _, fragment := range []string{
			"--permission-mode acceptEdits",
			"--tools Bash,Edit,Write,Read,Glob,Grep,NotebookEdit",
			"--resume sess-9",
		} {
			if !strings.Contains(joined, fragment) {
				t.Fatalf("argv lost %q: %s", fragment, joined)
			}
		}
		if strings.Contains(joined, "--add-dir") {
			t.Fatal("a write envelope adds no read roots")
		}
	})
	t.Run("host mode is acceptEdits with no settings", func(t *testing.T) {
		command, err := BuildClaudeCommand("", "opus", "{}", "", "", "5.00", "50")
		if err != nil {
			t.Fatal(err)
		}
		joined := strings.Join(command, " ")
		if !strings.Contains(joined, "--permission-mode acceptEdits") || strings.Contains(joined, "--settings") {
			t.Fatalf("host argv = %s", joined)
		}
	})
}
