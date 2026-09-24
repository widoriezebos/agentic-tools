package missionrunner

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/mission"
)

// Each answer is for one exact call at one of the two ordinary filesystem roots.
// There is no repository state behind these answers.
type nestedScopeAnswer struct {
	root, method string
	args         []any
	value        any
	err          error
}

type nestedScopeFacts struct {
	t       *testing.T
	answers []nestedScopeAnswer
}

func (f *nestedScopeFacts) add(root, method string, value any, args ...any) {
	f.answers = append(f.answers, nestedScopeAnswer{root: root, method: method, args: args, value: value})
}

func (f *nestedScopeFacts) addErr(root, method string, value any, err error, args ...any) {
	f.answers = append(f.answers, nestedScopeAnswer{root: root, method: method, args: args, value: value, err: err})
}

func (f *nestedScopeFacts) next(root, method string, args ...any) nestedScopeAnswer {
	f.t.Helper()
	if len(f.answers) == 0 {
		f.t.Fatalf("undeclared nested scope call %q %s(%v)", root, method, args)
	}
	want := f.answers[0]
	f.answers = f.answers[1:]
	if root != want.root || method != want.method || !reflect.DeepEqual(args, want.args) {
		f.t.Fatalf("nested scope call %q %s(%v), want %q %s(%v)", root, method, args, want.root, want.method, want.args)
	}
	return want
}

func (f *nestedScopeFacts) done() {
	f.t.Helper()
	if len(f.answers) != 0 {
		f.t.Fatalf("unused nested scope answers: %+v", f.answers)
	}
}

type nestedScopeWorkspace struct {
	f    *nestedScopeFacts
	root string
}

func (w *nestedScopeWorkspace) answer(method string, args ...any) nestedScopeAnswer {
	return w.f.next(w.root, method, args...)
}

func (w *nestedScopeWorkspace) HistorySteeringFiles() ([]string, error) {
	r := w.answer("HistorySteeringFiles")
	return append([]string(nil), r.value.([]string)...), r.err
}
func (w *nestedScopeWorkspace) HeadCommit() (string, bool, error) {
	r := w.answer("HeadCommit")
	v := r.value.(scopePolicyHead)
	return v.oid, v.unborn, r.err
}
func (w *nestedScopeWorkspace) TreeOf(rev string) (string, error) {
	r := w.answer("TreeOf", rev)
	return r.value.(string), r.err
}
func (w *nestedScopeWorkspace) FilterTree(tree string, paths []string) (string, error) {
	r := w.answer("FilterTree", tree, paths)
	return r.value.(string), r.err
}
func (w *nestedScopeWorkspace) SymbolicHead() (string, bool, error) {
	r := w.answer("SymbolicHead")
	v := r.value.(scopePolicyBranch)
	return v.ref, v.detached, r.err
}
func (w *nestedScopeWorkspace) RefMap() (map[string]string, error) {
	r := w.answer("RefMap")
	v := r.value.(map[string]string)
	out := make(map[string]string, len(v))
	for name, oid := range v {
		out[name] = oid
	}
	return out, r.err
}
func (w *nestedScopeWorkspace) WorktreeCensus() ([]gittree.WorktreeRecord, error) {
	r := w.answer("WorktreeCensus")
	return append([]gittree.WorktreeRecord(nil), r.value.([]gittree.WorktreeRecord)...), r.err
}
func (w *nestedScopeWorkspace) StagedTree() (string, error) {
	r := w.answer("StagedTree")
	return r.value.(string), r.err
}
func (w *nestedScopeWorkspace) Prefix() (string, error) {
	r := w.answer("Prefix")
	return r.value.(string), r.err
}
func (w *nestedScopeWorkspace) TopLevel() (string, error) {
	r := w.answer("TopLevel")
	return r.value.(string), r.err
}
func (w *nestedScopeWorkspace) Snapshot(baseline string) (string, error) {
	r := w.answer("Snapshot", baseline)
	return r.value.(string), r.err
}
func (w *nestedScopeWorkspace) TopStagedPosture() (gittree.StagedPosture, error) {
	r := w.answer("TopStagedPosture")
	return r.value.(gittree.StagedPosture), r.err
}
func (w *nestedScopeWorkspace) SnapshotSeeded(seed, expected string, paths []string) (string, error) {
	r := w.answer("SnapshotSeeded", seed, expected, paths)
	return r.value.(string), r.err
}
func (w *nestedScopeWorkspace) ChangedPaths(from, to string) ([]string, error) {
	r := w.answer("ChangedPaths", from, to)
	return append([]string(nil), r.value.([]string)...), r.err
}
func (w *nestedScopeWorkspace) unexpected(method string) {
	w.f.t.Fatalf("undeclared nested scope method %q %s", w.root, method)
}
func (w *nestedScopeWorkspace) FileAt(string, string) ([]byte, bool, error) {
	w.unexpected("FileAt")
	return nil, false, nil
}
func (w *nestedScopeWorkspace) Apply(string, []byte) (string, error) {
	w.unexpected("Apply")
	return "", nil
}
func (w *nestedScopeWorkspace) Entries(string, []string) (map[string]gittree.Entry, error) {
	w.unexpected("Entries")
	return nil, nil
}
func (w *nestedScopeWorkspace) HeadTree() (string, error) {
	w.unexpected("HeadTree")
	return "", nil
}
func (w *nestedScopeWorkspace) Anchor(string, string) error {
	w.unexpected("Anchor")
	return nil
}
func (w *nestedScopeWorkspace) MaterializePaths(string, []string) error {
	w.unexpected("MaterializePaths")
	return nil
}

