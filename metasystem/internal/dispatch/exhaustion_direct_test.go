package dispatch

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// CritiqueExhaustionAction's guard paths and the small value helpers
// (Phase 6).

func TestCritiqueExhaustionGuards(t *testing.T) {
	root := t.TempDir()
	jobs := filepath.Join(root, "artifacts", "agents", "jobs")
	os.MkdirAll(jobs, 0o755)
	latest := filepath.Join(jobs, "chain-f2.json")
	message := filepath.Join(root, "message.md")

	// An unreadable successor message refuses by name.
	if _, err := CritiqueExhaustionAction(root, "chain", "design-critic", latest, message, "chain-f3", filepath.Join(root, "out.json")); err == nil ||
		!strings.Contains(err.Error(), "successor message is unreadable") {
		t.Fatalf("missing message not refused: %v", err)
	}
	os.WriteFile(message, []byte("continue\n"), 0o644)

	// An unreadable latest record refuses by name.
	if _, err := CritiqueExhaustionAction(root, "chain", "design-critic", latest, message, "chain-f3", filepath.Join(root, "out.json")); err == nil ||
		!strings.Contains(err.Error(), "follow-up job record is unreadable") {
		t.Fatalf("missing latest not refused: %v", err)
	}

	// A protocol-error latest deliberately reads nothing further: verdict
	// none, no error.
	os.WriteFile(latest, []byte(`{"jobId":"chain-f2","status":"failed","error":"protocol_error"}`), 0o644)
	action, err := CritiqueExhaustionAction(root, "chain", "design-critic", latest, message, "chain-f3", filepath.Join(root, "out.json"))
	if err != nil || action != "none" {
		t.Fatalf("protocol recovery: action=%q err=%v", action, err)
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

// ValidateMission's guard ladder, up to the liveness probe (Phase 6).
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
