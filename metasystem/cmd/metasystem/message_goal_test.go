package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// "Messages a Person Reads" for the goal family: a goal refusal's code is
// data. The default text says what happened in plain words and never the
// code; --json keeps the code in the result's details.
func TestMessageGoalRefusalCodeIsADetail(t *testing.T) {
	t.Parallel()
	bed := newGoalCLIBed(t, goalCLISeed{})
	gcliBudgetAnnounceParent(t, bed)
	gcliBudgetMust(t, bed, "goal", "release", "ship-widget", "--reason", "free the seat")
	gcliBudgetOpen(t, bed, "norm-parent", gcliBudgetTierThree, "Split it first.", "--tier", "3")
	gcliBudgetHumanMust(t, bed, "goal", "approve", "norm-parent", "--budget", "norm")
	gcliBudgetMust(t, bed, "goal", "claim", "norm-parent")
	over := append([]string{"goal", "budget", "norm-parent", "--elapsed-limit", "1d", "--attempt-limit", "2",
		"--reserved-job-minutes-limit", "1441", "--active-job-limit", "1", "--review-round-limit", "3"}, gcliBudgetHuman...)

	code, stdout, stderr := bed.public(over...)
	text := stdout + stderr
	if code == 0 || strings.Contains(text, "GOAL_NORM_REFUSED") || !strings.Contains(text, "goal norm-parent asks for 1441m") {
		t.Fatalf("the default refusal must say why in plain words, without its code: code=%d %q", code, text)
	}

	code, stdout, _ = bed.public(append(over, "--json")...)
	var result intentResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil || code == 0 {
		t.Fatalf("--json refusal: code=%d %v %q", code, err, stdout)
	}
	if strings.Contains(result.Summary, "GOAL_NORM_REFUSED") || !strings.Contains(strings.Join(result.Details, "\n"), "refusal code: GOAL_NORM_REFUSED") {
		t.Fatalf("--json must keep the code in details, not the summary: %+v", result)
	}
}

// A refusal whose fix is a command names that command with the values the
// engine knows: the goal is filled in, and only the unknowable permission is
// a placeholder.
func TestMessageGoalAllowWithoutAPermissionNamesTheCommand(t *testing.T) {
	t.Parallel()
	bed := newGoalCLIBed(t, goalCLISeed{})
	code, _, stderr := bed.public("goal", "allow", "ship-widget")
	if code == 0 || !strings.Contains(stderr, "no permission was named, so nothing was done") ||
		!strings.Contains(stderr, "run: metasystem goal allow ship-widget PERMISSION") || strings.Contains(stderr, "needed first") {
		t.Fatalf("a missing permission must name the command: code=%d %q", code, stderr)
	}
}
