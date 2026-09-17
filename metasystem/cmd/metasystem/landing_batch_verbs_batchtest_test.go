//go:build batchtest

package main

import (
	"strings"
	"testing"
)

func TestBatchCapabilitiesGate(t *testing.T) {
	if !batchCapabilitiesAvailable() {
		t.Fatal("all batchtest capabilities must be registered")
	}
	for _, capability := range batchTestCapabilities {
		t.Run(string(capability), func(t *testing.T) {
			restore := unregisterBatchCapabilityForTest(capability)
			defer restore()
			stderr, code := captureStderr(t, func() int { return dispatch([]string{"landing", "batch", "status"}) })
			if code != 1 || !strings.Contains(stderr, "BATCH_UNAVAILABLE") {
				t.Fatalf("missing %s = code %d, stderr %q", capability, code, stderr)
			}
		})
	}
}

func TestGoalHandoverRequiresCompleteInputs(t *testing.T) {
	stderr, code := captureStderr(t, func() int { return dispatch([]string{"goal", "handover"}) })
	if code != 2 || !strings.Contains(stderr, "goal handover needs") {
		t.Fatalf("empty handover = code %d, stderr %q", code, stderr)
	}
}
func TestGoalHandoverTargetAuthenticationFailsClosed(t *testing.T) {
	if state, _ := goalHandoverTargetLiveness(t.TempDir(), "landing", "lineage", 1); state.String() != "unknown" {
		t.Fatalf("unconfigured target liveness = %s, want unknown", state)
	}
}
