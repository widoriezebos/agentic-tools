package config

// U5i (engine-owns-disk-lifetimes Part B, 3.12 placement rules, R21):
// config validate refuses an evidence root that contains a cache root or
// lies inside one, and one whose top two levels hold a cache-shaped tree or
// a source copy, naming each tree, its size and what a person runs.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func placementBed(t *testing.T) (root, cache string) {
	t.Helper()
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	root, cache = filepath.Join(base, "evidence"), filepath.Join(base, "caches", "go-build")
	for _, dir := range []string{root, cache} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return root, cache
}

func oneProblem(t *testing.T, problems []string, wants ...string) {
	t.Helper()
	if len(problems) != 1 {
		t.Fatalf("want one problem, got %q", problems)
	}
	for _, want := range wants {
		if !strings.Contains(problems[0], want) {
			t.Fatalf("the problem names %q: %s", want, problems[0])
		}
	}
}

func TestPlacementRefusesARootThatContainsACacheRoot(t *testing.T) {
	t.Parallel()
	root, _ := placementBed(t)
	inside := filepath.Join(root, "go-build")
	if err := os.MkdirAll(inside, 0o755); err != nil {
		t.Fatal(err)
	}
	oneProblem(t, placementProblems(root, []string{inside}), EvidenceRootKey, inside, "contains the cache root")
}

func TestPlacementRefusesARootInsideACacheRoot(t *testing.T) {
	t.Parallel()
	_, cache := placementBed(t)
	root := filepath.Join(cache, "evidence")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	oneProblem(t, placementProblems(root, []string{cache}), EvidenceRootKey, cache, "lies inside the cache root")
}

// The cache root is compared by identity: a cache spelled through a
// symlink is the same directory.
func TestPlacementComparesCacheRootsByIdentity(t *testing.T) {
	t.Parallel()
	root, _ := placementBed(t)
	inside := filepath.Join(root, "caches", "go-build")
	if err := os.MkdirAll(inside, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(filepath.Dir(root), "spelled")
	if err := os.Symlink(filepath.Join(root, "caches"), link); err != nil {
		t.Fatal(err)
	}
	oneProblem(t, placementProblems(root, []string{filepath.Join(link, "go-build")}), "contains the cache root")
}

func TestPlacementRefusesACacheShapedTreeOrASourceCopyInTheTopTwoLevels(t *testing.T) {
	t.Parallel()
	for name, rel := range map[string]string{
		"a gocache-x directory at the top":        "gocache-x/00/entry",
		"a .git directory at the second level":    "hand-named/clone/.git/HEAD",
		"a checkout copy at the second level":     "agents/copy/metasystem/metasystem.conf",
		"a module cache at the top":               "mod/cache/download/example.com/@v/list",
		"a staticcheck cache at the second level": "hand/staticcheck/README",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root, _ := placementBed(t)
			path := filepath.Join(root, filepath.FromSlash(rel))
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			content := "x"
			if strings.HasSuffix(rel, "README") {
				content = "This directory holds cached build artifacts from staticcheck.\n"
			}
			if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}
			problems := placementProblems(root, nil)
			if len(problems) != 1 || !strings.Contains(problems[0], "rm -rf -- '") || !strings.Contains(problems[0], "work workspace") {
				t.Fatalf("one refusal naming the tree and what a person runs: %q", problems)
			}
		})
	}
}

func TestPlacementAcceptsAnOrdinaryRoot(t *testing.T) {
	t.Parallel()
	root, cache := placementBed(t)
	for _, dir := range []string{"agents/0123456789ab/chain-a/jobs", "suite-failures", "events", "hand-named-20260901/notes"} {
		if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(dir)), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if problems := placementProblems(root, []string{cache}); len(problems) != 0 {
		t.Fatalf("an ordinary root is accepted: %q", problems)
	}
	if problems := placementProblems(filepath.Join(root, "absent"), []string{cache}); len(problems) != 0 {
		t.Fatalf("a root not made yet holds nothing: %q", problems)
	}
}
