package hostsetup

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func hostFixture(t *testing.T, nested bool) (repo, installation string) {
	t.Helper()
	repo = filepath.Join(t.TempDir(), "repository with spaces")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "init", "-q", "-b", "main", repo)
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "GIT_") {
			cmd.Env = append(cmd.Env, entry)
		}
	}
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, output)
	}
	installation = repo
	if nested {
		installation = filepath.Join(repo, "metasystem")
		writeHostFile(t, filepath.Join(repo, "development", "metasystem-design.md"), "design\n", 0o644)
	}
	populateHostInstallation(t, installation)
	return repo, installation
}

func hostGitFreeNestedFixture(t *testing.T, allowedPaths ...string) (string, stateroot.Resolver) {
	t.Helper()
	repo := filepath.Join(t.TempDir(), "repository with spaces")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	writeHostFile(t, filepath.Join(repo, "development", "metasystem-design.md"), "design\n", 0o644)
	populateHostInstallation(t, filepath.Join(repo, "metasystem"))
	canonicalRepo, err := filepath.EvalSymlinks(repo)
	if err != nil {
		t.Fatal(err)
	}
	allowed := map[string]bool{canonicalRepo: true}
	for _, relative := range allowedPaths {
		allowed[filepath.Join(canonicalRepo, relative)] = true
	}
	seen := map[string]bool{}
	t.Cleanup(func() {
		for path := range allowed {
			if !seen[path] {
				t.Errorf("repository discovery did not inspect %q", path)
			}
		}
	})
	resolver := stateroot.NewResolver(func(path string) (string, error) {
		canonicalPath, err := filepath.EvalSymlinks(path)
		if err != nil {
			t.Fatalf("resolve repository path %q: %v", path, err)
		}
		relative, err := filepath.Rel(canonicalRepo, canonicalPath)
		if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || !allowed[canonicalPath] {
			t.Fatalf("unexpected repository discovery path %q for %q", canonicalPath, canonicalRepo)
		}
		seen[canonicalPath] = true
		return canonicalRepo, nil
	}, func() (string, error) {
		t.Fatal("unexpected executable lookup during host setup")
		return "", errors.New("unexpected executable lookup")
	})
	return repo, resolver
}

