package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
)

// flakyCheck fails one named test in the whole check; alone it passes unless the marker exists.
func flakyCheck(marker string) string {
	red := "printf 'LANDING-FAILED\\texample.com/flaky\\tTestFlaky\\nLANDING-CHECKED\\t1\\n'; exit 1\n"
	return "#!/bin/sh\nif test -n \"$LANDING_ONLY\" && ! test -f '" + marker + "'; then printf 'LANDING-CHECKED\\t0\\n'; exit 0; fi\n" + red
}

func carryExecutions(t *testing.T, records string) (checks, alone, aloneResults []string) {
	t.Helper()
	checks, _ = filepath.Glob(filepath.Join(records, "check-*", "result.json"))
	alone, _ = filepath.Glob(filepath.Join(records, "alone", "of-check-*", "alone-*"))
	aloneResults, _ = filepath.Glob(filepath.Join(records, "alone", "of-check-*", "alone-*", "result.json"))
	return checks, alone, aloneResults
}

func TestCarryFlakePublicPassesAndRepeatReuses(t *testing.T) {
	t.Parallel()
	b, owners, args, _ := carryCheckBed(t, flakyCheck)
	records := filepath.Join(b.root(), "artifacts", "unit-checks", "carry", args[3])
	code, stdout, stderr := b.run(owners, args...)
	if code != 0 || !strings.Contains(stderr, "warning: checks passed after a flake rerun: TestFlaky (example.com/flaky)") {
		t.Fatalf("flake refused or unannounced: %d stdout=%q stderr=%q", code, stdout, stderr)
	}
	checks, alone, results := carryExecutions(t, records)
	if len(checks) != 1 || len(alone) != 1 || len(results) != 1 || filepath.Base(filepath.Dir(alone[0])) != "of-"+filepath.Base(filepath.Dir(checks[0])) {
		t.Fatalf("executions: checks=%v alone=%v", checks, alone)
	}
	data, err := os.ReadFile(filepath.Join(records, "gate.json"))
	var gate branch.GateObservation
	var evidence struct{ Flake *branch.CheckFlake }
	if err != nil || json.Unmarshal(data, &gate) != nil || json.Unmarshal([]byte(gate.Evidence), &evidence) != nil || evidence.Flake == nil {
		t.Fatalf("gate carries no flake: %v %s", err, data)
	}
	flake := evidence.Flake
	if gate.RunID != filepath.Base(filepath.Dir(checks[0])) || flake.Cause != "flake" || strings.Join(flake.Flakes, ",") != "example.com/flaky TestFlaky" || flake.Alone != filepath.Base(alone[0]) || flake.AloneExit.Exit != 0 || !strings.Contains(strings.Join(flake.Selectors, " "), "LANDING_ONLY_TESTS=TestFlaky") || !strings.Contains(flake.Red, "TestFlaky") {
		t.Fatalf("flake record: %+v gate=%+v", flake, gate)
	}
	code, _, stderr = b.run(owners, args...)
	again, aloneAgain, _ := carryExecutions(t, records)
	if code != 0 || strings.Contains(stderr, "warning:") || len(again) != 1 || len(aloneAgain) != 1 {
		t.Fatalf("repeat did not reuse: %d stderr=%q checks=%v alone=%v", code, stderr, again, aloneAgain)
	}
	// An interrupted rerun is passed over and a new one runs.
	if err := os.Remove(results[0]); err != nil {
		t.Fatal(err)
	}
	code, _, stderr = b.run(owners, args...)
	again, aloneAgain, results = carryExecutions(t, records)
	if code != 0 || !strings.Contains(stderr, "warning:") || len(again) != 1 || len(aloneAgain) != 2 || len(results) != 1 {
		t.Fatalf("interrupted rerun: %d stderr=%q checks=%v alone=%v", code, stderr, again, aloneAgain)
	}
}

func TestCarryFlakePublicRedStaysRed(t *testing.T) {
	t.Parallel()
	b, owners, args, marker := carryCheckBed(t, flakyCheck)
	if err := os.WriteFile(marker, []byte("fail"), 0600); err != nil {
		t.Fatal(err)
	}
	records := filepath.Join(b.root(), "artifacts", "unit-checks", "carry", args[3])
	code, first := b.runJSON(owners, args...)
	if code != 1 || strings.Contains(first.Summary, "retained") {
		t.Fatalf("red alone rerun accepted: %d %+v", code, first)
	}
	code, result := b.runJSON(owners, args...)
	if code != 1 || !strings.Contains(result.Summary, "the retained subject check failed") || !strings.Contains(result.Summary, "--rerun") || !strings.Contains(result.Summary, args[len(args)-1]) {
		t.Fatalf("retained red lost its remedy: %d %+v", code, result)
	}
	checks, alone, _ := carryExecutions(t, records)
	if _, err := os.Stat(filepath.Join(records, "gate.json")); !os.IsNotExist(err) || len(checks) != 1 || len(alone) != 1 {
		t.Fatalf("red stays red: gate=%v checks=%v alone=%v", err, checks, alone)
	}
}

func TestCarryUnnamedRedStartsNoAloneRerun(t *testing.T) {
	t.Parallel()
	b, owners, args, marker := carryRecoveryBed(t)
	if err := os.WriteFile(marker, []byte("fail"), 0600); err != nil {
		t.Fatal(err)
	}
	b.runJSON(owners, args...)
	code, result := b.runJSON(owners, args...)
	_, err := os.Stat(filepath.Join(b.root(), "artifacts", "unit-checks", "carry", args[3], "alone"))
	if code != 1 || !strings.Contains(result.Summary, "the retained subject check failed") || !os.IsNotExist(err) {
		t.Fatalf("unnamed red: %d %+v alone=%v", code, result, err)
	}
}
