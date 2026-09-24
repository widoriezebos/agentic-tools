package missionrunner

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestGitAdapterHealRevParseReadsRepositoryHEAD(t *testing.T) {
	root := t.TempDir()
	for _, key := range []string{"GIT_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE", "GIT_OBJECT_DIRECTORY", "GIT_ALTERNATE_OBJECT_DIRECTORIES", "GIT_COMMON_DIR", "GIT_CONFIG_COUNT"} {
		old, present := os.LookupEnv(key)
		if err := os.Unsetenv(key); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if present {
				_ = os.Setenv(key, old)
			} else {
				_ = os.Unsetenv(key)
			}
		})
	}
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(root, "missing-global-config"))
	git := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %q: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	git("init", "-q")
	if err := os.WriteFile(filepath.Join(root, "README"), []byte("seed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", "README")
	git("-c", "user.name=test", "-c", "user.email=test@example.com", "commit", "-q", "-m", "seed")
	want := git("rev-parse", "HEAD")
	if len(want) != 40 {
		t.Fatalf("repository HEAD %q is not a 40-hex commit", want)
	}
	got, err := NewEngine(root, "demo").gitRevParse("HEAD")
	if err != nil || got != want {
		t.Fatalf("default Engine.gitRevParse(HEAD) = %q, %v; repository HEAD = %q", got, err, want)
	}
}