func populateHostInstallation(t *testing.T, installation string) {
	t.Helper()
	writeHostFile(t, filepath.Join(installation, "metasystem.conf"), "metasystem.runtimes=claude\n", 0o644)
	if err := os.MkdirAll(filepath.Join(installation, "scripts", "agents"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeHostFile(t, filepath.Join(installation, "scripts", "enforcement", "claude-code-hooks.json"), hostHookFixture(t, "claude-code"), 0o644)
	writeHostFile(t, filepath.Join(installation, "scripts", "enforcement", "codex-hooks.json"), hostHookFixture(t, "codex"), 0o644)
	writeHostFile(t, filepath.Join(installation, "scripts", "enforcement", "devin-hooks.json"), hostHookFixture(t, "devin"), 0o644)
	writeHostFile(t, filepath.Join(installation, "skills", "demo", "SKILL.md"), "demo skill\n", 0o640)
	writeHostFile(t, filepath.Join(installation, "skills", "demo", "agents", "claude-profile.md"), "claude profile\n", 0o644)
	writeHostFile(t, filepath.Join(installation, "skills", "demo", "agents", "devin", "AGENT.md"), "devin profile\n", 0o644)
	writeHostFile(t, filepath.Join(installation, "skills", "demo", "agents", "openai.yaml"), "interface: {}\n", 0o644)
}

func writeHostFile(t *testing.T, path, content string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := testexec.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, mode); err != nil {
		t.Fatal(err)
	}
}

func hostHookFixture(t *testing.T, name string) string {
	t.Helper()
	contents, err := os.ReadFile(filepath.Join(hostSetupModuleRoot(t), "scripts", "enforcement", name+"-hooks.json"))
	if err != nil {
		t.Fatal(err)
	}
	return string(contents)
}

// A fresh checkout must inherit whatever context window Claude Code gives the
// seat's model. The shipped template carried autoCompactWindow: 200000 until
// 2026-09-20; setting it capped every seat, and every in-process delegate of
// that seat, at a number chosen for one model. The transform still carries the
// template's other top-level keys, which is what keeps this a transform test.
func TestSetupImposesNoSeatWindowThroughClaudeTransform(t *testing.T) {
	t.Parallel()
	repo, resolver := hostGitFreeNestedFixture(t)
	if _, err := SetupWithResolver(Options{RepositoryPath: repo, Runtimes: []string{"claude"}}, resolver.ResolveLayout); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(repo, ".claude", "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatal(err)
	}
	if window, present := settings["autoCompactWindow"]; present {
		t.Fatalf("generated autoCompactWindow=%v, want no key at all", window)
	}
	if _, present := settings["hooks"]; !present {
		t.Fatal("the generated Claude settings carry no hooks")
	}
	if _, present := settings["_comment"]; present {
		t.Fatal("the generated Claude settings retained the source comment")
	}
}

func TestSetupNestedDefaultsAllPreservesUnrelatedStateModesAndIsIdempotent(t *testing.T) {
	repo, resolver := hostGitFreeNestedFixture(t, "sub/directory")
	writeHostFile(t, filepath.Join(repo, "AGENTS.md"), "application instructions\n", 0o600)
	writeHostFile(t, filepath.Join(repo, ".claude", "settings.json"), `{"unrelated":{"keep":true},"hooks":{"Foreign":[{"hooks":[{"command":"foreign-handler"}]}]}}`, 0o640)
	writeHostFile(t, filepath.Join(repo, ".agents", "skills", "foreign", "SKILL.md"), "foreign skill\n", 0o644)
	result, err := Setup(Options{RepositoryPath: filepath.Join(repo, "a", "missing")})
	if err == nil {
		t.Fatal("a missing subdirectory was accepted")
	}
	if err := os.MkdirAll(filepath.Join(repo, "sub", "directory"), 0o755); err != nil {
		t.Fatal(err)
	}
	result, err = SetupWithResolver(Options{RepositoryPath: filepath.Join(repo, "sub", "directory")}, resolver.ResolveLayout)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(result.Runtimes, ",") != "codex,devin,claude" {
		t.Fatalf("default runtimes = %v", result.Runtimes)
	}
	for _, path := range []string{".agents/skills/demo", ".devin/skills/demo", ".claude/skills/demo"} {
		info, err := os.Lstat(filepath.Join(repo, path))
		if err != nil || info.Mode()&os.ModeSymlink == 0 {
			t.Errorf("%s is not a link: %v", path, err)
		}
	}
	for _, path := range []string{".claude/agents/demo.md", ".devin/agents/demo/AGENT.md", ".codex/hooks.json", ".devin/config.json"} {
		if _, err := os.Stat(filepath.Join(repo, path)); err != nil {
			t.Errorf("missing %s: %v", path, err)
		}
	}
	foreign, err := os.ReadFile(filepath.Join(repo, ".agents", "skills", "foreign", "SKILL.md"))
	if err != nil || string(foreign) != "foreign skill\n" {
		t.Fatalf("foreign skill changed: %q, %v", foreign, err)
	}
	agents, _ := os.ReadFile(filepath.Join(repo, "AGENTS.md"))
	if !strings.Contains(string(agents), "application instructions") || strings.Count(string(agents), pointerBegin) != 1 {
		t.Fatalf("instruction merge failed: %s", agents)
	}
	if info, _ := os.Stat(filepath.Join(repo, "AGENTS.md")); info.Mode().Perm() != 0o600 {
		t.Fatalf("instruction mode changed to %o", info.Mode().Perm())
	}
	settingsPath := filepath.Join(repo, ".claude", "settings.json")
	settings, _ := os.ReadFile(settingsPath)
	if !strings.Contains(string(settings), `"unrelated"`) || !strings.Contains(string(settings), `"Foreign"`) || !strings.Contains(string(settings), `unset GIT_DIR`) || !strings.Contains(string(settings), `$repo/metasystem`) {
		t.Fatalf("settings merge lost state or movable command: %s", settings)
	}
	if info, _ := os.Stat(settingsPath); info.Mode().Perm() != 0o640 {
		t.Fatalf("settings mode changed to %o", info.Mode().Perm())
	}
	codex, _ := os.ReadFile(filepath.Join(repo, ".codex", "hooks.json"))
	if !strings.Contains(string(codex), "startup|resume|clear|compact") {
		t.Fatalf("Codex compact rejoin missing: %s", codex)
	}
	if _, err := SetupWithResolver(Options{RepositoryPath: repo, Check: true}, resolver.ResolveLayout); err != nil {
		t.Fatalf("check after setup: %v", err)
	}
	again, err := SetupWithResolver(Options{RepositoryPath: repo}, resolver.ResolveLayout)
	if err != nil || len(again.Changed) != 0 {
		t.Fatalf("repeat changed %v: %v", again.Changed, err)
	}
	if err := os.Remove(filepath.Join(repo, ".devin", "skills", "demo")); err != nil {
		t.Fatal(err)
	}
	staleStage := filepath.Join(repo, ".devin", "skills", ".demo.metasystem-link")
	if err := os.Symlink("interrupted-stage", staleStage); err != nil {
		t.Fatal(err)
	}
	retry, err := SetupWithResolver(Options{RepositoryPath: repo}, resolver.ResolveLayout)
	if err != nil || len(retry.Changed) != 1 {
		t.Fatalf("partial retry = %v, %v", retry.Changed, err)
	}
	if _, err := os.Lstat(staleStage); !os.IsNotExist(err) {
		t.Fatalf("interrupted link staging path survived retry: %v", err)
	}
}

func TestSetupUpgradesClaudeCompactMatcherAndPreservesForeignGroupModeAndIdempotence(t *testing.T) {
	repo, resolver := hostGitFreeNestedFixture(t)
	settingsPath := filepath.Join(repo, ".claude", "settings.json")
	legacy := `{"foreignTop":{"kept":true},"hooks":{"SessionStart":[{"matcher":"startup|resume|clear","groupMetadata":{"kept":true},"hooks":[{"type":"command","command":"cd \"$CLAUDE_PROJECT_DIR/metasystem\" && bash scripts/agents/supervision-hook.sh claude start","timeout":15},{"type":"command","command":"foreign-start-handler","timeout":7}]}]}}`
	writeHostFile(t, settingsPath, legacy, 0o600)

	first, err := SetupWithResolver(Options{RepositoryPath: repo, Runtimes: []string{"claude"}}, resolver.ResolveLayout)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Changed) == 0 {
		t.Fatal("the legacy three-source matcher was not upgraded")
	}
	upgraded, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	text := string(upgraded)
	for _, kept := range []string{`"foreignTop"`, `"groupMetadata"`, `"foreign-start-handler"`, `"matcher": "startup|resume|clear"`, `"matcher": "startup|resume|clear|compact"`} {
		if !strings.Contains(text, kept) {
			t.Fatalf("compact matcher upgrade lost %q: %s", kept, text)
		}
	}
	if strings.Count(text, "supervision-hook.sh claude start") != 1 {
		t.Fatalf("compact matcher upgrade duplicated the owned handler: %s", text)
	}
	if info, err := os.Stat(settingsPath); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("compact matcher upgrade changed settings mode: %v, %v", info, err)
	}

	second, err := SetupWithResolver(Options{RepositoryPath: repo, Runtimes: []string{"claude"}}, resolver.ResolveLayout)
	if err != nil || len(second.Changed) != 0 {
		t.Fatalf("repeated compact matcher setup changed %v: %v", second.Changed, err)
	}
	again, err := os.ReadFile(settingsPath)
	if err != nil || !bytes.Equal(again, upgraded) {
		t.Fatalf("repeated compact matcher setup was not byte-idempotent: %v", err)
	}
}

