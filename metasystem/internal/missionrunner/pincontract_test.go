package missionrunner

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"strings"
	"testing"
)

// pinVerifiedContract's ladder with real files (Phase 6): the start-mode
// bootstrap, the double-pin refusal, resume against absent fences, the
// digest mismatch, and identity checks.
func TestPinVerifiedContract(t *testing.T) {
	engine := &Engine{Root: t.TempDir(), Mission: "mr-pin"}
	snapshot := []byte("contract bytes\n")
	sum := sha256.Sum256(snapshot)
	sha := hex.EncodeToString(sum[:])

	// Resume before any start: refused by name.
	if err := engine.pinVerifiedContract("resume", snapshot, sha); err == nil ||
		!strings.Contains(err.Error(), "fence state is absent") {
		t.Fatalf("resume without fences: %v", err)
	}

	// A wrong digest never pins.
	if err := engine.pinVerifiedContract("start", snapshot, "deadbeef"); err == nil ||
		!strings.Contains(err.Error(), "does not match its verified raw-file sha256") {
		t.Fatalf("digest mismatch: %v", err)
	}

	// A clean start bootstraps the fences and lands the snapshot.
	if err := engine.pinVerifiedContract("start", snapshot, sha); err != nil {
		t.Fatalf("clean start: %v", err)
	}
	pinned, err := os.ReadFile(engine.approvedContractPath())
	if err != nil || string(pinned) != string(snapshot) {
		t.Fatalf("snapshot not pinned: %v %q", err, pinned)
	}

	// A second start against the pin is refused toward resume.
	if err := engine.pinVerifiedContract("start", snapshot, sha); err == nil ||
		!strings.Contains(err.Error(), "already pinned; use resume") {
		t.Fatalf("double pin: %v", err)
	}

	// Resume with the fences present succeeds.
	if err := engine.pinVerifiedContract("resume", snapshot, sha); err != nil {
		t.Fatalf("lawful resume: %v", err)
	}

	// A foreign missionId in the counters is an identity refusal.
	other := &Engine{Root: engine.Root, Mission: "mr-pin"}
	other.Mission = "mr-other"
	os.MkdirAll(other.missionDir(), 0o755)
	os.WriteFile(other.fencesPath(), []byte(`{"schemaVersion":1,"missionId":"mr-pin","startedAt":"x","cycles":0,"reservations":{}}`), 0o644)
	if err := other.pinVerifiedContract("resume", snapshot, sha); err == nil ||
		!strings.Contains(err.Error(), "invalid identity") {
		t.Fatalf("foreign identity: %v", err)
	}
}
