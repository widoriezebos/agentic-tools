package validate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func isolationFixture(t *testing.T) (source, destination, manifest string) {
	t.Helper()
	base := t.TempDir()
	source = filepath.Join(base, "primary")
	destination = filepath.Join(base, "second")
	writeFile(t, filepath.Join(source, ".claude", "settings.local.json"), `{"key":"value"}`)
	writeFile(t, filepath.Join(source, ".config", "runtime", "state.json"), `{"deep":true}`)
	if err := os.MkdirAll(destination, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest = filepath.Join(base, "paths.txt")
	writeFile(t, manifest, ".claude/settings.local.json\n.config\nmissing/path.json\n")
	return source, destination, manifest
}

func TestSessionIsolationCopiesAndResolvesHarness(t *testing.T) {
	source, destination, manifest := isolationFixture(t)
	got, err := SessionIsolation(source, destination, manifest, source)
	if err != nil {
		t.Fatal(err)
	}
	want, err := filepath.EvalSymlinks(destination)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("new harness = %s, want %s", got, want)
	}
	copied := readFile(t, filepath.Join(destination, ".claude", "settings.local.json"))
	if copied != `{"key":"value"}` {
		t.Fatalf("declared file was not copied: %q", copied)
	}
	deep := readFile(t, filepath.Join(destination, ".config", "runtime", "state.json"))
	if deep != `{"deep":true}` {
		t.Fatalf("declared directory was not copied: %q", deep)
	}
}

func TestSessionIsolationKeepsExistingTargets(t *testing.T) {
	source, destination, manifest := isolationFixture(t)
	writeFile(t, filepath.Join(destination, ".claude", "settings.local.json"), `{"mine":true}`)
	if _, err := SessionIsolation(source, destination, manifest, source); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, filepath.Join(destination, ".claude", "settings.local.json")); got != `{"mine":true}` {
		t.Fatalf("an existing target must never be overwritten, got %q", got)
	}
}

func TestSessionIsolationRejectsUnsafeManifestPath(t *testing.T) {
	source, destination, manifest := isolationFixture(t)
	writeFile(t, manifest, "../escape.json\n")
	_, err := SessionIsolation(source, destination, manifest, source)
	if err == nil || !strings.Contains(err.Error(), "adapter local-config-path is unsafe: ../escape.json") {
		t.Fatalf("err = %v, want an unsafe-path refusal", err)
	}
}

func TestSessionIsolationRejectsSymlinkIntoPrimary(t *testing.T) {
	source, destination, manifest := isolationFixture(t)
	if err := os.MkdirAll(filepath.Join(destination, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(source, ".claude", "settings.local.json"),
		filepath.Join(destination, ".claude", "settings.local.json")); err != nil {
		t.Fatal(err)
	}
	_, err := SessionIsolation(source, destination, manifest, source)
	if err == nil || !strings.Contains(err.Error(), "resolves outside the new worktree") {
		t.Fatalf("err = %v, want an isolation audit failure", err)
	}
}

func TestSessionIsolationRejectsSharedHarnessRoot(t *testing.T) {
	source, destination, manifest := isolationFixture(t)
	// A harness above the checkout resolves to itself from both
	// sessions: one shared artifacts root, which isolation forbids.
	shared := filepath.Dir(source)
	_, err := SessionIsolation(source, destination, manifest, shared)
	if err == nil || !strings.Contains(err.Error(), "both sessions resolve one metasystem artifacts root") {
		t.Fatalf("err = %v, want a shared-root refusal", err)
	}
}
