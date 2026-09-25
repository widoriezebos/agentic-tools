package missionrunner

import (
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/mission"
)

const birthTree = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
const birthRaw = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
const birthHead = "cccccccccccccccccccccccccccccccccccccccc"
const birthStaged = "dddddddddddddddddddddddddddddddddddddddd"
const birthClean = "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"
const birthDirty = "ffffffffffffffffffffffffffffffffffffffff"

func birthBlobOID(data []byte) string {
	h := sha1.New()
	fmt.Fprintf(h, "blob %d\x00", len(data))
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil))
}

type birthEffectCall struct{ method, root, mission, tree string }
type strictBirthEffects struct {
	t     *testing.T
	calls []birthEffectCall
}

func (f *strictBirthEffects) expect(method, root, mission, tree string) {
	f.t.Helper()
	if len(f.calls) == 0 {
		f.t.Fatalf("undeclared birth effect %s(%q,%q,%q)", method, root, mission, tree)
	}
	want := f.calls[0]
	f.calls = f.calls[1:]
	if want != (birthEffectCall{method, root, mission, tree}) {
		f.t.Fatalf("birth effect %v, want %v", birthEffectCall{method, root, mission, tree}, want)
	}
}
func (f *strictBirthEffects) CaptureAdmissionOrigins(root, mission string) (map[string]any, error) {
	f.expect("CaptureAdmissionOrigins", root, mission, "")
	raw, err := json.Marshal(testAdmissionOrigins())
	if err != nil {
		f.t.Fatal(err)
	}
	var copied map[string]any
	if err = json.Unmarshal(raw, &copied); err != nil {
		f.t.Fatal(err)
	}
	return copied, nil
}
func (f *strictBirthEffects) AnchorInitial(root, mission, tree string) error {
	f.expect("AnchorInitial", root, mission, tree)
	return nil
}
func (f *strictBirthEffects) DropAnchors(root, mission string) error {
	f.expect("DropAnchors", root, mission, "")
	return nil
}

// birthBed uses real contract, fence, ledger, and state files. Its queues
// declare repository facts and effects at each admission boundary.
type birthBed struct {
	t          *testing.T
	engine     *Engine
	snapshot   []byte
	lease      string
	reads      *strictWallReads
	workspace  *strictBaselineWorkspace
	effects    *strictBirthEffects
	continuity *birthContinuity
}

type birthContinuity struct {
	t                   *testing.T
	calls               []string
	root, state, ledger string
}

func (f *birthContinuity) next(method, state, root, ledger string) {
	f.t.Helper()
	if len(f.calls) == 0 || f.calls[0] != method || state != f.state || root != f.root || ledger != f.ledger {
		f.t.Fatalf("undeclared continuity %s(%q,%q,%q), remaining %v", method, state, root, ledger, f.calls)
	}
	f.calls = f.calls[1:]
}
func (f *birthContinuity) Reconcile(state, root, ledger string) (int, error) {
	f.next("Reconcile", state, root, ledger)
	_, _, err := mission.VerifyStateShape(state)
	return 0, err
}
func (f *birthContinuity) LedgerPin(root, missionID string) (string, error) {
	f.t.Fatalf("undeclared LedgerPin(%q,%q)", root, missionID)
	return "", nil
}
func (f *birthContinuity) VerifyStateWithAnchor(state, root, ledger string) (int64, string, error) {
	f.next("VerifyStateWithAnchor", state, root, ledger)
	return mission.VerifyStateShape(state)
}

