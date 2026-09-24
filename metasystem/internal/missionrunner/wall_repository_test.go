package missionrunner

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
)

// strictWallFacts supplies only the repository answers declared by the
// capture scenario. Every other method or query fails at its call site.
type strictWallFacts struct {
	t       *testing.T
	id      string
	root    string
	main    string
	top     string
	queries *[]string
}

func (f *strictWallFacts) expect(root, query string) {
	f.t.Helper()
	if f.root != root {
		f.t.Fatalf("workspace root %q: undeclared query %s", f.root, query)
	}
	*f.queries = append(*f.queries, root+":"+query)
}

func (f *strictWallFacts) unexpected(method string) {
	f.t.Helper()
	f.t.Fatalf("undeclared wall workspace method %s at %q", method, f.root)
}

func (f *strictWallFacts) FileAt(string, string) ([]byte, bool, error) {
	f.unexpected("FileAt")
	panic("unreachable")
}
func (f *strictWallFacts) ChangedPaths(string, string) ([]string, error) {
	f.unexpected("ChangedPaths")
	panic("unreachable")
}
func (f *strictWallFacts) Apply(string, []byte) (string, error) {
	f.unexpected("Apply")
	panic("unreachable")
}
func (f *strictWallFacts) Entries(string, []string) (map[string]gittree.Entry, error) {
	f.unexpected("Entries")
	panic("unreachable")
}
func (f *strictWallFacts) Snapshot(baseline string) (string, error) {
	f.expect(f.top, "Snapshot:"+baseline)
	if baseline != "head-"+f.id {
		f.unexpected("Snapshot baseline")
	}
	return "top-" + f.id, nil
}
func (f *strictWallFacts) SnapshotSeeded(string, string, []string) (string, error) {
	f.unexpected("SnapshotSeeded")
	panic("unreachable")
}
func (f *strictWallFacts) FilterTree(tree string, paths []string) (string, error) {
	f.expect(f.main, "FilterTree:"+tree)
	if !reflect.DeepEqual(paths, []string{missionLedgerRel("demo")}) {
		f.unexpected("FilterTree paths")
	}
	switch tree {
	case "raw-" + f.id:
		return "headtree-" + f.id, nil
	case "staged-" + f.id:
		return "stagedtree-" + f.id, nil
	default:
		f.unexpected("FilterTree tree")
		panic("unreachable")
	}
}
func (f *strictWallFacts) HeadTree() (string, error) {
	f.unexpected("HeadTree")
	panic("unreachable")
}
func (f *strictWallFacts) HeadCommit() (string, bool, error) {
	f.expect(f.main, "HeadCommit")
	return "head-" + f.id, false, nil
}
func (f *strictWallFacts) TreeOf(rev string) (string, error) {
	f.expect(f.main, "TreeOf:"+rev)
	if rev != "head-"+f.id {
		f.unexpected("TreeOf revision")
	}
	return "raw-" + f.id, nil
}
func (f *strictWallFacts) RefMap() (map[string]string, error) {
	f.expect(f.main, "RefMap")
	return map[string]string{"refs/heads/" + f.id: "head-" + f.id}, nil
}
func (f *strictWallFacts) SymbolicHead() (string, bool, error) {
	f.expect(f.main, "SymbolicHead")
	return "refs/heads/" + f.id, false, nil
}
func (f *strictWallFacts) WorktreeCensus() ([]gittree.WorktreeRecord, error) {
	f.expect(f.main, "WorktreeCensus")
	return nil, nil
}
func (f *strictWallFacts) StagedTree() (string, error) {
	f.expect(f.main, "StagedTree")
	return "staged-" + f.id, nil
}
func (f *strictWallFacts) TopStagedPosture() (gittree.StagedPosture, error) {
	f.expect(f.main, "TopStagedPosture")
	return gittree.StagedPosture{Tree: "topstaged-" + f.id}, nil
}
func (f *strictWallFacts) Prefix() (string, error) {
	f.expect(f.main, "Prefix")
	return "nested/", nil
}
func (f *strictWallFacts) TopLevel() (string, error) {
	f.expect(f.main, "TopLevel")
	return f.top, nil
}
func (f *strictWallFacts) HistorySteeringFiles() ([]string, error) {
	f.expect(f.main, "HistorySteeringFiles")
	return nil, nil
}
func (f *strictWallFacts) Anchor(string, string) error {
	f.unexpected("Anchor")
	panic("unreachable")
}
func (f *strictWallFacts) MaterializePaths(string, []string) error {
	f.unexpected("MaterializePaths")
	panic("unreachable")
}