func TestSetupPreservesStructurallyReadySettingsBytes(t *testing.T) {
	repo, resolver := hostGitFreeNestedFixture(t)
	if _, err := SetupWithResolver(Options{RepositoryPath: repo, Runtimes: []string{"claude"}}, resolver.ResolveLayout); err != nil {
		t.Fatal(err)
	}
	settingsPath := filepath.Join(repo, ".claude", "settings.json")
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatal(err)
	}
	settings["env"] = map[string]any{"FOREIGN_SETTING": "preserved"}
	start := settings["hooks"].(map[string]any)["SessionStart"].([]any)[0].(map[string]any)
	start["groupMetadata"] = map[string]any{"foreign": true}
	start["hooks"] = append(start["hooks"].([]any), map[string]any{
		"type": "command", "command": "foreign-start-handler", "async": true,
	})
	hooksJSON, err := json.MarshalIndent(settings["hooks"], "    ", "    ")
	if err != nil {
		t.Fatal(err)
	}
	rewritten := []byte("{\n    \"hooks\": " + string(hooksJSON) + ",\n    \"env\": {\n        \"FOREIGN_SETTING\": \"preserved\"\n    }\n}\n")
	var validate map[string]any
	if err := json.Unmarshal(rewritten, &validate); err != nil {
		t.Fatalf("test rewrite is invalid: %v\n%s", err, rewritten)
	}
	if err := os.WriteFile(settingsPath, rewritten, 0o640); err != nil {
		t.Fatal(err)
	}
	checked, err := SetupWithResolver(Options{RepositoryPath: repo, Runtimes: []string{"claude"}, Check: true}, resolver.ResolveLayout)
	if err != nil || len(checked.Changed) != 0 {
		t.Fatalf("semantic check changed %v: %v", checked.Changed, err)
	}
	applied, err := SetupWithResolver(Options{RepositoryPath: repo, Runtimes: []string{"claude"}}, resolver.ResolveLayout)
	if err != nil || len(applied.Changed) != 0 {
		t.Fatalf("semantic setup changed %v: %v", applied.Changed, err)
	}
	after, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(after, rewritten) {
		t.Fatalf("structurally ready settings were reserialized:\n%s", after)
	}
}

