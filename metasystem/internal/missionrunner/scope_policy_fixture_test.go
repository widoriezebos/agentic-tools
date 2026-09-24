package missionrunner

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/mission"
)

// Each answer belongs to one call. The fixture has no repository state or
// command interpretation; every production query consumes the next answer.
type scopePolicyAnswer struct {
	method string
	args   []any
	value  any
	err    error
}

type scopePolicyFacts struct {
	t       *testing.T
	root    string
	answers []scopePolicyAnswer
}

func (f *scopePolicyFacts) add(method string, value any, args ...any) {
	f.answers = append(f.answers, scopePolicyAnswer{method: method, args: args, value: value})
}

func (f *scopePolicyFacts) addErr(method string, value any, err error, args ...any) {
	f.answers = append(f.answers, scopePolicyAnswer{method: method, args: args, value: value, err: err})
}

func (f *scopePolicyFacts) next(method string, args ...any) scopePolicyAnswer {
	f.t.Helper()
	if len(f.answers) == 0 {
		f.t.Fatalf("undeclared scope policy call %s(%v) at %q", method, args, f.root)
	}
	want := f.answers[0]
	f.answers = f.answers[1:]
	if method != want.method || !reflect.DeepEqual(args, want.args) {
		f.t.Fatalf("scope policy call %s(%v) at %q, want %s(%v)", method, args, f.root, want.method, want.args)
	}
	return want
}

func (f *scopePolicyFacts) done() {
	f.t.Helper()
	if len(f.answers) != 0 {
		f.t.Fatalf("unused scope policy answers: %+v", f.answers)
	}
}

func (f *scopePolicyFacts) Snapshot(baseline string) (string, error) {
	r := f.next("Snapshot", baseline)
	return r.value.(string), r.err
}
func (f *scopePolicyFacts) FilterTree(tree string, paths []string) (string, error) {
	r := f.next("FilterTree", tree, paths)
	return r.value.(string), r.err
}
func (f *scopePolicyFacts) HistorySteeringFiles() ([]string, error) {
	r := f.next("HistorySteeringFiles")
	return r.value.([]string), r.err
}
func (f *scopePolicyFacts) HeadCommit() (string, bool, error) {
	r := f.next("HeadCommit")
	v := r.value.(scopePolicyHead)
	return v.oid, v.unborn, r.err
}
func (f *scopePolicyFacts) TreeOf(rev string) (string, error) {
	r := f.next("TreeOf", rev)
	return r.value.(string), r.err
}
func (f *scopePolicyFacts) SymbolicHead() (string, bool, error) {
	r := f.next("SymbolicHead")
	v := r.value.(scopePolicyBranch)
	return v.ref, v.detached, r.err
}
func (f *scopePolicyFacts) RefMap() (map[string]string, error) {
	r := f.next("RefMap")
	v := r.value.(map[string]string)
	copy := make(map[string]string, len(v))
	for k, oid := range v {
		copy[k] = oid
	}
	return copy, r.err
}
func (f *scopePolicyFacts) WorktreeCensus() ([]gittree.WorktreeRecord, error) {
	r := f.next("WorktreeCensus")
	v := r.value.([]gittree.WorktreeRecord)
	return append([]gittree.WorktreeRecord(nil), v...), r.err
}
func (f *scopePolicyFacts) StagedTree() (string, error) {
	r := f.next("StagedTree")
	return r.value.(string), r.err
}
func (f *scopePolicyFacts) Prefix() (string, error) {
	r := f.next("Prefix")
	return r.value.(string), r.err
}
func (f *scopePolicyFacts) SnapshotSeeded(seed, expected string, paths []string) (string, error) {
	r := f.next("SnapshotSeeded", seed, expected, paths)
	return r.value.(string), r.err
}
func (f *scopePolicyFacts) TopLevel() (string, error) {
	r := f.next("TopLevel")
	return r.value.(string), r.err
}
func (f *scopePolicyFacts) ChangedPaths(from, to string) ([]string, error) {
	r := f.next("ChangedPaths", from, to)
	return append([]string(nil), r.value.([]string)...), r.err
}
func (f *scopePolicyFacts) Git(root string, args ...string) (string, string, int) {
	r := f.next("Git", root, args)
	v := r.value.(scopePolicyGit)
	return v.stdout, v.stderr, v.code
}
func (f *scopePolicyFacts) LedgerBlobOID(root string, state map[string]any, path string) (string, error) {
	r := f.next("LedgerBlobOID", root, state, path)
	return r.value.(string), r.err
}

