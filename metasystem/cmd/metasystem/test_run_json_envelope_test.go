package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os/exec"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/strictjson"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/verbresult"
)

// TestProofRunJSONPrintsOneEnvelope: internal test run --json, the landing
// owner's child, answers on stdout with exactly one envelope on every exit,
// driven here against a real one-group suite (structured-output U1). The
// run's banner and the suite's output go to stderr, and a run that starts
// no child (a retry required) writes its PROOF-RESULT only to the result
// file, never stdout (R2). Its outcome follows its disposition (R3): a pass
// is confirmed, a sufficient reuse unchanged, a red run failed and a retry
// required refused with its code.
func TestProofRunJSONPrintsOneEnvelope(t *testing.T) {
	fixture := newPortableProofFixture(t)
	run := func(tree string, extra ...string) (verbresult.Result, string) {
		t.Helper()
		args := append([]string{"internal", "test", "run", "--root", fixture.root, "--goal", "portable", "--tree", tree,
			"--mode", "auto", "--purpose", "delivery", "--json"}, extra...)
		command := fixture.proofCommand.command(fixture.commandEnvironment(), fixture.engine, args...)
		command.Dir = fixture.root
		var stdout, stderr bytes.Buffer
		command.Stdout, command.Stderr = &stdout, &stderr
		err := command.Run()
		var exit *exec.ExitError
		if err != nil && !errors.As(err, &exit) {
			t.Fatal(err)
		}
		result, readErr := verbresult.Read(stdout.Bytes(), "internal test run", command.ProcessState.ExitCode(), stderr.String())
		if readErr != nil {
			t.Fatalf("stdout is not exactly one envelope: %v\nstdout=%q", readErr, stdout.String())
		}
		for _, word := range []string{"PROOF-RESULT", "TESTING-CONTRACT", "TEST-RESULT"} {
			if strings.Contains(stdout.String(), word) {
				t.Fatalf("stdout carries the run's own text %q: %q", word, stdout.String())
			}
		}
		return result, stderr.String()
	}
	base := fixture.git("rev-parse", "HEAD^{tree}")
	passed, _ := run(base)
	var reported proofrun.TestResult
	if passed.Outcome != verbresult.Confirmed || passed.Exit != 0 || json.Unmarshal(passed.Data, &reported) != nil || reported.AttemptID == "" {
		t.Fatalf("the one-group pass = %+v data=%s", passed, passed.Data)
	}
	reused, _ := run(base)
	if reused.Outcome != verbresult.Unchanged || reused.Exit != proofrun.ExitReusableSuccess {
		t.Fatalf("the unchanged rerun = %+v", reused)
	}
	fixture.write("app/a.txt", "red a\n", 0o644)
	red := fixture.commit("make A red")
	failed, _ := run(red)
	if failed.Outcome != verbresult.Failed || failed.Exit == 0 {
		t.Fatalf("the red run = %+v", failed)
	}
	resultPath := fixture.root + "/retry-result.json"
	retry, stderr := run(red, "--result", resultPath)
	var decision proofrun.LaunchResult
	if retry.Outcome != verbresult.Refused || retry.Exit != proofrun.ExitRetryRequired || retry.Code != "TEST_RETRY_REQUIRED" ||
		json.Unmarshal(retry.Data, &decision) != nil || decision.Disposition != proofrun.DispositionRetryRequired {
		t.Fatalf("the retry without a decision = %+v data=%s stderr=%s", retry, retry.Data, stderr)
	}
	var written proofrun.LaunchResult
	if err := strictjson.Read(resultPath, &written); err != nil || written.Disposition != proofrun.DispositionRetryRequired {
		t.Fatalf("the no-child result file = %+v err=%v", written, err)
	}
}
