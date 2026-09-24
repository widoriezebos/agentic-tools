package missionrunner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// The cycle steps' guard paths, driven directly through the cycle context
// (Phase 3b): each asserts the step's verdict shape — an error ends the
// cycle as a runner defect, done=true ends it with the mission's state.

func TestCycleReserveRefusesBrokenFences(t *testing.T) {
	root := t.TempDir()
	engine := &Engine{Root: root, Mission: "mr-steps"}
	// Reservation must fail (no mission dirs), which parks on the fence —
	// and parking needs a readable state file to propose against.
	stateDir := filepath.Join(root, "artifacts", "agents", "missions", "mr-steps")
	os.MkdirAll(stateDir, 0o755)
	statePath := filepath.Join(stateDir, "state.json")
	os.WriteFile(statePath, []byte(`{broken`), 0o644)
	lockPath := filepath.Join(stateDir, "mission-fence.lock")
	clockSampled := false
	engine.Now = func() time.Time {
		file, openErr := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o644)
		if openErr != nil {
			t.Fatalf("open mission lock during runner clock sample: %v", openErr)
		}
		lockErr := unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB)
		if lockErr == nil {
			_ = unix.Flock(int(file.Fd()), unix.LOCK_UN)
			_ = file.Close()
			t.Fatal("mission runner sampled its clock before the mission owner acquired the lock")
		}
		_ = file.Close()
		if lockErr != unix.EWOULDBLOCK && lockErr != unix.EAGAIN {
			t.Fatalf("inspect mission lock during runner clock sample: %v", lockErr)
		}
		clockSampled = true
		return time.Date(2026, time.September, 21, 12, 0, 0, 0, time.UTC)
	}
	c := &cycleContext{statePath: statePath, ledger: filepath.Join(stateDir, "ledger.md"), state: map[string]any{}}
	final, done, err := engine.cycleReserveAndBuildTurn(c)
	if !clockSampled {
		t.Fatal("mission runner reservation did not sample its clock")
	}
	if !done {
		t.Fatalf("a failed reservation must end the cycle: final=%v err=%v", final, err)
	}
	// The park itself then fails on the unreadable state — a runner defect,
	// surfaced as an error rather than a silent continue.
	if err == nil {
		t.Fatalf("an unreadable state during the park must error, got final=%v", final)
	}
}

func TestCycleStepsSequenceStopsOnDone(t *testing.T) {
	// The dispatcher's contract: a step reporting done short-circuits the
	// rest. Proven with the real oneCycle entry against an engine whose
	// first step must fail-and-park (same broken fixture as above), so no
	// later step ever runs.
	root := t.TempDir()
	engine := &Engine{Root: root, Mission: "mr-seq"}
	stateDir := filepath.Join(root, "artifacts", "agents", "missions", "mr-seq")
	os.MkdirAll(stateDir, 0o755)
	statePath := filepath.Join(stateDir, "state.json")
	os.WriteFile(statePath, []byte(`{broken`), 0o644)
	_, err := engine.oneCycle(statePath, filepath.Join(stateDir, "ledger.md"), map[string]any{}, "", "", new(bool))
	if err == nil {
		t.Fatal("the sequence ran past a failing first step")
	}
}

// The gate-prompt step: an unassemblable prompt fails the turn before
// launch, patching the turn record and ending the cycle.
func TestCycleGatePromptRefusal(t *testing.T) {
	root := t.TempDir()
	engine := &Engine{Root: root, Mission: "mr-gate"}
	turnDir := filepath.Join(root, "turns", "t1")
	os.MkdirAll(turnDir, 0o755)
	turnPath := filepath.Join(turnDir, "turn.json")
	os.WriteFile(turnPath, []byte("{\"status\":\"pending\"}\n"), 0o644)
	stateDir := filepath.Join(root, "artifacts", "agents", "missions", "mr-gate")
	os.MkdirAll(stateDir, 0o755)
	statePath := filepath.Join(stateDir, "state.json")
	os.WriteFile(statePath, []byte(`{broken`), 0o644)
	c := &cycleContext{
		statePath: statePath, ledger: filepath.Join(stateDir, "ledger.md"),
		state: map[string]any{}, turnID: "t1", turnDir: turnDir, turnPath: turnPath,
	}
	final, done, err := engine.cycleGatePrompt(c)
	if !done {
		t.Fatalf("an unassemblable prompt must end the cycle: %v %v", final, err)
	}
	// The turn record carries the failure regardless of how the park fared.
	data, _ := os.ReadFile(turnPath)
	if !strings.Contains(string(data), "prompt-refused") && err == nil {
		t.Fatalf("neither a patched turn nor an error: %s", data)
	}
}
