package audit

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"
)

type stopSurfaceRead struct {
	operation string
	first     string
	second    string
}

type stopSurfaceFake struct {
	t          *testing.T
	root       string
	owned      map[string]bool
	snapshots  map[string]map[string][]byte
	commits    map[string]string
	head       string
	unborn     bool
	refs       map[string]string
	mergeBases map[string][]string
	expected   []stopSurfaceRead
	readIndex  int
}

func newStopSurfaceFake(t *testing.T, root string) *stopSurfaceFake {
	t.Helper()
	f := &stopSurfaceFake{
		t: t, root: root, owned: make(map[string]bool),
		snapshots: make(map[string]map[string][]byte),
		commits:   make(map[string]string), refs: make(map[string]string),
		mergeBases: make(map[string][]string),
	}
	t.Cleanup(f.assertConsumed)
	return f
}

func (f *stopSurfaceFake) own(path string) {
	f.t.Helper()
	f.owned[path] = true
}

func (f *stopSurfaceFake) freeze(root string) string {
	f.t.Helper()
	if root != f.root {
		f.t.Fatalf("freeze root %q, want %q", root, f.root)
	}
	number := len(f.commits) + 1
	commit := fmt.Sprintf("commit-%03d", number)
	tree := fmt.Sprintf("tree-%03d", number)
	snapshot := make(map[string][]byte, len(f.owned))
	for path := range f.owned {
		content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			f.t.Fatal(err)
		}
		snapshot[path] = append([]byte(nil), content...)
	}
	f.snapshots[tree] = snapshot
	f.commits[commit] = tree
	f.head = commit
	f.unborn = false
	return commit
}

func (f *stopSurfaceFake) setOriginMain(commit string) {
	f.t.Helper()
	if _, ok := f.commits[commit]; !ok {
		f.t.Fatalf("unknown origin/main %q", commit)
	}
	f.refs["refs/remotes/origin/main"] = commit
}

func (f *stopSurfaceFake) withoutOriginMain() {
	delete(f.refs, "refs/remotes/origin/main")
}

func (f *stopSurfaceFake) setMergeBase(head, main, base string) {
	f.t.Helper()
	if _, ok := f.commits[base]; !ok {
		f.t.Fatalf("unknown merge base %q", base)
	}
	f.mergeBases[head+"\x00"+main] = []string{base}
}

func (f *stopSurfaceFake) dependencies() stopSurfaceDependencies {
	return stopSurfaceDependencies{workspace: f, candidateInventory: f.candidateInventory, treeInventory: f.treeInventory}
}

func (f *stopSurfaceFake) expectInspection(root string, options StopSurfaceOptions, declarations bool) {
	f.t.Helper()
	f.assertConsumed()
	if root != f.root {
		f.t.Fatalf("inspection root %q, want %q", root, f.root)
	}
	var base string
	if options.Base != "" {
		f.expect("ResolveCommit", options.Base, "")
		base = options.Base
	} else {
		f.expect("HeadCommit", "", "")
		if f.unborn {
			return
		}
		f.expect("RefMap", "", "")
		base = f.head
		if main, ok := f.refs["refs/remotes/origin/main"]; ok {
			f.expect("MergeBases", f.head, main)
			bases := f.mergeBases[f.head+"\x00"+main]
			if len(bases) != 1 {
				f.t.Fatalf("no declared merge base for %q and %q", f.head, main)
			}
			base = bases[0]
		}
	}
	tree, ok := f.commits[base]
	if !ok {
		f.t.Fatalf("unknown base commit %q", base)
	}
	f.expect("TreeOf", base, "")
	f.expect("TreeInventory", root, tree)
	f.expect("CandidateInventory", root, "")
	for _, path := range stopSurfaceEligiblePaths(f.snapshots[tree]) {
		f.expect("FileAt", tree, path)
	}
	if declarations {
		entries, err := os.ReadDir(filepath.Join(root, filepath.FromSlash(stopMoveDirectory)))
		if err != nil && !os.IsNotExist(err) {
			f.t.Fatal(err)
		}
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".txt") {
				f.expect("FileAt", tree, stopMoveDirectory+"/"+entry.Name())
			}
		}
	}
}

