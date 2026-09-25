package missionrunner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
)

func TestGitAdapterWallRecoveryRestoresUndeclaredScribble(t *testing.T) {
	t.Parallel()
	// Recovery windows livelock under the compressed package scale
	// (window expires before its real fact on every retry); real scale
	// until audited — tracked under timing-tests-synthetic-clock.

	engine, statePath, ledgerPath, turnDir := recoveryBed(t)
	scribbled := filepath.Join(engine.Root, "scripts", "assert-return-complete.sh")
	original, err := os.ReadFile(scribbled)
	if err != nil {
		t.Fatal(err)
	}
	writeText(t, scribbled, string(original)+"# host scribble\n")
	writeText(t, filepath.Join(engine.Root, "host-scribble.txt"), "junk\n")

	ctx, final, violated, err := engine.wallGate(statePath, ledgerPath, "alpha-t1-live", turnDir, 1, nil, false, true, nil)
	if err != nil {
		t.Fatal(err)
	}
	if violated || ctx == nil {
		t.Fatalf("the dominant case must recover into a pass: violated=%v final=%v", violated, final)
	}
	if ctx.Recovered == nil {
		t.Fatal("the pass must carry the recovery record")
	}
	if v, _ := ctx.Recovered["violation"].(string); !strings.Contains(v, "undeclared host-authored change") {
		t.Fatalf("the record must name the offense: %v", ctx.Recovered)
	}
	restored, _ := ctx.Recovered["restoredPaths"].([]any)
	joined := ""
	for _, p := range restored {
		path, _ := p.(string)
		joined += path + "\n"
	}
	if !strings.Contains(joined, "scripts/assert-return-complete.sh") || !strings.Contains(joined, "host-scribble.txt") {
		t.Fatalf("both scribbles must be in the restore set: %q", joined)
	}
	after, err := os.ReadFile(scribbled)
	if err != nil || string(after) != string(original) {
		t.Fatalf("the tracked scribble must be restored: err=%v", err)
	}
	if _, err := os.Lstat(filepath.Join(engine.Root, "host-scribble.txt")); !os.IsNotExist(err) {
		t.Fatal("the stray file must be removed")
	}
	wallDoc := readTestDoc(t, filepath.Join(turnDir, "wall.json"))
	if wallDoc["verdict"] != "passed" || wallDoc["recovered"] == nil || wallDoc["violation"] != nil {
		t.Fatalf("the evidence must record a recovered pass: %v", wallDoc)
	}
	state := readTestDoc(t, statePath)
	entries, _ := state["workspaceTaint"].(map[string]any)["entries"].([]any)
	if len(entries) != 0 || state["parkReason"] == "wall-violation" {
		t.Fatalf("a recovered pass books no taint and no park: %v", state["workspaceTaint"])
	}
}

func TestGitAdapterWallRecoveryRestoresComposedTree(t *testing.T) {
	t.Parallel()
	// Under the compressed package scale this scenario livelocks: a
	// recovery window expires before its real fact on every retry.
	// Real scale until the recovery windows are audited — tracked
	// under timing-tests-synthetic-clock.
	engine := copyFullCycleRoot(t, "FAKEHOST:close-stream")
	statePath, err := seedCrashedMissionState(t, engine)
	if err != nil {
		t.Fatal(err)
	}
	product := filepath.Join(engine.Root, "product.txt")
	writeText(t, product, "A\n")
	openFixtureTurn(t, engine.Root, statePath, "alpha-t1-live", 1)
	ledgerPath := filepath.Join(engine.missionDir(), "ledger.md")
	if err := engine.anchor(statePath, ledgerPath, "open"); err != nil {
		t.Fatal(err)
	}
	turnDir := filepath.Join(engine.missionDir(), "turns", "alpha-t1-live")
	os.MkdirAll(turnDir, 0o755)
	writeJSONFile(t, filepath.Join(turnDir, "turn.json"),
		map[string]any{"missionId": engine.Mission, "turnId": "alpha-t1-live", "cycle": 1,
			"runtime": "fake", "model": "fixture", "status": "running"})

	workspace := gittree.Workspace{Dir: engine.Root}
	pre, err := wallSnapshot(workspace, engine.Mission)
	if err != nil {
		t.Fatal(err)
	}
	writeText(t, product, "B\n")
	reviewed, err := wallSnapshot(workspace, engine.Mission)
	if err != nil {
		t.Fatal(err)
	}
	digest := wallAuthorization(t, engine.Root, engine.Mission, pre, reviewed, nil)
	writeText(t, product, "C\n")
	certified := []map[string]any{{"jobId": "job-w", "verdict": "accepted", "authorizationDigest": digest}}

	ctx, final, violated, err := engine.wallGate(statePath, ledgerPath, "alpha-t1-live", turnDir, 1, certified, false, true, nil)
	if err != nil {
		t.Fatal(err)
	}
	if violated || ctx == nil {
		t.Fatalf("the scribbled authorized path must recover: %v", final)
	}
	got, err := os.ReadFile(product)
	if err != nil || string(got) != "B\n" {
		t.Fatalf("the restore must return the REVIEWED bytes, never the pre-tree: %q err=%v", got, err)
	}
	if len(ctx.OrderedDigests) != 1 {
		t.Fatalf("the authorization must still be consumed: %v", ctx.OrderedDigests)
	}
}
