package missionrunner

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/mission"
)

const resolutionSolo = "package solo\n"

// resolutionFileBed keeps mission documents on disk and declares only the
// repository facts that an ordinary checkout would provide.
type resolutionFileBed struct {
	*recoveryFileBed
	facts        *resolutionFacts
	anchorHash   string
	anchorSHA    string
	anchored     []byte
	pins         int
	nextIdentity string
}

type resolutionFacts struct{ *recoveryFacts }

func newResolutionFileBed(t *testing.T) *resolutionFileBed {
	t.Helper()
	r := newRecoveryFileBed(t, map[string]string{"solo.go": resolutionSolo})
	writeText(t, filepath.Join(r.e.Root, "metasystem.conf"), "metasystem.runtimes=fake\n")
	b := &resolutionFileBed{recoveryFileBed: r, nextIdentity: "Wido"}
	f := &resolutionFacts{r.facts}
	b.facts = f
	r.e.wallWorkspaceFactory = func(root string) wallWorkspace {
		if root != r.e.Root {
			t.Fatalf("workspace root = %q, want %q", root, r.e.Root)
		}
		return f
	}
	r.e.wallReadFacts = f
	r.e.continuityFacts = b
	r.e.pinnedAnchorEffect = b.pin
	state := f.stateDoc()
	park, err := ParkProposal(r.e.Root, r.e.Mission, state, "wall-violation", "2026-08-18T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	if len(park.Asks) != 1 {
		t.Fatalf("park asks = %d", len(park.Asks))
	}
	park.Asks[0]["taintId"] = int64(1)
	if _, err := appendTaintEntry(park.State, "alpha-t1-live", "undeclared host-authored change: solo.go"); err != nil {
		t.Fatal(err)
	}
	if err := r.e.writeProposedAsks(park.Asks); err != nil {
		t.Fatal(err)
	}
	b.writePark(park.State)
	stageHumanShell(t)
	return b
}

func (b *resolutionFileBed) writePark(proposed map[string]any) {
	b.t.Helper()
	_, before, err := mission.VerifyStateShape(b.state)
	if err != nil {
		b.t.Fatal(err)
	}
	delete(proposed, "integrity")
	source := b.state + ".resolution.src"
	if err := atomicWriteJSON(source, proposed); err != nil {
		b.t.Fatal(err)
	}
	if err := mission.WriteState(b.state, source, before); err != nil {
		b.t.Fatal(err)
	}
	b.recordAnchor()
}

func (b *resolutionFileBed) recordAnchor() {
	b.t.Helper()
	_, hash, err := mission.VerifyStateShape(b.state)
	if err != nil {
		b.t.Fatal(err)
	}
	ledger, err := os.ReadFile(b.ledger)
	if err != nil {
		b.t.Fatal(err)
	}
	b.anchorHash = hash
	b.anchorSHA = sha256Hex(string(ledger))
	b.anchored = append([]byte(nil), ledger...)
	b.facts.original = append([]byte(nil), ledger...)
}

func (b *resolutionFileBed) VerifyStateWithAnchor(state, root, ledger string) (int64, string, error) {
	if state != b.state || root != b.e.Root || ledger != b.ledger {
		b.t.Fatalf("continuity arguments = %q %q %q", state, root, ledger)
	}
	sequence, hash, err := mission.VerifyStateShape(state)
	if err != nil {
		return 0, "", err
	}
	if hash != b.anchorHash {
		return 0, "", fmt.Errorf("state hash %s differs from anchor %s", hash, b.anchorHash)
	}
	return sequence, hash, nil
}

func (b *resolutionFileBed) Reconcile(state, root, ledger string) (int, error) {
	b.t.Fatalf("unexpected Reconcile(%q, %q, %q)", state, root, ledger)
	return 3, fmt.Errorf("unexpected reconcile")
}

func (b *resolutionFileBed) LedgerPin(root, missionID string) (string, error) {
	if root != b.e.Root || missionID != b.e.Mission {
		b.t.Fatalf("ledger pin arguments = %q %q", root, missionID)
	}
	return b.anchorSHA, nil
}

func (b *resolutionFileBed) pin(state, ledger, identity, hash, sha string) error {
	if state != b.state || ledger != b.ledger || identity != b.nextIdentity {
		b.t.Fatalf("pinned anchor arguments = %q %q %q", state, ledger, identity)
	}
	_, actualHash, err := mission.VerifyStateShape(state)
	if err != nil {
		return err
	}
	current, err := os.ReadFile(ledger)
	if err != nil {
		return err
	}
	if actualHash != hash || sha256Hex(string(current)) != sha {
		return fmt.Errorf("pinned anchor differs from written state or current ledger")
	}
	b.anchorHash, b.anchorSHA = hash, sha
	b.anchored = append([]byte(nil), current...)
	b.pins++
	return nil
}

