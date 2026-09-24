package proofrun

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

type testSnapshotEntry struct {
	data   []byte
	mode   os.FileMode
	target string
}

func testSnapshotFile(data string, mode os.FileMode) testSnapshotEntry {
	return testSnapshotEntry{data: []byte(data), mode: mode}
}

type testSnapshotFactory struct {
	mu          sync.Mutex
	root, tree  string
	files       map[string]testSnapshotEntry
	expected    int
	paths       []string
	closedPaths []string
}

// A snapshot declares candidate bytes once and writes the same ordinary files
// to the source root. Every open gets its own disposable candidate directory.
func newTestSnapshotFactory(t *testing.T, root, tree string, files map[string]testSnapshotEntry, expectedOpens int) *testSnapshotFactory {
	t.Helper()
	if !filepath.IsAbs(root) || !regexp.MustCompile(`^[0-9a-f]{40}$`).MatchString(tree) || expectedOpens < 1 {
		t.Fatalf("invalid snapshot declaration root=%q tree=%q opens=%d", root, tree, expectedOpens)
	}
	copyFiles := make(map[string]testSnapshotEntry, len(files))
	for path, entry := range files {
		if !validTestSnapshotPath(path) || (entry.mode.Perm() == 0 && entry.mode&os.ModeSymlink == 0) || entry.mode&^(os.ModePerm|os.ModeSymlink) != 0 ||
			(entry.mode&os.ModeSymlink != 0 && (entry.target == "" || len(entry.data) != 0)) ||
			(entry.mode&os.ModeSymlink == 0 && entry.target != "") {
			t.Fatalf("invalid snapshot entry %q: %+v", path, entry)
		}
		entry.data = append([]byte(nil), entry.data...)
		copyFiles[path] = entry
		if err := writeTestSnapshotEntry(filepath.Join(root, filepath.FromSlash(path)), entry); err != nil {
			t.Fatal(err)
		}
	}
	factory := &testSnapshotFactory{root: root, tree: tree, files: copyFiles, expected: expectedOpens}
	t.Cleanup(func() {
		factory.mu.Lock()
		defer factory.mu.Unlock()
		if len(factory.paths) != expectedOpens || len(factory.closedPaths) != expectedOpens {
			t.Errorf("snapshot opens=%d closes=%d, want %d each", len(factory.paths), len(factory.closedPaths), expectedOpens)
		}
		closed := make(map[string]int, len(factory.closedPaths))
		for _, path := range factory.closedPaths {
			closed[path]++
		}
		for _, path := range factory.paths {
			if closed[path] != 1 {
				t.Errorf("candidate %s closed %d times, want once", path, closed[path])
			}
			if _, err := os.Lstat(path); !os.IsNotExist(err) {
				t.Errorf("candidate directory remains after close: %s (%v)", path, err)
			}
		}
	})
	return factory
}

func validTestSnapshotPath(path string) bool {
	if path == "" || filepath.IsAbs(path) || filepath.ToSlash(filepath.Clean(path)) != path {
		return false
	}
	for _, part := range strings.Split(path, "/") {
		if part == "." || part == ".." || part == ".git" || part == "" {
			return false
		}
	}
	return true
}

func writeTestSnapshotEntry(path string, entry testSnapshotEntry) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if entry.mode&os.ModeSymlink != 0 {
		return os.Symlink(entry.target, path)
	}
	return testexec.WriteFile(path, entry.data, entry.mode)
}

func (factory *testSnapshotFactory) open(projectRoot, candidateTree string) (candidateWorkspace, error) {
	if projectRoot != factory.root || candidateTree != factory.tree {
		return nil, fmt.Errorf("undeclared candidate snapshot root=%q tree=%q", projectRoot, candidateTree)
	}
	factory.mu.Lock()
	defer factory.mu.Unlock()
	if len(factory.paths) >= factory.expected {
		return nil, fmt.Errorf("candidate snapshot opened more than %d times", factory.expected)
	}
	path, err := os.MkdirTemp("", "proofrun-test-candidate-")
	if err != nil {
		return nil, err
	}
	if relative, err := filepath.Rel(factory.root, path); err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		os.RemoveAll(path)
		return nil, fmt.Errorf("candidate %s is inside control root %s", path, factory.root)
	}
	for name, entry := range factory.files {
		if err := writeTestSnapshotEntry(filepath.Join(path, filepath.FromSlash(name)), entry); err != nil {
			os.RemoveAll(path)
			return nil, err
		}
	}
	factory.paths = append(factory.paths, path)
	return &testSnapshotWorkspace{factory: factory, dir: path}, nil
}

type testSnapshotWorkspace struct {
	mu      sync.Mutex
	factory *testSnapshotFactory
	dir     string
	closed  bool
}

func (workspace *testSnapshotWorkspace) Workspace() gittree.Workspace {
	return gittree.Workspace{Dir: workspace.dir}
}

func (workspace *testSnapshotWorkspace) Close() error {
	workspace.mu.Lock()
	defer workspace.mu.Unlock()
	if workspace.closed {
		return fmt.Errorf("candidate snapshot %s closed twice", workspace.dir)
	}
	if err := os.RemoveAll(workspace.dir); err != nil {
		return err
	}
	workspace.closed = true
	workspace.factory.mu.Lock()
	workspace.factory.closedPaths = append(workspace.factory.closedPaths, workspace.dir)
	workspace.factory.mu.Unlock()
	return nil
}

func testSnapshotScript(name, status string, exit int) testSnapshotEntry {
	body := "<testsuite><testcase classname=\"fixture\" name=\"" + name + "\">"
	if status == "failed" {
		body += "<failure message=\"red\"/>"
	}
	body += "</testcase></testsuite>"
	script := "#!/usr/bin/env bash\nset -euo pipefail\nmkdir -p reports-" + name + "\nprintf '%s\\n' " + strconv.Quote(body) + " > reports-" + name + "/tests.xml\nexit " + strconv.Itoa(exit) + "\n"
	return testSnapshotFile(script, 0o755)
}
