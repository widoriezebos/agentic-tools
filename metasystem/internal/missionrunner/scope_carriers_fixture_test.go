package missionrunner

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/mission"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/validate"
)

// The carrier facts describe one repository root. The shared policy queue
// rejects every undeclared capture or history query; these two additional
// methods admit only the patch and entry reads needed by authorization.
type scopeCarrierFacts struct{ *scopePolicyFacts }

func (f *scopeCarrierFacts) Apply(base string, patch []byte) (string, error) {
	r := f.next("Apply", base, patch)
	return r.value.(string), r.err
}

func (f *scopeCarrierFacts) Entries(tree string, paths []string) (map[string]gittree.Entry, error) {
	r := f.next("Entries", tree, paths)
	return r.value.(map[string]gittree.Entry), r.err
}

type scopeCarrierPosture struct {
	staged, post string
	census       []gittree.WorktreeRecord
}

type scopeCarrierBed struct {
	t      *testing.T
	root   string
	engine *Engine
	facts  *scopeCarrierFacts
	state  map[string]any
	origin *scopeOrigin
}

const (
	carrierOpen       = "open-commit"
	carrierPreRaw     = "raw-pre-tree"
	carrierPre        = "pre-tree"
	carrierStagedRaw  = "raw-staged-tree"
	carrierStagedTree = "staged-tree"
)

func newScopeCarrierBed(t *testing.T) *scopeCarrierBed {
	t.Helper()
	root := t.TempDir()
	facts := &scopeCarrierFacts{scopePolicyFacts: &scopePolicyFacts{t: t, root: root}}
	engine := &Engine{Root: root, Mission: "demo", wallReadFacts: facts}
	engine.wallWorkspaceFactory = func(requested string) wallWorkspace {
		if requested != root {
			t.Fatalf("workspace root %q, want %q", requested, root)
		}
		return facts
	}
	t.Cleanup(facts.done)
	bed := &scopeCarrierBed{t: t, root: root, engine: engine, facts: facts}
	bed.state = map[string]any{
		"branch": "main", "turnLog": []any{}, "initialBaseline": carrierPre,
		"workspaceTaint": map[string]any{"next": 1, "segment": 0, "entries": []any{}},
	}
	statePath := filepath.Join(missionDirPath(root, "demo"), "state.json")
	writeJSONFile(t, statePath, bed.state)
	state, err := readJSONDoc(statePath)
	if err != nil {
		t.Fatal(err)
	}
	bed.state = state
	facts.add("Snapshot", carrierPreRaw, "HEAD")
	facts.add("FilterTree", carrierPre, carrierPreRaw, []string{missionLedgerRel("demo")})
	pre, err := wallSnapshotWithWorkspace(engine.wallWorkspace(root), "demo")
	if err != nil || pre != carrierPre {
		t.Fatalf("open snapshot = %q, %v", pre, err)
	}
	return bed
}

func (b *scopeCarrierBed) main(pseudorefs ...gittree.Pseudoref) gittree.WorktreeRecord {
	return gittree.WorktreeRecord{Path: b.root, HeadOID: carrierOpen, Branch: "refs/heads/main",
		PostureReadable: true, Pseudorefs: pseudorefs, Staged: gittree.StagedPosture{Tree: carrierPreRaw}}
}

func (b *scopeCarrierBed) expectCapture(p scopeCarrierPosture, expected string) {
	f := b.facts
	ledger := []string{missionLedgerRel("demo")}
	f.add("HistorySteeringFiles", []string{})
	f.add("HeadCommit", scopePolicyHead{oid: carrierOpen})
	f.add("TreeOf", carrierPreRaw, carrierOpen)
	f.add("FilterTree", carrierPre, carrierPreRaw, ledger)
	f.add("SymbolicHead", scopePolicyBranch{ref: "refs/heads/main"})
	f.add("RefMap", map[string]string{"refs/heads/main": carrierOpen})
	f.add("WorktreeCensus", p.census)
	f.add("StagedTree", p.staged)
	filtered := carrierPre
	if p.staged == carrierStagedRaw {
		filtered = carrierStagedTree
	}
	f.add("FilterTree", filtered, p.staged, ledger)
	f.add("Prefix", "")
	if expected != "" {
		f.add("SnapshotSeeded", "raw-"+p.post, carrierOpen, expected, []string{})
		f.add("FilterTree", p.post, "raw-"+p.post, ledger)
	}
}

func (b *scopeCarrierBed) captureOrigin() {
	b.t.Helper()
	b.expectCapture(scopeCarrierPosture{staged: carrierPreRaw, census: []gittree.WorktreeRecord{b.main()}}, "")
	c, err := b.engine.captureWallPosture("", nil)
	if err != nil {
		b.t.Fatal(err)
	}
	b.origin = &scopeOrigin{Head: c.Head, RefMap: c.RefMap, StagedPost: c.StagedTree}
	for _, raw := range mission.WorktreeCensusDoc(c.Census) {
		b.origin.Census = append(b.origin.Census, raw.(map[string]any))
	}
}

func (b *scopeCarrierBed) expectLedger() {
	b.facts.addErr("LedgerBlobOID", "", mission.ErrNoAnchor, b.root, b.state,
		filepath.Join(missionDirPath(b.root, "demo"), "ledger.md"))
}

func (b *scopeCarrierBed) judge(p scopeCarrierPosture, auths []scopeAuth, expected string, afterCapture func()) string {
	b.t.Helper()
	b.expectCapture(p, expected)
	b.facts.add("Prefix", "")
	b.expectLedger()
	b.expectLedger()
	if afterCapture != nil {
		afterCapture()
	}
	c, err := b.engine.captureWallPosture(expected, nil)
	if err != nil {
		b.t.Fatal(err)
	}
	a, err := b.engine.newWallAccountant(carrierPre, b.state, auths, nil)
	if err != nil {
		b.t.Fatal(err)
	}
	v, err := b.engine.judgeScope(b.origin, c, a, b.state)
	if err != nil {
		b.t.Fatalf("judge: %v", err)
	}
	if len(b.facts.answers) != 0 {
		b.t.Fatalf("unused carrier answers: %+v", b.facts.answers)
	}
	return v
}

// The issued record and patch live under the ordinary mission directory.
// Their digest binds the exact patch bytes that the wall later consumes.
func (b *scopeCarrierBed) authorization(reviewed string, patch []byte) string {
	b.t.Helper()
	patchSum := sha256.Sum256(patch)
	record := map[string]any{
		"jobId": "job-staged", "rootJob": "job-staged", "mission": b.engine.Mission,
		"baseTree": carrierPre, "reviewedTree": reviewed,
		"baseSequencePoint": map[string]any{"sequence": 0, "segment": 0},
		"patchDigest":       hex.EncodeToString(patchSum[:]),
		"changedPaths":      []any{"staged.go"}, "supersedes": []any{},
	}
	digest, err := validate.AuthorizationRecordDigest(record)
	if err != nil {
		b.t.Fatal(err)
	}
	record["authorizationDigest"] = digest
	dir := filepath.Join(missionDirPath(b.root, b.engine.Mission), "authorizations")
	writeJSONFile(b.t, filepath.Join(dir, digest+".json"), record)
	if err := os.WriteFile(filepath.Join(dir, digest+".patch"), patch, 0o644); err != nil {
		b.t.Fatal(err)
	}
	return digest
}
