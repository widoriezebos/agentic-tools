package missionrunner

import (
	"os"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/mission"
)

// healContract carries the streams InitState reads plus the sealed gate the
// replay-semantics best marker folds under.
const healContract = "# Intent\n\n```mission\n" +
	"candidate.branch=main\n" +
	"stream.primary=Do the work\n" +
	"gate.direction=max\n" +
	"gate.threshold.score=>=10\n" +
	"gate.noise-floor.score=0.5\n" +
	"ledger.cycle-budget=10\n" +
	"ledger.no-gain-budget=5\n" +
	"```\n\n```mission-seal\n" +
	"sealed.baseline.score=5\n" +
	"```\n"

func TestHealReservedCycleRecordsLostTurn(t *testing.T) {
	bed := crashedFileMission(t, 2, 3)
	bed.expectHEAD()
	engine, statePath, ledgerPath, head := bed.engine, bed.statePath, bed.ledgerPath, fileHealHEAD
	healed, err := engine.healReservedCycle(statePath, ledgerPath, readTestDoc(t, statePath))
	if err != nil || !healed {
		t.Fatalf("crash window must heal: healed=%v err=%v", healed, err)
	}
	ledger, _ := os.ReadFile(ledgerPath)
	want := "### Cycle 3\n- Classification: no-progress; candidate-sha=" + head +
		"; observed=unmeasurable:turn-lost; best=no"
	if !strings.Contains(string(ledger), want) {
		t.Fatalf("the lost turn's line is missing or misshapen:\n%s", ledger)
	}
	state := readTestDoc(t, statePath)
	if cycles, _ := jsonInt(state["ledger"].(map[string]any)["cycles"]); cycles != 3 {
		t.Fatalf("state ledger cycles must adopt the healed count: %v", state["ledger"])
	}
	if cycles, _ := jsonInt(state["fences"].(map[string]any)["cycles"]); cycles != 3 {
		t.Fatalf("state fence projection must adopt the healed count: %v", state["fences"])
	}
	// The healed ledger stays fully parseable and the next cycle appends
	// contiguously — the wedge is gone.
	if _, _, cycles, err := mission.ParseLedger(ledgerPath); err != nil || len(cycles) != 3 {
		t.Fatalf("healed ledger must parse with 3 cycles: %v (%d)", err, len(cycles))
	}
	if _, err := mission.AppendCycle(ledgerPath, 4, "unresolved", testSHA, "score=5", "no"); err != nil {
		t.Fatalf("the next cycle must append contiguously after the heal: %v", err)
	}
}

func TestHealReservedCycleIsIdempotent(t *testing.T) {
	bed := crashedFileMission(t, 2, 3)
	bed.expectHEAD()
	engine, statePath, ledgerPath := bed.engine, bed.statePath, bed.ledgerPath
	if healed, err := engine.healReservedCycle(statePath, ledgerPath, readTestDoc(t, statePath)); err != nil || !healed {
		t.Fatalf("first heal: healed=%v err=%v", healed, err)
	}
	before, _ := os.ReadFile(ledgerPath)
	if healed, err := engine.healReservedCycle(statePath, ledgerPath, readTestDoc(t, statePath)); err != nil || healed {
		t.Fatalf("second heal must find nothing to do: healed=%v err=%v", healed, err)
	}
	after, _ := os.ReadFile(ledgerPath)
	if string(before) != string(after) {
		t.Fatalf("a second heal must not touch the ledger:\n%s", after)
	}
	if strings.Count(string(after), "### Cycle") != 3 {
		t.Fatalf("the healed cycle must exist exactly once:\n%s", after)
	}
}

func TestHealReservedCycleNoopWithoutGap(t *testing.T) {
	bed := crashedFileMission(t, 2, 2)
	engine, statePath, ledgerPath := bed.engine, bed.statePath, bed.ledgerPath
	before, _ := os.ReadFile(ledgerPath)
	stateBytesBefore, _ := os.ReadFile(statePath)
	stateBefore := readTestDoc(t, statePath)
	healed, err := engine.healReservedCycle(statePath, ledgerPath, stateBefore)
	if err != nil || healed {
		t.Fatalf("a clean resume must heal nothing: healed=%v err=%v", healed, err)
	}
	after, _ := os.ReadFile(ledgerPath)
	if string(before) != string(after) {
		t.Fatal("a clean resume must not touch the ledger")
	}
	stateBytesAfter, _ := os.ReadFile(statePath)
	if string(stateBytesBefore) != string(stateBytesAfter) {
		t.Fatal("a clean resume must not touch the state")
	}
	if cycles, _ := jsonInt(readTestDoc(t, statePath)["ledger"].(map[string]any)["cycles"]); cycles != 2 {
		t.Fatal("a clean resume must not advance the state")
	}
}

