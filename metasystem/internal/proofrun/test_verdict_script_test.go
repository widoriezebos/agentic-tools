package proofrun

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

func TestFailedScriptGroupReasonNamesScenarioAndVerdict(t *testing.T) {
	result := runScriptVerdictFixture(t, []string{"alpha"}, true, 1, 1)
	want := "failed scenarios: alpha (process exit 1)"
	if result.Status != "failed" || result.NotRunReason != want {
		t.Fatalf("script result status=%q reason=%q, want failed and %q", result.Status, result.NotRunReason, want)
	}
	logBytes, err := os.ReadFile(result.LogPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(string(logBytes), "TEST-VERDICT script-verdict status=failed exit=1 reason="+want+"\n") {
		t.Fatalf("script log does not end in its scenario verdict:\n%s", logBytes)
	}
}

func TestFailedScriptGroupReasonCapsScenarioNames(t *testing.T) {
	names := []string{"one", "two", "three", "four", "five", "six", "seven"}
	result := runScriptVerdictFixture(t, names, true, 1, 1)
	want := "failed scenarios: one, two, three, four, five and 2 more (process exit 1)"
	if result.NotRunReason != want {
		t.Fatalf("script reason=%q, want %q", result.NotRunReason, want)
	}
}

func TestFailedScriptGroupWithoutMarkerNeverHasEmptyReason(t *testing.T) {
	result := runScriptVerdictFixture(t, nil, false, 1, 1)
	want := "process exit 1 with no failed-scenarios block in the log"
	if result.Status != "failed" || result.NotRunReason == "" || result.NotRunReason != want {
		t.Fatalf("script result status=%q reason=%q, want failed and %q", result.Status, result.NotRunReason, want)
	}
}

func TestFailedScriptGroupKeepsPresetReason(t *testing.T) {
	result := runScriptVerdictFixture(t, []string{"alpha"}, true, 2, 1)
	want := "section result exit 2 disagrees with native process exit 1"
	if result.Status != "invalid" || result.NotRunReason != want {
		t.Fatalf("script result status=%q reason=%q, want invalid and %q", result.Status, result.NotRunReason, want)
	}
}

func TestScriptFailureReasonReadsOnlyLast256KiB(t *testing.T) {
	path := filepath.Join(t.TempDir(), "script.log")
	marker := "=== bed failed scenarios ===\n- hidden (rc=1)\n=== end bed failed scenarios ===\n"
	if err := os.WriteFile(path, []byte(marker+strings.Repeat("x", 256*1024)), 0o600); err != nil {
		t.Fatal(err)
	}
	want := "process exit 1 with no failed-scenarios block in the log"
	if got := scriptFailureReason(path, 1); got != want {
		t.Fatalf("reason from marker outside bounded tail=%q, want %q", got, want)
	}
}

func runScriptVerdictFixture(t *testing.T, names []string, markers bool, reportedExit, processExit int) GroupResult {
	t.Helper()
	root := t.TempDir()
	var script strings.Builder
	fmt.Fprintf(&script, "#!/usr/bin/env bash\nset -eu\nprintf 'section\\tfixture\\tfail\\t%d\\tfixture failure\\n' >\"$METASYSTEM_ENUMERATION_STAGE_RESULTS_OUT\"\n", reportedExit)
	if markers {
		script.WriteString("printf '%s\\n' '=== bed failed scenarios ==='\n")
		for _, name := range names {
			fmt.Fprintf(&script, "printf '%%s\\n' '- %s (rc=1)' '  output tail:' '    failed'\n", name)
		}
		script.WriteString("printf '%s\\n' '=== end bed failed scenarios ==='\n")
	} else {
		script.WriteString("printf '%s\\n' 'fixture failed without a summary'\n")
	}
	fmt.Fprintf(&script, "exit %d\n", processExit)
	tree := verdictFixtureTree(t, "script")
	snapshot := newTestSnapshotFactory(t, root, tree, map[string]testSnapshotEntry{
		"scripts/agents/validate-section-selector.sh": testSnapshotFile(script.String(), 0o755),
	}, 1)
	group := testpolicy.Group{ID: "script-verdict", Kind: "integration", Adapter: "section", CWD: ".",
		Inputs: []string{"scripts/**"}, Platforms: []string{"any"}, TargetMS: 1000, Section: "fixture"}
	return runTestGroup(context.Background(), TestRunRequest{ProjectRoot: root, CandidateTree: tree, openCandidate: snapshot.open,
		LogRoot: filepath.Join(root, "logs")}, group)
}

func verdictFixtureTree(t *testing.T, specimen string) string {
	t.Helper()
	digest := sha256.Sum256([]byte(t.Name() + "/" + specimen))
	return fmt.Sprintf("%x", digest[:20])
}

// A section group that names its own script bed runs that argv with the
// candidate engine installed at cwd/bin/metasystem, never consults the
// section selector (the snapshot has none), receives the proof custody
// environment without the selector's stage-results channel, and is judged by
// its exit status alone.
func TestSectionArgvBedRunsWithCandidateEngineAndJudgesExit(t *testing.T) {
	for _, test := range []struct {
		name       string
		exit       int
		wantStatus string
	}{
		{name: "green bed", exit: 0, wantStatus: "passed"},
		{name: "red bed", exit: 3, wantStatus: "failed"},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			engineBytes := "#!/bin/sh\necho candidate-engine\n"
			engine := filepath.Join(t.TempDir(), "metasystem")
			if err := testexec.WriteFile(engine, []byte(engineBytes), 0o755); err != nil {
				t.Fatal(err)
			}
			script := fmt.Sprintf(`#!/usr/bin/env bash
set -eu
[[ "$(bin/metasystem)" == candidate-engine ]] || { echo "candidate engine missing" >&2; exit 90; }
[[ -z "${METASYSTEM_ENUMERATION_STAGE_RESULTS_OUT:-}" ]] || { echo "selector stage-results channel leaked" >&2; exit 91; }
[[ -z "${METASYSTEM_ENUMERATION_ENGINE_DEPENDENCY:-}" ]] || { echo "selector engine dependency leaked" >&2; exit 92; }
[[ -n "${METASYSTEM_PROOF_AUTH_BIN:-}" && "${METASYSTEM_SUITE_PROGRESS_ACTIVE:-}" == 1 ]] || { echo "proof custody missing" >&2; exit 93; }
echo "bed ran"
exit %d
`, test.exit)
			tree := verdictFixtureTree(t, "argv-bed")
			snapshot := newTestSnapshotFactory(t, root, tree, map[string]testSnapshotEntry{
				"scripts/bed.sh": testSnapshotFile(script, 0o755),
			}, 1)
			group := testpolicy.Group{ID: "section/bed", Kind: "integration", Adapter: "section", CWD: ".",
				Inputs: []string{"scripts/**"}, Platforms: []string{"any"}, TargetMS: 1000, Section: "bed",
				Argv: []string{"bash", "scripts/bed.sh"}}
			result := runTestGroup(context.Background(), TestRunRequest{ProjectRoot: root, CandidateTree: tree, openCandidate: snapshot.open,
				LogRoot: filepath.Join(root, "logs"), CandidateEngine: engine, CandidateEngineDigest: digestBytes([]byte(engineBytes))}, group)
			logBytes, _ := os.ReadFile(result.LogPath)
			if result.Status != test.wantStatus || result.NativeExitStatus == nil || *result.NativeExitStatus != test.exit {
				t.Fatalf("argv bed status=%q exit=%v reason=%q, want %s with exit %d\n%s", result.Status, result.NativeExitStatus, result.NotRunReason, test.wantStatus, test.exit, logBytes)
			}
			if !strings.Contains(string(logBytes), "bed ran") {
				t.Fatalf("argv bed did not run its own script:\n%s", logBytes)
			}
			if got := strings.Join(result.Argv, " "); got != "bash scripts/bed.sh" {
				t.Fatalf("argv bed recorded argv %q", got)
			}
		})
	}
}