func TestSetupValidatesConflictsBeforeAnyWrite(t *testing.T) {
	repo, resolver := hostGitFreeNestedFixture(t)
	writeHostFile(t, filepath.Join(repo, ".agents", "skills", "demo"), "foreign\n", 0o644)
	if _, err := SetupWithResolver(Options{RepositoryPath: repo}, resolver.ResolveLayout); err == nil || !strings.Contains(err.Error(), "foreign") {
		t.Fatalf("foreign skill did not conflict: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo, "AGENTS.md")); !os.IsNotExist(err) {
		t.Fatal("instruction pointer was written before conflict validation")
	}
	if _, err := SetupWithResolver(Options{RepositoryPath: repo, Runtimes: []string{"unknown"}}, resolver.ResolveLayout); err == nil {
		t.Fatal("unknown runtime was accepted")
	}
	if _, err := SetupWithResolver(Options{RepositoryPath: repo, Runtimes: []string{"codex", "codex"}}, resolver.ResolveLayout); err == nil {
		t.Fatal("duplicate runtime was accepted")
	}
}

func TestSetupRejectsChangedProfileAndMalformedManagedBlockWithoutWrites(t *testing.T) {
	repo, resolver := hostGitFreeNestedFixture(t)
	writeHostFile(t, filepath.Join(repo, ".claude", "agents", "demo.md"), "locally changed profile\n", 0o644)
	if _, err := SetupWithResolver(Options{RepositoryPath: repo, Runtimes: []string{"claude"}}, resolver.ResolveLayout); err == nil || !strings.Contains(err.Error(), "changed profile") {
		t.Fatalf("changed profile did not conflict: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo, "AGENTS.md")); !os.IsNotExist(err) {
		t.Fatal("instruction pointer was written before the profile conflict")
	}

	if err := os.Remove(filepath.Join(repo, ".claude", "agents", "demo.md")); err != nil {
		t.Fatal(err)
	}
	writeHostFile(t, filepath.Join(repo, "AGENTS.md"), pointerBegin+"\nunterminated\n", 0o644)
	if _, err := SetupWithResolver(Options{RepositoryPath: repo, Runtimes: []string{"claude"}}, resolver.ResolveLayout); err == nil || !strings.Contains(err.Error(), "malformed managed pointer") {
		t.Fatalf("malformed managed block did not conflict: %v", err)
	}
}

func TestSetupCopyModeInAdoptedRootPreservesCanonicalInstructionsAndGitSteering(t *testing.T) {
	repo, _ := hostFixture(t, false)
	writeHostFile(t, filepath.Join(repo, "AGENTS.md"), "canonical adopted contract\n", 0o644)
	poison, _ := hostFixture(t, false)
	t.Setenv("GIT_DIR", filepath.Join(poison, ".git"))
	t.Setenv("GIT_WORK_TREE", poison)
	result, err := Setup(Options{RepositoryPath: filepath.Join(repo, "skills"), Runtimes: []string{"devin", "codex"}, CopySkills: true})
	if err != nil {
		t.Fatal(err)
	}
	wantRepo, err := filepath.EvalSymlinks(repo)
	if err != nil {
		t.Fatal(err)
	}
	if result.Layout.RepositoryRoot != wantRepo {
		t.Fatalf("Git steering selected %s, want %s", result.Layout.RepositoryRoot, wantRepo)
	}
	info, err := os.Lstat(filepath.Join(repo, ".agents", "skills", "demo"))
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		t.Fatalf("copy mode did not copy: %v %v", info, err)
	}
	data, _ := os.ReadFile(filepath.Join(repo, "AGENTS.md"))
	if string(data) != "canonical adopted contract\n" {
		t.Fatalf("adopted instruction changed: %s", data)
	}
	partial := filepath.Join(repo, ".agents", "skills", "demo", "agents", "openai.yaml")
	if err := os.Remove(partial); err != nil {
		t.Fatal(err)
	}
	retry, err := Setup(Options{RepositoryPath: repo, Runtimes: []string{"devin", "codex"}, CopySkills: true})
	if err != nil || len(retry.Changed) != 1 {
		t.Fatalf("copy-mode partial retry = %v, %v", retry.Changed, err)
	}
	writeHostFile(t, filepath.Join(repo, ".devin", "skills", "demo", "SKILL.md"), "locally changed copy\n", 0o640)
	if _, err := Setup(Options{RepositoryPath: repo, Runtimes: []string{"devin"}, CopySkills: true}); err == nil || !strings.Contains(err.Error(), "differs from its source") {
		t.Fatalf("changed copied skill did not conflict: %v", err)
	}
	writeHostFile(t, filepath.Join(repo, ".codex", "hooks.json"), `{"hooks":`, 0o644)
	if _, err := Setup(Options{RepositoryPath: repo, Runtimes: []string{"codex"}}); err == nil {
		t.Fatal("malformed existing JSON was accepted")
	}
}

func TestGeneratedHookCommandIgnoresGitSteeringEnvironment(t *testing.T) {
	for _, nested := range []bool{false, true} {
		t.Run(map[bool]string{false: "adopted", true: "nested"}[nested], func(t *testing.T) {
			repo, installation := hostFixture(t, nested)
			capture := filepath.Join(t.TempDir(), "hook capture")
			script := "#!/usr/bin/env bash\nprintf '%s\\n' \"$PWD|$*\" >\"$HOOK_CAPTURE\"\n"
			writeHostFile(t, filepath.Join(installation, "scripts", "agents", "supervision-hook.sh"), script, 0o755)
			if _, err := Setup(Options{RepositoryPath: repo, Runtimes: []string{"codex"}}); err != nil {
				t.Fatal(err)
			}
			var settings struct {
				Hooks map[string][]struct {
					Hooks []struct {
						Command string `json:"command"`
					} `json:"hooks"`
				} `json:"hooks"`
			}
			data, err := os.ReadFile(filepath.Join(repo, ".codex", "hooks.json"))
			if err != nil || json.Unmarshal(data, &settings) != nil {
				t.Fatalf("read rendered settings: %v", err)
			}
			command := settings.Hooks["SessionStart"][0].Hooks[0].Command
			poison, _ := hostFixture(t, false)
			subdir := filepath.Join(repo, "directory with spaces")
			if err := os.MkdirAll(subdir, 0o755); err != nil {
				t.Fatal(err)
			}
			invoke := exec.Command("bash", "-c", command)
			invoke.Dir = subdir
			invoke.Env = append(os.Environ(), "GIT_DIR="+filepath.Join(poison, ".git"), "GIT_WORK_TREE="+poison, "HOOK_CAPTURE="+capture)
			if output, err := invoke.CombinedOutput(); err != nil {
				t.Fatalf("execute generated command: %v: %s", err, output)
			}
			observed, err := os.ReadFile(capture)
			if err != nil {
				t.Fatal(err)
			}
			wantDirectory := repo
			if nested {
				wantDirectory = installation
			}
			wantDirectory, _ = filepath.EvalSymlinks(wantDirectory)
			if strings.TrimSpace(string(observed)) != wantDirectory+"|codex start" {
				t.Fatalf("generated command selected wrong root: %s", observed)
			}
		})
	}
}

func TestGeneratedFreshNonGitLifecycleNoopsOnlyBeforeHandlerInvocation(t *testing.T) {
	installation := filepath.Join(t.TempDir(), "fresh non-git installation")
	populateHostInstallation(t, installation)
	capture := filepath.Join(t.TempDir(), "handler capture")
	script := "#!/usr/bin/env bash\nprintf '%s\\n' \"$PWD|$*\" >\"$HOOK_CAPTURE\"\nif [[ ${HOOK_FAIL:-0} == 1 ]]; then echo \"handler failed: $*\" >&2; exit 23; fi\n"
	writeHostFile(t, filepath.Join(installation, "scripts", "agents", "supervision-hook.sh"), script, 0o755)
	if _, err := Setup(Options{RepositoryPath: installation, Runtimes: []string{"claude"}}); err != nil {
		t.Fatal(err)
	}

	var settings struct {
		Hooks map[string][]struct {
			Hooks []struct {
				Command string `json:"command"`
			} `json:"hooks"`
		} `json:"hooks"`
	}
	data, err := os.ReadFile(filepath.Join(installation, ".claude", "settings.json"))
	if err != nil || json.Unmarshal(data, &settings) != nil {
		t.Fatalf("read fresh rendered settings: %v", err)
	}
	commands := map[string]string{}
	for _, groups := range settings.Hooks {
		for _, group := range groups {
			for _, handler := range group.Hooks {
				for _, action := range []string{"start", "stop", "end"} {
					if strings.Contains(handler.Command, "supervision-hook.sh claude "+action) {
						commands[action] = handler.Command
					}
				}
			}
		}
	}
	for _, action := range []string{"start", "stop", "end"} {
		if commands[action] == "" {
			t.Fatalf("fresh settings omitted %s command", action)
		}
	}

	run := func(command string) ([]byte, []byte, error) {
		invoke := exec.Command("bash", "-c", command)
		invoke.Dir = installation
		invoke.Env = append(os.Environ(), "HOOK_CAPTURE="+capture, "HOOK_FAIL=1")
		var stdout, stderr bytes.Buffer
		invoke.Stdout = &stdout
		invoke.Stderr = &stderr
		err := invoke.Run()
		return stdout.Bytes(), stderr.Bytes(), err
	}
	// A Stop launcher that fails before the hook can answer allows the
	// stop under a degraded notice naming its own bootstrap; it never
	// blocks and never stays silent.
	assertDegradedAllowance := func(stdout []byte) {
		t.Helper()
		var verdict struct {
			Decision      string `json:"decision"`
			SystemMessage string `json:"systemMessage"`
		}
		if err := json.Unmarshal(bytes.TrimSpace(stdout), &verdict); err != nil || verdict.Decision != "" ||
			!strings.Contains(verdict.SystemMessage, "hook-bootstrap-failed") || !strings.Contains(verdict.SystemMessage, "Stop allowed") {
			t.Fatalf("Stop output is not a degraded JSON allowance: %q, %v", stdout, err)
		}
	}
	for _, action := range []string{"start", "end"} {
		stdout, stderr, err := run(commands[action])
		if err != nil || len(stdout) != 0 || len(stderr) != 0 {
			t.Fatalf("fresh non-Git %s = stdout %q, stderr %q, error %v; want quiet success", action, stdout, stderr, err)
		}
	}
	if _, err := os.Stat(capture); !os.IsNotExist(err) {
		t.Fatalf("fresh non-Git lifecycle invoked its handler: %v", err)
	}
	stdout, _, err := run(commands["stop"])
	if err != nil {
		t.Fatalf("fresh non-Git Stop command failed instead of reporting its verdict: %v", err)
	}
	assertDegradedAllowance(stdout)
	if _, err := os.Stat(capture); !os.IsNotExist(err) {
		t.Fatalf("fresh non-Git Stop unexpectedly reached its handler: %v", err)
	}

	writeHostFile(t, filepath.Join(installation, ".git"), "gitdir: /definitely/missing/metasystem-git-dir\n", 0o644)
	if stdout, stderr, err := run(commands["start"]); err == nil || len(stdout) != 0 || len(stderr) == 0 {
		t.Fatalf("malformed Git repository was treated as absent: stdout %q, stderr %q, error %v", stdout, stderr, err)
	}
	if err := os.Remove(filepath.Join(installation, ".git")); err != nil {
		t.Fatal(err)
	}

	init := exec.Command("git", "init", "-q", "-b", "main", installation)
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "GIT_") {
			init.Env = append(init.Env, entry)
		}
	}
	if output, err := init.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, output)
	}
	for _, action := range []string{"start", "end"} {
		_ = os.Remove(capture)
		stdout, stderr, err := run(commands[action])
		exit, ok := err.(*exec.ExitError)
		if !ok || exit.ExitCode() != 23 || len(stdout) != 0 || !strings.Contains(string(stderr), "handler failed: claude "+action) {
			t.Fatalf("Git %s handler failure = stdout %q, stderr %q, error %v; want handler exit 23", action, stdout, stderr, err)
		}
	}
	_ = os.Remove(capture)
	stdout, stderr, err := run(commands["stop"])
	if err != nil || !strings.Contains(string(stderr), "handler failed: claude stop") {
		t.Fatalf("Git Stop handler failure lost its degraded allowance: stdout %q, stderr %q, error %v", stdout, stderr, err)
	}
	assertDegradedAllowance(stdout)

	if err := os.Remove(filepath.Join(installation, "scripts", "agents", "supervision-hook.sh")); err != nil {
		t.Fatal(err)
	}
	if stdout, stderr, err := run(commands["start"]); err == nil || len(stdout) != 0 || !strings.Contains(string(stderr), "supervision-hook.sh") {
		t.Fatalf("missing invoked handler was hidden: stdout %q, stderr %q, error %v", stdout, stderr, err)
	}
}

