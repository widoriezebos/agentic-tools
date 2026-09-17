package proofrun

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

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
	writeTestResultFile(t, filepath.Join(root, "scripts", "agents", "validate-section-selector.sh"), []byte(script.String()), 0o755)
	runTestResultGit(t, root, "init", "-q")
	runTestResultGit(t, root, "add", "scripts")
	runTestResultGit(t, root, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "fixture")
	tree := runTestResultGit(t, root, "rev-parse", "HEAD^{tree}")
	group := testpolicy.Group{ID: "script-verdict", Kind: "integration", Adapter: "section", CWD: ".",
		Inputs: []string{"scripts/**"}, Platforms: []string{"any"}, TargetMS: 1000, Section: "fixture"}
	return runTestGroup(context.Background(), TestRunRequest{ProjectRoot: root, CandidateTree: tree,
		LogRoot: filepath.Join(root, "logs")}, group)
}
