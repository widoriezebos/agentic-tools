package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/enginecause"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testrun"
)

// A refusal whose remedy is the pinned engine's own internal command names,
// on its second line, the public command a person runs to see why; the
// internal command is a detail --verbose prints. Beside --json the two
// lines stay plain.
func TestTestingRefusalNamesAPublicCommandForThePinnedEnginesFailure(t *testing.T) {
	t.Parallel()
	internal := enginecause.Command("/state/engines/metasystem", "test", "plan", "--root", "/repo/metasystem", "--tree", "abc", "--json", "--policy-child")
	for _, token := range []string{"child-failed", "child-output"} {
		refused := enginecause.RefuseWith(token, []enginecause.Fact{enginecause.Path("engine", "/state/engines/metasystem"),
			enginecause.Value("command", internal)}, "the pinned engine failed while choosing which tests to run", "exit status 2")
		for _, verbose := range []bool{false, true} {
			var stderr bytes.Buffer
			printTestingRefusal(&stderr, refused, testrun.SelectionRequest{GoalID: "my-goal", Verbose: verbose})
			lines := strings.Split(strings.TrimSuffix(stderr.String(), "\n"), "\n")
			if len(lines) < 2 || lines[0] != "✗ the pinned engine failed while choosing which tests to run" ||
				lines[1] != "  → metasystem test plan --verbose --goal my-goal" {
				t.Fatalf("%s verbose=%t printed:\n%s", token, verbose, stderr.String())
			}
			if shown := strings.Contains(stderr.String(), "--policy-child"); shown != verbose {
				t.Fatalf("%s verbose=%t: the internal command shown=%t:\n%s", token, verbose, shown, stderr.String())
			}
			var plain bytes.Buffer
			printTestingRefusalAs(&plain, refused, testrun.SelectionRequest{GoalID: "my-goal"}, true)
			if plain.String() != "the pinned engine failed while choosing which tests to run\nrun: metasystem test plan --verbose --goal my-goal\n" {
				t.Fatalf("%s beside --json printed:\n%s", token, plain.String())
			}
		}
	}
	// An engine that refused in its own words keeps its own remedy.
	worded := enginecause.RefuseWith("child-failed", []enginecause.Fact{enginecause.Value("command", internal)},
		"this checkout's engine is not enrolled\nrun: metasystem system start", "")
	var own bytes.Buffer
	printTestingRefusal(&own, worded, testrun.SelectionRequest{})
	if own.String() != "✗ this checkout's engine is not enrolled\n  → metasystem system start\n" {
		t.Fatalf("an engine's own refusal printed:\n%s", own.String())
	}
	// Any other refusal keeps its own remedy.
	other := enginecause.RefuseWith("mutation-lock", nil, "a proof mutation is under way", "")
	var stderr bytes.Buffer
	printTestingRefusal(&stderr, other, testrun.SelectionRequest{})
	reason, remedy, _ := strings.Cut(other.Error(), "\nrun: ")
	if stderr.String() != "✗ "+reason+"\n  → "+remedy+"\n" {
		t.Fatalf("another refusal printed:\n%s", stderr.String())
	}
}