var _ wallWorkspace = (*strictWallFacts)(nil)

func TestWallWorkspaceEngineIsolationWithoutGit(t *testing.T) {
	t.Parallel()
	type caseFacts struct {
		id, root, top      string
		engine             *Engine
		requested, queries []string
	}
	cases := make([]*caseFacts, 0, 2)
	for _, id := range []string{"alpha", "beta"} {
		c := &caseFacts{id: id, root: "/virtual/" + id + "/nested", top: "/virtual/" + id}
		c.engine = &Engine{Root: c.root, Mission: "demo"}
		c.engine.wallWorkspaceFactory = func(got string) wallWorkspace {
			if got != c.root && got != c.top {
				t.Fatalf("%s factory received undeclared root %q", c.id, got)
			}
			c.requested = append(c.requested, got)
			return &strictWallFacts{t: t, id: c.id, root: got, main: c.root, top: c.top, queries: &c.queries}
		}
		cases = append(cases, c)
	}
	for _, c := range cases {
		t.Run(c.id, func(t *testing.T) {
			cap, err := c.engine.captureWallPosture("", nil)
			if err != nil {
				t.Fatal(err)
			}
			if cap.Head != "head-"+c.id || cap.Branch != "refs/heads/"+c.id ||
				cap.HeadTree != "headtree-"+c.id || cap.TopTree != "top-"+c.id ||
				cap.StagedTree != "stagedtree-"+c.id || cap.TopStaged.Tree != "topstaged-"+c.id ||
				len(cap.RefMap) != 1 || cap.RefMap["refs/heads/"+c.id] != "head-"+c.id {
				t.Fatalf("engine %s received another source's facts: %+v", c.id, cap)
			}
			if !reflect.DeepEqual(c.requested, []string{c.root, c.top}) {
				t.Fatalf("engine %s requested roots %v", c.id, c.requested)
			}
			want := []string{
				c.root + ":HistorySteeringFiles", c.root + ":HeadCommit", c.root + ":TreeOf:head-" + c.id,
				c.root + ":FilterTree:raw-" + c.id, c.root + ":SymbolicHead", c.root + ":RefMap",
				c.root + ":WorktreeCensus", c.root + ":StagedTree", c.root + ":FilterTree:staged-" + c.id,
				c.root + ":Prefix", c.root + ":TopLevel", c.top + ":Snapshot:head-" + c.id,
				c.root + ":TopStagedPosture",
			}
			if !reflect.DeepEqual(c.queries, want) {
				t.Fatalf("queries: got %v, want %v", c.queries, want)
			}
		})
	}
}

func TestWallWorkspaceDefaultUsesRealRepository(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	run := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	run("init", "-q", "-b", "main")
	if err := os.WriteFile(filepath.Join(root, "note.txt"), []byte("wall adapter\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "note.txt")
	run("commit", "-qm", "baseline")
	e := &Engine{Root: filepath.Join(root, "subdir"), Mission: "demo"}
	got := e.wallWorkspace(root)
	adapter, ok := got.(gittree.Workspace)
	if !ok || adapter.Dir != root {
		t.Fatalf("default adapter at %q: %T", root, got)
	}
	head, unborn, err := got.HeadCommit()
	if err != nil || unborn || head != run("rev-parse", "HEAD") {
		t.Fatalf("real repository HEAD: oid=%q unborn=%t err=%v", head, unborn, err)
	}
}
