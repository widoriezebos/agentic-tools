package missionrunner

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/covenant"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/mission"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/validate"
)

type wallPolicyReply struct {
	method  string
	from    string
	to      string
	paths   []string
	changed []string
	patch   []byte
	tree    string
	entries map[string]gittree.Entry
	file    []byte
	present bool
	err     error
}

// The policy bed declares repository answers in call order. It owns no Git
// state: authorization evidence remains ordinary files under its root.
type wallPolicyBed struct {
	t      *testing.T
	root   string
	engine *Engine
	facts  *strictWallPolicyFacts
}

type strictWallPolicyFacts struct {
	t       *testing.T
	root    string
	replies []wallPolicyReply
}

func newWallPolicyBed(t *testing.T) *wallPolicyBed {
	t.Helper()
	root := t.TempDir()
	facts := &strictWallPolicyFacts{t: t, root: root}
	e := &Engine{Root: root, Mission: "demo", wallReadFacts: facts}
	e.wallWorkspaceFactory = func(got string) wallWorkspace {
		if got != root {
			t.Fatalf("undeclared wall workspace root %q, want %q", got, root)
		}
		return facts
	}
	return &wallPolicyBed{t: t, root: root, engine: e, facts: facts}
}

func (b *wallPolicyBed) authorization(base, reviewed string, changed []string, patch []byte, writePatch bool, mutate func(map[string]any)) string {
	b.t.Helper()
	paths := make([]any, len(changed))
	for i, path := range changed {
		paths[i] = path
	}
	patchSum := sha256.Sum256(patch)
	record := map[string]any{
		"jobId": "job-w", "rootJob": "job-w", "mission": b.engine.Mission,
		"baseTree": base, "reviewedTree": reviewed,
		"baseSequencePoint": map[string]any{"sequence": 0, "segment": 0},
		"patchDigest":       hex.EncodeToString(patchSum[:]),
		"changedPaths":      paths, "supersedes": []any{},
	}
	if mutate != nil {
		mutate(record)
	}
	digest, err := validate.AuthorizationRecordDigest(record)
	if err != nil {
		b.t.Fatal(err)
	}
	record["authorizationDigest"] = digest
	dir := filepath.Join(missionDirPath(b.root, b.engine.Mission), "authorizations")
	if writePatch {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			b.t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, digest+".patch"), patch, 0o644); err != nil {
			b.t.Fatal(err)
		}
	}
	writeJSONFile(b.t, filepath.Join(dir, digest+".json"), record)
	return digest
}

func (b *wallPolicyBed) inspect(pre string, state map[string]any, certified []map[string]any, declared map[string]bool) (*wallInspection, error) {
	return b.inspectWithGuardrails(pre, state, certified, declared, nil, "")
}

func (b *wallPolicyBed) inspectWithGuardrails(pre string, state map[string]any, certified []map[string]any, declared map[string]bool, guardrails *mission.GuardrailClass, declarationViolation string) (*wallInspection, error) {
	b.t.Helper()
	inspection, err := inspectWallWithWorkspace(b.engine.wallWorkspace(b.root), b.root, b.engine.Mission, pre,
		state, certified, declared, guardrails, declarationViolation, b.snapshot)
	if len(b.facts.replies) != 0 {
		b.t.Fatalf("missing declared wall replies after inspection: %+v", b.facts.replies)
	}
	return inspection, err
}

func (b *wallPolicyBed) governance(pre, reviewed, digest string) (string, error) {
	b.t.Helper()
	violation, err := covenantGovernanceViolationWithWorkspace(b.engine.wallWorkspace(b.root), pre, reviewed, digest)
	if len(b.facts.replies) != 0 {
		b.t.Fatalf("missing declared wall replies after governance check: %+v", b.facts.replies)
	}
	return violation, err
}

func (b *wallPolicyBed) snapshot(expected string) (string, error) {
	r := b.facts.next("Snapshot", expected, "", nil, nil)
	return r.tree, r.err
}

func (f *strictWallPolicyFacts) expectApply(base string, patch []byte, result string) {
	f.replies = append(f.replies, wallPolicyReply{method: "Apply", from: base, patch: append([]byte(nil), patch...), tree: result})
}

func (f *strictWallPolicyFacts) expectEntries(tree string, paths []string, entries map[string]gittree.Entry) {
	f.replies = append(f.replies, wallPolicyReply{method: "Entries", from: tree, paths: append([]string(nil), paths...), entries: entries})
}

func (f *strictWallPolicyFacts) expectChanged(from, to string, paths []string) {
	f.replies = append(f.replies, wallPolicyReply{method: "ChangedPaths", from: from, to: to, changed: append([]string(nil), paths...)})
}

func (f *strictWallPolicyFacts) expectSnapshot(expected, post string) {
	f.replies = append(f.replies, wallPolicyReply{method: "Snapshot", from: expected, tree: post})
}

func (f *strictWallPolicyFacts) expectFileAt(tree string, file []byte, present bool, err error) {
	f.replies = append(f.replies, wallPolicyReply{method: "FileAt", from: tree, to: covenant.Filename, file: append([]byte(nil), file...), present: present, err: err})
}

func (f *strictWallPolicyFacts) next(method, from, to string, paths []string, patch []byte) wallPolicyReply {
	f.t.Helper()
	if len(f.replies) == 0 {
		f.t.Fatalf("undeclared wall call %s(%q, %q, %v, %q) at root %q", method, from, to, paths, patch, f.root)
	}
	r := f.replies[0]
	if r.method != method || r.from != from || r.to != to || !reflect.DeepEqual(r.paths, paths) || !bytes.Equal(r.patch, patch) {
		f.t.Fatalf("wall call %s(%q, %q, %v, %q) at root %q, want %+v", method, from, to, paths, patch, f.root, r)
	}
	f.replies = f.replies[1:]
	return r
}

