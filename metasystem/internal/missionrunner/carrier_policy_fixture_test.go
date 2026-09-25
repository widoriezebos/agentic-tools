package missionrunner

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
)

const (
	carrierPolicyStagedRaw = "1111111111111111111111111111111111111111"
	carrierCommit          = "2222222222222222222222222222222222222222"
	carrierCommitRaw       = "3333333333333333333333333333333333333333"
	carrierCommitTree      = "4444444444444444444444444444444444444444"
	carrierLedgerRaw       = "5555555555555555555555555555555555555555"
	carrierLedgerCommit    = "6666666666666666666666666666666666666666"
)

func carrierBlobOID(data []byte) string {
	payload := append([]byte(fmt.Sprintf("blob %d\x00", len(data))), data...)
	sum := sha1.Sum(payload)
	return hex.EncodeToString(sum[:])
}

// The ledger path's raw entries are declared per call; the file and its
// authenticated bytes are read independently of the repository facts.
type carrierLedgerFacts struct {
	*recoveryFacts
	anchorOID, foreignOID, foreignTree string
	stageProbe, headProbe              bool
}

func newCarrierLedgerBed(t *testing.T) (*recoveryFileBed, *carrierLedgerFacts) {
	b := newRecoveryFileBed(t)
	f := &carrierLedgerFacts{recoveryFacts: b.facts, anchorOID: carrierBlobOID(b.facts.original)}
	b.e.wallWorkspaceFactory = func(root string) wallWorkspace {
		if root != b.e.Root {
			t.Fatalf("workspace root %q", root)
		}
		return f
	}
	b.e.wallReadFacts = f
	return b, f
}

func (f *carrierLedgerFacts) LedgerBlobOID(root string, state map[string]any, path string) (string, error) {
	r := f.next("LedgerBlobOID", root, state, path)
	if root != f.bed.e.Root || path != f.bed.ledger || !reflect.DeepEqual(state, f.stateDoc()) {
		f.t.Fatalf("ledger anchor arguments %q %q", root, path)
	}
	current, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(current, f.original) || r.value != f.anchorOID {
		f.t.Fatalf("ledger anchor differs from unchanged file: %v", err)
	}
	return f.anchorOID, r.err
}

// A staged foreign ledger entry may be read before or after the clean HEAD
// entry because the policy visits those two scopes through a Go map.
func (f *carrierLedgerFacts) Entries(tree string, paths []string) (map[string]gittree.Entry, error) {
	if f.foreignTree != "" {
		ledger := missionLedgerRel(f.bed.e.Mission)
		if !f.headProbe && !f.stageProbe {
			f.next("CarrierLedgerEntries", recoveryPre, f.foreignTree, []string{ledger})
		}
		if !reflect.DeepEqual(paths, []string{ledger}) {
			f.t.Fatalf("ledger entry paths %v", paths)
		}
		switch tree {
		case recoveryPre:
			if f.headProbe || f.stageProbe {
				f.t.Fatal("duplicate or late HEAD ledger probe")
			}
			f.headProbe = true
			return map[string]gittree.Entry{}, nil
		case f.foreignTree:
			if f.stageProbe {
				f.t.Fatal("duplicate staged ledger probe")
			}
			f.stageProbe = true
			return map[string]gittree.Entry{ledger: {OID: f.foreignOID, Mode: "100644"}}, nil
		default:
			f.t.Fatalf("undeclared staged carrier tree %q", tree)
		}
	}
	return f.recoveryFacts.Entries(tree, paths)
}

func (f *carrierLedgerFacts) expectCapture(head, headRaw, stagedRaw, stagedFiltered string) {
	ledger := []string{missionLedgerRel(f.bed.e.Mission)}
	f.add("HistorySteeringFiles", []string{})
	f.add("HeadCommit", scopePolicyHead{oid: head})
	f.add("TreeOf", headRaw, head)
	f.add("FilterTree", recoveryPre, headRaw, ledger)
	f.add("SymbolicHead", scopePolicyBranch{ref: "refs/heads/main"})
	refs := f.refs()
	refs["refs/heads/main"] = head
	f.add("RefMap", refs)
	f.add("WorktreeCensus", []gittree.WorktreeRecord{{Path: f.bed.e.Root, HeadOID: head,
		Branch: "refs/heads/main", PostureReadable: true, Staged: gittree.StagedPosture{Tree: stagedRaw}}})
	f.add("StagedTree", stagedRaw)
	f.add("FilterTree", stagedFiltered, stagedRaw, ledger)
	f.add("Prefix", "")
	f.add("SnapshotSeeded", recoverySnapshot{post: recoveryPre}, head, recoveryPre, []string{})
	f.add("FilterTree", recoveryPre, recoveryPre, ledger)
}

func (f *carrierLedgerFacts) expectLedgerOID() {
	f.add("LedgerBlobOID", f.anchorOID, f.bed.e.Root, f.stateDoc(), f.bed.ledger)
}

type carrierResolutionFacts struct {
	*resolutionFacts
	anchorOID  string
	guardTrees map[string]bool
}

func newCarrierResolutionBed(t *testing.T) (*resolutionFileBed, *carrierResolutionFacts) {
	b := newResolutionFileBed(t)
	f := &carrierResolutionFacts{resolutionFacts: b.facts, anchorOID: carrierBlobOID(b.anchored)}
	b.e.wallWorkspaceFactory = func(root string) wallWorkspace {
		if root != b.e.Root {
			t.Fatalf("workspace root %q", root)
		}
		return f
	}
	b.e.wallReadFacts = f
	t.Cleanup(func() {
		if len(f.guardTrees) != 0 {
			t.Errorf("unconsumed raw ledger guard entries: %v", f.guardTrees)
		}
	})
	return b, f
}