func (f *nestedScopeFacts) Git(root string, args ...string) (string, string, int) {
	r := f.next(root, "Git", args)
	v := r.value.(scopePolicyGit)
	return v.stdout, v.stderr, v.code
}
func (f *nestedScopeFacts) LedgerBlobOID(root string, state map[string]any, path string) (string, error) {
	r := f.next(root, "LedgerBlobOID", state, path)
	return r.value.(string), r.err
}
func (f *nestedScopeFacts) unexpected(method string) {
	f.t.Fatalf("undeclared nested raw read %s", method)
}
func (f *nestedScopeFacts) BlobOID(string, []byte) (string, error) {
	f.unexpected("BlobOID")
	return "", nil
}
func (f *nestedScopeFacts) LedgerTruth(string, map[string]any, string) (string, string, error) {
	f.unexpected("LedgerTruth")
	return "", "", nil
}
func (f *nestedScopeFacts) AuthenticateLedger(string, map[string]any, string) error {
	f.unexpected("AuthenticateLedger")
	return nil
}

var _ wallWorkspace = (*nestedScopeWorkspace)(nil)
var _ wallReads = (*nestedScopeFacts)(nil)

type nestedScopePosture struct {
	head, workspaceTree, topTree string
	topStaged                    gittree.StagedPosture
}

// The side chain's interior is a declared relation; the wall queries its
// merge base and accumulated delta, not each interior parent or tree.
type nestedScopeSideCommit struct {
	oid, parent, topTree, workspaceTree string
}

type nestedScopeBed struct {
	t                 *testing.T
	engine            *Engine
	facts             *nestedScopeFacts
	top, root, prefix string
	pre               string
	state             map[string]any
	origin            *scopeOrigin
}

func newStrictNestedScopeBed(t *testing.T) *nestedScopeBed {
	t.Helper()
	top := t.TempDir()
	root := filepath.Join(top, "ws")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(top, "sibling"), 0o755); err != nil {
		t.Fatal(err)
	}
	rel, err := filepath.Rel(top, root)
	if err != nil {
		t.Fatal(err)
	}
	facts := &nestedScopeFacts{t: t}
	b := &nestedScopeBed{t: t, facts: facts, top: top, root: root,
		prefix: filepath.ToSlash(rel) + "/", pre: "workspace-pre"}
	b.engine = &Engine{Root: root, Mission: "demo", wallReadFacts: facts}
	b.engine.wallWorkspaceFactory = func(got string) wallWorkspace {
		if got != root && got != top {
			t.Fatalf("unexpected nested workspace root %q", got)
		}
		return &nestedScopeWorkspace{f: facts, root: got}
	}
	b.state = map[string]any{"branch": "main", "turnLog": []any{}, "initialBaseline": b.pre,
		"workspaceTaint": map[string]any{"next": 1, "segment": 0, "entries": []any{}}}
	t.Cleanup(facts.done)
	facts.add(root, "Snapshot", "raw-workspace-pre", "HEAD")
	facts.add(root, "FilterTree", b.pre, "raw-workspace-pre", b.ledgerPaths())
	pre, err := wallSnapshotWithWorkspace(b.engine.wallWorkspace(root), "demo")
	if err != nil || pre != b.pre {
		t.Fatalf("initial workspace snapshot: %q %v", pre, err)
	}
	return b
}

