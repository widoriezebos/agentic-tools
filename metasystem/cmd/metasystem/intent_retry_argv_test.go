package main

import (
	"slices"
	"strings"
	"testing"
)

// The retry a refusal names is the command as typed, less the option it
// refused: a value option loses its value, a switch only itself, and a
// target missing from the typed command goes right after the command words.
func TestTypedArgvLessDropsValueOptionsAndSwitches(t *testing.T) {
	t.Parallel()
	command := intentCommand{name: "goal claim", flags: []intentFlag{
		{name: "label", value: "LABEL", repeat: true},
		{name: "arc"},
		{name: "reason", value: "TEXT"},
	}}
	inv := &intentInvocation{command: command, raw: []string{"g1", "--label", "x", "--arc", "--reason=why", "--label=y", "--json"}}
	if got, want := inv.typedArgvLess("label", "arc", "reason"), []string{"metasystem", "goal", "claim", "g1", "--json"}; !slices.Equal(got, want) {
		t.Fatalf("typedArgvLess = %q, want %q", got, want)
	}
	if got, want := inv.typedArgvLess("arc"), []string{"metasystem", "goal", "claim", "g1", "--label", "x", "--reason=why", "--label=y", "--json"}; !slices.Equal(got, want) {
		t.Fatalf("typedArgvLess(arc) = %q, want %q", got, want)
	}
	bare := &intentInvocation{command: command, raw: []string{"--arc"}}
	if got, want := bare.typedArgvFor("GOAL"), []string{"metasystem", "goal", "claim", "GOAL", "--arc"}; !slices.Equal(got, want) {
		t.Fatalf("typedArgvFor = %q, want %q", got, want)
	}
	if got, want := bare.typedArgvWith("--reason", "TEXT"), []string{"metasystem", "goal", "claim", "--arc", "--reason", "TEXT"}; !slices.Equal(got, want) {
		t.Fatalf("typedArgvWith = %q, want %q", got, want)
	}
}

// resultWords is a result's line 1 and its details: where a refusal's code
// and the owner's own account are read since codes left the default text.
func resultWords(result intentResult) string {
	return result.Summary + "\n" + strings.Join(result.Details, "\n")
}