func stopSurfaceEligiblePaths(files map[string][]byte) []string {
	paths := make([]string, 0, len(files))
	for path := range files {
		if _, selected := stopSurfaceFileKind(path); selected {
			paths = append(paths, path)
		}
	}
	sort.Strings(paths)
	return paths
}

func stopSurfaceInventory(paths []string) []byte {
	if len(paths) == 0 {
		return nil
	}
	return []byte(strings.Join(paths, "\x00") + "\x00")
}

func (f *stopSurfaceFake) expect(operation, first, second string) {
	f.expected = append(f.expected, stopSurfaceRead{operation, first, second})
}

func (f *stopSurfaceFake) consume(operation, first, second string) {
	f.t.Helper()
	got := stopSurfaceRead{operation, first, second}
	if f.readIndex >= len(f.expected) {
		f.t.Fatalf("unexpected repository read %+v", got)
	}
	want := f.expected[f.readIndex]
	if got != want {
		f.t.Fatalf("repository read %d = %+v, want %+v", f.readIndex, got, want)
	}
	f.readIndex++
}

func (f *stopSurfaceFake) assertConsumed() {
	f.t.Helper()
	if f.readIndex != len(f.expected) {
		f.t.Fatalf("unconsumed repository reads: got %d of %d; next %+v", f.readIndex, len(f.expected), f.expected[f.readIndex])
	}
	f.expected = nil
	f.readIndex = 0
}

func (f *stopSurfaceFake) ResolveCommit(revision string) (string, error) {
	f.consume("ResolveCommit", revision, "")
	if _, ok := f.commits[revision]; !ok {
		return "", fmt.Errorf("unknown revision %q", revision)
	}
	return revision, nil
}

func (f *stopSurfaceFake) HeadCommit() (string, bool, error) {
	f.consume("HeadCommit", "", "")
	return f.head, f.unborn, nil
}

func (f *stopSurfaceFake) RefMap() (map[string]string, error) {
	f.consume("RefMap", "", "")
	refs := make(map[string]string, len(f.refs))
	for name, commit := range f.refs {
		refs[name] = commit
	}
	return refs, nil
}

func (f *stopSurfaceFake) MergeBases(head, main string) ([]string, error) {
	f.consume("MergeBases", head, main)
	bases, ok := f.mergeBases[head+"\x00"+main]
	if !ok {
		return nil, fmt.Errorf("unknown merge base pair %q, %q", head, main)
	}
	return append([]string(nil), bases...), nil
}

func (f *stopSurfaceFake) TreeOf(commit string) (string, error) {
	f.consume("TreeOf", commit, "")
	tree, ok := f.commits[commit]
	if !ok {
		return "", fmt.Errorf("unknown commit %q", commit)
	}
	return tree, nil
}

func (f *stopSurfaceFake) FileAt(tree, path string) ([]byte, bool, error) {
	f.consume("FileAt", tree, path)
	snapshot, ok := f.snapshots[tree]
	if !ok {
		return nil, false, fmt.Errorf("unknown tree %q", tree)
	}
	content, found := snapshot[path]
	return append([]byte(nil), content...), found, nil
}

