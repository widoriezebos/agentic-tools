//go:build batchtest

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
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

func TestBatchTrunkRedLedgerOwnerCapability(t *testing.T) {
	restore := unregisterBatchCapabilityForTest(trunkRedLedgerOwner)
	if batchCapabilitiesAvailable() {
		t.Fatal("automatic landing remained available without the trunk-red ledger owner marker")
	}
	restore()
	root := t.TempDir()
	owner, err := productionTrunkRedLedgerOwner(root)
	if err != nil {
		t.Fatal(err)
	}
	red := batch.TrunkRed{BatchID: "batch", AttemptID: "attempt", BaseTree: "tree", Groups: []batch.RedGroup{{ID: "group", Status: "failed"}}}
	first, err := owner.Record("op-1", red)
	if err != nil {
		t.Fatal(err)
	}
	second, err := owner.Record("op-1", red)
	if err != nil || len(first) != 1 || len(second) != 1 || first[0] != second[0] {
		t.Fatalf("idempotent refs first=%v second=%v error=%v", first, second, err)
	}
	data, err := os.ReadFile(filepath.Join(root, "memory", "flake-registry.md"))
	if err != nil || strings.Count(string(data), "batch-trunk-red:") != 1 {
		t.Fatalf("test-only ledger bytes=%q error=%v", data, err)
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
