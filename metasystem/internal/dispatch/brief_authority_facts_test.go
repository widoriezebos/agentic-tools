package dispatch

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type briefAuthorityFixture struct {
	root  string
	facts *fakeBriefTreeFacts
}

type fakeBriefTreeFacts struct {
	root       string
	prefix     string
	current    string
	nextCommit int
	snapshots  map[string]map[string]bool
}

func newBriefAuthorityRepo(t *testing.T) briefAuthorityFixture {
	t.Helper()
	root := t.TempDir()
	paths := []string{
		"records/.keep", "docs/.keep", "scripts/.keep",
		"metasystem/internal/.keep", "metasystem/records/.keep", "metasystem/scripts/.keep",
	}
	for _, name := range paths {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, filepath.FromSlash(name))), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	facts := &fakeBriefTreeFacts{root: root, current: "brief-commit-1", nextCommit: 1, snapshots: map[string]map[string]bool{}}
	facts.snapshots[facts.current] = pathSet(paths)
	return briefAuthorityFixture{root: root, facts: facts}
}

func pathSet(paths []string) map[string]bool {
	set := make(map[string]bool, len(paths))
	for _, name := range paths {
		set[name] = true
	}
	return set
}

func (f *fakeBriefTreeFacts) commitPaths(names ...string) {
	previous := f.snapshots[f.current]
	next := make(map[string]bool, len(previous)+len(names))
	for name := range previous {
		next[name] = true
	}
	for _, name := range names {
		next[name] = true
	}
	f.nextCommit++
	f.current = fmt.Sprintf("brief-commit-%d", f.nextCommit)
	f.snapshots[f.current] = next
}

func (f *fakeBriefTreeFacts) checkRoot(root string) error {
	if root != f.root {
		return fmt.Errorf("unsupported brief tree root %q", root)
	}
	return nil
}

func (f *fakeBriefTreeFacts) InstallPrefix(root string) (string, error) {
	if err := f.checkRoot(root); err != nil {
		return "", err
	}
	return f.prefix, nil
}

func (f *fakeBriefTreeFacts) BaseCommit(root string) (string, error) {
	if err := f.checkRoot(root); err != nil {
		return "", err
	}
	if _, ok := f.snapshots[f.current]; !ok {
		return "", fmt.Errorf("unsupported brief commit %q", f.current)
	}
	return f.current, nil
}

func (f *fakeBriefTreeFacts) Directories(root, treeish string) (map[string]bool, error) {
	if err := f.checkRoot(root); err != nil {
		return nil, err
	}
	commit, subtree, nested := strings.Cut(treeish, ":")
	if nested && subtree != "metasystem" {
		return nil, fmt.Errorf("unsupported brief tree %q", treeish)
	}
	files, ok := f.snapshots[commit]
	if !ok {
		return nil, fmt.Errorf("unsupported brief commit %q", commit)
	}
	result := map[string]bool{}
	for name := range files {
		if nested {
			if !strings.HasPrefix(name, "metasystem/") {
				continue
			}
			name = strings.TrimPrefix(name, "metasystem/")
		}
		if first, _, found := strings.Cut(name, "/"); found {
			result[first] = true
		}
	}
	return result, nil
}

func TestBriefAuthorityAdmitsARuntimePathOfTheServingInstallation(t *testing.T) {
	t.Parallel()
	for _, prefix := range []string{"metasystem", "tools/engine"} {
		t.Run(prefix, func(t *testing.T) {
			t.Parallel()
			repo := newBriefAuthorityRepo(t)
			repo.facts.prefix = prefix
			repo.facts.commitPaths(prefix + "/plans/.keep")
			primary := t.TempDir()
			_, serving, _ := ResolveTool(repo.root, func(root string) (string, string) {
				if root != repo.root {
					t.Fatalf("serving root = %q, want %q", root, repo.root)
				}
				return filepath.Join(primary, prefix), primary
			}, func(string) (string, bool) { return "", false })
			git := func(root string, args ...string) (string, error) {
				switch args[0] {
				case "rev-parse":
					if args[1] == "--show-prefix" {
						return prefix + "/", nil
					}
					return repo.facts.BaseCommit(root)
				case "ls-tree":
					if strings.Contains(args[len(args)-1], ":") {
						return "plans", nil
					}
					return "plans\n" + strings.Split(prefix, "/")[0], nil
				case "check-ignore":
					if strings.HasSuffix(args[len(args)-1], "plans/goals/g.md") {
						return "", nil
					}
				}
				return "", fmt.Errorf("unsupported Git facts: %v", args)
			}
			for _, cited := range []string{"artifacts/agents/handoff/state.json", prefix + "/artifacts/agents/handoff/state.json", "plans/goals/g.md"} {
				state := filepath.Join(primary, filepath.FromSlash(cited))
				writeSeatFile(t, state, "{}\n")
				brief := writeBriefAuthorityFile(t, repo.root, "continuation.md", "Working Mode: build\nRead `"+cited+"` first.\n")
				admit := func() error {
					_, err := ReadReviewBriefAdmissionWithServingCheckout(brief, repo.root, repo.root, repo.root, "", serving, git)
					return err
				}
				if err := admit(); err != nil {
					t.Fatalf("the serving installation's runtime state was refused: %v", err)
				}
				if err := os.Remove(state); err != nil {
					t.Fatal(err)
				}
				var refusal *BriefAuthorityRefusal
				if err := admit(); !errors.As(err, &refusal) {
					t.Fatalf("missing runtime state must refuse: %v", err)
				}
				for _, detail := range []string{cited, "runtime path", repo.root, primary} {
					if !strings.Contains(refusal.Error(), detail) {
						t.Errorf("runtime refusal omits %q: %v", detail, refusal)
					}
				}
			}
		})
	}
}

func (f *fakeBriefTreeFacts) HasPath(root, commit, name string) (bool, error) {
	if err := f.checkRoot(root); err != nil {
		return false, err
	}
	files, ok := f.snapshots[commit]
	if !ok {
		return false, fmt.Errorf("unsupported brief commit %q", commit)
	}
	if files[name] {
		return true, nil
	}
	for file := range files {
		if strings.HasPrefix(file, strings.TrimSuffix(name, "/")+"/") {
			return true, nil
		}
	}
	return false, nil
}

func (f *fakeBriefTreeFacts) ResolveCommit(root, revision string) (string, error) {
	if err := f.checkRoot(root); err != nil {
		return "", err
	}
	if _, ok := f.snapshots[revision]; !ok {
		return "", fmt.Errorf("unknown reviewed commit %s", revision)
	}
	return revision, nil
}
