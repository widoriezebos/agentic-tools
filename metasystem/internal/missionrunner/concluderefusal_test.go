package missionrunner

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The concludeCycle recovery branch is
// EXECUTED — a proposal refusal parks host-failure with an open ask, the
// state's ledger cycle count matches the block the cycle already appended,
// and the post-park anchor runs. Removing the branch, or only its
// setLedgerCycles call, must turn this red.
func TestConcludeCycleParksOnProposalRefusal(t *testing.T) {
	bed := newRecoveryFileBedWithContract(t, nil, strings.Replace(healContract, "stream.primary=Do the work", "stream.solo=Do solo", 1))
	engine, statePath, ledgerPath := bed.e, bed.state, bed.ledger
	turnID := engine.Mission + "-t1-live"
	bed.facts.permitAppendPrefix = true
	anchored := 0
	engine.anchorFn = func(state, ledger, identity string) error {
		bed.facts.next("StateAnchor", state, ledger, identity)
		anchored++
		if state != statePath || ledger != ledgerPath || identity != turnID {
			t.Fatalf("post-park anchor arguments: %q %q %q", state, ledger, identity)
		}
		return nil
	}
	runnersDir := filepath.Join(engine.Root, "artifacts", "agents", "missions", "runners")
	os.MkdirAll(runnersDir, 0o755)
	record, _ := json.Marshal(engine.runnerRecord(os.Getpid(), os.Getpid(), 1, "fixture"))
	os.WriteFile(filepath.Join(runnersDir, engine.Mission+".json"), append(record, byte(10)), 0o644)

	state, err := engine.verifyState(statePath, false)
	if err != nil {
		t.Fatal(err)
	}
	turnDir := bed.turnDir
	bed.facts.ledgerGuard()
	bed.facts.pass(recoveryPre)
	bed.facts.add("Git", scopePolicyGit{stdout: recoveryHead + "\n"}, engine.Root,
		[]string{"-C", engine.Root, "rev-parse", "main"})
	bed.facts.capture(recoveryPre, recoveryPre)
	bed.facts.namespace()
	bed.facts.add("StateAnchor", nil, statePath, ledgerPath, turnID)

	final, err := engine.concludeCycle(statePath, ledgerPath, state, concludeSpec{
		turnID: turnID, cycle: 1, turnDir: turnDir,
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
	if reread["missionId"] != engine.Mission {
		t.Fatalf("refused proposal leaked: %v", reread["missionId"])
	}
}
