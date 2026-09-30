package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"slices"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
)

// test run, test plan and test status take --verbose (group 4 of Messages a
// Person Reads): a refusal prints its two lines and --verbose adds the code,
// cause and facts. The help documents the option, and the public test run
// hands it to the runner, which prints the detail.
func TestMessageTestVerbsDocumentAndPassVerbose(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"test run", "test plan", "test status"} {
		command, ok := findIntentCommand(name)
		if !ok {
			t.Fatalf("%s is not a command", name)
		}
		if !slices.ContainsFunc(command.flags, func(flag intentFlag) bool { return flag.name == "verbose" && !flag.hidden }) {
			t.Errorf("help %s does not document --verbose", name)
		}
	}
	bed := newWorkBed(t)
	owners := bed.workOwners()
	var passed []string
	owners.work.testRun = func(_ string, argv []string, _ io.Writer) ([]byte, int, error) {
		passed = argv
		return []byte(`{"outcome":"red"}`), 1, nil
	}
	var stdout, stderr bytes.Buffer
	runIntentIn(mustIntentArgvCommand(t, []string{"test", "run"}), []string{"--verbose", "--json"}, &stdout, &stderr, bed.root(), owners)
	if !slices.Contains(passed, "--verbose") {
		t.Fatalf("test run --verbose did not reach the runner: %v", passed)
	}
	passed = nil
	runIntentIn(mustIntentArgvCommand(t, []string{"test", "run"}), []string{"--json"}, &stdout, &stderr, bed.root(), owners)
	if slices.Contains(passed, "--verbose") {
		t.Fatalf("test run without --verbose passed it: %v", passed)
	}
}

// A build refused because another session holds the goal keeps the goal
// branch's refusal code as a detail, for --verbose and --json.
func TestMessageBuildHolderRefusalKeepsItsCode(t *testing.T) {
	t.Parallel()
	bed := newWorkBed(t)
	brief := bed.brief("brief.md", "Build the unit.\n")
	owners := bed.workOwners()
	owners.connection.claimCheck = func(string, string, goal.Endpoint) func() error {
		return func() error { return errors.New("goal standing-validation is held by another session") }
	}
	command, rest, ok := resolveIntentArgv(append([]string{"work", "build", bed.id, "u1", "--brief", brief}, workCheck...))
	if !ok {
		t.Fatal("no work build")
	}
	var stdout, stderr bytes.Buffer
	code := runIntentIn(command, append([]string{"--json"}, rest...), &stdout, &stderr, bed.root(), owners)
	var result intentResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("%v %q %q", err, stdout.String(), stderr.String())
	}
	if code == 0 || !strings.Contains(result.Summary, "held by another session") || strings.Contains(result.Summary, branch.NotHolderCode) ||
		!slices.Contains(result.Details, "refusal code: "+branch.NotHolderCode) {
		t.Fatalf("build at a goal another session holds = %d %+v", code, result)
	}
}
