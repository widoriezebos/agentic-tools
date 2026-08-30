package missionrunner

// The recovery ladder's mechanical rung (D117, slice A): the ONE case
// the runner restores by itself is undeclared workspace content
// diverging from the composed expected tree, judged over a stable
// capture with every carrier clean and no prior recovery in the
// mission. Everything else — and every doubt — stays a parked taint on
// the human path. These fixtures pin both directions and the record the
// recovery leaves behind.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
)

// recoveryBed is the wall bed one scribble away from the dominant case:
// a provisioned mission with an open fixture turn and an anchored
// ledger.
func recoveryBed(t *testing.T) (*Engine, string, string, string) {
	t.Helper()
	engine := copyFullCycleRoot(t, "FAKEHOST:close-stream")
	statePath, err := seedCrashedMissionState(t, engine)
	if err != nil {
		t.Fatal(err)
	}
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
	return engine, statePath, ledgerPath, turnDir
}

// The dominant case end to end: a host scribble on undeclared paths is
// put back mechanically, the whole posture re-verifies, the turn
// passes, and the recovery record rides the evidence.
func TestWallMechanicalRecoveryRestoresUndeclaredScribble(t *testing.T) {
	// Recovery windows livelock under the compressed package scale
	// (window expires before its real fact on every retry); real scale
	// until audited — tracked under timing-tests-synthetic-clock.

	engine, statePath, ledgerPath, turnDir := recoveryBed(t)
	scribbled := filepath.Join(engine.Root, "scripts", "assert-turn-prompt.sh")
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
	if !strings.Contains(joined, "scripts/assert-turn-prompt.sh") || !strings.Contains(joined, "host-scribble.txt") {
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

// The S2-R3-02 correction: the restore target is the COMPOSED expected
// tree. A file taken from A to B by a consumed authorization and then
// scribbled to C by the host comes back as B — reviewed work is never
// discarded by recovery.
func TestWallMechanicalRecoveryRestoresTheComposedTree(t *testing.T) {
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

// A violation in the ledger domain never reaches the rung: the
// mechanical case is workspace content only.
func TestWallRecoveryLeavesLedgerDomainToTheHuman(t *testing.T) {
	// Recovery windows livelock under the compressed package scale
	// (window expires before its real fact on every retry); real scale
	// until audited — tracked under timing-tests-synthetic-clock.

	engine, statePath, ledgerPath, turnDir := recoveryBed(t)
	tampered, err := os.ReadFile(ledgerPath)
	if err != nil {
		t.Fatal(err)
	}
	writeText(t, ledgerPath, string(tampered)+"- forged line\n")

	_, final, violated, err := engine.wallGate(statePath, ledgerPath, "alpha-t1-live", turnDir, 1, nil, false, true, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !violated {
		t.Fatalf("a ledger tamper must park even with recovery allowed: %v", final)
	}
	state := readTestDoc(t, statePath)
	if reason := unresolvedTaint(state); !strings.Contains(reason, "ledger") {
		t.Fatalf("the taint must stand and name the ledger: %q", reason)
	}
	// The ask arrives with the ladder's context: the human reads what
	// the rung ruled out without reconstructing it from events.
	asks, _ := filepath.Glob(filepath.Join(asksDirPath(engine.Root, engine.Mission), "wall-violation*.json"))
	if len(asks) != 1 {
		t.Fatalf("the park must raise one wall-violation ask: %v", asks)
	}
	ask := readTestDoc(t, asks[0])
	if note, _ := ask["recoveryNote"].(string); !strings.Contains(note, "declaration or ledger domain") {
		t.Fatalf("the ask must carry the rung's refusal: %v", ask["recoveryNote"])
	}
	// The evidence carries the same story: the turn dir answers the
	// whole question without the event stream.
	wallDoc := readTestDoc(t, filepath.Join(turnDir, "wall.json"))
	if note, _ := wallDoc["recovery"].(string); !strings.Contains(note, "declaration or ledger domain") {
		t.Fatalf("the evidence must carry the rung's refusal: %v", wallDoc["recovery"])
	}
}

// Detected evidence stays sticky ABOVE the rung: a wall.json already
// recording a violation is a crash tail, and a crash is a doubt that
// belongs to the human — the recorded violation parks verbatim.
func TestWallRecoveryStickyViolationOutranksTheRung(t *testing.T) {
	// Recovery windows livelock under the compressed package scale
	// (window expires before its real fact on every retry); real scale
	// until audited — tracked under timing-tests-synthetic-clock.

	engine, statePath, ledgerPath, turnDir := recoveryBed(t)
	recorded := "undeclared host-authored change: ghost.txt (recorded before a crash)"
	writeJSONFile(t, filepath.Join(turnDir, "wall.json"),
		map[string]any{"verdict": "violated", "violation": recorded})

	_, final, violated, err := engine.wallGate(statePath, ledgerPath, "alpha-t1-live", turnDir, 1, nil, false, true, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !violated {
		t.Fatalf("the sticky violation must park: %v", final)
	}
	state := readTestDoc(t, statePath)
	if reason := unresolvedTaint(state); reason != recorded {
		t.Fatalf("the park must carry the recorded violation verbatim: %q", reason)
	}
}

// A published recovery whose anchored record never landed is a crash
// tail: rewritable evidence is never promoted into the chain, so the
// gate parks the recorded offense verbatim for the human — the rung
// never re-earns a pass whose record already vanished once.
func TestWallRecoveryCrashTailParksThePublishedPass(t *testing.T) {
	// Recovery windows livelock under the compressed package scale
	// (window expires before its real fact on every retry); real scale
	// until audited — tracked under timing-tests-synthetic-clock.

	engine, statePath, ledgerPath, turnDir := recoveryBed(t)
	block := map[string]any{
		"violation":     "undeclared host-authored change: x.txt",
		"restoredPaths": []any{"x.txt"},
		"restoredAt":    "2026-08-23T00:00:00Z",
	}
	writeJSONFile(t, filepath.Join(turnDir, "wall.json"),
		map[string]any{"verdict": "passed", "recovered": block})

	_, final, violated, err := engine.wallGate(statePath, ledgerPath, "alpha-t1-live", turnDir, 1, nil, true, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !violated {
		t.Fatalf("the crash tail must park for the human: %v", final)
	}
	state := readTestDoc(t, statePath)
	if reason := unresolvedTaint(state); reason != "undeclared host-authored change: x.txt" {
		t.Fatalf("the park must carry the recovered offense verbatim: %q", reason)
	}
	asks, _ := filepath.Glob(filepath.Join(asksDirPath(engine.Root, engine.Mission), "wall-violation*.json"))
	if len(asks) != 1 {
		t.Fatalf("the crash tail must raise one ask: %v", asks)
	}
	ask := readTestDoc(t, asks[0])
	if note, _ := ask["recoveryNote"].(string); !strings.Contains(note, "before the record landed") {
		t.Fatalf("the ask must explain the crash tail: %v", ask["recoveryNote"])
	}
}

// The stability rerun hands its live record down: an in-pass re-run
// over the restored workspace passes WITH the record — while the same
// evidence with no live record and a landed acceptance also proceeds,
// because the chain already holds the offense.
func TestWallRecoveryInPassRecordRidesTheRerun(t *testing.T) {
	// Recovery windows livelock under the compressed package scale
	// (window expires before its real fact on every retry); real scale
	// until audited — tracked under timing-tests-synthetic-clock.

	engine, statePath, ledgerPath, turnDir := recoveryBed(t)
	block := map[string]any{
		"violation":     "undeclared host-authored change: x.txt",
		"restoredPaths": []any{"x.txt"},
		"restoredAt":    "2026-08-23T00:00:00Z",
	}
	writeJSONFile(t, filepath.Join(turnDir, "wall.json"),
		map[string]any{"verdict": "passed", "recovered": block})

	ctx, final, violated, err := engine.wallGate(statePath, ledgerPath, "alpha-t1-live", turnDir, 1, nil, true, false, block)
	if err != nil {
		t.Fatal(err)
	}
	if violated || ctx == nil {
		t.Fatalf("the in-pass rerun must keep the pass: %v", final)
	}
	if v, _ := ctx.Recovered["violation"].(string); v != "undeclared host-authored change: x.txt" {
		t.Fatalf("the live record must ride the rerun: %v", ctx.Recovered)
	}
	wallDoc := readTestDoc(t, filepath.Join(turnDir, "wall.json"))
	rec, _ := wallDoc["recovered"].(map[string]any)
	if wallDoc["verdict"] != "passed" || rec == nil {
		t.Fatalf("the rewritten evidence must keep the record: %v", wallDoc)
	}

	// The record rides into the acceptance payload, and a payload whose
	// record disagrees with the gate's verdict is tampered evidence.
	state := readTestDoc(t, statePath)
	payload, consumed, err := wallEntryPayload(engine.Root, engine.Mission, "alpha-t1-live", state)
	if err != nil {
		t.Fatal(err)
	}
	if payload["recovered"] == nil {
		t.Fatal("the acceptance payload must carry the recovery record")
	}
	entry := map[string]any{
		"turnId": "alpha-t1-live", "wall": payload,
		"consumedAuthorizations": consumed, "gatePassed": false,
	}
	proposed := map[string]any{"missionId": engine.Mission, "turnLog": []any{entry}}
	if mismatch := acceptancePayloadMismatch(proposed, "alpha-t1-live", ctx, false, nil); mismatch != "" {
		t.Fatalf("a faithful payload must verify: %s", mismatch)
	}
	delete(payload, "recovered")
	if mismatch := acceptancePayloadMismatch(proposed, "alpha-t1-live", ctx, false, nil); !strings.Contains(mismatch, "recovery record") {
		t.Fatalf("a payload dropping the record must be refused: %q", mismatch)
	}

	// The landed-acceptance shape: the same evidence, no live record,
	// but the chain already carries the turn's acceptance — the gate
	// proceeds without a park and without resurrecting the record.
	landed := readTestDoc(t, statePath)
	fullPayload, fullConsumed, err := wallEntryPayload(engine.Root, engine.Mission, "alpha-t1-live", landed)
	if err != nil {
		t.Fatal(err)
	}
	log, _ := landed["turnLog"].([]any)
	landed["turnLog"] = append(log, map[string]any{
		"turnId": "alpha-t1-live", "wall": fullPayload,
		"consumedAuthorizations": fullConsumed,
	})
	writeJSONFile(t, statePath, landed)
	after, _, violatedAfter, err := engine.wallGate(statePath, ledgerPath, "alpha-t1-live", turnDir, 1, nil, true, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	if violatedAfter || after == nil {
		t.Fatal("a landed acceptance must not re-park the published pass")
	}
	if after.Recovered != nil {
		t.Fatalf("evidence must never resurrect a record the chain already holds: %v", after.Recovered)
	}
}

// The rung runs once per mission: any acceptance already carrying a
// recovery record makes the next offense a repeat for the human.
func TestWallRecoveryRefusesRepeatOffense(t *testing.T) {
	// Recovery windows livelock under the compressed package scale
	// (window expires before its real fact on every retry); real scale
	// until audited — tracked under timing-tests-synthetic-clock.

	engine, statePath, ledgerPath, turnDir := recoveryBed(t)
	state := readTestDoc(t, statePath)
	tree := strings.Repeat("d", 40)
	priorLog, _ := state["turnLog"].([]any)
	state["turnLog"] = append(priorLog, map[string]any{
		"turnId": "alpha-t0", "consumedAuthorizations": []any{},
		"wall": map[string]any{
			"verdict": "passed", "preTree": tree, "expectedTree": tree,
			"postTree": tree, "orderedDigests": []any{},
			"sequencePoint":  map[string]any{"sequence": 1, "segment": 0},
			"headCommitPost": strings.Repeat("c", 40), "refMapPost": map[string]any{},
			"stagedTreePost": tree, "topTreePost": nil, "topStagedPost": nil,
			"worktreeCensusPost": []any{}, "capturedAt": "2026-08-23T00:00:00Z",
			"recovered": map[string]any{
				"violation":     "undeclared host-authored change: prior.txt",
				"restoredPaths": []any{"prior.txt"},
				"restoredAt":    "2026-08-23T00:00:00Z",
			},
		},
	})
	inspection := &wallInspection{
		PreTree: strings.Repeat("a", 40), ExpectedTree: strings.Repeat("a", 40),
		PostTree: strings.Repeat("b", 40), UndeclaredOnly: true,
		Violation: "undeclared host-authored change: y.txt", Unaccounted: []string{"y.txt"},
	}
	block, refusal, ok := engine.attemptWallRecovery(inspection, &wallCapture{}, true, "", map[string]bool{}, &scopeOrigin{}, state, "alpha-t1-live")
	if ok || block != nil {
		t.Fatal("a repeat offense must refuse the rung")
	}
	if !strings.Contains(refusal, "repeat offense") {
		t.Fatalf("the refusal must name the repeat for the ask's context: %q", refusal)
	}
	_ = statePath
	_ = ledgerPath
	_ = turnDir
	if missionHasRecoveredAcceptance(map[string]any{"turnLog": []any{}}) {
		t.Fatal("an empty chain has no recovery")
	}
	if !missionHasRecoveredAcceptance(state) {
		t.Fatal("the seeded chain carries one")
	}
}

// The window between a successful restore and its re-verification: a
// late mutation landing there must become a fresh violation — the park
// carries the failed-re-verification note on ask AND evidence, and no
// recovery record ever reaches a pass.
func TestWallRecoveryLateMutationFailsTheReverification(t *testing.T) {
	// Recovery windows livelock under the compressed package scale
	// (window expires before its real fact on every retry); real scale
	// until audited — tracked under timing-tests-synthetic-clock.

	engine, statePath, ledgerPath, turnDir := recoveryBed(t)
	writeText(t, filepath.Join(engine.Root, "host-scribble.txt"), "junk\n")
	engine.postRestoreHook = func() {
		writeText(t, filepath.Join(engine.Root, "late-mutation.txt"), "raced in\n")
	}

	_, final, violated, err := engine.wallGate(statePath, ledgerPath, "alpha-t1-live", turnDir, 1, nil, false, true, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !violated {
		t.Fatalf("the late mutation must park: %v", final)
	}
	state := readTestDoc(t, statePath)
	if reason := unresolvedTaint(state); !strings.Contains(reason, "late-mutation.txt") {
		t.Fatalf("the park must carry the fresh violation: %q", reason)
	}
	asks, _ := filepath.Glob(filepath.Join(asksDirPath(engine.Root, engine.Mission), "wall-violation*.json"))
	if len(asks) != 1 {
		t.Fatalf("the park must raise one ask: %v", asks)
	}
	ask := readTestDoc(t, asks[0])
	if note, _ := ask["recoveryNote"].(string); !strings.Contains(note, "re-verification still refused") {
		t.Fatalf("the ask must carry the failed-re-verification note: %v", ask["recoveryNote"])
	}
	wallDoc := readTestDoc(t, filepath.Join(turnDir, "wall.json"))
	if note, _ := wallDoc["recovery"].(string); !strings.Contains(note, "re-verification still refused") {
		t.Fatalf("the evidence must carry the same note: %v", wallDoc["recovery"])
	}
	if wallDoc["verdict"] != "violated" || wallDoc["recovered"] != nil {
		t.Fatalf("no recovery record may survive a failed re-verification: %v", wallDoc)
	}
}
