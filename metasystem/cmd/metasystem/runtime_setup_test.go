package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

func setupCLIFixture(t *testing.T) (repo, installation string) {
	t.Helper()
	repo = filepath.Join(t.TempDir(), "host repository with spaces")
	installation = filepath.Join(repo, "metasystem")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	setupCLIWrite(t, filepath.Join(repo, "development", "metasystem-design.md"), "design\n", 0o644)
	setupCLIWrite(t, filepath.Join(installation, "metasystem.conf"), "metasystem.runtimes=claude\nmetasystem.template=true\n", 0o644)
	if err := os.MkdirAll(installation, 0o755); err != nil {
		t.Fatal(err)
	}
	if marker, err := os.OpenFile(filepath.Join(installation, "metasystem.conf"), os.O_CREATE|os.O_WRONLY, 0o644); err != nil {
		t.Fatal(err)
	} else {
		marker.Close()
	}
	setupCLIWrite(t, filepath.Join(installation, "internal", "runtimes", "enforcement", "claude-code-hooks.json"), runtimeHookFixture(t, "claude-code"), 0o644)
	setupCLIWrite(t, filepath.Join(installation, "internal", "runtimes", "enforcement", "codex-hooks.json"), runtimeHookFixture(t, "codex"), 0o644)
	setupCLIWrite(t, filepath.Join(installation, "internal", "runtimes", "enforcement", "devin-hooks.json"), runtimeHookFixture(t, "devin"), 0o644)
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
		repositoryProbe := path
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			repositoryProbe = filepath.Dir(path)
		}
		canonical, err := filepath.EvalSymlinks(repositoryProbe)
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
	contents, err := os.ReadFile(filepath.Join(root, "internal", "runtimes", "enforcement", name+"-hooks.json"))
	if err != nil {
		t.Fatal(err)
	}
	return string(contents)
}
