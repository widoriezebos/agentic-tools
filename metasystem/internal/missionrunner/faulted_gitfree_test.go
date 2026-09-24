package missionrunner

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/mission"
)

// faultedLedgerFacts retains the bytes present at each state anchor. A wall
// read compares that saved fact with the separately read live ledger.
type faultedLedgerFacts struct {
	*hostCycleReads
	root, missionID, statePath, ledgerPath string
	anchored                               []byte
	stateHash, ledgerSHA                   string
	cycles                                 int64
	anchors                                int
}

func (f *faultedLedgerFacts) check(root string, state map[string]any, path string) error {
	if root != f.root || path != f.ledgerPath || state["missionId"] != f.missionID || f.anchors == 0 {
		err := fmt.Errorf("undeclared faulted ledger read: %q %q", root, path)
		f.t.Error(err)
		return err
	}
	integrity, _ := state["integrity"].(map[string]any)
	ledger, _ := state["ledger"].(map[string]any)
	cycle, ok := jsonInt(ledger["cycles"])
	if integrity["hash"] != f.stateHash || !ok || cycle != f.cycles || f.workspace.refs[f.workspace.stateRef()] != hostCandidateCommit {
		return fmt.Errorf("faulted ledger anchor disagrees with state position")
	}
	if path != filepath.Join(root, missionLedgerRel(f.missionID)) {
		return fmt.Errorf("faulted ledger path is outside its declared mission")
	}
	sum := sha256.Sum256(f.anchored)
	if hex.EncodeToString(sum[:]) != f.ledgerSHA {
		return fmt.Errorf("faulted ledger anchor bytes disagree with pinned hash")
	}
	return nil
}

func (f *faultedLedgerFacts) anchor(state, ledger, name string) error {
	if state != f.statePath || ledger != f.ledgerPath || name == "" {
		return fmt.Errorf("undeclared faulted state anchor: %q %q %q", state, ledger, name)
	}
	_, hash, err := mission.VerifyStateShape(state)
	if err != nil {
		return err
	}
	doc := readTestDoc(f.t, state)
	position, _ := doc["ledger"].(map[string]any)
	cycle, ok := jsonInt(position["cycles"])
	if !ok || doc["missionId"] != f.missionID || doc["branch"] != "main" {
		return fmt.Errorf("faulted state anchor has unexpected position")
	}
	if f.workspace.refs[f.workspace.treeRef()] != hostTree {
		return fmt.Errorf("faulted state anchor preceded the baseline tree anchor")
	}
	data, err := os.ReadFile(ledger)
	if err != nil {
		return err
	}
	_, _, entries, err := mission.ParseLedger(ledger)
	if err != nil {
		return err
	}
	if int64(len(entries)) != cycle {
		return fmt.Errorf("faulted anchor ledger cycle disagrees with state")
	}
	sum := sha256.Sum256(data)
	sha := hex.EncodeToString(sum[:])
	if f.stateHash == hash && f.ledgerSHA != "" && f.ledgerSHA != sha {
		return fmt.Errorf("faulted ledger bytes moved without a state write")
	}
	f.stateHash, f.ledgerSHA, f.cycles = hash, sha, cycle
	f.anchored = append([]byte(nil), data...)
	f.anchors++
	f.workspace.refs[f.workspace.stateRef()] = hostCandidateCommit
	return nil
}

func (f *faultedLedgerFacts) LedgerTruth(root string, state map[string]any, path string) (string, string, error) {
	if err := f.check(root, state, path); err != nil {
		return "", "", err
	}
	current, err := os.ReadFile(path)
	return string(f.anchored), string(current), err
}

func (f *faultedLedgerFacts) LedgerBlobOID(root string, state map[string]any, path string) (string, error) {
	if err := f.check(root, state, path); err != nil {
		return "", err
	}
	return birthBlobOID(f.anchored), nil
}

func (f *faultedLedgerFacts) AuthenticateLedger(root string, state map[string]any, path string) error {
	anchored, current, err := f.LedgerTruth(root, state, path)
	if err != nil {
		return err
	}
	if current == anchored {
		return nil
	}
	if !strings.HasPrefix(current, anchored) {
		return fmt.Errorf("live ledger does not extend anchored bytes")
	}
	_, _, entries, err := mission.ParseLedger(path)
	if err != nil {
		return err
	}
	if int64(len(entries)) != f.cycles+1 || !strings.HasPrefix(strings.TrimLeft(current[len(anchored):], "\n"), fmt.Sprintf("### Cycle %d", f.cycles+1)) {
		return fmt.Errorf("live ledger has no single appended cycle")
	}
	stamp := readTestDoc(f.t, filepath.Join(filepath.Dir(path), "pending-block.json"))
	cycle, ok := jsonInt(stamp["cycle"])
	sum := sha256.Sum256([]byte(current))
	if !ok || cycle != f.cycles+1 || stamp["ledgerSha256"] != hex.EncodeToString(sum[:]) {
		return fmt.Errorf("live ledger lacks the exact pending stamp")
	}
	return nil
}

func installFaultedRepository(t *testing.T, e *Engine, source *hostCycleSource) (*hostCycleWorkspace, *faultedLedgerFacts) {
	t.Helper()
	w := &hostCycleWorkspace{t: t, root: e.Root, mission: e.Mission,
		contractOID: birthBlobOID(source.signed), refs: map[string]string{
			"refs/heads/main": hostCandidateCommit, "refs/tags/instruments": hostGateCommit,
		}}
	reads := &faultedLedgerFacts{hostCycleReads: &hostCycleReads{t: t, root: e.Root, workspace: w},
		root: e.Root, missionID: e.Mission, statePath: filepath.Join(e.missionDir(), "state.json"),
		ledgerPath: filepath.Join(e.missionDir(), "ledger.md")}
	e.wallWorkspaceFactory = func(root string) wallWorkspace {
		if root != e.Root {
			t.Fatalf("workspace root %q", root)
		}
		return w
	}
	e.wallReadFacts = reads
	e.anchorFn = reads.anchor
	source.refs = w.refs
	t.Cleanup(source.done)
	return w, reads
}

func openFaultedTurn(t *testing.T, e *Engine, statePath, turnID string, cycle int, w *hostCycleWorkspace) {
	t.Helper()
	state := readTestDoc(t, statePath)
	if baseline, _ := state["initialBaseline"].(string); baseline != "" {
		if err := w.Anchor(e.Mission, baseline); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Anchor(e.Mission, hostTree); err != nil {
		t.Fatal(err)
	}
	if err := e.anchorFn(statePath, filepath.Join(e.missionDir(), "ledger.md"), "fixture-before-open"); err != nil {
		t.Fatal(err)
	}
	openFixtureTurnWithSource(t, e.Root, statePath, turnID, cycle, e.wallWorkspace(e.Root), e.anchorFn)
}