func TestHealReservedCycleConsumesDrainStall(t *testing.T) {
	bed := crashedFileMission(t, 2, 3)
	bed.expectHEAD()
	engine, statePath, ledgerPath, head := bed.engine, bed.statePath, bed.ledgerPath, fileHealHEAD
	// The drain-stalled unpark left the durable label naming exactly this
	// reserved cycle.
	proposed := deepCopyDoc(readTestDoc(t, statePath))
	proposed["lastDrainStall"] = map[string]any{"cycle": 3, "survivors": []any{"job-a", "job-b"}}
	if _, err := engine.writeState(statePath, proposed); err != nil {
		t.Fatal(err)
	}
	healed, err := engine.healReservedCycle(statePath, ledgerPath, readTestDoc(t, statePath))
	if err != nil || !healed {
		t.Fatalf("the drain-stalled gap must heal: healed=%v err=%v", healed, err)
	}
	ledger, _ := os.ReadFile(ledgerPath)
	want := "### Cycle 3\n- Classification: no-progress; candidate-sha=" + head +
		"; observed=unmeasurable:drain-stalled; best=no\n- Drain: stalled:2\n"
	if !strings.Contains(string(ledger), want) {
		t.Fatalf("the healed drain-stalled line and its annotation are missing or misshapen:\n%s", ledger)
	}
	state := readTestDoc(t, statePath)
	if _, present := state["lastDrainStall"]; present {
		t.Fatalf("the label must be consumed in the same conclude write: %v", state["lastDrainStall"])
	}
	if cycles, _ := jsonInt(state["ledger"].(map[string]any)["cycles"]); cycles != 3 {
		t.Fatalf("state ledger cycles must adopt the healed count: %v", state["ledger"])
	}
	// The healed ledger stays fully parseable and contiguous.
	if _, _, cycles, err := mission.ParseLedger(ledgerPath); err != nil || len(cycles) != 3 {
		t.Fatalf("healed ledger must parse with 3 cycles: %v (%d)", err, len(cycles))
	}
	if _, err := mission.AppendCycle(ledgerPath, 4, "unresolved", testSHA, "score=5", "no"); err != nil {
		t.Fatalf("the next cycle must append contiguously after the heal: %v", err)
	}
}

func TestHealReservedCycleIgnoresMismatchedDrainStall(t *testing.T) {
	bed := crashedFileMission(t, 2, 3)
	bed.expectHEAD()
	engine, statePath, ledgerPath := bed.engine, bed.statePath, bed.ledgerPath
	// A label from some older stall that does not name this gap's cycle:
	// the gap heals as a plain lost turn, exactly as shipped, and the label
	// is not consumed.
	proposed := deepCopyDoc(readTestDoc(t, statePath))
	proposed["lastDrainStall"] = map[string]any{"cycle": 2, "survivors": []any{"job-a"}}
	if _, err := engine.writeState(statePath, proposed); err != nil {
		t.Fatal(err)
	}
	healed, err := engine.healReservedCycle(statePath, ledgerPath, readTestDoc(t, statePath))
	if err != nil || !healed {
		t.Fatalf("the gap must still heal: healed=%v err=%v", healed, err)
	}
	ledger, _ := os.ReadFile(ledgerPath)
	if !strings.Contains(string(ledger), "observed=unmeasurable:turn-lost") ||
		strings.Contains(string(ledger), "drain-stalled") {
		t.Fatalf("a mismatched label heals as plain turn-lost:\n%s", ledger)
	}
	if _, present := readTestDoc(t, statePath)["lastDrainStall"]; !present {
		t.Fatal("a label that was not consumed must survive")
	}
}

func TestHealReservedCycleLeavesWiderGapsAlone(t *testing.T) {
	// A gap of more than one cycle is not this crash's signature: the heal
	// covers exactly the one reserve a runner life can leave unappended, and
	// anything wider stays a human's call rather than fabricated history.
	bed := crashedFileMission(t, 1, 3)
	engine, statePath, ledgerPath := bed.engine, bed.statePath, bed.ledgerPath
	before, _ := os.ReadFile(ledgerPath)
	stateBefore, _ := os.ReadFile(statePath)
	healed, err := engine.healReservedCycle(statePath, ledgerPath, readTestDoc(t, statePath))
	if err != nil || healed {
		t.Fatalf("a wider gap must not be healed: healed=%v err=%v", healed, err)
	}
	after, _ := os.ReadFile(ledgerPath)
	if string(before) != string(after) {
		t.Fatal("a wider gap must leave the ledger untouched")
	}
	stateAfter, _ := os.ReadFile(statePath)
	if string(stateBefore) != string(stateAfter) {
		t.Fatal("a wider gap must leave the state untouched")
	}
}
