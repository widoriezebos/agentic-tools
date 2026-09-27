package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// The coverage delta's cache-miss proof binds the ratchet and every package
// into the proof identity and relaunches the delta as the proof's worker,
// authenticated by the same engine, exactly as the retired script did.
func TestCoverageDeltaLaunchArgumentsRelaunchAnAuthenticatedWorker(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ratchet := filepath.Join(root, "ratchet.json")
	if err := os.WriteFile(ratchet, []byte(`{"floors":{"internal/x":1}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	engine := filepath.Join(root, "bin", "metasystem")
	args, err := coverageDeltaLaunchArguments(root, engine, ratchet, "run-1", []string{"internal/x", "internal/y"})
	if err != nil {
		t.Fatal(err)
	}
	separator := slices.Index(args, "--")
	if separator < 0 {
		t.Fatalf("no suite command: %q", args)
	}
	launch, command := args[:separator], args[separator+1:]
	wantCommand := []string{"env", "METASYSTEM_COVERAGE_DELTA_RELAUNCHED=1", "METASYSTEM_PROOF_AUTH_BIN=" + engine,
		engine, "internal", "proof-run", "coverage-delta", "--root", root, "--ratchet", ratchet, "--", "internal/x", "internal/y"}
	if !slices.Equal(command, wantCommand) {
		t.Fatalf("suite command = %q\nwant %q", command, wantCommand)
	}
	joined := strings.Join(launch, " ")
	for _, want := range []string{
		"--suite coverage-delta", "--scope coverage", "--command-class coverage-delta",
		"--identity-input coverage-package=internal/x", "--identity-input coverage-package=internal/y",
		"--identity-input coverage-ratchet=", "coverage-delta-run-1.progress.jsonl", "suite-cost suite=coverage-delta",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("launch arguments lack %q: %q", want, launch)
		}
	}
	if _, err := coverageDeltaLaunchArguments(root, engine, filepath.Join(root, "absent.json"), "run-2", []string{"internal/x"}); err == nil {
		t.Fatal("an unreadable ratchet was bound")
	}
}
