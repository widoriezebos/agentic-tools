package turn

import (
	"reflect"
	"testing"
)

func TestOrchestratorMayRaise(t *testing.T) {
	for _, reason := range []string{"reserved-decision", "red-test", "merge-conflict", "host-failure"} {
		if !OrchestratorMayRaise(reason) {
			t.Fatalf("%q must be orchestrator-raisable", reason)
		}
	}
	for _, reason := range []string{"fence", "stop-loss", "drain-stalled", "invented"} {
		if OrchestratorMayRaise(reason) {
			t.Fatalf("%q must NOT be orchestrator-raisable", reason)
		}
	}
}

// The prompt vocabulary is the orchestrator's set plus the runner's three —
// the equality the turn-prompt validator depends on (B-3).
func TestPromptMayCarry(t *testing.T) {
	for _, reason := range []string{
		"reserved-decision", "red-test", "merge-conflict", "host-failure",
		"fence", "stop-loss", "drain-stalled",
	} {
		if !PromptMayCarry(reason) {
			t.Fatalf("%q must be carryable in a prompt", reason)
		}
	}
	if PromptMayCarry("invented") {
		t.Fatal("an unknown reason must not be carryable")
	}
}

func TestOrchestratorAskReasonsIsSorted(t *testing.T) {
	want := []string{"host-failure", "merge-conflict", "red-test", "reserved-decision"}
	if got := OrchestratorAskReasons(); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}