func TestGeneratedShippedClaudeStopLauncherAllowsDegradedOutsideGit(t *testing.T) {
	t.Parallel()
	repo, resolver := hostGitFreeNestedFixture(t)
	if _, err := SetupWithResolver(Options{RepositoryPath: repo, Runtimes: []string{"claude"}}, resolver.ResolveLayout); err != nil {
		t.Fatal(err)
	}
	commands := generatedClaudeCommands(t, filepath.Join(repo, ".claude", "settings.json"), "stop")
	if len(commands) != 1 {
		t.Fatalf("generated shipped settings contain %d Claude Stop commands; want one", len(commands))
	}
	nonGit := filepath.Join(t.TempDir(), "separate non-Git cwd")
	if err := os.MkdirAll(nonGit, 0o755); err != nil {
		t.Fatal(err)
	}
	physicalNonGit, err := filepath.EvalSymlinks(nonGit)
	if err != nil {
		t.Fatal(err)
	}
	privateBin := t.TempDir()
	gitCalls := filepath.Join(t.TempDir(), "git-calls")
	writeHostFile(t, filepath.Join(privateBin, "git"), `#!/bin/sh
if [ "$(pwd -P)" != "$HOSTSETUP_EXPECT_GIT_DIR" ] || [ "$#" -ne 2 ] ||
   [ "$1" != rev-parse ] || [ "$2" != --show-toplevel ]; then
  printf 'unexpected git call: %s |' "$(pwd -P)" >> "$HOSTSETUP_GIT_CALLS"
  printf ' <%s>' "$@" >> "$HOSTSETUP_GIT_CALLS"
  printf '\n' >> "$HOSTSETUP_GIT_CALLS"
  exit 97
fi
printf 'consumed\n' >> "$HOSTSETUP_GIT_CALLS"
exit 128
`, 0o755)
	command := exec.Command("bash", "-c", commands[0])
	command.Dir = nonGit
	command.Env = append(hostSetupCleanEnvironment(),
		"PATH="+privateBin+string(os.PathListSeparator)+os.Getenv("PATH"),
		"HOSTSETUP_EXPECT_GIT_DIR="+physicalNonGit,
		"HOSTSETUP_GIT_CALLS="+gitCalls)
	output, err := command.Output()
	if err != nil {
		t.Fatalf("generated Stop launcher returned an error instead of a safe verdict: %v", err)
	}
	calls, err := os.ReadFile(gitCalls)
	if err != nil || string(calls) != "consumed\n" {
		t.Fatalf("generated Stop launcher Git calls = %q, %v; want one negative discovery", calls, err)
	}
	var verdict struct {
		Decision      string `json:"decision"`
		Reason        string `json:"reason"`
		SystemMessage string `json:"systemMessage"`
	}
	if err := json.Unmarshal(bytes.TrimSpace(output), &verdict); err != nil {
		t.Fatalf("generated Stop launcher did not emit one JSON object: %q: %v", output, err)
	}
	// The installed line is the one seats run: a launcher that fails in its
	// own bootstrap allows the stop under a degraded notice and never blocks.
	if verdict.Decision != "" || verdict.Reason != "" ||
		!strings.Contains(verdict.SystemMessage, "hook-bootstrap-failed") || !strings.Contains(verdict.SystemMessage, "Stop allowed") {
		t.Fatalf("generated Stop launcher verdict = %#v", verdict)
	}
}

