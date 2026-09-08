package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestHooksCheckSupportsEveryHostAndHistoricalClaudePositionals(t *testing.T) {
	repo, installation := setupCLIFixture(t)
	if _, code := captureStdout(t, func() int { return runRuntimeSetup([]string{"--repo", repo}) }); code != 0 {
		t.Fatalf("setup exit = %d", code)
	}
	paths := map[string][2]string{
		"claude": {filepath.Join(repo, ".claude", "settings.json"), filepath.Join(installation, "scripts", "enforcement", "claude-code-hooks.json")},
		"codex":  {filepath.Join(repo, ".codex", "hooks.json"), filepath.Join(installation, "scripts", "enforcement", "codex-hooks.json")},
		"devin":  {filepath.Join(repo, ".devin", "config.json"), filepath.Join(installation, "scripts", "enforcement", "devin-hooks.json")},
	}
	for runtime, pair := range paths {
		if code := runHooksCheck([]string{"--runtime", runtime, pair[0], pair[1]}); code != 0 {
			t.Fatalf("%s hook check exit = %d", runtime, code)
		}
	}
	claude := paths["claude"]
	if code := runHooksCheck([]string{claude[0], claude[1]}); code != 0 {
		t.Fatalf("historical Claude positional check exit = %d", code)
	}

	codex := paths["codex"]
	data, err := os.ReadFile(codex[0])
	if err != nil {
		t.Fatal(err)
	}
	data = []byte(strings.Replace(string(data), "startup|resume|clear|compact", "startup|resume|compact", 1))
	if err := os.WriteFile(codex[0], data, 0o644); err != nil {
		t.Fatal(err)
	}
	if code := runHooksCheck([]string{"--runtime", "codex", codex[0], codex[1]}); code != 1 {
		t.Fatalf("wrong Codex matcher exit = %d, want 1", code)
	}
}

func TestHooksCheckRequiresSynchronousLifecycleAndAllCodexStartSources(t *testing.T) {
	repo, installation := setupCLIFixture(t)
	if _, code := captureStdout(t, func() int { return runRuntimeSetup([]string{"--repo", repo}) }); code != 0 {
		t.Fatalf("setup exit = %d", code)
	}

	codexPath := filepath.Join(repo, ".codex", "hooks.json")
	data, err := os.ReadFile(codexPath)
	if err != nil {
		t.Fatal(err)
	}
	var codex map[string]any
	if err := json.Unmarshal(data, &codex); err != nil {
		t.Fatal(err)
	}
	start := codex["hooks"].(map[string]any)["SessionStart"].([]any)[0].(map[string]any)
	matcher := start["matcher"].(string)
	startPattern, err := regexp.Compile("^(?:" + matcher + ")$")
	if err != nil {
		t.Fatal(err)
	}
	for _, source := range []string{"startup", "resume", "clear", "compact"} {
		if !startPattern.MatchString(source) {
			t.Errorf("Codex SessionStart matcher %q does not match %q", matcher, source)
		}
	}

	claudePath := filepath.Join(repo, ".claude", "settings.json")
	data, err = os.ReadFile(claudePath)
	if err != nil {
		t.Fatal(err)
	}
	var claude map[string]any
	if err := json.Unmarshal(data, &claude); err != nil {
		t.Fatal(err)
	}
	stopGroups := claude["hooks"].(map[string]any)["Stop"].([]any)
	var ownedStop map[string]any
	for _, rawGroup := range stopGroups {
		for _, rawHandler := range rawGroup.(map[string]any)["hooks"].([]any) {
			handler := rawHandler.(map[string]any)
			if command, _ := handler["command"].(string); strings.Contains(command, " claude stop") {
				ownedStop = handler
			}
		}
	}
	if ownedStop == nil {
		t.Fatal("generated Claude Stop handler not found")
	}
	ownedStop["async"] = true
	data, err = json.MarshalIndent(claude, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(claudePath, append(data, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	shipped := filepath.Join(installation, "scripts", "enforcement", "claude-code-hooks.json")
	if code := runHooksCheck([]string{"--runtime", "claude", claudePath, shipped}); code != 1 {
		t.Fatalf("async Claude Stop check exit = %d, want 1", code)
	}
	ownedStop["async"] = false
	data, err = json.MarshalIndent(claude, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(claudePath, append(data, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	if code := runHooksCheck([]string{"--runtime", "claude", claudePath, shipped}); code != 0 {
		t.Fatalf("explicit synchronous Claude Stop check exit = %d", code)
	}
}
