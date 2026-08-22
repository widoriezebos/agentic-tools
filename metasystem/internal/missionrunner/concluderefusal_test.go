package missionrunner

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// The concludeCycle recovery branch is
// EXECUTED — a proposal refusal parks host-failure with an open ask, the
// state's ledger cycle count matches the block the cycle already appended,
// and the post-park anchor runs. Removing the branch, or only its
// setLedgerCycles call, must turn this red.
func TestConcludeCycleParksOnProposalRefusal(t *testing.T) {
	engine, statePath, ledgerPath, _ := crashedMission(t, 0, 1)
	anchored := 0
	engine.anchorFn = func(string, string, string) error { anchored++; return nil }
	runnersDir := filepath.Join(engine.Root, "artifacts", "agents", "missions", "runners")
	os.MkdirAll(runnersDir, 0o755)
	record, _ := json.Marshal(engine.runnerRecord(os.Getpid(), os.Getpid(), 1, "fixture"))
	os.WriteFile(filepath.Join(runnersDir, "demo.json"), append(record, byte(10)), 0o644)

	openFixtureTurn(t, engine.Root, statePath, "demo-t1-aaaa", 1)
	state, err := engine.verifyState(statePath, false)
	if err != nil {
		t.Fatal(err)
	}
	turnDir := filepath.Join(engine.missionDir(), "turns", "demo-t1-aaaa")
	os.MkdirAll(turnDir, 0o755)

	final, err := engine.concludeCycle(statePath, ledgerPath, state, concludeSpec{
		turnID: "demo-t1-aaaa", cycle: 1, turnDir: turnDir,
		propose: func(measurementValue any, gatePassed bool) (map[string]any, error) {
			violating := deepCopyDoc(state)
			violating["missionId"] = "someone-else" // immutable identity
			return violating, nil
		},
	})
	if err != nil {
		t.Fatalf("the refusal took the fail ramp: %v", err)
	}
	if final["status"] != "parked" || final["parkReason"] != "host-failure" {
		t.Fatalf("not parked host-failure: status=%v reason=%v", final["status"], final["parkReason"])
	}
	ledgerRef, _ := final["ledger"].(map[string]any)
	if cycles, ok := jsonInt(ledgerRef["cycles"]); !ok || cycles != 1 {
		t.Fatalf("state ledger cycles = %v, want 1 (the appended block)", ledgerRef["cycles"])
	}
	if anchored != 1 {
		t.Fatalf("anchor ran %d times, want 1", anchored)
	}
	asks, _ := filepath.Glob(filepath.Join(engine.missionDir(), "asks", "*.json"))
	if len(asks) != 1 {
		t.Fatalf("want exactly one park ask, got %v", asks)
	}
	// The refused proposal never landed: the disk state validates and
	// still names this mission.
	reread, err := engine.verifyState(statePath, false)
	if err != nil {
		t.Fatalf("post-park state invalid: %v", err)
	}
	if reread["missionId"] != "demo" {
		t.Fatalf("refused proposal leaked: %v", reread["missionId"])
	}
}
