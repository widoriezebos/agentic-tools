package validate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// newFileConformanceFixture supplies the files needed by record consumers
// without creating a repository or running a command.
func newFileConformanceFixture(t *testing.T) *conformanceFixture {
	t.Helper()
	root := t.TempDir()
	f := &conformanceFixture{
		t:          t,
		controller: filepath.Join(root, "controller"),
		worktree:   filepath.Join(root, "worktree"),
		baseSha:    strings.Repeat("a", 40),
	}
	pathClasses, err := os.ReadFile("../../scripts/agents/path-classes.txt")
	if err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{
		"scripts/agents/path-classes.txt": pathClasses,
		".gitignore":                      []byte("artifacts/\nlocal.conf\n"),
		"source.txt":                      []byte("base\n"),
		"docs/note.md":                    []byte("base\n"),
		"metasystem.conf":                 []byte("metasystem.version=1\nrole.code-critic.runtime=fake\n"),
	}
	for _, dir := range []string{f.controller, f.worktree} {
		for name, data := range files {
			path := filepath.Join(dir, name)
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, data, 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(path, 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	return f
}
