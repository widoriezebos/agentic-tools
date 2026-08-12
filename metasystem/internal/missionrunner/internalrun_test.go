package missionrunner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// internalRun's spine to the refusal, in-process (Phase 6): the runner
// record lands, the lease is taken, the missing mission state refuses, and
// the fail ramp settles everything — signal written, lease freed, runner
// record finalized as failed.
func TestInternalRunFailRamp(t *testing.T) {
	engine := &Engine{Root: t.TempDir(), Mission: "mr-ramp"}
	os.MkdirAll(engine.missionDir(), 0o755)
	signal := filepath.Join(t.TempDir(), "start.signal")

	code := engine.internalRun("resume", "mr-ramp-tag", signal)
	if code == 0 {
		t.Fatal("a stateless resume reported success")
	}
	// The launcher's signal carries the refusal.
	data, err := os.ReadFile(signal)
	if err != nil {
		t.Fatalf("no start signal written: %v", err)
	}
	if !strings.Contains(string(data), "false") {
		t.Fatalf("the signal does not carry the failure: %s", data)
	}
	// The lease was released on the ramp.
	if pathExists(filepath.Join(engine.missionDir(), "lease.d")) {
		t.Fatal("the fail ramp leaked the lease marker")
	}
	// The runner record is finalized as failed.
	recordPath, _, _ := engine.runnerPaths()
	record, err := readJSONDoc(recordPath)
	if err != nil {
		t.Fatalf("runner record unreadable: %v", err)
	}
	if record["status"] != "failed" {
		t.Fatalf("runner record not finalized: %v", record["status"])
	}
}