func (f *scopePolicyFacts) unexpected(method string) {
	f.t.Fatalf("undeclared scope policy method %s at %q", method, f.root)
}
func (f *scopePolicyFacts) FileAt(string, string) ([]byte, bool, error) {
	f.unexpected("FileAt")
	return nil, false, nil
}
func (f *scopePolicyFacts) Apply(string, []byte) (string, error) {
	f.unexpected("Apply")
	return "", nil
}
func (f *scopePolicyFacts) Entries(string, []string) (map[string]gittree.Entry, error) {
	f.unexpected("Entries")
	return nil, nil
}
func (f *scopePolicyFacts) HeadTree() (string, error) { f.unexpected("HeadTree"); return "", nil }
func (f *scopePolicyFacts) TopStagedPosture() (gittree.StagedPosture, error) {
	f.unexpected("TopStagedPosture")
	return gittree.StagedPosture{}, nil
}
func (f *scopePolicyFacts) Anchor(string, string) error { f.unexpected("Anchor"); return nil }
func (f *scopePolicyFacts) MaterializePaths(string, []string) error {
	f.unexpected("MaterializePaths")
	return nil
}
func (f *scopePolicyFacts) BlobOID(string, []byte) (string, error) {
	f.unexpected("BlobOID")
	return "", nil
}
func (f *scopePolicyFacts) LedgerTruth(string, map[string]any, string) (string, string, error) {
	f.unexpected("LedgerTruth")
	return "", "", nil
}
func (f *scopePolicyFacts) AuthenticateLedger(string, map[string]any, string) error {
	f.unexpected("AuthenticateLedger")
	return nil
}

var _ wallWorkspace = (*scopePolicyFacts)(nil)
var _ wallReads = (*scopePolicyFacts)(nil)

type scopePolicyHead struct {
	oid    string
	unborn bool
}
type scopePolicyBranch struct {
	ref      string
	detached bool
}
type scopePolicyGit struct {
	stdout, stderr string
	code           int
}

type scopePolicyPosture struct {
	head, tree, post, staged, branch string
	detached                         bool
	refs                             map[string]string
	steering                         []string
}

type scopePolicyBed struct {
	t      *testing.T
	engine *Engine
	facts  *scopePolicyFacts
	pre    string
	state  map[string]any
	origin *scopeOrigin
}

func newScopePolicyBed(t *testing.T) *scopePolicyBed {
	t.Helper()
	root := t.TempDir()
	facts := &scopePolicyFacts{t: t, root: root}
	b := &scopePolicyBed{t: t, facts: facts, engine: &Engine{Root: root, Mission: "demo", wallReadFacts: facts}}
	b.engine.wallWorkspaceFactory = func(got string) wallWorkspace {
		if got != root {
			t.Fatalf("workspace root %q, want %q", got, root)
		}
		return facts
	}
	t.Cleanup(facts.done)
	facts.add("Snapshot", "raw-pre", "HEAD")
	facts.add("FilterTree", "tree-pre", "raw-pre", []string{missionLedgerRel("demo")})
	pre, err := wallSnapshotWithWorkspace(b.engine.wallWorkspace(root), "demo")
	if err != nil {
		t.Fatal(err)
	}
	b.pre = pre
	b.state = map[string]any{"branch": "main", "turnLog": []any{}, "initialBaseline": pre,
		"workspaceTaint": map[string]any{"next": 1, "segment": 0, "entries": []any{}}}
	return b
}

func (b *scopePolicyBed) posture(head, tree string) scopePolicyPosture {
	return scopePolicyPosture{head: head, tree: tree, post: tree, staged: tree,
		branch: "refs/heads/main", refs: map[string]string{"refs/heads/main": head}}
}

