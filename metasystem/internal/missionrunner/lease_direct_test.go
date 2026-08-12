package missionrunner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Direct tests for the mission lease and state verification (Phase 6).

func TestAcquireLeaseLifecycle(t *testing.T) {
	engine := &Engine{Root: t.TempDir(), Mission: "mr-lease"}
	leasePath, err := engine.acquireLease("mr-lease-tag")
	if err != nil {
		t.Fatalf("fresh acquire: %v", err)
	}
	data, readErr := os.ReadFile(leasePath)
	if readErr != nil {
		t.Fatalf("lease not written: %v", readErr)
	}
	text := string(data)
	if !strings.Contains(text, `"missionId": "mr-lease"`) || !strings.Contains(text, "mr-lease-tag") {
		t.Fatalf("lease identity wrong: %s", text)
	}
	// A second acquire against the held marker is the busy refusal.
	if _, err := engine.acquireLease("thief"); err == nil ||
		!strings.Contains(err.Error(), "mission lease is busy") {
		t.Fatalf("busy lease not refused: %v", err)
	}
	// Release frees everything; a re-acquire then succeeds; releasing an
	// already-released lease is a no-op.
	engine.releaseLease()
	if _, err := engine.acquireLease("mr-lease-tag-2"); err != nil {
		t.Fatalf("re-acquire after release: %v", err)
	}
	engine.releaseLease()
	engine.releaseLease()
}

func TestVerifyStateShapeRefusals(t *testing.T) {
	engine := &Engine{Root: t.TempDir(), Mission: "mr-verify"}
	dir := t.TempDir()
	statePath := filepath.Join(dir, "state.json")
	// Absent state: the failure carries the runner's own exit class.
	if _, err := engine.verifyState(statePath, false); err == nil {
		t.Fatal("absent state verified")
	}
	// Malformed state: refused with the shape checker's wording.
	os.WriteFile(statePath, []byte("{broken"), 0o644)
	if _, err := engine.verifyState(statePath, false); err == nil {
		t.Fatal("malformed state verified")
	}
}