func (f *resolutionFacts) LedgerTruth(root string, state map[string]any, path string) (string, string, error) {
	f.next("LedgerTruth", root, state, path)
	if root != f.bed.e.Root || path != f.bed.ledger || !reflect.DeepEqual(state, f.stateDoc()) {
		f.t.Fatalf("ledger truth arguments = %q %q", root, path)
	}
	current, err := os.ReadFile(path)
	if err != nil {
		return "", "", err
	}
	return string(f.bed.e.continuityFacts.(*resolutionFileBed).anchored), string(current), nil
}

func (f *resolutionFacts) checkedTree(tree string) {
	f.t.Helper()
	bytes, err := os.ReadFile(filepath.Join(f.bed.e.Root, "solo.go"))
	switch tree {
	case recoveryPre:
		if !os.IsNotExist(err) {
			f.t.Fatalf("safe projection still has solo.go: %q, %v", bytes, err)
		}
	case recoveryPost:
		if err != nil || string(bytes) != resolutionSolo {
			f.t.Fatalf("disputed projection requires solo.go bytes: %q, %v", bytes, err)
		}
	default:
		f.t.Fatalf("undeclared projection %q", tree)
	}
}

func (f *resolutionFacts) Snapshot(baseline string) (string, error) {
	r := f.next("Snapshot", baseline)
	tree := r.value.(string)
	f.checkedTree(tree)
	return tree, r.err
}

func (f *resolutionFacts) SnapshotSeeded(seed, expected string, paths []string) (string, error) {
	r := f.next("SnapshotSeeded", seed, expected, paths)
	tree := r.value.(string)
	f.checkedTree(tree)
	return tree, r.err
}

func (b *resolutionFileBed) snapshot(tree string) {
	b.facts.add("Snapshot", tree, "HEAD")
	b.facts.add("FilterTree", tree, tree, []string{missionLedgerRel(b.e.Mission)})
}

func (b *resolutionFileBed) capture(tree string) {
	f := b.facts
	ledger := []string{missionLedgerRel(b.e.Mission)}
	f.add("HistorySteeringFiles", []string{})
	f.add("HeadCommit", scopePolicyHead{oid: recoveryHead})
	f.add("TreeOf", recoveryPre, recoveryHead)
	f.add("FilterTree", recoveryPre, recoveryPre, ledger)
	f.add("SymbolicHead", scopePolicyBranch{ref: "refs/heads/main"})
	f.add("RefMap", f.refs())
	f.add("WorktreeCensus", resolutionCensus(b.e.Root))
	f.add("StagedTree", recoveryPre)
	f.add("FilterTree", recoveryPre, recoveryPre, ledger)
	f.add("Prefix", "")
	f.add("SnapshotSeeded", tree, recoveryHead, tree, []string{})
	f.add("FilterTree", tree, tree, ledger)
}

func resolutionCensus(root string) []gittree.WorktreeRecord {
	return []gittree.WorktreeRecord{{Path: root, HeadOID: recoveryHead, Branch: "refs/heads/main",
		PostureReadable: true, Staged: gittree.StagedPosture{Tree: recoveryPre}}}
}

func (b *resolutionFileBed) expectResolve(tree string, restore, last, ledgerDomain bool) {
	b.snapshot(tree)
	if !ledgerDomain || !restore {
		b.facts.add("Anchor", nil, b.e.Mission, tree)
		b.snapshot(tree)
		b.capture(tree)
		if restore {
			b.facts.add("Prefix", "")
			b.facts.add("LedgerBlobOID", nil, b.e.Root, b.facts.stateDoc(), b.ledger)
			b.expectJudge()
		}
		if !ledgerDomain {
			b.ledgerTruth()
		}
		b.capture(tree)
		b.expectIntegrity()
		if last {
			b.dropOpenHead()
		}
	}
}

func (b *resolutionFileBed) expectJudge() {
	f := b.facts
	f.add("LedgerBlobOID", nil, b.e.Root, f.stateDoc(), b.ledger)
	f.add("Git", scopePolicyGit{code: 1}, b.e.Root, []string{"cat-file", "-e", recoveryPre})
	f.add("AuthenticateLedger", nil, b.e.Root, f.stateDoc(), b.ledger)
	f.add("TopLevel", b.e.Root)
}

func (b *resolutionFileBed) expectIntegrity() {
	f := b.facts
	f.add("LedgerBlobOID", nil, b.e.Root, f.stateDoc(), b.ledger)
	f.add("Git", scopePolicyGit{code: 1}, b.e.Root, []string{"cat-file", "-e", recoveryPre})
	f.add("AuthenticateLedger", nil, b.e.Root, f.stateDoc(), b.ledger)
}

func (b *resolutionFileBed) ledgerTruth() {
	b.facts.add("LedgerTruth", nil, b.e.Root, b.facts.stateDoc(), b.ledger)
}

func (b *resolutionFileBed) dropOpenHead() {
	ref := mission.MissionRefNamespace(b.e.Mission) + "turn-open-head"
	b.facts.add("Git", scopePolicyGit{code: 0}, b.e.Root, []string{"update-ref", "-d", ref})
}