func (f *stopSurfaceFake) candidateInventory(root string) ([]byte, error) {
	f.consume("CandidateInventory", root, "")
	paths := make([]string, 0, len(f.owned))
	for path := range f.owned {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return append([]byte(nil), stopSurfaceInventory(paths)...), nil
}

func (f *stopSurfaceFake) treeInventory(root, tree string) ([]byte, error) {
	f.consume("TreeInventory", root, tree)
	snapshot, ok := f.snapshots[tree]
	if !ok {
		return nil, fmt.Errorf("unknown tree %q", tree)
	}
	paths := make([]string, 0, len(snapshot))
	for path := range snapshot {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return append([]byte(nil), stopSurfaceInventory(paths)...), nil
}

func (f *stopSurfaceFixture) auditDecision(options StopSurfaceOptions) (StopSurfaceResult, error) {
	f.t.Helper()
	f.repository.expectInspection(f.root, options, true)
	result, err := auditStopDecisionSurface(f.root, options, f.repository.dependencies())
	f.repository.assertConsumed()
	return result, err
}

func (f *stopSurfaceFixture) declareDecision(options StopSurfaceOptions, goalID, reason string) (string, error) {
	f.t.Helper()
	f.repository.expectInspection(f.root, options, false)
	path, err := declareStopDecisionSurface(f.root, options, goalID, reason, f.repository.dependencies())
	f.repository.assertConsumed()
	return path, err
}

// gitStopSurfaceFixture exercises the installed Git adapter against a repository.
type gitStopSurfaceFixture struct {
	t    *testing.T
	root string
}

func (f *gitStopSurfaceFixture) write(path, content string) {
	f.t.Helper()
	absolute := filepath.Join(f.root, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(absolute), 0o755); err != nil {
		f.t.Fatal(err)
	}
	if err := os.WriteFile(absolute, []byte(content), 0o644); err != nil {
		f.t.Fatal(err)
	}
}

func (f *gitStopSurfaceFixture) git(args ...string) string {
	f.t.Helper()
	command := exec.Command("git", append([]string{"-C", f.root}, args...)...)
	command.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_SYSTEM="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1")
	output, err := command.CombinedOutput()
	if err != nil {
		f.t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, output)
	}
	return strings.TrimSpace(string(output))
}

func (f *gitStopSurfaceFixture) commit(message string) string {
	f.t.Helper()
	f.git("add", "-A")
	f.git("commit", "-q", "-m", message)
	return f.git("rev-parse", "HEAD")
}

func (f *gitStopSurfaceFixture) audit(options StopSurfaceOptions) StopSurfaceResult {
	f.t.Helper()
	if options.GoalRecord == nil {
		options.GoalRecord = stopSurfaceGoalReaderStub
	}
	result, err := AuditStopDecisionSurface(f.root, options)
	if err != nil {
		f.t.Fatal(err)
	}
	return result
}

// TestStopSurfaceGitBaseAdapterResolvesDivergedHistory checks that the default
// adapter reads Git HEAD, origin/main, their merge base, the base tree and file,
// and both Git inventories in a repository whose branches diverged.
func TestStopSurfaceGitBaseAdapterResolvesDivergedHistory(t *testing.T) {
	t.Parallel()
	f := &gitStopSurfaceFixture{t: t, root: t.TempDir()}
	f.git("init", "-q", "-b", "main")
	f.git("config", "--local", "user.name", "Stop Surface Fixture")
	f.git("config", "--local", "user.email", "stop-surface@invalid")
	f.git("config", "--local", "commit.gpgsign", "false")
	const path = "nested/decision_test.go"
	const baseLine = "base := Verdict{ShouldBlock: true}"
	const localLine = "local := Verdict{BlockSource: source}"
	f.write(path, goStopSurfaceFixture("nested", baseLine+"\n"))
	base := f.commit("shared base")
	f.git("checkout", "-q", "-b", "origin-side")
	f.write(path, goStopSurfaceFixture("nested", "remote := Verdict{ShouldBlock: false}\n"))
	origin := f.commit("origin change")
	f.git("update-ref", "refs/remotes/origin/main", origin)
	f.git("checkout", "-q", "main")
	f.write(path, goStopSurfaceFixture("nested", baseLine+"\n"+localLine+"\n"))
	f.commit("local change")
	result := f.audit(stopSurfaceTestOptions())
	want := []StopSurfaceLine{{File: path, Line: localLine}}
	if result.Base != base || result.Refused() || !slices.Equal(result.Added, want) || len(result.Removed) != 0 || len(result.Moved) != 0 || len(result.Problems) != 0 {
		t.Fatalf("diverged Git history result = %+v, want base %s and additions %+v", result, base, want)
	}
}
