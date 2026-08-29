package stateroot

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func installFixture(t *testing.T, template bool) (installation, app string) {
	t.Helper()
	app = t.TempDir()
	installation = app
	if template {
		installation = filepath.Join(app, "metasystem")
		if err := os.MkdirAll(filepath.Join(app, "development"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(app, "development", "metasystem-design.md"), []byte("design\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(installation, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(installation, "metasystem.conf"), []byte("evidence.root="+filepath.Join(app, "durable")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return installation, app
}

func withRoots(t *testing.T, installation, app string) {
	t.Helper()
	priorExecutable := executablePath
	priorTop := repositoryTop
	executablePath = func() (string, error) { return filepath.Join(installation, "bin", "metasystem"), nil }
	repositoryTop = func(string) (string, error) { return app, nil }
	t.Cleanup(func() {
		executablePath = priorExecutable
		repositoryTop = priorTop
	})
}

func TestStateRootResolvesEveryKindInTemplateAndAdoptedModes(t *testing.T) {
	tests := []struct {
		kind Kind
		rel  string
	}{
		{Registers, "memory"}, {Receipts, "memory"}, {Records, "records"},
		{Goals, "plans/goals"}, {OpenWork, "plans"},
		{Steward, "artifacts/agents/steward"},
	}
	for _, template := range []bool{true, false} {
		t.Run(map[bool]string{true: "template", false: "adopted"}[template], func(t *testing.T) {
			installation, app := installFixture(t, template)
			withRoots(t, installation, app)
			base := app
			if template {
				base = installation
			}
			gotBase, err := RootForInstallation(installation)
			if err != nil || gotBase != base {
				t.Fatalf("RootForInstallation() = %q, %v; want %q", gotBase, err, base)
			}
			for _, test := range tests {
				got, err := StateRoot(test.kind)
				if err != nil || got != filepath.Join(base, filepath.FromSlash(test.rel)) {
					t.Errorf("StateRoot(%q) = %q, %v; want %q", test.kind, got, err, filepath.Join(base, filepath.FromSlash(test.rel)))
				}
			}
			got, err := StateRoot(Evidence)
			if err != nil || got != filepath.Join(app, "durable") {
				t.Errorf("StateRoot(%q) = %q, %v; want configured durable root", Evidence, got, err)
			}
		})
	}
}

func TestStateRootRefusesUnknownKindsAndInvalidInstallationFacts(t *testing.T) {
	if got, err := RelativeRoot(Receipts); err != nil || got != "memory" {
		t.Fatalf("RelativeRoot(%q) = %q, %v; want memory", Receipts, got, err)
	}
	if _, err := RelativeRoot(Kind("unknown")); err == nil {
		t.Fatal("an unknown relative state kind must refuse")
	}
	if _, err := StateRoot(Kind("unknown")); err == nil {
		t.Fatal("an unknown state kind must refuse")
	}
	prior := executablePath
	executablePath = func() (string, error) { return "", errors.New("unavailable") }
	t.Cleanup(func() { executablePath = prior })
	if _, err := StateRoot(Registers); err == nil {
		t.Fatal("an unavailable executable path must refuse")
	}
}

func TestRootForInstallationUsesOnlyTheExactTemplateMarker(t *testing.T) {
	root := t.TempDir()
	design := filepath.Join(root, "development", "metasystem-design.md")
	if err := os.MkdirAll(filepath.Dir(design), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(design, []byte("design\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	template := filepath.Join(root, "metasystem")
	if err := os.MkdirAll(template, 0o755); err != nil {
		t.Fatal(err)
	}
	priorTop := repositoryTop
	repositoryTop = func(string) (string, error) {
		return "", errors.New("template must not consult Git")
	}
	got, err := RootForInstallation(template)
	repositoryTop = priorTop
	if err != nil || got != template {
		t.Fatalf("exact template marker resolved to %q, %v; want %q", got, err, template)
	}

	adopted := filepath.Join(root, "metasystem-copy")
	if err := os.MkdirAll(adopted, 0o755); err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(root, "application")
	repositoryTop = func(seen string) (string, error) {
		if seen != adopted {
			t.Fatalf("repository lookup received %q; want %q", seen, adopted)
		}
		return want, nil
	}
	t.Cleanup(func() { repositoryTop = priorTop })
	got, err = RootForInstallation(adopted)
	if err != nil || got != want {
		t.Fatalf("adopted installation resolved to %q, %v; want %q", got, err, want)
	}
}

func TestRootForInstallationPropagatesAdoptedRepositoryFailure(t *testing.T) {
	installation := filepath.Join(t.TempDir(), "metasystem")
	if err := os.MkdirAll(installation, 0o755); err != nil {
		t.Fatal(err)
	}
	priorTop := repositoryTop
	repositoryTop = func(string) (string, error) { return "", errors.New("not in a repository") }
	t.Cleanup(func() { repositoryTop = priorTop })
	if _, err := RootForInstallation(installation); err == nil || !strings.Contains(err.Error(), "not in a repository") {
		t.Fatalf("adopted repository failure was hidden: %v", err)
	}
}

func TestEvidenceRootMustBeConfiguredAndAbsolute(t *testing.T) {
	installation, app := installFixture(t, false)
	withRoots(t, installation, app)
	if err := os.WriteFile(filepath.Join(app, "metasystem.conf"), []byte("evidence.root=relative\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := StateRoot(Evidence); err == nil {
		t.Fatal("a relative durable evidence root must refuse")
	}
}

func TestStateRootGitDiscoveryIgnoresSteeringEnvironment(t *testing.T) {
	initRepository := func(path string) {
		t.Helper()
		command := exec.Command("git", "-C", path, "init", "-q", "-b", "main")
		command.Env = scrubGitSteering(os.Environ())
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("initialize repository: %v: %s", err, output)
		}
	}
	app := t.TempDir()
	installation := filepath.Join(app, "metasystem")
	if err := os.MkdirAll(filepath.Join(installation, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(installation, "metasystem.conf"), []byte("evidence.root="+filepath.Join(app, "durable")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	initRepository(app)

	poison := t.TempDir()
	initRepository(poison)
	poisonGit := filepath.Join(poison, ".git")
	values := map[string]string{
		"GIT_DIR": poisonGit, "GIT_WORK_TREE": poison, "GIT_COMMON_DIR": poisonGit,
		"GIT_INDEX_FILE": filepath.Join(poisonGit, "index"), "GIT_CEILING_DIRECTORIES": app,
		"GIT_OBJECT_DIRECTORY":             filepath.Join(poisonGit, "objects"),
		"GIT_ALTERNATE_OBJECT_DIRECTORIES": filepath.Join(poisonGit, "objects"),
		"GIT_CONFIG":                       filepath.Join(poisonGit, "config"), "GIT_CONFIG_PARAMETERS": "poison",
		"GIT_CONFIG_COUNT": "1", "GIT_CONFIG_GLOBAL": filepath.Join(poisonGit, "config"),
		"GIT_CONFIG_SYSTEM": filepath.Join(poisonGit, "config"), "GIT_CONFIG_NOSYSTEM": "1",
		"GIT_GRAFT_FILE":   filepath.Join(poisonGit, "info", "grafts"),
		"GIT_SHALLOW_FILE": filepath.Join(poisonGit, "shallow"), "GIT_REPLACE_REF_BASE": "refs/poison/",
		"GIT_DISCOVERY_ACROSS_FILESYSTEM": "0", "GIT_IMPLICIT_WORK_TREE": "0",
		"GIT_NO_REPLACE_OBJECTS": "1", "GIT_PREFIX": "poison/",
	}
	for name, value := range values {
		t.Setenv(name, value)
	}

	priorExecutable := executablePath
	executablePath = func() (string, error) { return filepath.Join(installation, "bin", "metasystem"), nil }
	t.Cleanup(func() { executablePath = priorExecutable })
	got, err := StateRoot(Registers)
	canonicalApp, canonicalErr := filepath.EvalSymlinks(app)
	if canonicalErr != nil {
		t.Fatal(canonicalErr)
	}
	want := filepath.Join(canonicalApp, "memory")
	if err != nil || got != want {
		t.Fatalf("StateRoot(%q) with poisoned Git environment = %q, %v; want %q", Registers, got, err, want)
	}
}
