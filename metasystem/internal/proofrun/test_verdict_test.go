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

func TestFailedGroupReasonNamesTheSkippedTestWhenNoTestFailed(t *testing.T) {
	result := runVerdictFixture(t, "skipped-verdict", "skipped")
	if result.Status != "failed" || result.NativeExitStatus == nil || *result.NativeExitStatus != 0 || !result.CollectionComplete {
		t.Fatalf("skipped verdict = status %q, exit %v, complete %v; want failed, 0, true", result.Status, result.NativeExitStatus, result.CollectionComplete)
	}
	if !strings.Contains(result.NotRunReason, "no test failed") || !strings.Contains(result.NotRunReason, "fixture.skipped-verdict skipped") {
		t.Fatalf("skipped verdict reason = %q; want the absence of a failed test and the skipped test name", result.NotRunReason)
	}
}

func TestRedGroupLogCarriesTheVerdictLineAndDigest(t *testing.T) {
	failed := runVerdictFixture(t, "red-log", "skipped")
	logBytes, err := os.ReadFile(failed.LogPath)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSuffix(string(logBytes), "\n"), "\n")
	last := lines[len(lines)-1]
	if !strings.HasPrefix(last, "TEST-VERDICT red-log status=failed") || !strings.Contains(last, "fixture.red-log skipped") {
		t.Fatalf("red group log last line = %q; want failed verdict naming the skipped test", last)
	}
	wantDigest := fmt.Sprintf("%x", sha256.Sum256(logBytes))
	if failed.LogDigest != wantDigest {
		t.Fatalf("red group log digest = %q, want %q", failed.LogDigest, wantDigest)
	}
	passed := runVerdictFixture(t, "passed-log", "passed")
	passedLog, err := os.ReadFile(passed.LogPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(passedLog), "TEST-VERDICT") {
		t.Fatalf("passed group log contains a verdict line:\n%s", passedLog)
	}
}

func runVerdictFixture(t *testing.T, name, status string) GroupResult {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "scripts"), 0o700); err != nil {
		t.Fatal(err)
	}
	scriptPath := filepath.Join(root, "scripts", name+".sh")
	reportDir := "reports-" + name
	reportPath := filepath.ToSlash(filepath.Join(reportDir, "tests.xml"))
	testcase := fmt.Sprintf(`<testcase classname="fixture" name="%s"></testcase>`, name)
	if status == "skipped" {
		testcase = fmt.Sprintf(`<testcase classname="fixture" name="%s"><skipped message="not here"/></testcase>`, name)
	}
	report := fmt.Sprintf(`<testsuite tests="1">%s</testsuite>`, testcase)
	nativeOutput := ""
	if status == "skipped" {
		nativeOutput = "printf 'unterminated native output'\n"
	}
	if err := testexec.WriteFile(scriptPath, []byte(fmt.Sprintf("#!/usr/bin/env bash\nset -eu\nmkdir -p %s\nprintf '%%s\\n' '%s' > %s\n%s", reportDir, report, reportPath, nativeOutput)), 0o700); err != nil {
		t.Fatal(err)
	}
	runTestResultGit(t, root, "init", "-q")
	runTestResultGit(t, root, "add", "scripts")
	runTestResultGit(t, root, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "-q", "-m", "fixture")
	candidateTree := strings.TrimSpace(runTestResultGit(t, root, "rev-parse", "HEAD^{tree}"))
	group := testpolicy.Group{
		ID:            name,
		Kind:          "standard",
		CWD:           ".",
		Adapter:       "command",
		Format:        "junit-xml",
		Argv:          []string{"bash", filepath.ToSlash(filepath.Join("scripts", name+".sh"))},
		Reports:       []string{reportDir},
		Outputs:       []string{reportDir},
		Inputs:        []string{filepath.ToSlash(filepath.Join("scripts", name+".sh"))},
		ExpectedTests: []testpolicy.ExpectedTest{{Report: reportPath, Classname: "fixture", Name: name}},
	}
	return runTestGroup(context.Background(), TestRunRequest{
		ProjectRoot:   root,
		CandidateTree: candidateTree,
		LogRoot:       filepath.Join(root, "logs"),
	}, group)
}
