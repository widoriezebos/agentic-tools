package dispatch

import (
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
