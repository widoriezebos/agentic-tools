package main

import (
	"cmp"
	"fmt"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
)

// reviewSubjectWords are the words review reads as a subject kind rather
// than a goal id; review goal G reaches every goal, these included.
var reviewSubjectWords = []string{"design", "job", "run", "commit", "changes", "diff", "goal"}

// reviewGoalWords is review's goal form: review G, or review goal G when the
// goal's id is one of review's subject words.
func reviewGoalWords(id string) []string {
	if slices.Contains(reviewSubjectWords, id) {
		return []string{"review", "goal", id}
	}
	return []string{"review", id}
}

const diagnosticReviewNote = "diagnostic feedback only: it is not a goal review, approves nothing and cannot be landed"

// runIntentReviewDiagnostic asks independent readers for feedback on the
// checkout's current changes (patch empty) or on a supplied patch, through
// the launch owner's standalone read. The request is frozen by the owner
// before any read starts, so repeating it rejoins the same reads.
func runIntentReviewDiagnostic(inv *intentInvocation, patch string) int {
	for _, other := range []string{"work", "dispositions", "after", "finding", "test", "model", "tool-calls"} {
		if inv.input.has(other) {
			return inv.render(intentResult{Outcome: intentRefused, code: 2,
				Summary: fmt.Sprintf("review changes and review diff take --brief, --goal and --retry, not --%s; nothing was done", other)})
		}
	}
	if !inv.input.has("brief") {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "diagnostic review needs --brief FILE: what the readers should examine; nothing was done"})
	}
	retry := 0
	if inv.input.has("retry") {
		retry, _ = strconv.Atoi(inv.input.text("retry"))
	}
	if patch != "" && !filepath.IsAbs(patch) {
		patch = filepath.Join(inv.cwd, patch)
	}
	brief := inv.input.text("brief")
	if !filepath.IsAbs(brief) {
		brief = filepath.Join(inv.cwd, brief)
	}
	if problem := inv.selectRoot(); problem != nil {
		return inv.render(*problem)
	}
	runner := inv.unitRunner()
	result, err := runner.StartRead(launch.ReadRequest{Directory: inv.cwd, Patch: patch, Brief: brief, Goal: inv.input.text("goal"), Retry: retry})
	subject := "changes"
	if patch != "" {
		subject = "diff"
	}
	return inv.render(inv.diagnosticReadResult(result, err, subject))
}

// runIntentReviewRef shows, waits for or stops a diagnostic review by the
// public ref its result gave.
func runIntentReviewRef(inv *intentInvocation, verb, ref string) int {
	if problem := inv.selectRoot(); problem != nil {
		return inv.render(*problem)
	}
	runner := inv.unitRunner()
	var result launch.ReadResult
	var err error
	switch verb {
	case "show":
		result, err = runner.InspectRead(ref)
	case "wait":
		timeout, problem := inv.waitTimeout()
		if problem != nil {
			return inv.render(*problem)
		}
		result, err = runner.AdvanceRead(ref, timeout)
	case "stop":
		result, err = runner.StopRead(ref)
	}
	return inv.render(inv.diagnosticReadResult(result, err, ""))
}

