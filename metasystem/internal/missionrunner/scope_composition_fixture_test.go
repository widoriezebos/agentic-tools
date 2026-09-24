package missionrunner

import (
	"path/filepath"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/mission"
)

// The composition fixtures provide finite repository answers to the real wall rules.
type scopeCompositionFacts struct{ *scopePolicyFacts }

func (f *scopeCompositionFacts) Entries(tree string, paths []string) (map[string]gittree.Entry, error) {
	r := f.next("Entries", tree, paths)
	return r.value.(map[string]gittree.Entry), r.err
}

type scopeCompositionBed struct {
	t      *testing.T
	engine *Engine
	facts  *scopeCompositionFacts
	pre    string
	state  map[string]any
	origin *scopeOrigin
}

func newScopeCompositionBed(t *testing.T, pre string) *scopeCompositionBed {
	t.Helper()
	root := t.TempDir()
	facts := &scopeCompositionFacts{scopePolicyFacts: &scopePolicyFacts{t: t, root: root}}
	e := &Engine{Root: root, Mission: "demo", wallReadFacts: facts}
	e.wallWorkspaceFactory = func(got string) wallWorkspace {
		if got != root {
			t.Fatalf("composition workspace root %q, want %q", got, root)
		}
		return facts
	}
	t.Cleanup(facts.done)
	return &scopeCompositionBed{t: t, engine: e, facts: facts, pre: pre,
		state: map[string]any{"branch": "main", "turnLog": []any{}, "initialBaseline": pre,
			"workspaceTaint": map[string]any{"next": 1, "segment": 0, "entries": []any{}}}}
}

func (b *scopeCompositionBed) refs(head string, side string) map[string]string {
	refs := map[string]string{"refs/heads/main": head}
	if side != "" {
		refs["refs/heads/agent/job-x"] = side
	}
	return refs
}

func (b *scopeCompositionBed) expectCapture(head, tree, post, expected string, refs map[string]string, declared []string) {
	f := b.facts
	ledger := []string{missionLedgerRel("demo")}
	f.add("HistorySteeringFiles", []string{})
	f.add("HeadCommit", scopePolicyHead{oid: head})
	f.add("TreeOf", "raw-"+tree, head)
	f.add("FilterTree", tree, "raw-"+tree, ledger)
	f.add("SymbolicHead", scopePolicyBranch{ref: "refs/heads/main"})
	f.add("RefMap", refs)
	f.add("WorktreeCensus", []gittree.WorktreeRecord{{Path: b.engine.Root, HeadOID: head,
		Branch: "refs/heads/main", PostureReadable: true, Staged: gittree.StagedPosture{Tree: tree}}})
	f.add("StagedTree", "raw-"+tree)
	f.add("FilterTree", tree, "raw-"+tree, ledger)
	f.add("Prefix", "")
	if post != "" {
		f.add("SnapshotSeeded", "raw-"+post, head, expected, declared)
		f.add("FilterTree", post, "raw-"+post, ledger)
	}
}

func (b *scopeCompositionBed) captureOrigin(head, tree string, refs map[string]string) {
	b.t.Helper()
	b.expectCapture(head, tree, "", "", refs, nil)
	capture, err := b.engine.captureWallPosture("", nil)
	if err != nil {
		b.t.Fatal(err)
	}
	b.origin = &scopeOrigin{Head: capture.Head, RefMap: capture.RefMap}
}

func (b *scopeCompositionBed) expectAccountantAndGuard() {
	b.facts.add("Prefix", "")
	b.expectNoLedger()
	b.expectNoLedger()
}

func (b *scopeCompositionBed) expectNoLedger() {
	b.facts.addErr("LedgerBlobOID", "", mission.ErrNoAnchor, b.engine.Root, b.state,
		filepath.Join(missionDirPath(b.engine.Root, "demo"), "ledger.md"))
}

func (b *scopeCompositionBed) expectGit(stdout string, args ...string) {
	b.facts.add("Git", scopePolicyGit{stdout: stdout}, b.engine.Root, args)
}

func (b *scopeCompositionBed) expectCommitTree(commit, tree string) {
	b.facts.add("TreeOf", "raw-"+tree, commit)
	b.facts.add("FilterTree", tree, "raw-"+tree, []string{missionLedgerRel("demo")})
}

func (b *scopeCompositionBed) expectEntries(tree string, paths []string, entries map[string]gittree.Entry) {
	b.facts.add("Entries", entries, tree, paths)
}

func (b *scopeCompositionBed) authorization(base, reviewed, digest string, paths []string, entries map[string]gittree.Entry) scopeAuth {
	b.t.Helper()
	b.facts.add("ChangedPaths", paths, base, reviewed)
	b.expectEntries(reviewed, paths, entries)
	workspace := b.engine.wallWorkspace(b.engine.Root)
	changed, err := workspace.ChangedPaths(base, reviewed)
	if err != nil {
		b.t.Fatal(err)
	}
	reviewedEntries, err := workspace.Entries(reviewed, changed)
	if err != nil {
		b.t.Fatal(err)
	}
	return scopeAuth{digest: digest, changedPaths: changed, reviewedTree: reviewed, reviewedEntries: reviewedEntries}
}

func (b *scopeCompositionBed) judge(auths []scopeAuth, expected string) string {
	b.t.Helper()
	capture, err := b.engine.captureWallPosture(expected, nil)
	if err != nil {
		b.t.Fatal(err)
	}
	acct, err := b.engine.newWallAccountant(b.pre, b.state, auths, nil)
	if err != nil {
		b.t.Fatal(err)
	}
	acct.noteExpected(expected)
	violation, err := b.engine.judgeScope(b.origin, capture, acct, b.state)
	if err != nil {
		b.t.Fatalf("judge: %v", err)
	}
	return violation
}
