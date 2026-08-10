package census

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPathBelow(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "a", "b")
	os.MkdirAll(sub, 0o755)
	if !PathBelow(sub, root) {
		t.Fatal("a subdir must be below root")
	}
	if !PathBelow(root, root) {
		t.Fatal("root is below itself (relative_to succeeds)")
	}
	outside := t.TempDir()
	if PathBelow(outside, root) {
		t.Fatal("a sibling dir must not be below root")
	}
	// A non-existent path UNDER root is still "below" — realpath resolves the
	// existing prefix and relative_to succeeds, exactly as python path_below.
	if !PathBelow(filepath.Join(root, "does-not-exist"), root) {
		t.Fatal("a non-existent path under root is below (prefix resolves)")
	}
	// A path OUTSIDE root is not below.
	if PathBelow(filepath.Join(outside, "x"), root) {
		t.Fatal("a path outside root is not below")
	}
}

func TestShellSplit(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{`a b c`, []string{"a", "b", "c"}},
		{`a  "b c"  d`, []string{"a", "b c", "d"}},
		{`'single quoted'`, []string{"single quoted"}},
		{`--repo /a/b --flag`, []string{"--repo", "/a/b", "--flag"}},
		{`x\ y`, []string{"x y"}},
		{`"esc \" quote"`, []string{`esc " quote`}},
		{``, nil},
	}
	for _, c := range cases {
		got, err := shellSplit(c.in)
		if err != nil {
			t.Fatalf("shellSplit(%q): %v", c.in, err)
		}
		if len(got) != len(c.want) {
			t.Fatalf("shellSplit(%q) = %v, want %v", c.in, got, c.want)
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Fatalf("shellSplit(%q)[%d] = %q, want %q", c.in, i, got[i], c.want[i])
			}
		}
	}
	if _, err := shellSplit(`unbalanced "quote`); err == nil {
		t.Fatal("an unbalanced quote must error")
	}
}

func TestArgvPaths(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	os.MkdirAll(repo, 0o755)
	resolvedRepo, _ := filepath.EvalSymlinks(repo)

	// A --repo flag naming the repo, plus a bare absolute path, plus a URL to
	// skip, plus a relative path resolved against cwd.
	os.MkdirAll(filepath.Join(repo, "sub"), 0o755)
	argv := "codex exec --repo " + repo + " /etc/hosts https://x.example/y ./sub/file"
	paths, err := ArgvPaths(argv, repo)
	if err != nil {
		t.Fatal(err)
	}
	// Expect: the repo (from --repo), /etc/hosts (bare abs), and repo/sub/file
	// (relative resolved against cwd). The URL is skipped.
	foundRepo, foundRelative, foundURL := false, false, false
	for _, p := range paths {
		if p == resolvedRepo {
			foundRepo = true
		}
		if p == filepath.Join(resolvedRepo, "sub", "file") {
			foundRelative = true
		}
		if p == "https://x.example/y" {
			foundURL = true
		}
	}
	if !foundRepo {
		t.Fatalf("--repo path not extracted: %v", paths)
	}
	if !foundRelative {
		t.Fatalf("relative path not resolved against cwd: %v", paths)
	}
	if foundURL {
		t.Fatal("a URL must not be treated as a path")
	}
}

func TestArgvPathsFlagEqualsValue(t *testing.T) {
	paths, err := ArgvPaths("cmd --workspace=/ws/here other", "")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, p := range paths {
		if p == "/ws/here" {
			found = true
		}
	}
	if !found {
		t.Fatalf("--flag=value path not extracted: %v", paths)
	}
}

func TestArgvPathsMalformed(t *testing.T) {
	if _, err := ArgvPaths(`cmd "unterminated`, ""); err == nil {
		t.Fatal("a malformed argv must error")
	}
}
