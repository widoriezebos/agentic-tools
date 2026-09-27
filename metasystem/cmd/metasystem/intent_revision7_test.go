package main

import (
	"bytes"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// Revision 7 (design section 3.4) folds mechanism actions into the intents
// that need them. These tests pin each fold's route to the owner the removed
// action ran; the owners themselves are tested where they live.

func sameHandler(left, right func([]string) int) bool {
	return reflect.ValueOf(left).Pointer() == reflect.ValueOf(right).Pointer()
}

// TestIntentSessionHandoffSplit: session handoff reaches the handoff owner,
// the context-status owner for --status and the verify owner for
// --verify NONCE, each with exactly the words it took before.
func TestIntentSessionHandoffSplit(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		args    []string
		handler func([]string) int
		rest    []string
	}{
		{[]string{"--root", "/i", "--note", "n.md"}, runContextHandoff, []string{"--root", "/i", "--note", "n.md"}},
		{[]string{"--root", "/i", "--cancel", "3f2a"}, runContextHandoff, []string{"--root", "/i", "--cancel", "3f2a"}},
		{[]string{"--status", "--root", "/i", "--json"}, runContextStatus, []string{"--root", "/i", "--json"}},
		{[]string{"--root=/i", "--status"}, runContextStatus, []string{"--root=/i"}},
		{[]string{"--root", "/i", "--verify", "3f2a"}, runContextVerify, []string{"--root", "/i", "--nonce", "3f2a"}},
		{[]string{"--verify=3f2a", "--root", "/i"}, runContextVerify, []string{"--root", "/i", "--nonce", "3f2a"}},
	} {
		handler, rest := sessionHandoffRoute(row.args)
		if !sameHandler(handler, row.handler) || !slices.Equal(rest, row.rest) {
			t.Errorf("session handoff %q reaches the wrong owner or hands it %q, want %q", row.args, rest, row.rest)
		}
	}
}

// TestIntentTestStatusAndReceiptRoutes: test status reads retained proof,
// or with --result a recorded result's cost; receipt add appends, or with
// --corrects EPOCH:SHA1 appends a correction of that line.
func TestIntentTestStatusAndReceiptRoutes(t *testing.T) {
	t.Parallel()
	if !sameHandler(testStatusRoute([]string{"--tree", "t", "--root", "."}), runTestVerify) {
		t.Error("test status --tree does not reach the retained-proof verifier")
	}
	if !sameHandler(testStatusRoute([]string{"--result", "r.json", "--expensive-ms", "5"}), runTestReport) {
		t.Error("test status --result does not reach the result report")
	}
	words, problem := receiptAddWords([]string{"--type", "implementation", "--outcome", "done"})
	if problem != "" || !slices.Equal(words, []string{"add", "--type", "implementation", "--outcome", "done"}) {
		t.Errorf("receipt add = %q %q", words, problem)
	}
	words, problem = receiptAddWords([]string{"--corrects", "1788441779:abc", "--field", "outcome", "--now", "rework", "--reason", "r"})
	if problem != "" || !slices.Equal(words, []string{"correct", "--ref-epoch", "1788441779", "--ref-sha1", "abc", "--field", "outcome", "--now", "rework", "--reason", "r"}) {
		t.Errorf("receipt add --corrects = %q %q", words, problem)
	}
	if _, problem := receiptAddWords([]string{"--corrects", "12"}); !strings.Contains(problem, "EPOCH:SHA1") {
		t.Errorf("a malformed --corrects = %q", problem)
	}
}

// TestIntentWaitExitCodeRoutes: work wait --exit-code blocks in the job
// waiter for j2:J (or a bare id) and in the tracked-run waiter for --run ID,
// under --repo or the calling directory, and returns the waiter's pinned
// exit code unchanged. Other wait options and other reference kinds are
// refused before any waiter runs.
func TestIntentWaitExitCodeRoutes(t *testing.T) {
	t.Parallel()
	var calls [][]string
	owners := intentOwners{work: intentWorkOwners{
		jobWatch: func(args []string) int { calls = append(calls, append([]string{"job"}, args...)); return 7 },
		runWatch: func(args []string) int { calls = append(calls, append([]string{"run"}, args...)); return 5 },
	}}
	command, _ := findIntentAction("work", "wait")
	run := func(args ...string) (int, string) {
		var stdout, stderr bytes.Buffer
		code := runIntentIn(command, args, &stdout, &stderr, "/caller", owners)
		return code, stdout.String() + stderr.String()
	}
	for _, row := range []struct {
		args []string
		code int
		want []string
	}{
		{[]string{"j2:impl-01", "--exit-code", "--repo", "/c", "--caller-pid", "7"}, 7, []string{"job", "--root", "/c", "--job", "impl-01", "--caller-pid", "7"}},
		{[]string{"impl-01", "--exit-code"}, 7, []string{"job", "--root", "/caller", "--job", "impl-01"}},
		{[]string{"--run", "r1", "--exit-code", "--repo", "rel"}, 5, []string{"run", "--root", "/caller/rel", "--id", "r1"}},
	} {
		calls = nil
		if code, output := run(row.args...); code != row.code || len(calls) != 1 || !slices.Equal(calls[0], row.want) {
			t.Errorf("work wait %q = %d %q, waiter calls %q, want %q", row.args, code, output, calls, row.want)
		}
	}
	for _, args := range [][]string{
		{"run:r1", "--exit-code"},
		{"j2:x", "--exit-code", "--timeout", "1m"},
		{"j2:x", "--exit-code", "--run", "r1"},
		{"--exit-code"},
		{"j2:x", "--caller-pid", "7"},
		{"--run", "r1"},
		{"--list", "--exit-code"},
		{"j2:x", "--session", "s"},
	} {
		calls = nil
		if code, output := run(args...); code != 2 || len(calls) != 0 || !strings.Contains(output, "nothing was done") {
			t.Errorf("work wait %q = %d %q, waiter calls %q; want a refusal before any wait", args, code, output, calls)
		}
	}
}

// TestIntentReviewCheckOnlyRefusesReviewOptions: --check-only asks no critic,
// so a review's own options are refused, and the check's options are refused
// without it.
func TestIntentReviewCheckOnlyRefusesReviewOptions(t *testing.T) {
	t.Parallel()
	command, _ := findIntentAction("work", "review")
	design, _ := findIntentAction("design", "review")
	for _, row := range []struct {
		command intentCommand
		args    []string
	}{
		{command, []string{"j2:x", "--check-only", "--stage", "review", "--tool-calls", "4"}},
		{command, []string{"g", "--check-only", "--changes"}},
		{command, []string{"--check-only"}},
		{command, []string{"j2:x", "--check-only"}},
		{command, []string{"run:r", "--check-only", "--stage", "review"}},
		{command, []string{"j2:x", "--check-only", "--stage", "review", "--findings", "f.json", "--dispositions", "d.md"}},
		{command, []string{"j2:x", "--stage", "review"}},
		{command, []string{"--findings", "f.json", "--dispositions", "d.md"}},
		{design, []string{"page.md", "--check-only", "--dispositions", "d.md"}},
		{design, []string{"page.md", "--check-only", "--tool-calls", "4"}},
	} {
		var stdout, stderr bytes.Buffer
		if code := runIntentIn(row.command, row.args, &stdout, &stderr, t.TempDir(), intentOwners{}); code != 2 || !strings.Contains(stdout.String()+stderr.String(), "nothing was done") {
			t.Errorf("%s %q = %d %q %q; want a refusal", row.command.name, row.args, code, stdout.String(), stderr.String())
		}
	}
}