func (f *carrierResolutionFacts) LedgerBlobOID(root string, state map[string]any, path string) (string, error) {
	r := f.next("LedgerBlobOID", root, state, path)
	if root != f.bed.e.Root || path != f.bed.ledger || !reflect.DeepEqual(state, f.stateDoc()) {
		f.t.Fatalf("ledger anchor arguments %q %q", root, path)
	}
	current, err := os.ReadFile(path)
	b := f.bed.e.continuityFacts.(*resolutionFileBed)
	if err != nil || !bytes.Equal(current, b.anchored) || r.value != f.anchorOID {
		f.t.Fatalf("ledger anchor differs from unchanged file: %v", err)
	}
	return f.anchorOID, r.err
}

func (f *carrierResolutionFacts) Entries(tree string, paths []string) (map[string]gittree.Entry, error) {
	if f.guardTrees != nil {
		if len(f.guardTrees) == 2 {
			f.next("CarrierGuardEntries", carrierCommitRaw, recoveryPre, []string{missionLedgerRel(f.bed.e.Mission)})
		}
		if !reflect.DeepEqual(paths, []string{missionLedgerRel(f.bed.e.Mission)}) || !f.guardTrees[tree] {
			f.t.Fatalf("undeclared raw ledger guard entry %q %v", tree, paths)
		}
		delete(f.guardTrees, tree)
		if len(f.guardTrees) == 0 {
			f.guardTrees = nil
		}
		return map[string]gittree.Entry{}, nil
	}
	return f.recoveryFacts.Entries(tree, paths)
}

func (f *carrierResolutionFacts) checkSafeFiles() {
	f.checkedTree(recoveryPre)
	for _, name := range []string{"staged-extra.go", "committed-extra.go"} {
		if _, err := os.Stat(filepath.Join(f.bed.e.Root, name)); !os.IsNotExist(err) {
			f.t.Fatalf("safe worktree still has %s: %v", name, err)
		}
	}
}

func (f *carrierResolutionFacts) Snapshot(seed string) (string, error) {
	f.checkSafeFiles()
	return f.resolutionFacts.Snapshot(seed)
}

func (f *carrierResolutionFacts) SnapshotSeeded(seed, expected string, paths []string) (string, error) {
	f.checkSafeFiles()
	return f.resolutionFacts.SnapshotSeeded(seed, expected, paths)
}

func (f *carrierResolutionFacts) expectOID() {
	f.add("LedgerBlobOID", f.anchorOID, f.bed.e.Root, f.stateDoc(), f.bed.ledger)
}

func (f *carrierResolutionFacts) expectCapture(head, headRaw, headTree, stagedRaw, stagedTree string, pseudorefs []gittree.Pseudoref) {
	ledger := []string{missionLedgerRel(f.bed.e.Mission)}
	f.add("HistorySteeringFiles", []string{})
	f.add("HeadCommit", scopePolicyHead{oid: head})
	f.add("TreeOf", headRaw, head)
	f.add("FilterTree", headTree, headRaw, ledger)
	f.add("SymbolicHead", scopePolicyBranch{ref: "refs/heads/main"})
	refs := f.refs()
	refs["refs/heads/main"] = head
	f.add("RefMap", refs)
	f.add("WorktreeCensus", []gittree.WorktreeRecord{{Path: f.bed.e.Root, HeadOID: head,
		Branch: "refs/heads/main", PostureReadable: true, Pseudorefs: pseudorefs,
		Staged: gittree.StagedPosture{Tree: stagedRaw}}})
	f.add("StagedTree", stagedRaw)
	f.add("FilterTree", stagedTree, stagedRaw, ledger)
	f.add("Prefix", "")
	f.add("SnapshotSeeded", recoveryPre, head, recoveryPre, []string{})
	f.add("FilterTree", recoveryPre, recoveryPre, ledger)
}

func (f *carrierResolutionFacts) expectRawLedger(tree string) {
	f.add("Entries", map[string]gittree.Entry{}, tree, []string{missionLedgerRel(f.bed.e.Mission)})
}

func (f *carrierResolutionFacts) expectLedgerGuard(head, raw, stagedRaw string) {
	f.expectOID()
	f.add("TreeOf", raw, head)
	f.add("StagedTree", stagedRaw)
	if raw == stagedRaw {
		f.expectRawLedger(raw)
		f.expectRawLedger(raw)
	} else {
		f.guardTrees = map[string]bool{raw: true, stagedRaw: true}
		f.add("CarrierGuardEntries", nil, raw, stagedRaw, []string{missionLedgerRel(f.bed.e.Mission)})
	}
}

func (f *carrierResolutionFacts) expectNamespace() {
	f.add("Git", scopePolicyGit{code: 1}, f.bed.e.Root, []string{"cat-file", "-e", recoveryPre})
	f.add("AuthenticateLedger", nil, f.bed.e.Root, f.stateDoc(), f.bed.ledger)
}

func (f *carrierResolutionFacts) expectAccountant() {
	f.add("Prefix", "")
	f.expectOID()
}

func (f *carrierResolutionFacts) expectSafeJudge() {
	f.expectLedgerGuard(recoveryHead, recoveryPre, recoveryPre)
	f.expectNamespace()
	f.add("TopLevel", f.bed.e.Root)
}
