package main

import (
	"bytes"
	"io"
	"slices"
	"testing"
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
