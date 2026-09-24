package main

import (
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
	"os"
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
	setupCLIWrite(t, filepath.Join(repo, "development", "metasystem-design.md"), "design\n", 0o644)
	setupCLIWrite(t, filepath.Join(installation, "metasystem.conf"), "metasystem.runtimes=claude\n", 0o644)
	if err := os.MkdirAll(filepath.Join(installation, "scripts", "agents"), 0o755); err != nil {
		t.Fatal(err)
	}
	setupCLIWrite(t, filepath.Join(installation, "scripts", "enforcement", "claude-code-hooks.json"), runtimeHookFixture(t, "claude-code"), 0o644)
	setupCLIWrite(t, filepath.Join(installation, "scripts", "enforcement", "codex-hooks.json"), runtimeHookFixture(t, "codex"), 0o644)
	setupCLIWrite(t, filepath.Join(installation, "scripts", "enforcement", "devin-hooks.json"), runtimeHookFixture(t, "devin"), 0o644)
	setupCLIWrite(t, filepath.Join(installation, "skills", "demo", "SKILL.md"), "demo\n", 0o644)
	setupCLIWrite(t, filepath.Join(installation, "skills", "demo", "agents", "claude-profile.md"), "claude\n", 0o644)
	setupCLIWrite(t, filepath.Join(installation, "skills", "demo", "agents", "devin", "AGENT.md"), "devin\n", 0o644)
	setupCLIWrite(t, filepath.Join(installation, "skills", "demo", "agents", "openai.yaml"), "interface: {}\n", 0o644)
	return repo, installation
}

type runtimeLayoutRecorder struct {
	t    *testing.T
	root string
	want []string
	seen []string
}

func newRuntimeLayoutRecorder(t *testing.T, root string) *runtimeLayoutRecorder {
	t.Helper()
	r := &runtimeLayoutRecorder{t: t, root: root}
	t.Cleanup(func() {
		if len(r.want) != 0 {
			t.Errorf("layout requests not consumed: %v; saw %v", r.want, r.seen)
		}
	})
	return r
}

func (r *runtimeLayoutRecorder) expect(path string) { r.want = append(r.want, path) }

func (r *runtimeLayoutRecorder) resolve(path string) (stateroot.Layout, error) {
	r.t.Helper()
	r.seen = append(r.seen, string(append([]byte(nil), path...)))
	if len(r.want) == 0 || r.want[0] != path {
		r.t.Fatalf("unexpected layout request %q; pending %v", path, r.want)
	}
	r.want = r.want[1:]
	var seenTop []string
	layout, resolveErr := stateroot.NewResolver(func(seen string) (string, error) {
		seenTop = append(seenTop, string(append([]byte(nil), seen...)))
		if len(seenTop) != 1 {
			r.t.Fatalf("repeated repository request %q", seen)
		}
		canonical, err := filepath.EvalSymlinks(path)
		if err != nil || seen != canonical {
			r.t.Fatalf("unexpected repository request %q; want %q (%v)", seen, canonical, err)
		}
		return r.root, nil
	}, nil).ResolveLayout(path)
	if len(seenTop) != 1 {
		r.t.Fatalf("repository requests = %v", seenTop)
	}
	return layout, resolveErr
}

func setupCLIWrite(t *testing.T, path, content string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := testexec.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
}

func runtimeHookFixture(t *testing.T, name string) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(filepath.Join(root, "scripts", "enforcement", name+"-hooks.json"))
	if err != nil {
		t.Fatal(err)
	}
	return string(contents)
}

