package missionrunner

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/mission"
)

const (
	recoveryPre  = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	recoveryHead = "cccccccccccccccccccccccccccccccccccccccc"
	recoveryPost = "dddddddddddddddddddddddddddddddddddddddd"
	recoveryB    = "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"
)

type recoveryMaterialize struct {
	before map[string]string
	after  map[string]*string
}

type recoverySnapshot struct {
	post        string
	requireLate bool
}

// The ordered facts cover repository answers and effects. The mission files
// remain ordinary files, so policy code reads and writes its real documents.
type recoveryFacts struct {
	*scopePolicyFacts
	bed      *recoveryFileBed
	original []byte
}

type recoveryFileBed struct {
	t       *testing.T
	e       *Engine
	state   string
	ledger  string
	turnDir string
	facts   *recoveryFacts
}

func newRecoveryFileBed(t *testing.T, initial ...map[string]string) *recoveryFileBed {
	t.Helper()
	if len(initial) > 1 {
		t.Fatal("one initial file set per recovery bed")
	}
	root := t.TempDir()
	e := &Engine{Root: root, Mission: "alpha"}
	b := &recoveryFileBed{t: t, e: e, state: filepath.Join(e.missionDir(), "state.json"), ledger: filepath.Join(e.missionDir(), "ledger.md")}
	contract := "```mission\ncandidate.branch=main\nstream.solo=Do solo\n```\n```mission-seal\ncandidate.branch=main\n```\n"
	writeText(t, e.approvedContractPath(), contract)
	sum := sha256.Sum256([]byte(contract))
	writeJSONFile(t, e.fencesPath(), map[string]any{"cycles": 0, "approvedContractSha256": hex.EncodeToString(sum[:])})
	if err := mission.InitLedger(b.ledger, 5, 3); err != nil {
		t.Fatal(err)
	}
	origins := map[string]any{
		"headCommit": recoveryHead, "topTree": nil, "topStaged": nil,
		"refMap":         map[string]any{"refs/heads/main": recoveryHead},
		"worktreeCensus": []any{}, "capturedAt": "2026-01-01T00:00:00Z",
	}
	if err := mission.InitStateWithBaseline(b.state, e.approvedContractPath(), b.ledger, "", "main", recoveryPre, origins); err != nil {
		t.Fatal(err)
	}
	if len(initial) == 1 {
		for path, content := range initial[0] {
			if filepath.IsAbs(path) || strings.HasPrefix(path, "../") {
				t.Fatalf("invalid initial recovery path %q", path)
			}
			writeText(t, filepath.Join(root, path), content)
		}
	}
	b.openTurn(recoveryPre, recoveryHead, recoveryPre, map[string]string{"refs/heads/main": recoveryHead})
	b.turnDir = filepath.Join(e.missionDir(), "turns", "alpha-t1-live")
	writeJSONFile(t, filepath.Join(b.turnDir, "turn.json"), map[string]any{
		"missionId": e.Mission, "turnId": "alpha-t1-live", "cycle": 1,
		"runtime": "fake", "model": "fixture", "status": "running",
	})
	original, err := os.ReadFile(b.ledger)
	if err != nil {
		t.Fatal(err)
	}
	f := &recoveryFacts{scopePolicyFacts: &scopePolicyFacts{t: t, root: root}, bed: b, original: original}
	b.facts = f
	e.wallWorkspaceFactory = func(got string) wallWorkspace {
		if got != root {
			t.Fatalf("wall workspace root %q, want %q", got, root)
		}
		return f
	}
	e.wallReadFacts = f
	e.anchorFn = func(state, ledger, identity string) error {
		f.next("StateAnchor", state, ledger, identity)
		if state != b.state || ledger != b.ledger || identity != "alpha-t1-live" {
			t.Fatalf("unexpected state anchor %q %q %q", state, ledger, identity)
		}
		if _, _, err := mission.VerifyStateShape(state); err != nil {
			t.Fatalf("state anchor shape: %v", err)
		}
		return nil
	}
	t.Cleanup(f.done)
	return b
}

func (b *recoveryFileBed) openTurn(pre, head, headTree string, refs map[string]string) {
	b.t.Helper()
	sequence, hash, err := mission.VerifyStateShape(b.state)
	if err != nil {
		b.t.Fatal(err)
	}
	doc, err := readJSONDoc(b.state)
	if err != nil {
		b.t.Fatal(err)
	}
	taint, _ := doc["workspaceTaint"].(map[string]any)
	segment, _ := jsonInt(taint["segment"])
	doc["openTurn"] = map[string]any{
		"turnId": "alpha-t1-live", "cycle": 1, "preTree": pre,
		"sequence": sequence, "segment": segment, "openedAt": "2026-08-18T00:00:00Z",
		"headCommit": head, "headTree": headTree, "topTree": nil,
		"refMap": mission.RecordableRefMap(refs, b.e.Mission), "topStaged": nil,
	}
	delete(doc, "integrity")
	source := b.state + ".open-turn.src"
	if err := atomicWriteJSON(source, doc); err != nil {
		b.t.Fatal(err)
	}
	if err := mission.WriteState(b.state, source, hash); err != nil {
		b.t.Fatal(err)
	}
	if _, _, err := mission.VerifyStateShape(b.state); err != nil {
		b.t.Fatalf("open-turn shape: %v", err)
	}
}