func newBirthBed(t *testing.T) *birthBed {
	t.Helper()
	e := &Engine{Root: t.TempDir(), Mission: "alpha"}
	// Keep the authored contract bytes used by the physical fixture. The
	// generated seal supplies the branch that state birth requires.
	snapshot := []byte(fixtureContract + "\n```mission-seal\ncandidate.branch=goal/fixture\n```\n\nApproval: name=Fixture Human; date=2026-08-12; contract-sha256=" + strings.Repeat("a", 64) + "\n")
	if err := os.MkdirAll(filepath.Dir(e.contractPath()), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(e.contractPath(), snapshot, 0644); err != nil {
		t.Fatal(err)
	}
	b := &birthBed{t: t, engine: e, snapshot: snapshot, lease: filepath.Join(e.Root, "artifacts", "agents", "checkout.lease.json")}
	b.reads = &strictWallReads{t: t}
	e.wallReadFacts = b.reads
	b.workspace = &strictBaselineWorkspace{t: t}
	e.wallWorkspaceFactory = func(root string) wallWorkspace {
		if root != e.Root {
			t.Fatalf("workspace root %q", root)
		}
		return b.workspace
	}
	b.effects = &strictBirthEffects{t: t}
	e.birthEffects = b.effects
	b.continuity = &birthContinuity{t: t, root: e.Root, state: filepath.Join(e.missionDir(), "state.json"), ledger: filepath.Join(e.missionDir(), "ledger.md")}
	e.continuityFacts = b.continuity
	e.anchorFn = func(state, ledger, identity string) error {
		if state != b.continuity.state || ledger != b.continuity.ledger || identity != e.Mission {
			t.Fatalf("state anchor args %q %q %q", state, ledger, identity)
		}
		_, _, err := mission.VerifyStateShape(state)
		return err
	}
	t.Cleanup(b.done)
	b.pin()
	return b
}
func (b *birthBed) done() {
	b.reads.done()
	if len(b.workspace.queries) != 0 {
		b.t.Fatalf("unconsumed workspace facts: %+v", b.workspace.queries)
	}
	if len(b.effects.calls) != 0 {
		b.t.Fatalf("unconsumed birth effects: %+v", b.effects.calls)
	}
	if len(b.continuity.calls) != 0 {
		b.t.Fatalf("unconsumed continuity: %+v", b.continuity.calls)
	}
}
func (b *birthBed) expectStart() {
	e := b.engine
	b.reads.git = append(b.reads.git, wallGitReply{root: e.Root, args: []string{"for-each-ref", "--format=%(refname)", "refs/metasystem/missions/" + e.Mission + "/"}})
}
func (b *birthBed) fileMode(value string) {
	b.reads.git = append(b.reads.git, wallGitReply{root: b.engine.Root, args: []string{"config", "--local", "--type=bool", "--get", "core.fileMode"}, stdout: value + "\n"})
}
func (b *birthBed) pin() {
	b.t.Helper()
	b.expectStart()
	digest := sha256.Sum256(b.snapshot)
	if err := b.engine.pinVerifiedContract("start", b.snapshot, hex.EncodeToString(digest[:])); err != nil {
		b.t.Fatalf("pin: %v", err)
	}
}
func (b *birthBed) admission(live []byte, dirty bool) {
	e := b.engine
	b.fileMode("true")
	contract := filepath.Join("plans", "mission-"+e.Mission+".contract.md")
	ledger := missionLedgerRel(e.Mission)
	entry := map[string]gittree.Entry{contract: {Mode: "100644", OID: birthBlobOID(live)}}
	pair := []string{ledger, contract}
	observed := birthClean
	if dirty {
		observed = birthDirty
	}
	b.workspace.queries = append(b.workspace.queries, []baselineQuery{
		{method: "Snapshot", tree: "HEAD", result: birthRaw},
		{method: "FilterTree", tree: birthRaw, paths: pair, result: observed},
		{method: "HeadTree", result: birthHead},
		{method: "FilterTree", tree: birthHead, paths: pair, result: birthClean},
		{method: "Entries", tree: birthRaw, paths: []string{contract}, entries: entry},
		{method: "Entries", tree: birthHead, paths: []string{contract}, entries: entry},
	}...)
	b.reads.bytes = append(b.reads.bytes, wallByteReply{root: e.Root, content: append([]byte(nil), b.snapshot...), oid: birthBlobOID(b.snapshot)})
	if !reflect.DeepEqual(live, b.snapshot) {
		return
	}
	b.workspace.queries = append(b.workspace.queries, []baselineQuery{
		{method: "RefMap"},
		{method: "FilterTree", tree: birthRaw, paths: []string{ledger}, result: birthTree},
		{method: "StagedTree", result: birthStaged},
		{method: "FilterTree", tree: birthStaged, paths: []string{ledger}, result: birthTree},
		{method: "FilterTree", tree: birthHead, paths: []string{ledger}, result: birthTree},
	}...)
}
func (b *birthBed) birth() {
	b.expectStart()
	b.admission(b.snapshot, false)
	b.admission(b.snapshot, false)
	e := b.engine
	b.effects.calls = append(b.effects.calls,
		birthEffectCall{"CaptureAdmissionOrigins", e.Root, e.Mission, ""},
		birthEffectCall{"CaptureAdmissionOrigins", e.Root, e.Mission, ""},
		birthEffectCall{"AnchorInitial", e.Root, e.Mission, birthTree})
}
func (b *birthBed) verifyBirth() {
	b.continuity.calls = append(b.continuity.calls, "VerifyStateWithAnchor")
}
func (b *birthBed) drop() {
	e := b.engine
	b.effects.calls = append(b.effects.calls, birthEffectCall{"DropAnchors", e.Root, e.Mission, ""})
}
func (b *birthBed) resume() {
	b.fileMode("true")
	b.continuity.calls = append(b.continuity.calls, "Reconcile", "VerifyStateWithAnchor")
}
func (b *birthBed) atState() string  { return filepath.Join(b.engine.missionDir(), "state.json") }
func (b *birthBed) atLedger() string { return filepath.Join(b.engine.missionDir(), "ledger.md") }

var _ birthRepositoryEffects = (*strictBirthEffects)(nil)
var _ missionContinuity = (*birthContinuity)(nil)
