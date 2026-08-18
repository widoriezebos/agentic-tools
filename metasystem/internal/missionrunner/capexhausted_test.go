package missionrunner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Issue #6 finding 3: the named cap outcome is certified — a provider
// result carrying is_error + error_max_turns names the cutoff; a crash
// without it stays generic; absence stays generic.
func TestProviderErrorSubtype(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) string {
		path := filepath.Join(dir, name)
		os.WriteFile(path, []byte(body), 0o644)
		return path
	}
	capped := write("capped.json", `{"is_error":true,"subtype":"error_max_turns","num_turns":51}`)
	if got := providerErrorSubtype(capped); got != "error_max_turns" {
		t.Fatalf("capped subtype = %q", got)
	}
	crash := write("crash.json", `{"is_error":true,"subtype":"error_during_execution"}`)
	if got := providerErrorSubtype(crash); got != "error_during_execution" {
		t.Fatalf("crash subtype = %q", got)
	}
	fine := write("fine.json", `{"is_error":false,"subtype":"success"}`)
	if got := providerErrorSubtype(fine); got != "" {
		t.Fatalf("non-error subtype leaked: %q", got)
	}
	if got := providerErrorSubtype(filepath.Join(dir, "absent.json")); got != "" {
		t.Fatalf("absent file subtype = %q", got)
	}
}

// The cap detail rides the failure into the LEDGER's classification line,
// not just turn.json (acceptance trace (b) certified as bytes).
func TestCapExhaustedReachesLedger(t *testing.T) {
	engine, statePath, ledgerPath, _ := crashedMission(t, 0, 1)
	engine.anchorFn = func(string, string, string) error { return nil }
	runnersDir := filepath.Join(engine.Root, "artifacts", "agents", "missions", "runners")
	os.MkdirAll(runnersDir, 0o755)
	record := engine.runnerRecord(os.Getpid(), os.Getpid(), 1, "fixture")
	writeJSONFile(t, filepath.Join(runnersDir, "demo.json"), record)

	openFixtureTurn(t, engine.Root, statePath, "demo-t1-cap1", 1)
	state, err := engine.verifyState(statePath, false)
	if err != nil {
		t.Fatal(err)
	}
	turnDir := filepath.Join(engine.missionDir(), "turns", "demo-t1-cap1")
	os.MkdirAll(turnDir, 0o755)
	turnPath := filepath.Join(turnDir, "turn.json")
	writeJSONFile(t, turnPath, map[string]any{
		"missionId": "demo", "turnId": "demo-t1-cap1", "cycle": 1,
		"runtime": "fake", "model": "fake-model", "status": "running", "outcome": nil, "error": nil, "detail": nil,
		"startedAt": "2026-08-18T00:00:00Z", "endedAt": nil,
	})
	detail := "host-cap-exhausted: the adapter's native turn cap ended the turn (error_max_turns)"
	if _, err := engine.recordFailedTurn(statePath, ledgerPath, state, turnPath, detail, "failed", 1); err != nil {
		t.Fatalf("recordFailedTurn: %v", err)
	}
	ledger, err := os.ReadFile(ledgerPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(ledger), "host-cap-exhausted") {
		t.Fatalf("ledger does not name the cap:\n%s", ledger)
	}
}