func (inv *intentInvocation) diagnosticReadResult(result launch.ReadResult, err error, subject string) intentResult {
	targets := []intentTarget{{Kind: "review", ID: result.Ref}}
	if err != nil {
		message := err.Error()
		code, _, _ := strings.Cut(message, ":")
		code, _, _ = strings.Cut(code, " ")
		out := intentResult{Outcome: intentRefused, code: 2, Targets: targets, Summary: message, Data: map[string]any{"cause": code}}
		switch code {
		case "READ_BUSY":
			out.Outcome, out.code = intentInProgress, 124
		case "READ_RETRY_RUNNING":
			ref := ""
			if _, rest, found := strings.Cut(message, "ref="); found {
				ref, _, _ = strings.Cut(rest, " ")
			}
			out.code = 1
			if ref != "" {
				out.Targets = []intentTarget{{Kind: "review", ID: ref}}
				out.next, out.nextReason = inv.publicArgv("stop", "review", ref), "a retry follows only an attempt that ended; this stops the running one"
			}
		case "READ_RETRY_UNKNOWN", "READ_REQUEST_INVALID", "READ_PATCH_UNREADABLE", "READ_BRIEF_MISSING", "READ_INPUT_MISSING", "READ_CHECKOUT_UNAVAILABLE":
		default:
			out.Outcome, out.code = intentFailed, 1
		}
		return out
	}
	if result.Empty {
		return intentResult{Outcome: intentUnchanged, Summary: "there are no changes against " + shortCommit(result.Request.Base) + " to review; no read was started",
			Data: map[string]any{"base": result.Request.Base, "topLevel": result.Request.TopLevel}}
	}
	reports := make([]map[string]any, 0, len(result.Reports))
	var text []string
	for _, report := range result.Reports {
		reports = append(reports, map[string]any{"step": report.Step, "state": report.State, "verdict": report.Verdict, "counts": report.Counts, "path": report.Path, "retained": report.Retained})
		line := fmt.Sprintf("%s: %s", report.Step, report.State)
		if report.Verdict != "" {
			line += " (" + report.Verdict + ")"
		}
		if report.Retained {
			line += " " + report.Path
		}
		text = append(text, line)
	}
	data := map[string]any{"ref": result.Ref, "kind": result.Request.Kind, "base": result.Request.Base, "diffSha256": result.Request.DiffSHA256,
		"files": result.Request.Files, "attempt": result.Attempt.Number, "state": result.Attempt.State, "outcome": result.Attempt.Outcome,
		"complete": result.Complete, "reports": reports, "note": diagnosticReviewNote}
	if result.Request.Goal != "" {
		data["goal"] = result.Request.Goal
	}
	if result.Attempt.Limitation != "" {
		data["limitation"] = result.Attempt.Limitation
	}
	if len(result.Uncertain) > 0 {
		data["uncertain"] = result.Uncertain
	}
	out := intentResult{Targets: targets, Data: data, text: append([]string{diagnosticReviewNote}, text...)}
	again := inv.diagnosticRepeat(result, subject)
	switch {
	case len(result.Uncertain) > 0:
		out.Outcome, out.code = intentPartial, 1
		out.Summary = fmt.Sprintf("review %s: %d read launch(es) could not be proved stopped", result.Ref, len(result.Uncertain))
		out.next, out.nextReason = inv.publicArgv("stop", "review", result.Ref), "repeat the stop once the launches can be proved stopped"
	case result.Stopping:
		out.Outcome, out.code = intentInProgress, 124
		out.Summary = fmt.Sprintf("review %s: attempt %d is stopping; its launches are proved stopped and no further read starts", result.Ref, result.Attempt.Number)
		out.next, out.nextReason = inv.publicArgv("wait", "review", result.Ref, "--timeout", "1m"), "records the stopped attempt and offers its retry"
	case result.Attempt.State == "running":
		out.Outcome, out.code = intentInProgress, 124
		out.Summary = fmt.Sprintf("review %s: attempt %d is reading %d file(s)", result.Ref, result.Attempt.Number, len(result.Request.Files))
		out.next, out.nextReason = inv.publicArgv("wait", "review", result.Ref, "--timeout", "10m"), "continue the reads"
	case result.Complete:
		out.Outcome = intentConfirmed
		out.Summary = fmt.Sprintf("review %s: every read finished and counted; the feedback is in the reports", result.Ref)
	default:
		out.Outcome, out.code = intentPartial, 1
		out.Summary = fmt.Sprintf("review %s: attempt %d ended %s without complete feedback", result.Ref, result.Attempt.Number, cmp.Or(result.Attempt.Outcome, result.Attempt.State))
		if again != nil {
			out.next, out.nextReason = append(again, "--retry", strconv.Itoa(result.Attempt.Number)), "one new attempt of the same frozen request"
		}
	}
	return out
}

// diagnosticRepeat is the command that repeats this request, when the
// caller's own subject is known.
func (inv *intentInvocation) diagnosticRepeat(result launch.ReadResult, subject string) []string {
	if subject == "" {
		// Known only by its ref: the frozen request's own brief and patch
		// have the same identity, so they reach the same request.
		words := []string{"review", "changes", "--brief", result.Request.Brief}
		if result.Request.Kind == "patch" {
			words = []string{"review", "diff", result.Request.Diff, "--brief", result.Request.Brief}
		}
		if result.Request.Goal != "" {
			words = append(words, "--goal", result.Request.Goal)
		}
		return inv.publicArgv(words...)
	}
	words := []string{"review", "changes"}
	if subject == "diff" {
		words = []string{"review", "diff", inv.input.args[1]}
	}
	words = append(words, "--brief", inv.input.text("brief"))
	if result.Request.Goal != "" {
		words = append(words, "--goal", result.Request.Goal)
	}
	return inv.publicArgv(words...)
}
