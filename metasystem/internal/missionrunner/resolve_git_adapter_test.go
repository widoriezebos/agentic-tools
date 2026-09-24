package missionrunner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/mission"
)

// The default pinned effect publishes the verified position and leaves its
// anchor ref untouched when either file moves before publication.
func TestGitAdapterResolvePinnedAnchorRejectsMovedStateOrLedger(t *testing.T) {
	t.Parallel()
	for _, moved := range []string{"state", "ledger"} {
		t.Run(moved, func(t *testing.T) {
			root := wallRepo(t)
			e := &Engine{Root: root, Mission: "alpha"}
			state := filepath.Join(e.missionDir(), "state.json")
			ledger := filepath.Join(e.missionDir(), "ledger.md")
			contract := e.approvedContractPath()
			writeText(t, contract, "```mission\ncandidate.branch=main\nstream.solo=Do solo\n```\n```mission-seal\ncandidate.branch=main\n```\n")
			if err := mission.InitLedger(ledger, 5, 3); err != nil {
				t.Fatal(err)
			}
			origins := map[string]any{
				"headCommit": recoveryHead, "topTree": nil, "topStaged": nil,
				"refMap":         map[string]any{"refs/heads/main": recoveryHead},
				"worktreeCensus": []any{}, "capturedAt": "2026-01-01T00:00:00Z",
			}
			if err := mission.InitStateWithBaseline(state, contract, ledger, "", "main", recoveryPre, origins); err != nil {
				t.Fatal(err)
			}
			_, statePin, err := mission.VerifyStateShape(state)
			if err != nil {
				t.Fatal(err)
			}
			originalLedger, err := os.ReadFile(ledger)
			if err != nil {
				t.Fatal(err)
			}
			ledgerPin := sha256Hex(string(originalLedger))
			if err := e.anchorStatePinned(state, ledger, "Wido", statePin, ledgerPin); err != nil {
				t.Fatalf("matching pins must anchor: %v", err)
			}
			ref := mission.MissionRefNamespace(e.Mission) + "state-anchors"
			tip := func() string {
				t.Helper()
				stdout, stderr, code := gitCaptured(root, "rev-parse", "--verify", ref)
				if code != 0 {
					t.Fatalf("read anchor tip: %s", stderr)
				}
				return strings.TrimSpace(stdout)
			}
			anchoredTip := tip()
			if anchoredTip == "" {
				t.Fatal("matching pins did not publish an anchor tip")
			}
			if moved == "state" {
				proposed := deepCopyDoc(readTestDoc(t, state))
				proposed["fences"].(map[string]any)["cycles"] = 1
				source := filepath.Join(t.TempDir(), "moved-state.json")
				writeJSONFile(t, source, proposed)
				if err := mission.WriteState(state, source, statePin); err != nil {
					t.Fatal(err)
				}
			} else {
				writeText(t, ledger, string(originalLedger)+"\n")
			}
			err = e.anchorStatePinned(state, ledger, "Wido", statePin, ledgerPin)
			if err == nil || !strings.Contains(err.Error(), moved+" moved past the verified") {
				t.Fatalf("moved %s must refuse its original pin: %v", moved, err)
			}
			if after := tip(); after != anchoredTip {
				t.Fatalf("refused anchor moved the tip: %s -> %s", anchoredTip, after)
			}
		})
	}
}