func (f *recoveryFacts) done() {
	if len(f.answers) != 0 {
		f.t.Errorf("%d unconsumed recovery facts; first: %s%v", len(f.answers), f.answers[0].method, f.answers[0].args)
	}
}

func (f *recoveryFacts) stateDoc() map[string]any {
	f.t.Helper()
	state, err := readJSONDoc(f.bed.state)
	if err != nil {
		f.t.Fatal(err)
	}
	return state
}

func (f *recoveryFacts) ledgerGuard() {
	f.add("LedgerTruth", nil, f.bed.e.Root, f.stateDoc(), f.bed.ledger)
}

func (f *recoveryFacts) pass(expected string) {
	f.capture(expected, expected)
	f.judge()
	f.capture(expected, expected)
	f.namespace()
	if expected != recoveryPre {
		f.add("Anchor", nil, f.bed.e.Mission, expected)
		f.add("Anchor", nil, f.bed.e.Mission, expected)
	}
}

func (f *recoveryFacts) violation(expected, post string, paths []string, result recoveryMaterialize) {
	f.restore(expected, post, paths, result)
	f.pass(expected)
}

func (f *recoveryFacts) restore(expected, post string, paths []string, result recoveryMaterialize) {
	f.capture(expected, post)
	f.add("ChangedPaths", paths, expected, post)
	f.capture(expected, post)
	if expected != recoveryPre {
		f.add("Anchor", nil, f.bed.e.Mission, expected)
	}
	f.add("Anchor", nil, f.bed.e.Mission, post)
	f.add("ChangedPaths", paths, expected, post)
	f.capture(expected, post)
	f.judge()
	f.namespace()
	f.add("MaterializePaths", result, expected, paths)
}

func (f *recoveryFacts) failedRecheck(expected, post string, paths []string) {
	f.captureLate(expected, post)
	f.add("ChangedPaths", paths, expected, post)
	f.captureLate(expected, post)
	f.add("Anchor", nil, f.bed.e.Mission, post)
	f.add("ChangedPaths", paths, expected, post)
}

func (f *recoveryFacts) refs() map[string]string {
	ns := mission.MissionRefNamespace(f.bed.e.Mission)
	return map[string]string{
		"refs/heads/main":     recoveryHead,
		ns + "state-anchors":  recoveryHead,
		ns + "turn-open-head": recoveryHead,
	}
}

func (f *recoveryFacts) capture(expected, post string) {
	f.captureWithCheck(expected, post, false)
}

func (f *recoveryFacts) captureLate(expected, post string) {
	f.captureWithCheck(expected, post, true)
}

func (f *recoveryFacts) captureWithCheck(expected, post string, requireLate bool) {
	f.bed.t.Helper()
	ledger := []string{missionLedgerRel(f.bed.e.Mission)}
	f.add("HistorySteeringFiles", []string{})
	f.add("HeadCommit", scopePolicyHead{oid: recoveryHead})
	f.add("TreeOf", recoveryPre, recoveryHead)
	f.add("FilterTree", recoveryPre, recoveryPre, ledger)
	f.add("SymbolicHead", scopePolicyBranch{ref: "refs/heads/main"})
	f.add("RefMap", f.refs())
	f.add("WorktreeCensus", []gittree.WorktreeRecord{{Path: f.bed.e.Root, HeadOID: recoveryHead, Branch: "refs/heads/main",
		PostureReadable: true, Staged: gittree.StagedPosture{Tree: recoveryPre}}})
	f.add("StagedTree", recoveryPre)
	f.add("FilterTree", recoveryPre, recoveryPre, ledger)
	f.add("Prefix", "")
	f.add("SnapshotSeeded", recoverySnapshot{post: post, requireLate: requireLate}, recoveryHead, expected, []string{})
	f.add("FilterTree", post, post, ledger)
}

func (f *recoveryFacts) SnapshotSeeded(seed, expected string, paths []string) (string, error) {
	r := f.next("SnapshotSeeded", seed, expected, paths)
	snapshot := r.value.(recoverySnapshot)
	if snapshot.requireLate {
		path := filepath.Join(f.bed.e.Root, "late-mutation.txt")
		got, err := os.ReadFile(path)
		if err != nil || string(got) != "raced in\n" {
			f.t.Fatalf("late snapshot requires real mutation bytes: %q, %v", got, err)
		}
	}
	return snapshot.post, r.err
}