func TestGeneratedShippedClaudeStopHasNoSeparateReceiptHandler(t *testing.T) {
	repo, resolver := hostGitFreeNestedFixture(t)
	if _, err := SetupWithResolver(Options{RepositoryPath: repo, Runtimes: []string{"claude"}}, resolver.ResolveLayout); err != nil {
		t.Fatal(err)
	}
	settings := filepath.Join(repo, ".claude", "settings.json")
	if commands := generatedClaudeCommands(t, settings, "receipt"); len(commands) != 0 {
		t.Fatalf("generated shipped settings contain %d separate Claude receipt commands; want none", len(commands))
	}
	if commands := generatedClaudeCommands(t, settings, "stop"); len(commands) != 1 {
		t.Fatalf("generated shipped settings contain %d Claude Stop commands; want one", len(commands))
	}
}

func TestHostSetupIdentityEngineHelper(t *testing.T) {
	if os.Getenv("HOSTSETUP_IDENTITY_HELPER") != "1" {
		return
	}
	args := flag.Args()
	realEngine := os.Getenv("HOSTSETUP_REAL_ENGINE")
	if len(args) >= 2 && args[0] == "proc" && args[1] == "find-ancestor" {
		pid := ""
		for index := 2; index+1 < len(args); index++ {
			if args[index] == "--pid" {
				pid = args[index+1]
				break
			}
		}
		pidNumber, parseErr := strconv.Atoi(pid)
		startedOutput, startedErr := exec.Command(realEngine, "proc", "started-at", "--pid", pid).Output()
		started, startedParseErr := strconv.ParseInt(strings.TrimSpace(string(startedOutput)), 10, 64)
		if parseErr != nil || startedErr != nil || startedParseErr != nil {
			fmt.Fprintf(os.Stderr, "fixture identity unavailable: pid=%q parse=%v started=%v start-parse=%v\n", pid, parseErr, startedErr, startedParseErr)
			os.Exit(1)
		}
		if err := json.NewEncoder(os.Stdout).Encode(map[string]any{"runtime": "claude", "pid": pidNumber, "pidStartedAt": started}); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0)
	}
	command := exec.Command(realEngine, args...)
	command.Stdin = os.Stdin
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	if err := command.Run(); err != nil {
		if exit, ok := err.(*exec.ExitError); ok {
			os.Exit(exit.ExitCode())
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(0)
}

func generatedClaudeCommands(t *testing.T, settingsPath, action string) []string {
	t.Helper()
	var settings struct {
		Hooks map[string][]struct {
			Hooks []struct {
				Command string `json:"command"`
			} `json:"hooks"`
		} `json:"hooks"`
	}
	data, err := os.ReadFile(settingsPath)
	if err != nil || json.Unmarshal(data, &settings) != nil {
		t.Fatalf("read generated Claude settings: %v", err)
	}
	var commands []string
	for _, groups := range settings.Hooks {
		for _, group := range groups {
			for _, handler := range group.Hooks {
				if strings.Contains(handler.Command, "supervision-hook.sh claude "+action) {
					commands = append(commands, handler.Command)
				}
			}
		}
	}
	return commands
}

func hostSetupModuleRoot(t *testing.T) string {
	t.Helper()
	directory, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(directory, "go.mod")); err == nil {
			return directory
		}
		parent := filepath.Dir(directory)
		if parent == directory {
			t.Fatal("could not locate the module root")
		}
		directory = parent
	}
}