func (b *nestedScopeBed) ledgerPaths() []string { return []string{missionLedgerRel("demo")} }

func (b *nestedScopeBed) expectCapture(p nestedScopePosture, expected string) {
	f := b.facts
	f.add(b.root, "HistorySteeringFiles", []string(nil))
	f.add(b.root, "HeadCommit", scopePolicyHead{oid: p.head})
	f.add(b.root, "TreeOf", "raw-"+p.workspaceTree, p.head)
	f.add(b.root, "FilterTree", p.workspaceTree, "raw-"+p.workspaceTree, b.ledgerPaths())
	f.add(b.root, "SymbolicHead", scopePolicyBranch{ref: "refs/heads/main"})
	f.add(b.root, "RefMap", map[string]string{"refs/heads/main": p.head})
	f.add(b.root, "WorktreeCensus", []gittree.WorktreeRecord{{Path: b.top, HeadOID: p.head,
		Branch: "refs/heads/main", PostureReadable: true, Staged: p.topStaged}})
	f.add(b.root, "StagedTree", "raw-"+b.pre)
	f.add(b.root, "FilterTree", b.pre, "raw-"+b.pre, b.ledgerPaths())
	f.add(b.root, "Prefix", b.prefix)
	f.add(b.root, "TopLevel", b.top)
	f.add(b.top, "Snapshot", p.topTree, p.head)
	f.add(b.root, "TopStagedPosture", p.topStaged)
	if expected != "" {
		f.add(b.root, "SnapshotSeeded", "raw-"+b.pre, p.head, expected, []string{})
		f.add(b.root, "FilterTree", b.pre, "raw-"+b.pre, b.ledgerPaths())
	}
}

func (b *nestedScopeBed) captureOrigin(p nestedScopePosture) {
	b.t.Helper()
	b.expectCapture(p, "")
	c, err := b.engine.captureWallPosture("", nil)
	if err != nil {
		b.t.Fatal(err)
	}
	staged := c.TopStaged
	b.origin = &scopeOrigin{Head: c.Head, RefMap: c.RefMap, TopTree: c.TopTree,
		TopStaged: &staged, StagedPost: c.StagedTree}
	for _, item := range mission.WorktreeCensusDoc(c.Census) {
		b.origin.Census = append(b.origin.Census, item.(map[string]any))
	}
	if len(b.facts.answers) != 0 {
		b.t.Fatalf("unused origin capture answers: %+v", b.facts.answers)
	}
}

func (b *nestedScopeBed) expectLedgerGuard() {
	b.facts.addErr(b.root, "LedgerBlobOID", "", mission.ErrNoAnchor, b.state,
		filepath.Join(missionDirPath(b.root, "demo"), "ledger.md"))
}

func (b *nestedScopeBed) expectGit(stdout string, args ...string) {
	b.facts.add(b.root, "Git", scopePolicyGit{stdout: stdout}, args)
}

func (b *nestedScopeBed) expectJudgeStart(p nestedScopePosture, committedTopTree string) {
	b.expectCapture(p, b.pre)
	b.facts.add(b.root, "Prefix", b.prefix)
	b.expectLedgerGuard() // accountant's anchored-ledger read
	b.expectGit(committedTopTree+"\n", "rev-parse", p.head+"^{tree}")
	b.expectLedgerGuard() // committed HEAD and index carrier guard
}

func (b *nestedScopeBed) judge(p nestedScopePosture) string {
	b.t.Helper()
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
