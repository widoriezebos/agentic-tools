package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func setupCLIFixture(t *testing.T) (repo, installation string) {
	t.Helper()
	repo = filepath.Join(t.TempDir(), "host repository with spaces")
	installation = filepath.Join(repo, "metasystem")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	command := exec.Command("git", "init", "-q", "-b", "main", repo)
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "GIT_") {
			command.Env = append(command.Env, entry)
		}
	}
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, output)
	}
	setupCLIWrite(t, filepath.Join(repo, "development", "metasystem-design.md"), "design\n", 0o644)
	setupCLIWrite(t, filepath.Join(installation, "metasystem.conf"), "metasystem.runtimes=claude\n", 0o644)
	if err := os.MkdirAll(filepath.Join(installation, "scripts", "agents"), 0o755); err != nil {
		t.Fatal(err)
	}
	setupCLIWrite(t, filepath.Join(installation, "scripts", "enforcement", "claude-code-hooks.json"), cliClaudeHooks, 0o644)
	setupCLIWrite(t, filepath.Join(installation, "scripts", "enforcement", "codex-hooks.json"), cliCodexHooks, 0o644)
	setupCLIWrite(t, filepath.Join(installation, "scripts", "enforcement", "devin-hooks.json"), cliDevinHooks, 0o644)
	setupCLIWrite(t, filepath.Join(installation, "skills", "demo", "SKILL.md"), "demo\n", 0o644)
	setupCLIWrite(t, filepath.Join(installation, "skills", "demo", "agents", "claude-profile.md"), "claude\n", 0o644)
	setupCLIWrite(t, filepath.Join(installation, "skills", "demo", "agents", "devin", "AGENT.md"), "devin\n", 0o644)
	setupCLIWrite(t, filepath.Join(installation, "skills", "demo", "agents", "openai.yaml"), "interface: {}\n", 0o644)
	return repo, installation
}

func setupCLIWrite(t *testing.T, path, content string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
}

const cliClaudeHooks = `{"hooks":{"SessionStart":[{"matcher":"startup|resume|clear|compact","hooks":[{"type":"command","command":"bash scripts/agents/supervision-hook.sh claude start","timeout":15}]}],"Stop":[{"hooks":[{"type":"command","command":"bash scripts/agents/supervision-hook.sh claude receipt"},{"type":"command","command":"(bash scripts/agents/supervision-hook.sh claude stop) || printf '%s\\n' '{\"decision\":\"block\",\"reason\":\"Metasystem Stop hook launcher failed before a safe verdict; stopping is refused.\"}'","timeout":60}]}],"SessionEnd":[{"hooks":[{"type":"command","command":"bash scripts/agents/supervision-hook.sh claude end","timeout":3}]}]}}`
const cliCodexHooks = `{"hooks":{"SessionStart":[{"matcher":"startup|resume|clear|compact","hooks":[{"type":"command","command":"bash scripts/agents/supervision-hook.sh codex start","timeout":15}]}],"Stop":[{"hooks":[{"type":"command","command":"bash scripts/agents/supervision-hook.sh codex stop","timeout":60}]}],"SessionEnd":[{"hooks":[{"type":"command","command":"bash scripts/agents/supervision-hook.sh codex end","timeout":3}]}]}}`
const cliDevinHooks = `{"hooks":{"SessionStart":[{"hooks":[{"type":"command","command":"bash scripts/agents/supervision-hook.sh devin start","timeout":15}]}],"Stop":[{"hooks":[{"type":"command","command":"bash scripts/agents/supervision-hook.sh devin stop","timeout":60}]}],"SessionEnd":[{"hooks":[{"type":"command","command":"bash scripts/agents/supervision-hook.sh devin end","timeout":3}]}]}}`