func copyHostSourceFile(t *testing.T, root, relative, destination string, mode os.FileMode) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, relative))
	if err != nil {
		t.Fatal(err)
	}
	writeHostFile(t, destination, string(data), mode)
}

func hostSetupCleanEnvironment() []string {
	var environment []string
	for _, entry := range os.Environ() {
		if strings.HasPrefix(entry, "GIT_") || strings.HasPrefix(entry, "METASYSTEM_HOOK_DELEGATE_") || strings.HasPrefix(entry, "METASYSTEM_BIN=") {
			continue
		}
		environment = append(environment, entry)
	}
	return environment
}

func TestSetupSupportsNestedAdoptedInstallationWithoutOwningParent(t *testing.T) {
	repo, _ := hostFixture(t, false)
	relative := `nested $(touch METASYSTEM_PATH_EXECUTED); "$HOME" and spaces`
	installation := filepath.Join(repo, relative)
	populateHostInstallation(t, installation)
	parentSettings := filepath.Join(repo, ".codex", "hooks.json")
	writeHostFile(t, parentSettings, "{\n  \"foreign\": true\n}\n", 0o640)
	writeHostFile(t, filepath.Join(repo, "AGENTS.md"), "parent instructions\n", 0o600)
	writeHostFile(t, filepath.Join(installation, "AGENTS.md"), "nested instructions\n", 0o640)
	writeHostFile(t, filepath.Join(installation, ".codex", "hooks.json"), hostHookFixture(t, "codex"), 0o640)
	capture := filepath.Join(t.TempDir(), "nested hook capture")
	writeHostFile(t, filepath.Join(installation, "scripts", "agents", "supervision-hook.sh"), "#!/usr/bin/env bash\nprintf '%s\\n' \"$PWD|$*\" >\"$HOOK_CAPTURE\"\n", 0o755)

	result, err := Setup(Options{RepositoryPath: filepath.Join(installation, "skills", "demo")})
	if err != nil {
		t.Fatal(err)
	}
	wantRepo, _ := filepath.EvalSymlinks(repo)
	wantInstallation, _ := filepath.EvalSymlinks(installation)
	if result.Layout.GitRoot != wantRepo || result.Layout.RepositoryRoot != wantInstallation || result.Layout.InstallationRoot != wantInstallation || result.Layout.InstallationRel != filepath.ToSlash(relative) || result.Layout.Template {
		t.Fatalf("nested adopted setup selected the wrong layout: %+v", result.Layout)
	}
	if strings.Join(result.Runtimes, ",") != "codex,devin,claude" {
		t.Fatalf("nested adopted default runtimes = %v", result.Runtimes)
	}
	for _, path := range []string{".agents/skills/demo", ".devin/skills/demo", ".claude/skills/demo", ".claude/agents/demo.md", ".devin/agents/demo/AGENT.md", ".codex/hooks.json", ".devin/config.json", ".claude/settings.json"} {
		if _, err := os.Lstat(filepath.Join(installation, path)); err != nil {
			t.Errorf("nested adopted registration missing %s: %v", path, err)
		}
	}
	parentAfter, err := os.ReadFile(parentSettings)
	if err != nil || string(parentAfter) != "{\n  \"foreign\": true\n}\n" {
		t.Fatalf("parent installation settings changed: %q, %v", parentAfter, err)
	}
	parentInstructions, _ := os.ReadFile(filepath.Join(repo, "AGENTS.md"))
	nestedInstructions, _ := os.ReadFile(filepath.Join(installation, "AGENTS.md"))
	if string(parentInstructions) != "parent instructions\n" || string(nestedInstructions) != "nested instructions\n" {
		t.Fatalf("adopted instructions changed: parent=%q nested=%q", parentInstructions, nestedInstructions)
	}
	if _, err := Setup(Options{RepositoryPath: filepath.Join(installation, "skills"), Check: true}); err != nil {
		t.Fatalf("nested adopted check failed: %v", err)
	}
	again, err := Setup(Options{RepositoryPath: installation})
	if err != nil || len(again.Changed) != 0 {
		t.Fatalf("nested adopted repeat changed %v: %v", again.Changed, err)
	}

	var settings struct {
		Hooks map[string][]struct {
			Hooks []struct {
				Command string `json:"command"`
			} `json:"hooks"`
		} `json:"hooks"`
	}
	data, err := os.ReadFile(filepath.Join(installation, ".codex", "hooks.json"))
	if err != nil || json.Unmarshal(data, &settings) != nil {
		t.Fatalf("read nested rendered settings: %v", err)
	}
	command := settings.Hooks["SessionStart"][0].Hooks[0].Command
	if !strings.Contains(command, `\$(touch METASYSTEM_PATH_EXECUTED)`) || !strings.Contains(command, `\$HOME`) || !strings.Contains(command, `\"`) {
		t.Fatalf("nested installation path was not shell-quoted as data: %s", command)
	}
	launchDirectory := filepath.Join(repo, "launch directory")
	if err := os.MkdirAll(launchDirectory, 0o755); err != nil {
		t.Fatal(err)
	}
	invoke := exec.Command("bash", "-c", command)
	invoke.Dir = launchDirectory
	invoke.Env = append(os.Environ(), "HOOK_CAPTURE="+capture)
	if output, err := invoke.CombinedOutput(); err != nil {
		t.Fatalf("execute nested generated command: %v: %s", err, output)
	}
	observed, err := os.ReadFile(capture)
	if err != nil || strings.TrimSpace(string(observed)) != wantInstallation+"|codex start" {
		t.Fatalf("nested generated command selected wrong installation: %q, %v", observed, err)
	}
	if _, err := os.Stat(filepath.Join(launchDirectory, "METASYSTEM_PATH_EXECUTED")); !os.IsNotExist(err) {
		t.Fatalf("nested path bytes executed as shell code: %v", err)
	}
	gone := installation + ".missing"
	if err := os.Rename(installation, gone); err != nil {
		t.Fatal(err)
	}
	invoke = exec.Command("bash", "-c", command)
	invoke.Dir = launchDirectory
	invoke.Env = append(os.Environ(), "HOOK_CAPTURE="+capture)
	if output, err := invoke.CombinedOutput(); err == nil || len(output) == 0 {
		t.Fatalf("missing selected installation was treated as absent Git: output %q, error %v", output, err)
	}
}