func TestRuntimeSetupCLIConfiguresAllByDefaultChecksAndSelectsNone(t *testing.T) {
	repo, _ := setupCLIFixture(t)
	recorder := newRuntimeLayoutRecorder(t, repo)
	recorder.expect(filepath.Join(repo, "metasystem", "skills"))
	output, code := captureStdout(t, func() int {
		return runRuntimeSetupWithResolver([]string{"--repo", filepath.Join(repo, "metasystem", "skills")}, recorder.resolve)
	})
	if code != 0 {
		t.Fatalf("runtime setup exit = %d", code)
	}
	for _, runtime := range []string{"claude", "codex", "devin"} {
		if !strings.Contains(output, "CONFIG_READY "+runtime) || !strings.Contains(output, "PROVIDER_TRUST "+runtime) || !strings.Contains(output, "LIFECYCLE_OBSERVED "+runtime) {
			t.Fatalf("setup output omitted %s readiness dimensions: %s", runtime, output)
		}
	}
	recorder.expect(repo)
	if _, code := captureStdout(t, func() int { return runRuntimeSetupWithResolver([]string{"--repo", repo, "--check"}, recorder.resolve) }); code != 0 {
		t.Fatalf("setup check exit = %d", code)
	}

	emptyRepo, _ := setupCLIFixture(t)
	emptyRecorder := newRuntimeLayoutRecorder(t, emptyRepo)
	emptyRecorder.expect(emptyRepo)
	emptyOutput, code := captureStdout(t, func() int {
		return runRuntimeSetupWithResolver([]string{"--repo", emptyRepo, "--runtimes", "none"}, emptyRecorder.resolve)
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
	recorder := newRuntimeLayoutRecorder(t, repo)
	recorder.expect(repo)
	if _, code := captureStdout(t, func() int {
		return runRuntimeSetupWithResolver([]string{"--repo", repo, "--runtimes", "unknown"}, recorder.resolve)
	}); code != 1 {
		t.Fatalf("unknown runtime exit = %d, want 1", code)
	}
	recorder.expect(repo)
	if _, code := captureStdout(t, func() int { return runRuntimeSetupWithResolver([]string{"--repo", repo, "--check"}, recorder.resolve) }); code != 1 {
		t.Fatalf("pending check exit = %d, want 1", code)
	}
}

func TestFreshAdoptionSeparatesRuntimeAndTestingReadiness(t *testing.T) {
	repo, installation := setupCLIFixture(t)
	recorder := newRuntimeLayoutRecorder(t, repo)
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
	recorder.expect(repo)
	output, code := captureStdout(t, func() int {
		return runRuntimeSetupWithResolver([]string{"--repo", repo, "--runtimes", "none"}, recorder.resolve)
	})
	if code != 0 || !strings.Contains(output, "TEST_CONTRACT_REQUIRED") || strings.Contains(output, "CONFIG_READY") {
		t.Fatalf("readiness dimensions were conflated: exit=%d output=%q", code, output)
	}
}

func TestRuntimeSetupCLISupportsNestedAdoptedInstallationAndSubdirectory(t *testing.T) {
	app, installation := setupCLIFixture(t)
	recorder := newRuntimeLayoutRecorder(t, app)
	if err := os.Remove(filepath.Join(app, "development", "metasystem-design.md")); err != nil {
		t.Fatal(err)
	}
	recorder.expect(installation)
	output, code := captureStdout(t, func() int {
		return runRuntimeSetupWithResolver([]string{"--repo", installation, "--runtimes", "claude,codex,devin"}, recorder.resolve)
	})
	if code != 0 {
		t.Fatalf("nested adopted setup exit = %d", code)
	}
	wantInstallation, _ := filepath.EvalSymlinks(installation)
	if !strings.Contains(output, "repository="+wantInstallation+" installation="+wantInstallation) {
		t.Fatalf("nested adopted setup reported the wrong owner: %s", output)
	}
	recorder.expect(filepath.Join(installation, "skills", "demo"))
	if _, code := captureStdout(t, func() int {
		return runRuntimeSetupWithResolver([]string{"--repo", filepath.Join(installation, "skills", "demo"), "--runtimes", "claude,codex,devin", "--check"}, recorder.resolve)
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
