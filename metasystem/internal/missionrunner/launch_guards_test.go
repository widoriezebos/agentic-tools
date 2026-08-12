package missionrunner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The launch spine's guard ladder and armAndPreflight's refusal branches,
// driven with a stub arming neighbor (Phase 6). The stub stands in for
// arm-supervision.sh only — arming itself has its own fixtures; the unit
// here is the ORCHESTRATION: sequence, refusal wording, and the handoff
// into contract preflight.

func stubArming(t *testing.T, root, body string) {
	t.Helper()
	dir := filepath.Join(root, "scripts", "agents")
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, "arm-supervision.sh"),
		[]byte("#!/bin/sh\n"+body+"\n"), 0o755)
}

func TestLaunchGuardLadder(t *testing.T) {
	engine := &Engine{Root: t.TempDir(), Mission: "mr-launch"}
	os.MkdirAll(engine.missionDir(), 0o755)
	statePath := filepath.Join(engine.missionDir(), "state.json")

	// Resume with no state at all.
	if err := engine.launch("resume", false); err == nil ||
		!strings.Contains(err.Error(), "state does not exist") {
		t.Fatalf("resume without state: %v", err)
	}
	// Start over an existing state steers to resume.
	os.WriteFile(statePath, []byte(`{}`), 0o644)
	if err := engine.launch("start", false); err == nil ||
		!strings.Contains(err.Error(), "already exists; use resume") {
		t.Fatalf("start over state: %v", err)
	}
	// Resume over a malformed state surfaces the verifier's refusal.
	os.WriteFile(statePath, []byte(`{broken`), 0o644)
	if err := engine.launch("resume", false); err == nil {
		t.Fatal("resume verified a malformed state")
	}
}

func TestArmAndPreflightRefusals(t *testing.T) {
	// No arming script at all: the arm step refuses by name.
	bare := &Engine{Root: t.TempDir(), Mission: "mr-arm-a"}
	if err := bare.armAndPreflight("start"); err == nil ||
		!strings.Contains(err.Error(), "supervision did not arm") {
		t.Fatalf("armless root: %v", err)
	}

	// An arming script that fails: same named refusal, its stderr carried.
	failing := &Engine{Root: t.TempDir(), Mission: "mr-arm-b"}
	stubArming(t, failing.Root, `echo "deliberate refusal" >&2; exit 1`)
	if err := failing.armAndPreflight("start"); err == nil ||
		!strings.Contains(err.Error(), "supervision did not arm") ||
		!strings.Contains(err.Error(), "deliberate refusal") {
		t.Fatalf("failing armer: %v", err)
	}

	// An armer that reports ARMED hands off to contract preflight, which
	// refuses the absent contract by name.
	armed := &Engine{Root: t.TempDir(), Mission: "mr-arm-c"}
	stubArming(t, armed.Root, `echo ARMED`)
	if err := armed.armAndPreflight("start"); err == nil ||
		!strings.Contains(err.Error(), "refused by preflight") {
		t.Fatalf("preflight handoff: %v", err)
	}
}
