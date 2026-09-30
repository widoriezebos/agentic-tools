package main

import (
	"bytes"
	"slices"
	"strconv"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/landpath"
)

// landingRun is one run of the landing path as work land shows it ("Messages
// a Person Reads"): the stop's two lines by default, and the step log with
// everything else behind them only with --verbose.
type landingRun struct {
	stop    landpath.Stop
	details bytes.Buffer // the step log and each refusal's background
	told    bytes.Buffer // what the landing path wrote for the person
}

// output is everything the run wrote, for --json's output field.
func (run *landingRun) output() string {
	return run.details.String() + run.told.String()
}

// detailLines are the run's details, one per line.
func (run *landingRun) detailLines() []string {
	return nonEmptyLines(run.details.String())
}

// extraLines are the lines the landing path wrote for the person besides its
// stop's two (a staged set it could not put back, say): printed by default.
func (run *landingRun) extraLines() []string {
	var extra []string
	for _, line := range nonEmptyLines(run.told.String()) {
		if line == run.stop.Reason || strings.HasPrefix(line, "run: ") || strings.HasPrefix(line, "needed first: ") {
			continue
		}
		extra = append(extra, line)
	}
	return extra
}

// stopped is the result of a landing that stopped: line 1 its reason, line 2
// its command or what is needed first, the log in details. A stop with no
// reason (an owner that exited without saying why) names its exit status.
func (run *landingRun) stopped(result intentResult, status int, retry []string) intentResult {
	result.Summary = run.stop.Reason
	if result.Summary == "" {
		result.Summary = "the landing stopped (exit " + strconv.Itoa(status) + ") without saying why"
		result.next, result.nextReason = append(retry, "--verbose"), "shows the landing's log"
	}
	switch {
	case len(run.stop.Run) > 0:
		result.next, result.nextReason = run.stop.Run, run.stop.Then
	case run.stop.Then != "":
		result.Decision = run.stop.Then
	}
	result.text = append(result.text, run.extraLines()...)
	result.Details = append(result.Details, run.detailLines()...)
	return result
}

// withoutArgs is argv less the positional arguments named.
func withoutArgs(argv, args []string) []string {
	var kept []string
	for _, word := range argv {
		if !slices.Contains(args, word) {
			kept = append(kept, word)
		}
	}
	return kept
}
