//go:build batchtest

package main

import (
	"strings"
	"testing"
)

func TestBatchCapabilitiesGate(t *testing.T) {
	commands := [][]string{
		{"landing", "batch", "join"}, {"landing", "batch", "status"},
		{"landing", "batch", "withdraw"}, {"landing", "batch", "owner"},
		{"landing", "batch", "tick"}, {"landing", "batch", "wait"},
		{"goal", "handover"},
	}
	for _, command := range commands {
		if code := dispatch(command); code != 0 {
			t.Fatalf("all capabilities registered: %v exited %d", command, code)
		}
	}
	for _, capability := range batchTestCapabilities {
		t.Run(string(capability), func(t *testing.T) {
			restore := unregisterBatchCapabilityForTest(capability)
			defer restore()
			stderr, code := captureStderr(t, func() int {
				return dispatch([]string{"landing", "batch", "join"})
			})
			if code != 1 || !strings.Contains(stderr, "BATCH_UNAVAILABLE") {
				t.Fatalf("missing %s = code %d, stderr %q", capability, code, stderr)
			}
		})
	}
}
