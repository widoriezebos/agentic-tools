package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/hooks"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/hostsetup"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

// checkLiveHooks runs the owner check that runtime setup's output must pass:
// the live settings at live against the shipped hooks, under live's layout.
func checkLiveHooks(runtime, live, shipped string, resolve func(string) (stateroot.Layout, error)) error {
	layout, err := resolve(live)
	if err != nil {
		return err
	}
	liveData, err := os.ReadFile(live)
	if err != nil {
		return err
	}
	shippedData, err := os.ReadFile(shipped)
	if err != nil {
		return err
	}
	return hooks.CheckSettings(liveData, shippedData, runtime, layout.InstallationRel, layout.RepositoryRoot == layout.InstallationRoot)
}

func TestHooksCheckSupportsEveryHost(t *testing.T) {
	repo, installation := setupCLIFixture(t)
	recorder := newRuntimeLayoutRecorder(t, repo)
	recorder.expect(repo)
	if _, code := captureStdout(t, func() int { return runHostSetupForTest(repo, recorder.resolve) }); code != 0 {
		t.Fatalf("setup exit = %d", code)
	}
	paths := map[string][2]string{
		"claude": {filepath.Join(repo, ".claude", "settings.json"), filepath.Join(installation, "internal", "runtimes", "enforcement", "claude-code-hooks.json")},
		"codex":  {filepath.Join(repo, ".codex", "hooks.json"), filepath.Join(installation, "internal", "runtimes", "enforcement", "codex-hooks.json")},
		"devin":  {filepath.Join(repo, ".devin", "config.json"), filepath.Join(installation, "internal", "runtimes", "enforcement", "devin-hooks.json")},
	}
	for runtime, pair := range paths {
		recorder.expect(pair[0])
		if err := checkLiveHooks(runtime, pair[0], pair[1], recorder.resolve); err != nil {
			t.Fatalf("%s hook check: %v", runtime, err)
		}
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
	recorder.expect(codex[0])
	if err := checkLiveHooks("codex", codex[0], codex[1], recorder.resolve); err == nil {
		t.Fatal("wrong Codex matcher passed the hook check")
	}
}

func TestHooksCheckRequiresSynchronousLifecycleAndAllCodexStartSources(t *testing.T) {
	repo, installation := setupCLIFixture(t)
	recorder := newRuntimeLayoutRecorder(t, repo)
	recorder.expect(repo)
	if _, code := captureStdout(t, func() int { return runHostSetupForTest(repo, recorder.resolve) }); code != 0 {
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
	shipped := filepath.Join(installation, "internal", "runtimes", "enforcement", "claude-code-hooks.json")
	recorder.expect(claudePath)
	if err := checkLiveHooks("claude", claudePath, shipped, recorder.resolve); err == nil {
		t.Fatal("async Claude Stop passed the hook check")
	}
	ownedStop["async"] = false
	data, err = json.MarshalIndent(claude, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(claudePath, append(data, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	recorder.expect(claudePath)
	if err := checkLiveHooks("claude", claudePath, shipped, recorder.resolve); err != nil {
		t.Fatalf("explicit synchronous Claude Stop check: %v", err)
	}
}

// runHostSetupForTest registers every adoptable runtime in repo through the
// registration owner, as system setup does.
func runHostSetupForTest(repo string, resolve func(string) (stateroot.Layout, error)) int {
	if _, err := hostsetup.SetupWithResolver(hostsetup.Options{RepositoryPath: repo}, resolve); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}
