package missionrunner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The launch spine's guard ladder and armAndPreflight's refusal branches,
// driven with a stub arming neighbor (Phase 6). The stub stands in for the
// checkout engine's `up` entry only — arming itself has its own fixtures; the
// unit here is the ORCHESTRATION: sequence, refusal wording, and the handoff
// into contract preflight.

func stubArming(engine *Engine, stdout, stderr string, code int) {
	engine.ArmSupervision = func([]string) (string, string, int) { return stdout, stderr, code }
}

func TestLaunchGuardLadder(t *testing.T) {
	engine := &Engine{Root: t.TempDir(), Mission: "mr-launch"}
	os.MkdirAll(engine.missionDir(), 0o755)
	statePath := filepath.Join(engine.missionDir(), "state.json")

	// Resume with no state at all.
	if err := engine.launch("resume", false); err == nil ||
		!strings.Contains(err.Error(), "has started here") {
		t.Fatalf("resume without state: %v", err)
	}
	// Start over an existing state steers to resume.
	os.WriteFile(statePath, []byte(`{}`), 0o644)
	if err := engine.launch("start", false); err == nil ||
		!strings.Contains(err.Error(), "already exists; metasystem mission resume mr-launch continues it") {
		t.Fatalf("start over state: %v", err)
	}
	// Resume over a malformed state surfaces the verifier's refusal.
	os.WriteFile(statePath, []byte(`{broken`), 0o644)
	if err := engine.launch("resume", false); err == nil {
		t.Fatal("resume verified a malformed state")
	}
}

func TestArmAndPreflightRefusals(t *testing.T) {
	// No checkout engine at all: the arm step refuses by name.
	bare := &Engine{Root: t.TempDir(), Mission: "mr-arm-a"}
	if err := bare.armAndPreflight("start"); err == nil ||
		!strings.Contains(err.Error(), "supervision did not arm") {
		t.Fatalf("armless root: %v", err)
	}

	// An arming that fails: same named refusal, its stderr carried.
	failing := &Engine{Root: t.TempDir(), Mission: "mr-arm-b"}
	stubArming(failing, "", "deliberate refusal\n", 1)
	if err := failing.armAndPreflight("start"); err == nil ||
		!strings.Contains(err.Error(), "supervision did not arm") ||
		!strings.Contains(err.Error(), "deliberate refusal") {
		t.Fatalf("failing armer: %v", err)
	}

	// An armer that reports the typed armed outcome hands off to contract preflight, which
	// refuses the absent contract by name.
	armed := &Engine{Root: t.TempDir(), Mission: "mr-arm-c"}
	stubArming(armed, "up outcome=armed authority=writer\n", "", 0)
	if err := armed.armAndPreflight("start"); err == nil ||
		!strings.Contains(err.Error(), "refused by preflight") {
		t.Fatalf("preflight handoff: %v", err)
	}
}

// EM-13, EM-24: a start names a mission without a contract before anything
// is armed, and a resume names a mission that never started, with the
// public command that starts it.
func TestLaunchNamesAMissingMission(t *testing.T) {
	engine := &Engine{Root: t.TempDir(), Mission: "mr-none"}
	armed := 0
	engine.ArmSupervision = func([]string) (string, string, int) { armed++; return "up outcome=armed", "", 0 }
	err := engine.launch("start", false)
	if err == nil || !strings.Contains(err.Error(), "no mission contract mr-none") || armed != 0 {
		t.Fatalf("start without a contract: %v (armed %d)", err, armed)
	}
	err = engine.launch("resume", false)
	if err == nil || !strings.Contains(err.Error(), "no mission mr-none has started here") || !strings.Contains(err.Error(), "metasystem mission start mr-none") {
		t.Fatalf("resume without state: %v", err)
	}
}