func TestRuntimeSetupCLIConfiguresAllByDefaultChecksAndSelectsNone(t *testing.T) {
	repo, _ := setupCLIFixture(t)
	output, code := captureStdout(t, func() int { return runRuntimeSetup([]string{"--repo", filepath.Join(repo, "metasystem", "skills")}) })
	if code != 0 {
		t.Fatalf("runtime setup exit = %d", code)
	}
	for _, runtime := range []string{"claude", "codex", "devin"} {
		if !strings.Contains(output, "CONFIG_READY "+runtime) || !strings.Contains(output, "PROVIDER_TRUST "+runtime) || !strings.Contains(output, "LIFECYCLE_OBSERVED "+runtime) {
			t.Fatalf("setup output omitted %s readiness dimensions: %s", runtime, output)
		}
	}
	if _, code := captureStdout(t, func() int { return runRuntimeSetup([]string{"--repo", repo, "--check"}) }); code != 0 {
		t.Fatalf("setup check exit = %d", code)
	}

	emptyRepo, _ := setupCLIFixture(t)
	emptyOutput, code := captureStdout(t, func() int {
		return runRuntimeSetup([]string{"--repo", emptyRepo, "--runtimes", "none"})
	})
	if code != 0 || emptyOutput != "" {
		t.Fatalf("none selection = exit %d output %q", code, emptyOutput)
	}
	if _, err := os.Stat(filepath.Join(emptyRepo, "AGENTS.md")); !os.IsNotExist(err) {
		t.Fatal("none selection installed host instructions")
	}
}

func TestRuntimeSetupCLIRejectsUnknownInputAndPendingCheck(t *testing.T) {
	repo, _ := setupCLIFixture(t)
	if _, code := captureStdout(t, func() int { return runRuntimeSetup([]string{"--repo", repo, "--runtimes", "unknown"}) }); code != 1 {
		t.Fatalf("unknown runtime exit = %d, want 1", code)
	}
	if _, code := captureStdout(t, func() int { return runRuntimeSetup([]string{"--repo", repo, "--check"}) }); code != 1 {
		t.Fatalf("pending check exit = %d, want 1", code)
	}
}

func TestFreshAdoptionSeparatesRuntimeAndTestingReadiness(t *testing.T) {
	repo, installation := setupCLIFixture(t)
	contract := filepath.Join(installation, "testing.json")
	conf := filepath.Join(installation, "metasystem.conf")
	if err := os.WriteFile(conf, []byte("metasystem.runtimes=claude\ntesting.contract=testing.json\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code := runConfigTailor([]string{"--conf", conf, "--runtimes", "none", "--testing-contract", contract}); code != 0 {
		t.Fatalf("config tailor exit = %d", code)
	}
	data, err := os.ReadFile(contract)
	if err != nil || !strings.Contains(string(data), `"tailoringRequired": true`) {
		t.Fatalf("fresh testing contract is not explicitly incomplete: %q err=%v", data, err)
	}
	output, code := captureStdout(t, func() int { return runRuntimeSetup([]string{"--repo", repo, "--runtimes", "none"}) })
	if code != 0 || !strings.Contains(output, "TEST_CONTRACT_REQUIRED") || strings.Contains(output, "CONFIG_READY") {
		t.Fatalf("readiness dimensions were conflated: exit=%d output=%q", code, output)
	}
}

func TestRuntimeSetupCLISupportsNestedAdoptedInstallationAndSubdirectory(t *testing.T) {
	app, installation := setupCLIFixture(t)
	if err := os.Remove(filepath.Join(app, "development", "metasystem-design.md")); err != nil {
		t.Fatal(err)
	}
	output, code := captureStdout(t, func() int {
		return runRuntimeSetup([]string{"--repo", installation, "--runtimes", "claude,codex,devin"})
	})
	if code != 0 {
		t.Fatalf("nested adopted setup exit = %d", code)
	}
	wantInstallation, _ := filepath.EvalSymlinks(installation)
	if !strings.Contains(output, "repository="+wantInstallation+" installation="+wantInstallation) {
		t.Fatalf("nested adopted setup reported the wrong owner: %s", output)
	}
	if _, code := captureStdout(t, func() int {
		return runRuntimeSetup([]string{"--repo", filepath.Join(installation, "skills", "demo"), "--runtimes", "claude,codex,devin", "--check"})
	}); code != 0 {
		t.Fatalf("nested adopted setup check exit = %d", code)
	}
	for _, path := range []string{".claude/settings.json", ".codex/hooks.json", ".devin/config.json", ".agents/skills/demo"} {
		if _, err := os.Lstat(filepath.Join(installation, path)); err != nil {
			t.Errorf("nested adopted CLI registration missing %s: %v", path, err)
		}
	}
	if _, err := os.Stat(filepath.Join(app, ".claude")); !os.IsNotExist(err) {
		t.Fatalf("nested adopted CLI wrote parent runtime state: %v", err)
	}
}