func (f *recoveryFacts) namespace() {
	f.add("LedgerBlobOID", nil, f.bed.e.Root, f.stateDoc(), f.bed.ledger)
	f.add("Git", scopePolicyGit{code: 1}, f.bed.e.Root, []string{"cat-file", "-e", recoveryPre})
	f.add("AuthenticateLedger", nil, f.bed.e.Root, f.stateDoc(), f.bed.ledger)
}

func (f *recoveryFacts) Apply(base string, patch []byte) (string, error) {
	r := f.next("Apply", base, patch)
	return r.value.(string), r.err
}

func (f *recoveryFacts) Entries(tree string, paths []string) (map[string]gittree.Entry, error) {
	r := f.next("Entries", tree, paths)
	return r.value.(map[string]gittree.Entry), r.err
}

func (f *recoveryFacts) judge() {
	f.add("Prefix", "")
	f.add("LedgerBlobOID", nil, f.bed.e.Root, f.stateDoc(), f.bed.ledger)
	f.namespace()
	f.add("TopLevel", f.bed.e.Root)
}

func (f *recoveryFacts) park() {
	f.add("Git", scopePolicyGit{stdout: recoveryHead + "\n", code: 0}, f.bed.e.Root, []string{"rev-parse", "--verify", "--quiet", "HEAD^{commit}"})
	f.add("Git", scopePolicyGit{stdout: recoveryHead + "\n", code: 0}, f.bed.e.Root, []string{"-C", f.bed.e.Root, "rev-parse", "main"})
	f.add("StateAnchor", nil, f.bed.state, f.bed.ledger, "alpha-t1-live")
}

func (f *recoveryFacts) LedgerTruth(root string, state map[string]any, path string) (string, string, error) {
	f.next("LedgerTruth", root, state, path)
	if state == nil || root != f.bed.e.Root || path != f.bed.ledger {
		f.t.Fatalf("undeclared ledger truth root/state/path: %q %v %q", root, state != nil, path)
	}
	current, err := os.ReadFile(path)
	if err != nil {
		return "", "", err
	}
	return string(f.original), string(current), nil
}

func (f *recoveryFacts) LedgerBlobOID(root string, state map[string]any, path string) (string, error) {
	f.next("LedgerBlobOID", root, state, path)
	if state == nil || root != f.bed.e.Root || path != f.bed.ledger {
		f.t.Fatalf("undeclared ledger blob root/state/path: %q %v %q", root, state != nil, path)
	}
	return "", mission.ErrNoAnchor
}

func (f *recoveryFacts) AuthenticateLedger(root string, state map[string]any, path string) error {
	f.next("AuthenticateLedger", root, state, path)
	if state == nil || root != f.bed.e.Root || path != f.bed.ledger {
		f.t.Fatalf("undeclared ledger authentication root/state/path: %q %v %q", root, state != nil, path)
	}
	current, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if !bytes.Equal(current, f.original) {
		return fmt.Errorf("ledger differs from original bytes")
	}
	return nil
}

func (f *recoveryFacts) Anchor(missionID, tree string) error {
	f.next("Anchor", missionID, tree)
	if missionID != f.bed.e.Mission || len(tree) != 40 {
		f.t.Fatalf("undeclared tree anchor %q %q", missionID, tree)
	}
	if _, _, err := mission.VerifyStateShape(f.bed.state); err != nil {
		f.t.Fatalf("tree anchor state shape: %v", err)
	}
	return nil
}

func (f *recoveryFacts) MaterializePaths(tree string, paths []string) error {
	r := f.next("MaterializePaths", tree, paths)
	m := r.value.(recoveryMaterialize)
	if len(paths) != len(m.before) || len(paths) != len(m.after) {
		f.t.Fatalf("materialize declaration differs from paths: %v", paths)
	}
	for _, path := range paths {
		before, ok := m.before[path]
		if !ok || filepath.IsAbs(path) || strings.HasPrefix(path, "../") {
			f.t.Fatalf("undeclared materialize path %q", path)
		}
		got, err := os.ReadFile(filepath.Join(f.bed.e.Root, path))
		if err != nil || string(got) != before {
			f.t.Fatalf("materialize %q original bytes %q, want %q: %v", path, got, before, err)
		}
	}
	for _, path := range paths {
		to, ok := m.after[path]
		if !ok {
			f.t.Fatalf("undeclared restore result %q", path)
		}
		full := filepath.Join(f.bed.e.Root, path)
		if to == nil {
			if err := os.Remove(full); err != nil {
				return err
			}
		} else {
			writeText(f.t, full, *to)
		}
	}
	return nil
}

func recoveryString(s string) *string { return &s }

var _ wallWorkspace = (*recoveryFacts)(nil)
var _ wallReads = (*recoveryFacts)(nil)