func TestSetupNestedAdoptedSelectionModesAndConflictAreLocal(t *testing.T) {
	repo, _ := hostFixture(t, false)

	copyInstallation := filepath.Join(repo, "copy installation")
	populateHostInstallation(t, copyInstallation)
	if _, err := Setup(Options{RepositoryPath: copyInstallation, Runtimes: []string{"claude", "codex"}, CopySkills: true}); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{".claude/skills/demo", ".agents/skills/demo"} {
		info, err := os.Lstat(filepath.Join(copyInstallation, path))
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			t.Fatalf("nested copied registration %s = %v, %v", path, info, err)
		}
	}
	if _, err := Setup(Options{RepositoryPath: filepath.Join(copyInstallation, "skills"), Runtimes: []string{"claude", "codex"}, CopySkills: true, Check: true}); err != nil {
		t.Fatalf("nested copied check failed: %v", err)
	}

	noneInstallation := filepath.Join(repo, "none installation")
	populateHostInstallation(t, noneInstallation)
	result, err := Setup(Options{RepositoryPath: noneInstallation, Runtimes: []string{"none"}})
	if err != nil || len(result.Runtimes) != 0 || len(result.Changed) != 0 {
		t.Fatalf("nested none selection = %+v, %v", result, err)
	}
	if _, err := os.Stat(filepath.Join(noneInstallation, ".claude")); !os.IsNotExist(err) {
		t.Fatalf("nested none selection wrote runtime state: %v", err)
	}

	conflictInstallation := filepath.Join(repo, "conflict installation")
	populateHostInstallation(t, conflictInstallation)
	writeHostFile(t, filepath.Join(conflictInstallation, ".agents", "skills", "demo"), "foreign\n", 0o644)
	if _, err := Setup(Options{RepositoryPath: conflictInstallation, Runtimes: []string{"codex"}}); err == nil || !strings.Contains(err.Error(), "foreign") {
		t.Fatalf("nested foreign registration did not conflict: %v", err)
	}
	if _, err := os.Stat(filepath.Join(conflictInstallation, ".codex", "hooks.json")); !os.IsNotExist(err) {
		t.Fatalf("nested conflict wrote hook settings before refusing: %v", err)
	}
}

func TestSetupPreservesExistingInstructionAndConfigurationModes(t *testing.T) {
	repo, resolver := hostGitFreeNestedFixture(t)
	writeHostFile(t, filepath.Join(repo, "AGENTS.md"), "existing instructions\n", 0o600)
	writeHostFile(t, filepath.Join(repo, ".claude", "settings.json"), `{}`, 0o640)
	if _, err := SetupWithResolver(Options{RepositoryPath: repo, Runtimes: []string{"claude"}}, resolver.ResolveLayout); err != nil {
		t.Fatal(err)
	}
	for path, want := range map[string]os.FileMode{
		"AGENTS.md": 0o600, ".claude/settings.json": 0o640,
	} {
		info, err := os.Stat(filepath.Join(repo, path))
		if err != nil || info.Mode().Perm() != want {
			t.Fatalf("%s mode = %v, %v; want %o", path, info, err, want)
		}
	}
}

func TestSetupResolverRouting(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "missing")
	if _, err := Setup(Options{RepositoryPath: path}); err == nil || !strings.Contains(err.Error(), "inspect repository path") {
		t.Fatalf("default resolver result: %v", err)
	}
	called := 0
	_, err := SetupWithResolver(Options{RepositoryPath: path}, func(seen string) (stateroot.Layout, error) {
		called++
		if seen != path {
			t.Fatalf("injected resolver path = %q", seen)
		}
		return stateroot.Layout{}, errors.New("injected resolver")
	})
	if called != 1 || err == nil || !strings.Contains(err.Error(), "injected resolver") {
		t.Fatalf("injected resolver result: calls %d, error %v", called, err)
	}
}
