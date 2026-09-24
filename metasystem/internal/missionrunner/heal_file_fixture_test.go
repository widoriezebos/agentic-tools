package missionrunner

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/mission"
)

const fileHealHEAD = "dddddddddddddddddddddddddddddddddddddddd"

// fileHealMission represents a runner that spent a cycle before appending
// its ledger entry. Repository HEAD is supplied as one exact wall reply.
type fileHealMission struct {
	engine                *Engine
	statePath, ledgerPath string
	facts                 *strictWallReads
	wantAnchors, anchors  int
}

func crashedFileMission(t *testing.T, ledgerCycles, spentCycles int) *fileHealMission {
	t.Helper()
	root := t.TempDir()
	bed := &fileHealMission{engine: NewEngine(root, "demo")}
	bed.facts = &strictWallReads{t: t}
	bed.engine.wallReadFacts = bed.facts
	dir := bed.engine.missionDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	contractPath := bed.engine.approvedContractPath()
	if err := os.WriteFile(contractPath, []byte(healContract), 0o644); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte(healContract))
	writeJSONFile(t, bed.engine.fencesPath(), map[string]any{
		"schemaVersion": 1, "missionId": "demo", "startedAt": "2026-08-11T00:00:00Z",
		"cycles": spentCycles, "reservations": map[string]any{},
		"approvedContractSha256": hex.EncodeToString(sum[:]),
	})
	bed.statePath = filepath.Join(dir, "state.json")
	bed.ledgerPath = filepath.Join(dir, "ledger.md")
	if err := mission.InitLedger(bed.ledgerPath, 10, 5); err != nil {
		t.Fatal(err)
	}
	origins := map[string]any{
		"headCommit": "cccccccccccccccccccccccccccccccccccccccc",
		"topTree":    nil, "topStaged": nil, "refMap": map[string]any{},
		"worktreeCensus": []any{}, "capturedAt": "2026-01-01T00:00:00Z",
	}
	if err := mission.InitStateWithBaseline(bed.statePath, contractPath, bed.ledgerPath, "", "main", "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", origins); err != nil {
		t.Fatal(err)
	}
	for cycle := 1; cycle <= ledgerCycles; cycle++ {
		if _, err := mission.AppendCycle(bed.ledgerPath, cycle, "unresolved", testSHA, "score=5", "no"); err != nil {
			t.Fatal(err)
		}
	}
	if ledgerCycles > 0 {
		proposed := deepCopyDoc(readTestDoc(t, bed.statePath))
		proposed["ledger"].(map[string]any)["cycles"] = ledgerCycles
		proposed["fences"].(map[string]any)["cycles"] = ledgerCycles
		if _, err := bed.engine.writeState(bed.statePath, proposed); err != nil {
			t.Fatal(err)
		}
	}
	bed.engine.anchorFn = func(statePath, ledgerPath, identity string) error {
		bed.anchors++
		if statePath != bed.statePath || ledgerPath != bed.ledgerPath || identity != "demo" {
			t.Fatalf("unexpected heal anchor: state=%q ledger=%q identity=%q", statePath, ledgerPath, identity)
		}
		if _, _, err := mission.VerifyStateShape(statePath); err != nil {
			t.Fatalf("heal anchor state shape: %v", err)
		}
		state := readTestDoc(t, statePath)
		if got, ok := jsonInt(state["ledger"].(map[string]any)["cycles"]); !ok || got != int64(spentCycles) {
			t.Fatalf("heal anchor ledger cycle = %v, want %d", state["ledger"], spentCycles)
		}
		if got, ok := jsonInt(state["fences"].(map[string]any)["cycles"]); !ok || got != int64(spentCycles) {
			t.Fatalf("heal anchor fence cycle = %v, want %d", state["fences"], spentCycles)
		}
		return nil
	}
	t.Cleanup(func() {
		bed.facts.done()
		if bed.anchors != bed.wantAnchors {
			t.Errorf("heal anchors = %d, want %d", bed.anchors, bed.wantAnchors)
		}
	})
	return bed
}

func (b *fileHealMission) expectHEAD() {
	b.facts.git = append(b.facts.git, wallGitReply{
		root: b.engine.Root, args: []string{"-C", b.engine.Root, "rev-parse", "HEAD"},
		stdout: fileHealHEAD + "\n",
	})
	b.wantAnchors++
}
