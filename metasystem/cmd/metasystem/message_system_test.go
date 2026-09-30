package main

import (
	"bytes"
	"strings"
	"testing"
)

// Group 6 of "Messages a Person Reads": a failure whose cause may pass names
// the typed command again as line 2 (retry), and a refusal whose fix is a
// changed option names the command with that option changed (retryWith).

func systemMessageInvocation(t *testing.T, name string, raw ...string) (*intentInvocation, *bytes.Buffer) {
	t.Helper()
	command, ok := findIntentCommand(name)
	if !ok {
		t.Fatalf("no command %q", name)
	}
	input, problem := parseIntentArgs(command, raw)
	if problem != nil {
		t.Fatalf("parse %v: %+v", raw, problem)
	}
	var out bytes.Buffer
	return &intentInvocation{command: command, raw: raw, input: input, stdout: &out, stderr: &out}, &out
}

// resultLine2 is what a result prints as line 2: its command and reason,
// or its decision.
func resultLine2(result intentResult) string {
	if result.Next != nil {
		return shellCommand(result.Next.Argv) + "  (" + result.Next.Reason + ")"
	}
	return result.Decision
}

func TestMessageRetryIsTheTypedCommand(t *testing.T) {
	t.Parallel()
	inv, out := systemMessageInvocation(t, "app status", "--goal", "g1")
	inv.render(intentResult{Outcome: intentFailed, code: 1, Summary: "the application's run record could not be read", retry: "try again",
		Details: []string{"run record: permission denied"}})
	want := "✗ the application's run record could not be read\n" +
		"  → metasystem app status --goal g1  try again\n"
	if out.String() != want {
		t.Fatalf("retry render:\n got  %q\n want %q", out.String(), want)
	}
	verbose, verboseOut := systemMessageInvocation(t, "app status", "--goal", "g1", "--verbose")
	verbose.render(intentResult{Outcome: intentFailed, code: 1, Summary: "x", retry: "try again", Details: []string{"run record: permission denied"}})
	if !strings.Contains(verboseOut.String(), "permission denied") {
		t.Fatalf("--verbose hides the detail: %q", verboseOut.String())
	}
	// A Decision the owner gave wins over the retry.
	decided, decidedOut := systemMessageInvocation(t, "app status")
	decided.render(intentResult{Outcome: intentRefused, code: 1, Summary: "x", retry: "try again", Decision: "metasystem system check"})
	if strings.Contains(decidedOut.String(), "  → metasystem app status") {
		t.Fatalf("retry overrode a decision: %q", decidedOut.String())
	}
}

func TestMessageRetryWithChangesOnlyTheNamedOptions(t *testing.T) {
	t.Parallel()
	inv, _ := systemMessageInvocation(t, "app log", "--lines", "zero", "--follow", "--goal", "g1")
	got := strings.Join(inv.retryWith([]string{"lines"}, "--lines", "40"), " ")
	if got != "metasystem app log --follow --goal g1 --lines 40" {
		t.Fatalf("retryWith valued option = %q", got)
	}
	got = strings.Join(inv.retryWith([]string{"follow"}), " ")
	if got != "metasystem app log --lines zero --goal g1" {
		t.Fatalf("retryWith switch = %q", got)
	}
}
