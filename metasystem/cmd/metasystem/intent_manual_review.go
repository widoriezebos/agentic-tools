package main

import (
	"cmp"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
)

// reviewGoalWords are the words of a goal's review.
func reviewGoalWords(id string) []string {
	return []string{"work", "review", id}
}

// workVerbWords are the public words of a goal-directed act named by its
// verb: status is the top-level form, every other act belongs to work.
func workVerbWords(verb string) []string {
	if verb == "status" {
		return []string{"status"}
	}
	return []string{"work", verb}
}

const diagnosticReviewNote = "diagnostic feedback only: it is not a goal review, approves nothing and cannot be landed"

// runIntentReviewDiagnostic asks independent readers for feedback on the
// checkout's current changes (patch empty) or on a supplied patch, through
// the launch owner's standalone read. The request is frozen by the owner
// before any read starts, so repeating it rejoins the same reads.
func runIntentReviewDiagnostic(inv *intentInvocation, patch string) int {
	for _, other := range []string{"work", "dispositions", "after", "finding", "test", "model", "tool-calls", "commit"} {
		if inv.input.has(other) {
			return inv.render(intentResult{Outcome: intentRefused, code: 2,
				Summary: fmt.Sprintf("feedback on changes takes --brief, --goal and --retry, not --%s; nothing was done", other),
				next:    withoutOption(inv.typedArgv(), other),
				Details: []string{"feedback on changes is work review --changes, or --patch without a goal"}})
		}
	}
	if !inv.input.has("brief") {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "feedback on changes needs a brief saying what the readers should examine; nothing was done",
			next: append(inv.typedArgv(), "--brief", "FILE"), nextReason: "FILE holds what to examine"})
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
		subject = inv.input.text("patch")
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
	targets := []intentTarget{{Kind: "read", ID: readRefPrefix + result.Ref}}
	if err != nil {
		message, code := err.Error(), launch.ErrorCode(err)
		out := intentResult{Outcome: intentRefused, code: 2, Targets: targets, Summary: message, Data: map[string]any{"cause": code}, Details: []string{launch.ErrorDetail(err)},
			next: inv.sameCommand(), nextReason: "once the request is corrected"}
		switch code {
		case "READ_BUSY":
			out.Outcome, out.code = intentInProgress, 124
			out.next, out.nextReason = inv.sameCommand(), "tries again once the running read has ended"
		case "READ_RETRY_RUNNING":
			ref := ""
			if _, rest, found := strings.Cut(launch.ErrorDetail(err), "ref="); found {
				ref, _, _ = strings.Cut(rest, " ")
			}
			out.code = 1
			if ref != "" {
				out.Targets = []intentTarget{{Kind: "read", ID: readRefPrefix + ref}}
				out.next, out.nextReason = inv.publicArgv("work", "stop", readRefPrefix+ref), "a retry follows only an attempt that ended; this stops the running one"
			}
		case "READ_RETRY_UNKNOWN", "READ_REQUEST_INVALID", "READ_PATCH_UNREADABLE", "READ_BRIEF_MISSING", "READ_INPUT_MISSING", "READ_CHECKOUT_UNAVAILABLE":
		default:
			out.Outcome, out.code = intentFailed, 1
			out.next, out.nextReason = inv.sameCommand(), "tries again"
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
		out.Summary = fmt.Sprintf("read %s: %d read launch(es) could not be proved stopped", readRefPrefix+result.Ref, len(result.Uncertain))
		out.next, out.nextReason = inv.publicArgv("work", "stop", readRefPrefix+result.Ref), "repeat the stop once the launches can be proved stopped"
	case result.Stopping:
		out.Outcome, out.code = intentInProgress, 124
		out.Summary = fmt.Sprintf("read %s: attempt %d is stopping; its launches are proved stopped and no further read starts", readRefPrefix+result.Ref, result.Attempt.Number)
		out.next, out.nextReason = inv.publicArgv("work", "wait", readRefPrefix+result.Ref, "--timeout", "1m"), "records the stopped attempt and offers its retry"
	case result.Attempt.State == "running":
		out.Outcome, out.code = intentInProgress, 124
		out.Summary = fmt.Sprintf("read %s: attempt %d is reading %d file(s)", readRefPrefix+result.Ref, result.Attempt.Number, len(result.Request.Files))
		out.next, out.nextReason = inv.publicArgv("work", "wait", readRefPrefix+result.Ref, "--timeout", "10m"), "continue the reads"
	case result.Complete:
		out.Outcome = intentConfirmed
		out.Summary = fmt.Sprintf("read %s: every read finished and counted; the feedback is in the reports", readRefPrefix+result.Ref)
	default:
		out.Outcome, out.code = intentPartial, 1
		out.Summary = fmt.Sprintf("read %s: attempt %d ended %s without complete feedback", readRefPrefix+result.Ref, result.Attempt.Number, cmp.Or(result.Attempt.Outcome, result.Attempt.State))
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
		words := []string{"work", "review", "--changes", "--brief", result.Request.Brief}
		if result.Request.Kind == "patch" {
			words = []string{"work", "review", "--patch", result.Request.Diff, "--brief", result.Request.Brief}
		}
		if result.Request.Goal != "" {
			words = append(words, "--goal", result.Request.Goal)
		}
		return inv.publicArgv(words...)
	}
	words := []string{"work", "review", "--changes"}
	if subject != "" && subject != "changes" {
		words = []string{"work", "review", "--patch", subject}
	}
	words = append(words, "--brief", inv.input.text("brief"))
	if result.Request.Goal != "" {
		words = append(words, "--goal", result.Request.Goal)
	}
	return inv.publicArgv(words...)
}