func (b *scopePolicyBed) expectCapture(p scopePolicyPosture, expected string) {
	f := b.facts
	ledger := []string{missionLedgerRel("demo")}
	f.add("HistorySteeringFiles", p.steering)
	f.add("HeadCommit", scopePolicyHead{oid: p.head})
	f.add("TreeOf", "raw-"+p.tree, p.head)
	f.add("FilterTree", p.tree, "raw-"+p.tree, ledger)
	f.add("SymbolicHead", scopePolicyBranch{ref: p.branch, detached: p.detached})
	f.add("RefMap", p.refs)
	f.add("WorktreeCensus", []gittree.WorktreeRecord{{Path: b.engine.Root, HeadOID: p.head, Branch: p.branch,
		Detached: p.detached, PostureReadable: true, Staged: gittree.StagedPosture{Tree: p.staged}}})
	f.add("StagedTree", "raw-"+p.staged)
	f.add("FilterTree", p.staged, "raw-"+p.staged, ledger)
	f.add("Prefix", "")
	if expected != "" {
		f.add("SnapshotSeeded", "raw-"+p.post, p.head, expected, []string{})
		f.add("FilterTree", p.post, "raw-"+p.post, ledger)
	}
}

func (b *scopePolicyBed) captureOrigin(p scopePolicyPosture) {
	b.t.Helper()
	b.expectCapture(p, "")
	c, err := b.engine.captureWallPosture("", nil)
	if err != nil {
		b.t.Fatal(err)
	}
	b.origin = &scopeOrigin{Head: c.Head, RefMap: c.RefMap}
	if len(b.facts.answers) != 0 {
		b.t.Fatalf("unused origin capture answers: %+v", b.facts.answers)
	}
}

func (b *scopePolicyBed) expectAccountant() {
	b.facts.add("Prefix", "")
	b.facts.addErr("LedgerBlobOID", "", mission.ErrNoAnchor, b.engine.Root, b.state,
		filepath.Join(missionDirPath(b.engine.Root, "demo"), "ledger.md"))
}

func (b *scopePolicyBed) expectLedgerGuard() {
	b.facts.addErr("LedgerBlobOID", "", mission.ErrNoAnchor, b.engine.Root, b.state,
		filepath.Join(missionDirPath(b.engine.Root, "demo"), "ledger.md"))
}

func (b *scopePolicyBed) expectGit(stdout string, args ...string) {
	b.facts.add("Git", scopePolicyGit{stdout: stdout}, b.engine.Root, args)
}

func (b *scopePolicyBed) expectJudgeStart(p scopePolicyPosture) {
	b.expectCapture(p, b.pre)
	b.expectAccountant()
	if len(p.steering) == 0 {
		b.expectLedgerGuard()
	}
}

func (b *scopePolicyBed) judge(p scopePolicyPosture, afterGuard func()) string {
	b.t.Helper()
	b.expectJudgeStart(p)
	if afterGuard != nil {
		afterGuard()
	}
	c, err := b.engine.captureWallPosture(b.pre, nil)
	if err != nil {
		b.t.Fatal(err)
	}
	a, err := b.engine.newWallAccountant(b.pre, b.state, nil, nil)
	if err != nil {
		b.t.Fatal(err)
	}
	a.noteExpected(b.pre)
	v, err := b.engine.judgeScope(b.origin, c, a, b.state)
	if err != nil {
		b.t.Fatalf("judge: %v", err)
	}
	return v
}

func (b *scopePolicyBed) expectCensus() { b.facts.add("TopLevel", b.engine.Root) }

func (b *scopePolicyBed) expectFirstParent(head, oldest, parent string) {
	b.expectGit(oldest+"\n", "rev-list", "--first-parent", head, "--not", b.origin.Head)
	b.expectGit(parent+"\n", "rev-parse", "--verify", "--quiet", oldest+"^1")
}

func (b *scopePolicyBed) expectCommitTree(commit, tree string) {
	b.facts.add("TreeOf", "raw-"+tree, commit)
	b.facts.add("FilterTree", tree, "raw-"+tree, []string{missionLedgerRel("demo")})
}
