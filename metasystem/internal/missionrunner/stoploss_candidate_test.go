package missionrunner

import (
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/mission"
)

// The candidate-aware tuple's core properties, driven directly
// on the gate — (1) a passing candidate is a NEW BEST against a
// no-candidate history; (2) a candidate pass can NEVER outrank a real
// merge (gate-of-record components dominate lexicographically); (3) an
// absent candidate token reads as directed worst, never progress.
func TestCandidateTupleProperties(t *testing.T) {
	gate := &stopLossGate{
		direction:      "max",
		metrics:        []string{"self-assessment"},
		thresholds:     map[string]string{"self-assessment": ">=1"},
		noise:          map[string]float64{"self-assessment": 0},
		baseline:       map[string]float64{"self-assessment": 0},
		candidateAware: true,
	}
	baseline := gate.tuple(gate.baseline)

	// (3) a cycle with no candidate measurement is not progress.
	stagnant := gate.tuple(gate.observedValues("self-assessment=0"))
	if gate.qualifies(stagnant, baseline) {
		t.Fatal("no-candidate cycle qualified as a best")
	}

	// (1) the first PASSING candidate is a new best and would reset
	// stagnation.
	candidatePass := gate.tuple(gate.observedValues("self-assessment=0,candidate-self-assessment=1"))
	if !gate.qualifies(candidatePass, baseline) {
		t.Fatal("a passing candidate did not qualify as a new best")
	}

	// (2) a real merge (gate of record passing) outranks any candidate
	// state, and no candidate movement can outrank it back.
	merged := gate.tuple(gate.observedValues("self-assessment=1"))
	if !gate.qualifies(merged, candidatePass) {
		t.Fatal("a real merge did not outrank a candidate pass")
	}
	candidateAgain := gate.tuple(gate.observedValues("self-assessment=0,candidate-self-assessment=1"))
	if gate.qualifies(candidateAgain, merged) {
		t.Fatal("a candidate pass outranked a real merge")
	}

	// Replay determinism: the same observed bytes fold to the same
	// tuple, marker or not.
	again := gate.tuple(gate.observedValues("self-assessment=0,candidate-self-assessment=1"))
	for i := range candidatePass {
		if candidatePass[i] != again[i] {
			t.Fatalf("tuple not deterministic at %d: %v vs %v", i, candidatePass, again)
		}
	}
}

// On a MINIMIZATION gate the seed's absent candidate
// components must be the directed worst (+inf pre-negation), so the first
// real passing candidate still qualifies as a new best.
func TestCandidateSeedDirectedWorstOnMinGate(t *testing.T) {
	gate := &stopLossGate{
		direction:      "min",
		metrics:        []string{"defects"},
		thresholds:     map[string]string{"defects": "<=1"},
		noise:          map[string]float64{"defects": 0},
		baseline:       map[string]float64{"defects": 5},
		candidateAware: true,
	}
	seed := gate.tuple(gate.observedValues(""))
	pass := gate.tuple(gate.observedValues("defects=5,candidate-defects=1"))
	if !gate.qualifies(pass, seed) {
		t.Fatalf("a passing min-gate candidate did not beat the seed: seed=%v pass=%v", seed, pass)
	}
}

// The fold replays REAL LEDGER BYTES: a semantics-3 mission whose cycle 4
// carries a passing candidate token resets stagnation —
// and the same bytes without the token would park.
func TestCandidateTokenResetsStagnationInReplay(t *testing.T) {
	engine, ledgerPath := stopLossEngine(t)
	for cycle, observed := range []string{
		"score=5",
		"score=5",
		"score=5",
		"score=5,candidate-score=12",
		"score=5",
	} {
		if _, err := mission.AppendCycle(ledgerPath, cycle+1, "unresolved", testSHA, observed, ""); err != nil {
			t.Fatal(err)
		}
	}
	verdict, err := engine.stopLossVerdict(map[string]any{"ledgerSemantics": 3}, ledgerPath)
	if err != nil {
		t.Fatal(err)
	}
	if verdict.Tripped || verdict.Stagnant != 1 {
		t.Fatalf("candidate pass did not reset stagnation: %+v", verdict)
	}
	plain, err := engine.stopLossVerdict(map[string]any{"ledgerSemantics": 2}, ledgerPath)
	if err != nil {
		t.Fatal(err)
	}
	if plain.Stagnant != 5 || !plain.Tripped {
		t.Fatalf("semantics-2 replay of the same bytes must trip at stagnant 5: %+v", plain)
	}
}