func (f *strictWallPolicyFacts) unexpected(method string) {
	f.t.Helper()
	f.t.Fatalf("undeclared wall method %s at root %q", method, f.root)
}

func (f *strictWallPolicyFacts) Apply(base string, patch []byte) (string, error) {
	r := f.next("Apply", base, "", nil, patch)
	return r.tree, r.err
}
func (f *strictWallPolicyFacts) Entries(tree string, paths []string) (map[string]gittree.Entry, error) {
	r := f.next("Entries", tree, "", paths, nil)
	return r.entries, r.err
}
func (f *strictWallPolicyFacts) ChangedPaths(from, to string) ([]string, error) {
	r := f.next("ChangedPaths", from, to, nil, nil)
	return r.changed, r.err
}
func (f *strictWallPolicyFacts) FileAt(tree, path string) ([]byte, bool, error) {
	r := f.next("FileAt", tree, path, nil, nil)
	return append([]byte(nil), r.file...), r.present, r.err
}
func (f *strictWallPolicyFacts) Snapshot(string) (string, error) {
	f.unexpected("Snapshot")
	panic("unreachable")
}
func (f *strictWallPolicyFacts) SnapshotSeeded(string, string, []string) (string, error) {
	f.unexpected("SnapshotSeeded")
	panic("unreachable")
}
func (f *strictWallPolicyFacts) FilterTree(string, []string) (string, error) {
	f.unexpected("FilterTree")
	panic("unreachable")
}
func (f *strictWallPolicyFacts) HeadTree() (string, error) {
	f.unexpected("HeadTree")
	panic("unreachable")
}
func (f *strictWallPolicyFacts) HeadCommit() (string, bool, error) {
	f.unexpected("HeadCommit")
	panic("unreachable")
}
func (f *strictWallPolicyFacts) TreeOf(string) (string, error) {
	f.unexpected("TreeOf")
	panic("unreachable")
}
func (f *strictWallPolicyFacts) RefMap() (map[string]string, error) {
	f.unexpected("RefMap")
	panic("unreachable")
}
func (f *strictWallPolicyFacts) SymbolicHead() (string, bool, error) {
	f.unexpected("SymbolicHead")
	panic("unreachable")
}
func (f *strictWallPolicyFacts) WorktreeCensus() ([]gittree.WorktreeRecord, error) {
	f.unexpected("WorktreeCensus")
	panic("unreachable")
}
func (f *strictWallPolicyFacts) StagedTree() (string, error) {
	f.unexpected("StagedTree")
	panic("unreachable")
}
func (f *strictWallPolicyFacts) TopStagedPosture() (gittree.StagedPosture, error) {
	f.unexpected("TopStagedPosture")
	panic("unreachable")
}
func (f *strictWallPolicyFacts) Prefix() (string, error) {
	f.unexpected("Prefix")
	panic("unreachable")
}
func (f *strictWallPolicyFacts) TopLevel() (string, error) {
	f.unexpected("TopLevel")
	panic("unreachable")
}
func (f *strictWallPolicyFacts) HistorySteeringFiles() ([]string, error) {
	f.unexpected("HistorySteeringFiles")
	panic("unreachable")
}
func (f *strictWallPolicyFacts) Anchor(string, string) error {
	f.unexpected("Anchor")
	panic("unreachable")
}
func (f *strictWallPolicyFacts) AnchorCommit(string, string, string) error {
	f.unexpected("AnchorCommit")
	panic("unreachable")
}
func (f *strictWallPolicyFacts) MaterializePaths(string, []string) error {
	f.unexpected("MaterializePaths")
	panic("unreachable")
}

func (f *strictWallPolicyFacts) Git(string, ...string) (string, string, int) {
	f.unexpected("Git")
	panic("unreachable")
}
func (f *strictWallPolicyFacts) BlobOID(string, []byte) (string, error) {
	f.unexpected("BlobOID")
	panic("unreachable")
}
func (f *strictWallPolicyFacts) LedgerTruth(string, map[string]any, string) (string, string, error) {
	f.unexpected("LedgerTruth")
	panic("unreachable")
}
func (f *strictWallPolicyFacts) LedgerBlobOID(string, map[string]any, string) (string, error) {
	f.unexpected("LedgerBlobOID")
	panic("unreachable")
}
func (f *strictWallPolicyFacts) AuthenticateLedger(string, map[string]any, string) error {
	f.unexpected("AuthenticateLedger")
	panic("unreachable")
}

var _ wallWorkspace = (*strictWallPolicyFacts)(nil)
var _ wallReads = (*strictWallPolicyFacts)(nil)

var wallPolicyFile = gittree.Entry{Mode: "100644", OID: "ce013625030ba8dba906f756967f9e9ca394464a"}
var wallPolicyOtherFile = gittree.Entry{Mode: "100644", OID: "e69de29bb2d1d6434b8b29ae775ad8c2e48c5391"}
var wallPolicySymlink = gittree.Entry{Mode: "120000", OID: "ce013625030ba8dba906f756967f9e9ca394464a"}

func wallPolicyPatch(label string) []byte { return []byte(fmt.Sprintf("declared patch: %s\n", label)) }
