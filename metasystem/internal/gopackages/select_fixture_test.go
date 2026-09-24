package gopackages

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
)

const (
	selectionBaseRevision      = "base-revision"
	selectionCandidateRevision = "candidate-revision"
	selectionBaseTree          = "base-tree"
	selectionCandidateTree     = "candidate-tree"
)

// selectionFixture keeps the repository facts and the real file snapshots
// separate. The selector still parses both snapshots and runs its Go inventory.
type selectionFixture struct {
	t         *testing.T
	base      string
	candidate string
	path      string
	before    bool
	after     bool
	goMod     []byte
	openBase  bool
}

func newSelectionFixture(t *testing.T, base, path string, edit func(string), openBase bool) *selectionFixture {
	t.Helper()
	candidate := t.TempDir()
	if err := filepath.WalkDir(base, func(source string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(base, source)
		if err != nil {
			return err
		}
		target := filepath.Join(candidate, relative)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		content, err := os.ReadFile(source)
		if err != nil {
			return err
		}
		return os.WriteFile(target, content, 0o644)
	}); err != nil {
		t.Fatal(err)
	}
	before := selectionFileExists(t, base, path)
	edit(candidate)
	after := selectionFileExists(t, candidate, path)
	goMod, err := os.ReadFile(filepath.Join(candidate, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	return &selectionFixture{t: t, base: base, candidate: candidate, path: path, before: before, after: after, goMod: goMod, openBase: openBase}
}

func selectionFileExists(t *testing.T, root, path string) bool {
	t.Helper()
	_, err := os.Stat(filepath.Join(root, filepath.FromSlash(path)))
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	return err == nil
}

func (fixture *selectionFixture) selectWithTags(tags []string) (Selection, error) {
	fixture.t.Helper()
	return fixture.selectWithEnvironment(tags, os.Environ())
}

func (fixture *selectionFixture) selectWithEnvironment(tags, environment []string) (Selection, error) {
	fixture.t.Helper()
	run := &selectionFixtureRun{fixture: fixture}
	run.want = []selectionCall{
		{"FileAt", selectionBaseRevision, "go.mod"},
		{"TreeOf", selectionBaseRevision, ""},
		{"FileAt", selectionCandidateRevision, "go.mod"},
		{"TreeOf", selectionCandidateRevision, ""},
		{"ChangedPaths", selectionBaseTree, selectionCandidateTree},
		{"Entries", selectionBaseTree, fixture.path},
		{"Entries", selectionCandidateTree, fixture.path},
		{"FileAt", selectionCandidateTree, "go.mod"},
		{"Open", selectionCandidateTree, ""},
	}
	if fixture.openBase {
		run.want = append(run.want, selectionCall{"Open", selectionBaseTree, ""}, selectionCall{"Close", selectionBaseTree, ""})
	}
	run.want = append(run.want, selectionCall{"Close", selectionCandidateTree, ""})
	owner := selector{workspace: run, openSnapshot: run.openSnapshot}
	selected, err := owner.selectPackages(selectionBaseRevision, selectionCandidateRevision, tags, environment)
	if run.next != len(run.want) {
		fixture.t.Fatalf("selector left declared calls unconsumed: %v", run.want[run.next:])
	}
	return selected, err
}

type selectionCall struct{ operation, first, second string }

type selectionFixtureRun struct {
	fixture *selectionFixture
	want    []selectionCall
	next    int
}

func (run *selectionFixtureRun) consume(operation, first, second string) {
	run.fixture.t.Helper()
	actual := selectionCall{operation, first, second}
	if run.next == len(run.want) || run.want[run.next] != actual {
		run.fixture.t.Fatalf("undeclared selector call %v at %d; expected %v", actual, run.next, run.want[run.next:])
	}
	run.next++
}

func (run *selectionFixtureRun) FileAt(tree, path string) ([]byte, bool, error) {
	run.consume("FileAt", tree, path)
	if tree == selectionCandidateTree {
		return slices.Clone(run.fixture.goMod), true, nil
	}
	return nil, false, nil
}

func (run *selectionFixtureRun) TreeOf(revision string) (string, error) {
	run.consume("TreeOf", revision, "")
	if revision == selectionBaseRevision {
		return selectionBaseTree, nil
	}
	return selectionCandidateTree, nil
}

func (run *selectionFixtureRun) ChangedPaths(before, after string) ([]string, error) {
	run.consume("ChangedPaths", before, after)
	return []string{run.fixture.path}, nil
}

func (run *selectionFixtureRun) Entries(tree string, paths []string) (map[string]gittree.Entry, error) {
	if len(paths) != 1 {
		run.fixture.t.Fatalf("undeclared Entries paths %v", paths)
	}
	run.consume("Entries", tree, paths[0])
	entries := map[string]gittree.Entry{}
	if tree == selectionBaseTree && run.fixture.before || tree == selectionCandidateTree && run.fixture.after {
		entries[run.fixture.path] = gittree.Entry{Mode: "100644"}
	}
	return entries, nil
}

func (run *selectionFixtureRun) openSnapshot(tree string) (string, func() error, error) {
	run.consume("Open", tree, "")
	root := run.fixture.candidate
	if tree == selectionBaseTree {
		root = run.fixture.base
	}
	return root, func() error {
		run.consume("Close", tree, "")
		return nil
	}, nil
}

var _ selectionWorkspace = (*selectionFixtureRun)(nil)

func (call selectionCall) String() string {
	return fmt.Sprintf("%s(%q, %q)", call.operation, call.first, call.second)
}
