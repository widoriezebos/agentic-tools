package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestContextBudgetConfigDefaultsAndAccessor(t *testing.T) {
	clearContextEnvironment(t)
	root := t.TempDir()
	want := Budget{Ceiling: 250000, Margin: 145000, Trigger: 105000, Reserve: 0}
	if got, err := ContextBudget(root); err != nil || got != want {
		t.Fatalf("defaults = %+v, err=%v, want %+v", got, err, want)
	}
	putFile(t, filepath.Join(root, "metasystem.conf"), ContextCeilingTokensKey+"=250001\n"+ContextHandoffMarginTokensKey+"=158001\n"+ContextToolGateReserveCallsKey+"=1\n")
	if got, err := ContextBudget(root); err != nil || got != (Budget{250001, 158001, 92000, 1}) {
		t.Fatalf("committed budget = %+v, err=%v", got, err)
	}
	putFile(t, filepath.Join(root, "metasystem.conf.local"), ContextHandoffMarginTokensKey+"=145002\n")
	if _, err := ContextBudget(root); err == nil || !strings.Contains(err.Error(), "committed root configuration") {
		t.Fatalf("local context law was not refused: %v", err)
	}
	putFile(t, filepath.Join(root, "metasystem.conf"), "metasystem.runtimes=fake\n")
	t.Setenv(EnvName(ContextCeilingTokensKey), "250002")
	if got, err := ContextBudget(root); err != nil || got != (Budget{250002, 145002, 105000, 0}) {
		t.Fatalf("fixture layers = %+v, err=%v", got, err)
	}

	committedRoot := t.TempDir()
	claudeDirectory := filepath.Join(committedRoot, "claude-memory")
	codexDirectory := filepath.Join(committedRoot, "codex-memory")
	putFile(t, filepath.Join(committedRoot, "metasystem.conf"),
		"metasystem.runtimes=claude,codex\n"+ContextHandoffNoteDirectoryPrefix+"codex="+committedRoot+"//codex-memory\n")
	if got, err := ContextHandoffNoteDirectory(committedRoot, "claude", claudeDirectory); err != nil || got != claudeDirectory {
		t.Fatalf("Claude note directory = %q err=%v", got, err)
	}
	if got, err := ContextHandoffNoteDirectory(committedRoot, "codex", claudeDirectory); err != nil || got != codexDirectory {
		t.Fatalf("Codex note directory = %q err=%v", got, err)
	}
}

func TestContextConfKeysDocumented(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("..", "..", "metasystem.conf"))
	if err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{"context.ceiling.tokens=250000", "context.handoff.margin.tokens=145000", "context.handoff.note-directory.codex=", "context.toolgate.mode=deny", "context.toolgate.reserve.calls", "first observe day", "trigger 105000", "106638"} {
		if !strings.Contains(string(content), text) {
			t.Fatalf("metasystem.conf does not document %q", text)
		}
	}
}

func TestNoKeyLowersTheConstructionLine(t *testing.T) {
	t.Parallel()
	if ContextConstructionLineTokens != 106638 {
		t.Fatalf("construction line = %d", ContextConstructionLineTokens)
	}
	for key := range contextKeys {
		t.Run(key, func(t *testing.T) {
			if key == ContextToolGateReserveCallsKey {
				return
			}
			if key == ContextToolGateModeKey {
				for _, mode := range []string{"observe", "deny"} {
					root := t.TempDir()
					putFile(t, filepath.Join(root, "metasystem.conf"), fmt.Sprintf("%s=%d\n%s=%s\n", ContextCeilingTokensKey, DefaultContextHandoffMarginTokens+ContextConstructionLineTokens, key, mode))
					if budget, err := ContextBudget(root); err != nil || budget.Trigger != ContextConstructionLineTokens {
						t.Fatalf("%s=%s changed the construction line: budget=%+v err=%v", key, mode, budget, err)
					}
				}
				return
			}
			boundary, direction := DefaultContextCeilingTokens-ContextConstructionLineTokens, int64(-1)
			if key == ContextCeilingTokensKey {
				boundary, direction = DefaultContextHandoffMarginTokens+ContextConstructionLineTokens, 1
			}
			root := t.TempDir()
			putFile(t, filepath.Join(root, "metasystem.conf"), key+"="+fmt.Sprint(boundary)+"\n")
			if budget, err := ContextBudget(root); err != nil || budget.Trigger != ContextConstructionLineTokens {
				t.Fatalf("%s refused the construction line: budget=%+v err=%v", key, budget, err)
			}
			putFile(t, filepath.Join(root, "metasystem.conf"), key+"="+fmt.Sprint(boundary+direction)+"\n")
			if _, err := ContextBudget(root); err == nil || !strings.Contains(err.Error(), "construction line 106638") {
				t.Fatalf("%s lowered the construction line: %v", key, err)
			}
		})
	}
}

func TestReserveLineArithmetic(t *testing.T) {
	t.Parallel()
	want := []int64{106638, 92184, 77730, 63276}
	for reserve, line := range want {
		if got := ContextConstructionLine(int64(reserve)); got != line {
			t.Fatalf("reserve %d construction line = %d, want %d", reserve, got, line)
		}
	}
	if ContextConstructionLineTokens != ContextConstructionLine(0) {
		t.Fatalf("constant = %d, reserve-0 line = %d", ContextConstructionLineTokens, ContextConstructionLine(0))
	}
}

func TestToolGateModeRefusesOtherValues(t *testing.T) {
	clearContextEnvironment(t)
	root := t.TempDir()
	if mode, err := ToolGateMode(root); err != nil || mode != "observe" {
		t.Fatalf("absent mode = %q, %v", mode, err)
	}
	putFile(t, filepath.Join(root, "metasystem.conf"), ContextToolGateModeKey+"=deny\n")
	if mode, err := ToolGateMode(root); err != nil || mode != "deny" {
		t.Fatalf("deny mode = %q, %v", mode, err)
	}
	putFile(t, filepath.Join(root, "metasystem.conf"), ContextToolGateModeKey+"=audit\n")
	if _, err := ToolGateMode(root); err == nil || !strings.Contains(err.Error(), ContextToolGateModeKey) || !strings.Contains(err.Error(), "audit") {
		t.Fatalf("audit mode was not refused by key and value: %v", err)
	}
}

func TestShippedToolGateModeIsDeny(t *testing.T) {
	root := filepath.Join("..", "..")
	mode, err := ToolGateMode(root)
	if err != nil || mode != "deny" {
		t.Fatalf("shipped tool gate mode = %q, %v", mode, err)
	}
}

func clearContextEnvironment(t *testing.T) {
	t.Helper()
	for key := range contextKeys {
		name := EnvName(key)
		value, present := os.LookupEnv(name)
		_ = os.Unsetenv(name)
		t.Cleanup(func() {
			_ = os.Unsetenv(name)
			if present {
				_ = os.Setenv(name, value)
			}
		})
	}
}
