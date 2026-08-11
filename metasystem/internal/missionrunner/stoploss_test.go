package missionrunner

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/mission"
)

// twoMetricGate declares zeta before alpha so declaration order and
// alphabetical order diverge in later assertions.
func twoMetricGate() *stopLossGate {
	return &stopLossGate{
		direction:  "max",
		metrics:    []string{"zeta", "alpha"},
		thresholds: map[string]string{"zeta": ">=10", "alpha": ">=5"},
		noise:      map[string]float64{"zeta": 1, "alpha": 0.5},
		baseline:   map[string]float64{"zeta": 4, "alpha": 2},
	}
}

func cycleEvent(class, observed, best string) mission.LedgerEvent {
	return mission.LedgerEvent{Classification: class, Observed: observed, Best: best}
}

func TestNewBestQualificationMatrix(t *testing.T) {
	gate := twoMetricGate()
	best := gate.tuple(gate.baseline) // [0, 4, 2]
	cases := []struct {
		name      string
		candidate []float64
		want      bool
	}{
		{"equal tuple is no best", []float64{0, 4, 2}, false},
		{"thresholds-met count dominates despite a worse metric", []float64{1, 3, 2}, true},
		{"integer component needs no noise gate", []float64{1, 4, 2}, true},
		{"metric gain within its noise floor fails", []float64{0, 5, 2}, false},
		{"metric gain past its noise floor qualifies", []float64{0, 5.5, 2}, true},
		{"second metric decides when the first ties", []float64{0, 4, 2.6}, true},
		{"second metric within noise fails", []float64{0, 4, 2.5}, false},
		{"first differing component gates even a huge later gain", []float64{0, 4.5, 100}, false},
		{"lexicographically smaller never qualifies", []float64{0, 3, 100}, false},
	}
	for _, tc := range cases {
		if got := gate.qualifies(tc.candidate, best); got != tc.want {
			t.Fatalf("%s: qualifies=%v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestSingleMetricScalarRatchet(t *testing.T) {
	gate := &stopLossGate{
		direction:  "min",
		metrics:    []string{"latency"},
		thresholds: map[string]string{"latency": "<=100"},
		noise:      map[string]float64{"latency": 2},
		baseline:   map[string]float64{"latency": 150},
	}
	// With direction min the directed value is negated, so lower measured
	// values ratchet the best exactly like a scalar improvement.
	events := []mission.LedgerEvent{
		cycleEvent("unresolved", "latency=149", ""),          // within noise: not a best
		cycleEvent("contract-improved", "latency=140", ""),   // past noise: best
		cycleEvent("no-progress", "latency=160", ""),         // regression
		cycleEvent("contract-improved", "latency=139.5", ""), // above old best only within noise: not a best
	}
	verdict := replayStopLossVerdict(gate, 100, 3, events)
	// Cycles 1 counts (unresolved), cycle 2 resets, cycles 3 counts, cycle 4
	// is a recovery classified contract-improved: it neither counts nor
	// resets, and it never lowers the stagnant count (no decay).
	if verdict.Stagnant != 1 || verdict.Tripped {
		t.Fatalf("scalar ratchet verdict: %+v", verdict)
	}
}

func TestReplayArithmetic(t *testing.T) {
	gate := twoMetricGate()
	improved := "zeta=12,alpha=6" // clears both thresholds: a new best

	t.Run("no-progress and unresolved both count; fuse at budget", func(t *testing.T) {
		events := []mission.LedgerEvent{
			cycleEvent("no-progress", "zeta=4,alpha=2", ""),
			cycleEvent("unresolved", "zeta=4,alpha=2", ""),
			cycleEvent("falsified-continue", "zeta=4,alpha=2", ""),
		}
		verdict := replayStopLossVerdict(gate, 100, 3, events)
		if verdict.Stagnant != 2 || verdict.Tripped {
			t.Fatalf("lawful falsification cycles must not count: %+v", verdict)
		}
		events = append(events, cycleEvent("unresolved", "zeta=4,alpha=2", ""))
		verdict = replayStopLossVerdict(gate, 100, 3, events)
		if !verdict.Tripped || verdict.Kind != StopLossStagnation || verdict.Stagnant != 3 {
			t.Fatalf("fuse must fire at the budget: %+v", verdict)
		}
	})

	t.Run("a new best resets the count", func(t *testing.T) {
		events := []mission.LedgerEvent{
			cycleEvent("no-progress", "zeta=4,alpha=2", ""),
			cycleEvent("no-progress", "zeta=4,alpha=2", ""),
			cycleEvent("contract-improved", improved, ""),
			cycleEvent("unresolved", improved, ""),
		}
		verdict := replayStopLossVerdict(gate, 100, 3, events)
		if verdict.Stagnant != 1 || verdict.Tripped {
			t.Fatalf("stagnant must count from the last best: %+v", verdict)
		}
	})

	t.Run("a reset line resets the count", func(t *testing.T) {
		events := []mission.LedgerEvent{
			cycleEvent("no-progress", "zeta=4,alpha=2", ""),
			cycleEvent("no-progress", "zeta=4,alpha=2", ""),
			{Reset: true, AskID: "stop-loss", Reason: "human says continue"},
			cycleEvent("unresolved", "zeta=4,alpha=2", ""),
		}
		verdict := replayStopLossVerdict(gate, 100, 3, events)
		if verdict.Stagnant != 1 || verdict.Tripped {
			t.Fatalf("stagnant must count from the reset line: %+v", verdict)
		}
	})

	t.Run("regression then recovery never lowers the count", func(t *testing.T) {
		events := []mission.LedgerEvent{
			cycleEvent("contract-improved", improved, ""), // best
			cycleEvent("no-progress", "zeta=4,alpha=2", ""),
			cycleEvent("contract-improved", improved, "no"), // recovery back to the old best: no new best
			cycleEvent("unresolved", "zeta=4,alpha=2", ""),
		}
		verdict := replayStopLossVerdict(gate, 100, 3, events)
		if verdict.Stagnant != 2 || verdict.Tripped {
			t.Fatalf("recovery must not decrement (no decay): %+v", verdict)
		}
	})

	t.Run("cycle budget is enforced in the same verdict and dominates", func(t *testing.T) {
		events := []mission.LedgerEvent{
			cycleEvent("no-progress", "zeta=4,alpha=2", ""),
			cycleEvent("no-progress", "zeta=4,alpha=2", ""),
		}
		verdict := replayStopLossVerdict(gate, 2, 2, events)
		if !verdict.Tripped || verdict.Kind != StopLossCycleBudget {
			t.Fatalf("an exhausted cycle budget must dominate: %+v", verdict)
		}
	})
}

func TestMarkerWinsOverDerivation(t *testing.T) {
	gate := twoMetricGate()
	improved := "zeta=12,alpha=6"

	// best=no on a line that would derive as a best: the marker wins, so the
	// following stagnant cycles count from the baseline era.
	events := []mission.LedgerEvent{
		cycleEvent("contract-improved", improved, "no"),
		cycleEvent("unresolved", "zeta=4,alpha=2", ""),
		cycleEvent("unresolved", "zeta=4,alpha=2", ""),
	}
	if verdict := replayStopLossVerdict(gate, 100, 2, events); !verdict.Tripped || verdict.Stagnant != 2 {
		t.Fatalf("best=no must win over derivation: %+v", verdict)
	}

	// best=yes on a line derivation would refuse: the marker wins and resets.
	events = []mission.LedgerEvent{
		cycleEvent("unresolved", "zeta=4,alpha=2", ""),
		cycleEvent("unresolved", "zeta=4.2,alpha=2", "yes"),
		cycleEvent("unresolved", "zeta=4,alpha=2", ""),
	}
	if verdict := replayStopLossVerdict(gate, 100, 2, events); verdict.Tripped || verdict.Stagnant != 1 {
		t.Fatalf("best=yes must win over derivation: %+v", verdict)
	}
}

func TestLegacyLedgerConservativeReplay(t *testing.T) {
	gate := twoMetricGate()
	// Marker-less lines derive; unparseable observed values fold as the
	// sealed baseline, so they can never manufacture a new best.
	events := []mission.LedgerEvent{
		cycleEvent("no-progress", "unmeasurable:gate measurement failed with exit 1", ""),
		cycleEvent("unresolved", "zeta=garbage,alpha=", ""),
		cycleEvent("unresolved", "", ""),
	}
	verdict := replayStopLossVerdict(gate, 100, 3, events)
	if !verdict.Tripped || verdict.Kind != StopLossStagnation || verdict.Stagnant != 3 {
		t.Fatalf("conservative fold must count all three cycles: %+v", verdict)
	}
}

func TestLegacyStopLossVerdictMatchesScriptRules(t *testing.T) {
	cases := []struct {
		name    string
		classes []string
		budget  int
		noGain  int
		tripped bool
	}{
		{"dead end trips", []string{"contract-improved", "falsified-dead-end"}, 10, 5, true},
		{"two lifetime no-progress trip", []string{"no-progress", "contract-improved", "no-progress"}, 10, 5, true},
		{"cycle budget trips", []string{"contract-improved", "contract-improved", "contract-improved"}, 3, 5, true},
		{"trailing no-gain trips over a mixed tail", []string{"contract-improved", "unresolved", "falsified-continue"}, 10, 2, true},
		{"contract-improved resets the trailing count", []string{"unresolved", "contract-improved", "unresolved"}, 10, 2, false},
		{"unresolved alone stays under the budgets", []string{"unresolved"}, 10, 2, false},
	}
	for _, tc := range cases {
		var events []mission.LedgerEvent
		for _, class := range tc.classes {
			events = append(events, cycleEvent(class, "score=1", ""))
		}
		verdict := legacyStopLossVerdict(tc.budget, tc.noGain, events)
		if verdict.Tripped != tc.tripped {
			t.Fatalf("%s: tripped=%v (%+v)", tc.name, verdict.Tripped, verdict)
		}
		if verdict.Tripped && verdict.Kind != StopLossLegacy {
			t.Fatalf("%s: legacy trips carry the legacy kind: %+v", tc.name, verdict)
		}
	}
}

// replayContract is a minimal sealed contract snapshot for engine-level
// verdict tests: one metric, threshold >=10, noise 0.5, baseline 5, and
// deliberately different budgets from the ledger so the tests can prove
// which source each semantics reads.
const replayContract = "# Intent\n\nfixture\n\n```mission\n" +
	"gate.direction=max\n" +
	"gate.threshold.score=>=10\n" +
	"gate.noise-floor.score=0.5\n" +
	"ledger.cycle-budget=10\n" +
	"ledger.no-gain-budget=5\n" +
	"```\n\n```mission-seal\n" +
	"sealed.baseline.score=5\n" +
	"```\n"

// stopLossEngine pins the replay contract as the approved snapshot and
// seeds a ledger whose own budget lines are tighter than the contract's.
func stopLossEngine(t *testing.T) (*Engine, string) {
	t.Helper()
	root := t.TempDir()
	engine := NewEngine(root, "demo")
	approved := engine.approvedContractPath()
	if err := os.MkdirAll(filepath.Dir(approved), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(approved, []byte(replayContract), 0o644); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte(replayContract))
	writeJSONFile(t, engine.fencesPath(), map[string]any{
		"approvedContractSha256": hex.EncodeToString(sum[:]),
	})
	ledger := filepath.Join(engine.missionDir(), "ledger.md")
	if err := mission.InitLedger(ledger, 4, 2); err != nil {
		t.Fatal(err)
	}
	return engine, ledger
}

const testSHA = "abcdef0123456789abcdef0123456789abcdef01"

func TestStopLossVerdictKeyedBySemantics(t *testing.T) {
	engine, ledger := stopLossEngine(t)
	for cycle, class := range []string{"no-progress", "no-progress"} {
		if err := mission.AppendCycle(ledger, cycle+1, class, testSHA, "score=5", ""); err != nil {
			t.Fatal(err)
		}
	}

	// A state without the field replays under the legacy rules and the
	// ledger's own budgets: two lifetime no-progress cycles trip.
	legacy, err := engine.stopLossVerdict(map[string]any{}, ledger)
	if err != nil {
		t.Fatal(err)
	}
	if !legacy.Tripped || legacy.Semantics != 1 || legacy.CycleBudget != 4 {
		t.Fatalf("legacy verdict: %+v", legacy)
	}

	// The same ledger under pinned semantics 2 replays against the sealed
	// contract's budgets: two stagnant cycles stay under no-gain 5.
	replay, err := engine.stopLossVerdict(map[string]any{"ledgerSemantics": 2}, ledger)
	if err != nil {
		t.Fatal(err)
	}
	if replay.Tripped || replay.Semantics != 2 || replay.Stagnant != 2 || replay.CycleBudget != 10 {
		t.Fatalf("replay verdict: %+v", replay)
	}

	// A semantics this runner does not implement is refused, never guessed.
	if _, err := engine.stopLossVerdict(map[string]any{"ledgerSemantics": 3}, ledger); err == nil ||
		!strings.Contains(err.Error(), "newer than this runner") {
		t.Fatalf("unknown semantics must refuse: %v", err)
	}
}

func TestBestMarkerComputedAtAppend(t *testing.T) {
	engine, ledger := stopLossEngine(t)
	replayState := map[string]any{"ledgerSemantics": 2}

	// Against the sealed baseline of 5 with noise 0.5: 6 is a new best,
	// 5.2 is within the noise floor.
	if marker, err := engine.bestMarker(replayState, ledger, "score=6"); err != nil || marker != "yes" {
		t.Fatalf("improvement past noise: %q, %v", marker, err)
	}
	if marker, err := engine.bestMarker(replayState, ledger, "score=5.2"); err != nil || marker != "no" {
		t.Fatalf("improvement within noise: %q, %v", marker, err)
	}

	// Once a best is on the ledger, the fold measures against it.
	if err := mission.AppendCycle(ledger, 1, "contract-improved", testSHA, "score=6", "yes"); err != nil {
		t.Fatal(err)
	}
	if marker, err := engine.bestMarker(replayState, ledger, "score=6.4"); err != nil || marker != "no" {
		t.Fatalf("within noise of the folded best: %q, %v", marker, err)
	}
	if marker, err := engine.bestMarker(replayState, ledger, "score=7"); err != nil || marker != "yes" {
		t.Fatalf("past noise of the folded best: %q, %v", marker, err)
	}

	// Legacy missions keep marker-less lines.
	if marker, err := engine.bestMarker(map[string]any{}, ledger, "score=100"); err != nil || marker != "" {
		t.Fatalf("legacy missions must not gain markers: %q, %v", marker, err)
	}
}

// TestAnnotationsNeverChangeAReplayVerdict pins the replay invariant: cycle
// annotations are audit trail, never fuse input. Two ledgers with identical
// classification, best, and reset lines — one annotated, one not — must
// yield the identical verdict under both semantics.
func TestAnnotationsNeverChangeAReplayVerdict(t *testing.T) {
	engine, _ := stopLossEngine(t)
	build := func(name string, annotated bool) string {
		ledger := filepath.Join(t.TempDir(), name+".md")
		if err := mission.InitLedger(ledger, 4, 2); err != nil {
			t.Fatal(err)
		}
		annotations := func(values ...string) []string {
			if annotated {
				return values
			}
			return nil
		}
		if err := mission.AppendCycle(ledger, 1, "no-progress", testSHA, "score=5", "no",
			annotations(mission.ReturnRejectedAnnotation("session identity"))...); err != nil {
			t.Fatal(err)
		}
		if err := mission.AppendReset(ledger, "stop-loss", "keep going"); err != nil {
			t.Fatal(err)
		}
		if err := mission.AppendCycle(ledger, 2, "unresolved", testSHA, "score=5", "no",
			annotations(mission.CappedAnnotation)...); err != nil {
			t.Fatal(err)
		}
		return ledger
	}
	plain := build("plain", false)
	annotated := build("annotated", true)
	for _, state := range []map[string]any{{}, {"ledgerSemantics": 2}} {
		got, err := engine.stopLossVerdict(state, annotated)
		if err != nil {
			t.Fatal(err)
		}
		want, err := engine.stopLossVerdict(state, plain)
		if err != nil {
			t.Fatal(err)
		}
		if *got != *want {
			t.Fatalf("annotations changed the replay verdict: %+v vs %+v", *got, *want)
		}
	}
}

// TestReplayCountsHealedDrainStalledLineOnce pins the stop-loss reading of
// the heal's drain-stalled line: it counts as no-progress exactly once, and
// its survivor-count annotation never changes the verdict.
func TestReplayCountsHealedDrainStalledLineOnce(t *testing.T) {
	engine, _ := stopLossEngine(t)
	build := func(name string, annotated bool) string {
		ledger := filepath.Join(t.TempDir(), name+".md")
		if err := mission.InitLedger(ledger, 4, 2); err != nil {
			t.Fatal(err)
		}
		var annotations []string
		if annotated {
			annotations = []string{mission.DrainStalledAnnotation(2)}
		}
		if err := mission.AppendCycle(ledger, 1, "no-progress", testSHA, mission.DrainStalledObserved, "no", annotations...); err != nil {
			t.Fatal(err)
		}
		return ledger
	}
	annotated := build("annotated", true)
	verdict, err := engine.stopLossVerdict(map[string]any{"ledgerSemantics": 2}, annotated)
	if err != nil {
		t.Fatal(err)
	}
	if verdict.Cycles != 1 || verdict.Stagnant != 1 || verdict.Tripped {
		t.Fatalf("the healed line must count as no-progress exactly once: %+v", verdict)
	}
	plain, err := engine.stopLossVerdict(map[string]any{"ledgerSemantics": 2}, build("plain", false))
	if err != nil {
		t.Fatal(err)
	}
	if *verdict != *plain {
		t.Fatalf("the annotation changed the verdict: %+v vs %+v", *verdict, *plain)
	}
}

func TestThresholdDeclarationOrder(t *testing.T) {
	text := "```mission\ngate.threshold.zeta=>=1\ngate.noise-floor.zeta=0\ngate.threshold.alpha=>=1\ngate.noise-floor.alpha=0\n```\n"
	metrics, err := thresholdDeclarationOrder(text)
	if err != nil {
		t.Fatal(err)
	}
	if len(metrics) != 2 || metrics[0] != "zeta" || metrics[1] != "alpha" {
		t.Fatalf("declaration order must survive parsing: %v", metrics)
	}
}
