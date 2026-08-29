package dispatch

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// CritiqueExhaustionAdvance's guard paths and the small value helpers.

func TestCritiqueExhaustionGuards(t *testing.T) {
	root := t.TempDir()
	message := filepath.Join(root, "message.md")

	// An unreadable successor message refuses by name.
	if _, err := CritiqueExhaustionAdvance(root, "chain", "design-critic", message, "chain-f3"); err == nil ||
		!strings.Contains(err.Error(), "successor message is unreadable") {
		t.Fatalf("missing message not refused: %v", err)
	}
	os.WriteFile(message, []byte("continue\n"), 0o644)

	// An unreadable root record refuses by name.
	if _, err := CritiqueExhaustionAdvance(root, "chain", "design-critic", message, "chain-f3"); err == nil ||
		!strings.Contains(err.Error(), "root record chain is unreadable") {
		t.Fatalf("missing root not refused: %v", err)
	}
}

func TestOpErrorRenderings(t *testing.T) {
	withMessage := &OpError{Code: 3, Message: "named refusal"}
	if withMessage.Error() != "named refusal" {
		t.Fatalf("message lost: %q", withMessage.Error())
	}
	bare := &OpError{Code: 7}
	if !strings.Contains(bare.Error(), "code 7") {
		t.Fatalf("bare code not rendered: %q", bare.Error())
	}
	typed := &OpError{Code: CritiqueCapExhaustedExitCode, Reason: CritiqueCapExhaustedReason, Message: "terminal"}
	if got := typed.Error(); got != "reason=cap-exhausted-human-raise terminal" {
		t.Fatalf("machine-readable reason lost: %q", got)
	}
}

func TestNumStringSpellings(t *testing.T) {
	if got, ok := numString(json.Number("42")); !ok || got != "42" {
		t.Fatalf("json.Number: %q %v", got, ok)
	}
	if got, ok := numString(json.Number("1.5")); !ok || got != "1.5" {
		t.Fatalf("json.Number keeps its literal spelling: %q %v", got, ok)
	}
	if got, ok := numString(float64(7)); !ok || got != "7" {
		t.Fatalf("whole float: %q %v", got, ok)
	}
	if _, ok := numString(0.5); ok {
		t.Fatal("a fractional float64 has no round-name")
	}
	if _, ok := numString("7"); ok {
		t.Fatal("a string is not a number")
	}
}

// ValidateMission's guard ladder, up to the liveness probe.
func TestValidateMissionGuards(t *testing.T) {
	root := t.TempDir()
	leaseDir := filepath.Join(root, "artifacts", "agents", "missions", "m-1")
	os.MkdirAll(leaseDir, 0o755)
	lease := filepath.Join(leaseDir, "lease.json")

	if err := ValidateMission(root, "NOT VALID", lease); err == nil ||
		!strings.Contains(err.Error(), "invalid mission id") {
		t.Fatalf("bad id: %v", err)
	}
	if err := ValidateMission(root, "m-1", filepath.Join(root, "elsewhere.json")); err == nil ||
		!strings.Contains(err.Error(), "non-canonical") {
		t.Fatalf("foreign lease path: %v", err)
	}
	if err := ValidateMission(root, "m-1", lease); err == nil ||
		!strings.Contains(err.Error(), "no readable live lease") {
		t.Fatalf("absent lease: %v", err)
	}
	os.WriteFile(lease, []byte(`{"missionId":"m-1"}`), 0o644)
	if err := ValidateMission(root, "m-1", lease); err == nil ||
		!strings.Contains(err.Error(), "invalid shape or identity") {
		t.Fatalf("short lease: %v", err)
	}
	os.WriteFile(lease, []byte(`{"missionId":"other","pid":1,"pgid":1,"instanceTag":"t","startedAt":"x","renewedAt":"x"}`), 0o644)
	if err := ValidateMission(root, "m-1", lease); err == nil ||
		!strings.Contains(err.Error(), "invalid shape or identity") {
		t.Fatalf("foreign missionId: %v", err)
	}
	os.WriteFile(lease, []byte(`{"missionId":"m-1","pid":"NaN","pgid":1,"instanceTag":"t","startedAt":"x","renewedAt":"x"}`), 0o644)
	if err := ValidateMission(root, "m-1", lease); err == nil ||
		!strings.Contains(err.Error(), "invalid ownership fields") {
		t.Fatalf("bad pid: %v", err)
	}
	// A dead pid: choose one that cannot exist.
	os.WriteFile(lease, []byte(`{"missionId":"m-1","pid":1073741824,"pgid":1,"instanceTag":"t","startedAt":"x","renewedAt":"x"}`), 0o644)
	if err := ValidateMission(root, "m-1", lease); err == nil ||
		!strings.Contains(err.Error(), "holder is not alive") {
		t.Fatalf("dead holder: %v", err)
	}
}
