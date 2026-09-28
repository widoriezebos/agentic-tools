package missionrunner

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/stopfence"
)

// TestSupBRunLoopUnderTheClosedFenceReportsTheTwoStoppedLines ports the
// stop-fence mission run-loop row of supervision-fixtures part B: the loop
// refuses a completed stop with the stopped sentence and the agent-free start
// remedy, hands that refusal to its start signal, and takes no lease.
func TestSupBRunLoopUnderTheClosedFenceReportsTheTwoStoppedLines(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := stopfence.Write(root, stopfence.Record{State: stopfence.StateClosed, Phase: stopfence.PhaseStopped,
		Generation: 6, ChangedAt: "2026-09-27T12:30:00Z", Checkout: root, NotStopped: []stopfence.Survivor{},
		By: stopfence.Actor{Verb: "stop", Process: stopfence.Process{Pid: 72, PidStartedAt: 70}}}); err != nil {
		t.Fatal(err)
	}
	signalPath := filepath.Join(root, "stop-fence-loop.signal")
	if code := NewEngine(root, "stop-fence-loop").RunLoopAtGeneration("start", "stop-fence-loop", signalPath, 6, false); code != 3 {
		t.Fatalf("closed-fence run-loop exit = %d, want 3", code)
	}
	data, err := os.ReadFile(signalPath)
	if err != nil {
		t.Fatal(err)
	}
	var signal struct {
		Verified bool   `json:"verified"`
		Error    string `json:"error"`
	}
	if err := json.Unmarshal(data, &signal); err != nil {
		t.Fatal(err)
	}
	want := "the metasystem is stopped for " + root + " since 2026-09-27T12:30:00Z, by stop pid 72\n" +
		"at an agent-free terminal, run: metasystem system start --repo " + root
	if signal.Verified || signal.Error != want {
		t.Fatalf("closed-fence start signal = %+v, want error %q", signal, want)
	}
	if _, err := os.Stat(filepath.Join(root, "artifacts", "agents", "missions", "stop-fence-loop")); !os.IsNotExist(err) {
		t.Fatalf("the refused run-loop created mission state: %v", err)
	}
}
