package missionrunner

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
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

// crashedMission builds a running mission whose runner died inside a turn:
// the fence counters have spent spentCycles while the ledger holds only
// ledgerCycles appended lines (equal counts model a clean resume). The
// workspace is a real git repository so the heal can resolve HEAD; the
// anchor is stubbed as in the other engine tests, because anchoring shells
// out to the metasystem binary a unit test does not have.
func crashedMission(t *testing.T, ledgerCycles, spentCycles int) (engine *Engine, statePath, ledgerPath, head string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	root := t.TempDir()
	git := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.com",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.com")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
		return string(out)
	}
	git("init", "-q")
	// The deployment's projection boundary (HIW-O3): runtime state under
	// artifacts/ stays outside the wall's shippable snapshot.
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("artifacts/\nbin/\nmetasystem.conf\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "README"), []byte("seed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", ".gitignore", "README")
	git("commit", "-q", "-m", "seed")
	git("checkout", "-q", "-B", "main")
	head = strings.TrimSpace(git("rev-parse", "HEAD"))

	engine = NewEngine(root, "demo")
	engine.anchorFn = func(string, string, string) error { return nil }
	dir := engine.missionDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	contractPath := engine.approvedContractPath()
	if err := os.WriteFile(contractPath, []byte(healContract), 0o644); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte(healContract))
	writeJSONFile(t, engine.fencesPath(), map[string]any{
		"schemaVersion": 1, "missionId": "demo", "startedAt": "2026-08-11T00:00:00Z",
		"cycles": spentCycles, "reservations": map[string]any{},
		"approvedContractSha256": hex.EncodeToString(sum[:]),
	})

	statePath = filepath.Join(dir, "state.json")
	ledgerPath = filepath.Join(dir, "ledger.md")
	if err := mission.InitLedger(ledgerPath, 10, 5); err != nil {
		t.Fatal(err)
	}
	if err := mission.InitStateWithBaseline(statePath, contractPath, ledgerPath, "", "main", strings.Repeat("b", 40)); err != nil {
		t.Fatal(err)
	}
	for cycle := 1; cycle <= ledgerCycles; cycle++ {
		if err := mission.AppendCycle(ledgerPath, cycle, "unresolved", testSHA, "score=5", "no"); err != nil {
			t.Fatal(err)
		}
	}
	if ledgerCycles > 0 {
		// Bring the state to its last concluded cycle, as a real crash
		// leaves it: the reserved cycle exists only in the fence counters.
		proposed := deepCopyDoc(readTestDoc(t, statePath))
		proposed["ledger"].(map[string]any)["cycles"] = ledgerCycles
		proposed["fences"].(map[string]any)["cycles"] = ledgerCycles
		if _, err := engine.writeState(statePath, proposed); err != nil {
			t.Fatal(err)
		}
	}
	return engine, statePath, ledgerPath, head
}

func TestHealReservedCycleRecordsLostTurn(t *testing.T) {
	engine, statePath, ledgerPath, head := crashedMission(t, 2, 3)
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
	if err := mission.AppendCycle(ledgerPath, 4, "unresolved", testSHA, "score=5", "no"); err != nil {
		t.Fatalf("the next cycle must append contiguously after the heal: %v", err)
	}
}

func TestHealReservedCycleIsIdempotent(t *testing.T) {
	engine, statePath, ledgerPath, _ := crashedMission(t, 2, 3)
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
	engine, statePath, ledgerPath, _ := crashedMission(t, 2, 2)
	before, _ := os.ReadFile(ledgerPath)
	stateBefore := readTestDoc(t, statePath)
	healed, err := engine.healReservedCycle(statePath, ledgerPath, stateBefore)
	if err != nil || healed {
		t.Fatalf("a clean resume must heal nothing: healed=%v err=%v", healed, err)
	}
	after, _ := os.ReadFile(ledgerPath)
	if string(before) != string(after) {
		t.Fatal("a clean resume must not touch the ledger")
	}
	if cycles, _ := jsonInt(readTestDoc(t, statePath)["ledger"].(map[string]any)["cycles"]); cycles != 2 {
		t.Fatal("a clean resume must not advance the state")
	}
}

func TestHealReservedCycleConsumesDrainStall(t *testing.T) {
	engine, statePath, ledgerPath, head := crashedMission(t, 2, 3)
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
	if err := mission.AppendCycle(ledgerPath, 4, "unresolved", testSHA, "score=5", "no"); err != nil {
		t.Fatalf("the next cycle must append contiguously after the heal: %v", err)
	}
}

func TestHealReservedCycleIgnoresMismatchedDrainStall(t *testing.T) {
	engine, statePath, ledgerPath, _ := crashedMission(t, 2, 3)
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
	engine, statePath, ledgerPath, _ := crashedMission(t, 1, 3)
	before, _ := os.ReadFile(ledgerPath)
	healed, err := engine.healReservedCycle(statePath, ledgerPath, readTestDoc(t, statePath))
	if err != nil || healed {
		t.Fatalf("a wider gap must not be healed: healed=%v err=%v", healed, err)
	}
	after, _ := os.ReadFile(ledgerPath)
	if string(before) != string(after) {
		t.Fatal("a wider gap must leave the ledger untouched")
	}
}
