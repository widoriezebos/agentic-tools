package main

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	critiqueModel "github.com/widoriezebos/agentic-tools/metasystem/internal/critique"
	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/validate"
)

// A goal's work review ends in the author's decision about the examination's
// findings. The decisions file carries a machine-written binding to the goal,
// the work, the reviewed attempt, the committed subject and the exact return
// it answers, so a file written for one examination cannot decide another.
// An examination with no findings is decided by the structurally empty join,
// and the whole close owner then runs before the read is collected.

// reviewWorkContext is the work item review G is examining; the unit review
// fills the attempt and the subject's retain once it bound them.
type reviewWorkContext struct {
	goal, work string
	repair     bool
	retry      int64
	attempt    int
	retain     func(launch.UnitSubject) error
	subject    *launch.UnitSubject
}

// reviewBinding names one examination's return exactly.
type reviewBinding struct {
	Goal, Work  string
	Attempt     int
	Subject     string
	Examination string
	Round       int64
	Return      string
}

var reviewBindingLine = regexp.MustCompile(`(?m)^Review binding: goal=(\S+) work=(\S+) attempt=(\d+) subject=([0-9a-f]+) examination=(\S+) round=(\d+) return=([0-9a-f]{64})\s*$`)

func (b reviewBinding) line() string {
	return fmt.Sprintf("Review binding: goal=%s work=%s attempt=%d subject=%s examination=%s round=%d return=%s",
		b.Goal, b.Work, b.Attempt, b.Subject, b.Examination, b.Round, b.Return)
}

// readReviewBinding finds the one binding line of a decisions file.
func readReviewBinding(data []byte) (reviewBinding, error) {
	matches := reviewBindingLine.FindAllSubmatch(data, -1)
	if len(matches) != 1 {
		return reviewBinding{}, fmt.Errorf("the decisions file has %d review binding lines, not one; start from the template review G writes", len(matches))
	}
	match := matches[0]
	attempt, _ := strconv.Atoi(string(match[3]))
	round, _ := strconv.ParseInt(string(match[6]), 10, 64)
	return reviewBinding{Goal: string(match[1]), Work: string(match[2]), Attempt: attempt, Subject: string(match[4]),
		Examination: string(match[5]), Round: round, Return: string(match[7])}, nil
}

// decisionsDocument is the bound decisions file: the binding, the findings
// as the author reads them, and the dispositions table the close join reads.
func decisionsDocument(binding reviewBinding, findings []intentFinding) string {
	var text strings.Builder
	text.WriteString("# Review decisions\n\n")
	text.WriteString(binding.line() + "\n\n")
	text.WriteString("Written by metasystem review; keep the binding line unchanged. Decide every finding: accepted (a fix is\n")
	text.WriteString("required, sent with revise), refuted (with evidence), out-of-scope (citing the brief's declared scope) or\n")
	text.WriteString("noted (not material).\n\n")
	header := validate.DispositionsHeader()
	text.WriteString("| " + strings.Join(header, " | ") + " |\n")
	text.WriteString("|" + strings.Repeat(" --- |", len(header)) + "\n")
	for _, finding := range findings {
		text.WriteString(fmt.Sprintf("| %s | DECIDE | | |\n", finding.ID))
	}
	return text.String()
}

func reviewReturnDigest(path string) (string, []byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", nil, err
	}
	return fmt.Sprintf("%x", sha256.Sum256(data)), data, nil
}

// closeWorkReview decides and closes the finished examination of the work's
// subject. It returns nil once the chain is closed, so the caller collects;
// otherwise the result says what is still needed.
func (inv *intentInvocation) closeWorkReview(targets []intentTarget, root, commit, rootJob string) *intentResult {
	work := inv.reviewWork
	again := inv.publicArgv(append(reviewGoalWords(work.goal), "--work", work.work)...)
	data := map[string]any{"goal": work.goal, "work": work.work, "attempt": work.attempt, "examination": rootJob}
	newest, err := inv.newestRoundAt(root, rootJob)
	if err != nil {
		return &intentResult{Targets: targets, Outcome: intentFailed, code: 1, Data: data,
			Summary: fmt.Sprintf("the examination %s of work %s cannot be read: %v", rootJob, work.work, err), next: inv.publicArgv("check"), nextReason: "diagnose the record store"}
	}
	round, status := recordRound(newest), recordText(newest, "status")
	data["round"], data["status"] = round, status
	returnPath := inv.returnPathAt(root, rootJob, round)
	digest, _, readErr := reviewReturnDigest(returnPath)
	findings, verdict, parseErr := readIntentFindings(returnPath)
	if status != "completed" || readErr != nil || parseErr != nil {
		cause := status
		if readErr != nil || parseErr != nil {
			cause = fmt.Sprintf("%s with no readable return", status)
		}
		return &intentResult{Targets: targets, Outcome: intentFailed, code: 1, Data: data,
			Summary: fmt.Sprintf("examination round %d of work %s ended %s; it produced no findings to decide, and the review is not complete", round, work.work, cause),
			next:    append(again, "--retry", strconv.FormatInt(round, 10)), nextReason: "examine the same subject once more in the same review chain, under its round limit; the failed round is kept"}
	}
	binding := reviewBinding{Goal: work.goal, Work: work.work, Attempt: work.attempt, Subject: commit, Examination: rootJob, Round: round, Return: digest}
	data["binding"], data["verdict"], data["findings"] = binding.line(), verdict, findings
	if work.subject != nil && (work.subject.Examination != rootJob || work.subject.ExaminationRound != round) {
		work.subject.Examination, work.subject.ExaminationRound = rootJob, round
		if err := work.retain(*work.subject); err != nil {
			return &intentResult{Targets: targets, Outcome: intentFailed, code: 1, Data: data,
				Summary: "cannot record the examination with the work: " + err.Error(), next: again, nextReason: "the same command records it"}
		}
	}
	generated := filepath.Join(filepath.Dir(returnPath), "decisions.md")
	path := inv.flagPath("dispositions")
	if work.repair {
		if _, err := os.Stat(generated); err != nil {
			return &intentResult{Targets: targets, Outcome: intentRefused, code: 1, Data: data,
				Summary: fmt.Sprintf("examination %s round %d has no retained decisions, so there is no interrupted close to repair; nothing was done", rootJob, round),
				next:    again, nextReason: "the review itself decides and closes"}
		}
		path = generated
	}
	if path == "" {
		if len(findings) > 0 {
			if _, err := os.Stat(generated); err != nil {
				if err := os.WriteFile(generated, []byte(decisionsDocument(binding, findings)), 0o600); err != nil {
					return &intentResult{Targets: targets, Outcome: intentFailed, code: 1, Data: data, Summary: "cannot write the decisions template: " + err.Error()}
				}
			}
			lines := []string{}
			for _, finding := range findings {
				kind := "not material"
				if finding.Material {
					kind = "material"
				}
				lines = append(lines, fmt.Sprintf("  %s (%s): %s", finding.ID, kind, finding.Title))
			}
			data["template"] = generated
			return &intentResult{Targets: targets, Outcome: intentInProgress, code: 1, Data: data, text: lines,
				Summary:  fmt.Sprintf("the examination of work %s finished with %d finding(s) (%s); the review completes after the author's decisions", work.work, len(findings), verdict),
				Decision: fmt.Sprintf("decide every finding in %s, then run %s", generated, shellCommand(append(again, "--dispositions", generated)))}
		}
		if err := os.WriteFile(generated, []byte(decisionsDocument(binding, nil)), 0o600); err != nil {
			return &intentResult{Targets: targets, Outcome: intentFailed, code: 1, Data: data, Summary: "cannot write the empty decisions join: " + err.Error()}
		}
		path = generated
	} else {
		content, err := os.ReadFile(path)
		if err != nil {
			return &intentResult{Targets: targets, Outcome: intentRefused, code: 1, Data: data, Summary: "cannot read the decisions file: " + err.Error()}
		}
		bound, err := readReviewBinding(content)
		if err == nil && bound != binding {
			err = fmt.Errorf("the decisions file answers examination %s round %d of attempt %d (subject %s), not the current examination %s round %d of attempt %d (subject %s)",
				bound.Examination, bound.Round, bound.Attempt, shortSHA(bound.Subject), rootJob, round, work.attempt, shortSHA(commit))
		}
		if err != nil {
			if _, statErr := os.Stat(generated); statErr != nil {
				_ = os.WriteFile(generated, []byte(decisionsDocument(binding, findings)), 0o600)
			}
			return &intentResult{Targets: targets, Outcome: intentRefused, code: 1, Data: data, Summary: err.Error() + "; nothing was closed",
				Decision: fmt.Sprintf("decide the current findings in %s, then run %s", generated, shellCommand(append(again, "--dispositions", generated)))}
		}
		decisions, violations := validate.Dispositions(path)
		if len(violations) > 0 {
			return &intentResult{Targets: targets, Outcome: intentRefused, code: 1, Data: data, text: violations,
				Summary: "the decisions file is not a complete decision of the findings; nothing was closed", Decision: "correct the named rows and run the same command"}
		}
		// A person's accepted risk, recorded on the goal by its owner for
		// this finding of this examination, is the only lawful exception.
		risks := map[string]bool{}
		if projection, _, problem := inv.projection(); problem == nil {
			if file, _ := goalRecord(projection, work.goal); file != nil {
				for _, risk := range file.AcceptedRisks {
					if risk.Chain == rootJob {
						risks[risk.Finding] = true
					}
				}
			}
		}
		var accepted, excepted []string
		for _, finding := range findings {
			if finding.Material && decisions[finding.ID] == "accepted" {
				if risks[finding.ID] {
					excepted = append(excepted, finding.ID)
					continue
				}
				accepted = append(accepted, finding.ID)
			}
		}
		if len(excepted) > 0 {
			data["acceptedRisks"] = excepted
		}
		if path != generated {
			// The validated decisions are retained beside the return, so a
			// close interrupted after this point is repaired from them.
			if err := os.WriteFile(generated, content, 0o600); err != nil {
				return &intentResult{Targets: targets, Outcome: intentFailed, code: 1, Data: data, Summary: "cannot retain the decisions beside the examination: " + err.Error()}
			}
		}
		if len(accepted) > 0 {
			data["accepted"] = accepted
			return &intentResult{Targets: targets, Outcome: intentRefused, code: 1, Data: data,
				Summary:    fmt.Sprintf("accepted material finding(s) %s need a fix before this review can complete; nothing was closed", strings.Join(accepted, ", ")),
				next:       inv.publicArgv("revise", work.goal, "--work", work.work, "--after", fmt.Sprint(work.attempt), "--brief", "FILE", "--dispositions", path),
				nextReason: "the correction carries these findings and decisions to the builder; its result is reviewed again"}
		}
	}
	closer := *inv
	closer.layout.InstallationRoot = root
	closer.input = intentInput{values: map[string][]string{"dispositions": {path}}}
	closer.reviewWork = nil
	closed := closer.closeChain(rootJob)
	if closed.Outcome == intentConfirmed || closed.Outcome == intentUnchanged {
		return nil
	}
	closed.Targets, closed.Data = targets, mergeData(data, closed.Data)
	closed.Summary = "the examination finished, but the review's completion is pending: " + closed.Summary
	closed.Decision = strings.TrimSpace(closed.Decision)
	if _, fenced := data["owner"]; fenced || closed.Outcome == intentInProgress {
		// A coordination fence is temporary: the same review continues.
		closed.Outcome = intentInProgress
		closed.next, closed.nextReason = again, "the close waits for a coordination fence; the same review completes it (status "+work.goal+" shows the work meanwhile)"
	} else if cause, _ := data["cause"].(string); cause == "record-writer-refused" || cause == "authority-unestablished" {
		// The record-writer owner refused before anything was written; the
		// owner's decision names who may run it. Nothing to repair.
		closed.next, closed.nextReason = nil, ""
	} else {
		// Any later failure of the whole close is reported as it happened,
		// with the owner's output and its own close-check fact; the same
		// decisions replay it once the named cause is resolved.
		if checkErr := dispatchcore.CloseCheck(root, rootJob); checkErr != nil {
			data["closeCheck"] = checkErr.Error()
		}
		repair := inv.publicArgv("repair", "review", work.goal, "--work", work.work)
		closed.next, closed.nextReason = repair,
			"once the cause named above is resolved, this replays the whole close with the retained decisions; it never decides a finding"
		if inv.riskRemedy(&closed, root, work.goal, rootJob, repair) {
			closed.nextReason = "once a person has accepted each named risk, this replays the whole close with the retained decisions"
		}
	}
	if closed.Outcome == intentRefused && len(closed.text) == 0 {
		closed.text = []string{"The decisions are kept; nothing was accepted and no read was collected."}
	}
	return &closed
}

func mergeData(into map[string]any, extra any) map[string]any {
	if more, ok := extra.(map[string]any); ok {
		for key, value := range more {
			if _, taken := into[key]; !taken {
				into[key] = value
			}
		}
	}
	return into
}

func shortSHA(sha string) string {
	if len(sha) > 12 {
		return sha[:12]
	}
	return sha
}

// reviseDecisions checks a decisions file for revise against the work's
// recorded examinations and returns the document frozen with the request:
// the decisions and the exact findings return they answer. The file must
// answer an examination of an attempt at or before the corrected one, and no
// later attempt may have a completed examination of its own.
func (inv *intentInvocation) reviseDecisions(id string, work launch.NamedWork, after int) ([]byte, *intentResult) {
	path := inv.flagPath("dispositions")
	targets := workTargets(id, work)
	refuse := func(format string, args ...any) ([]byte, *intentResult) {
		return nil, &intentResult{Outcome: intentRefused, code: 1, Targets: targets, Summary: fmt.Sprintf(format, args...) + "; nothing was launched",
			next: inv.publicArgv(append(reviewGoalWords(id), "--work", work.Unit)...), nextReason: "the current examination and its bound decisions file"}
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return refuse("cannot read the decisions file: %v", err)
	}
	bound, err := readReviewBinding(content)
	if err != nil {
		return refuse("%v", err)
	}
	if bound.Goal != id || bound.Work != work.Unit {
		return refuse("the decisions file answers work %s of goal %s, not work %s of goal %s", bound.Work, bound.Goal, work.Unit, id)
	}
	if work.Record == nil {
		return refuse("work %s has no recorded attempts", work.Unit)
	}
	if after != 0 && after < bound.Attempt {
		return refuse("--after %d is before the reviewed attempt %d; a correction follows the attempt it corrects", after, bound.Attempt)
	}
	var examined *launch.UnitSubject
	for index := range work.Record.Subjects {
		subject := work.Record.Subjects[index]
		switch {
		case subject.Round == bound.Attempt && subject.Commit == bound.Subject && subject.Examination == bound.Examination && subject.ExaminationRound == bound.Round:
			examined = &subject
		case subject.Round > bound.Attempt && subject.Examination != "":
			return refuse("the decisions file answers attempt %d, but attempt %d has a later completed examination (%s) that supersedes it", bound.Attempt, subject.Round, subject.Examination)
		}
	}
	if examined == nil {
		return refuse("the decisions file names examination %s round %d of attempt %d, which is not a recorded examination of this work", bound.Examination, bound.Round, bound.Attempt)
	}
	install := inv.goalWorktreeInstallation(work.Record.Worktree)
	returnPath := inv.returnPathAt(install, bound.Examination, bound.Round)
	digest, findings, err := reviewReturnDigest(returnPath)
	if err != nil || digest != bound.Return {
		return refuse("the findings return of examination %s round %d is missing or no longer the one the decisions answer", bound.Examination, bound.Round)
	}
	if violations := validate.CritiqueClosed(returnPath, path); len(violations) > 0 {
		return nil, &intentResult{Outcome: intentRefused, code: 1, Targets: targets, text: violations,
			Summary: "the decisions file is not a complete decision of the reviewed findings; nothing was launched", Decision: "correct the named rows and run the same command"}
	}
	document := string(content) + "\n## Reviewed findings (examination " + bound.Examination + " round " + fmt.Sprint(bound.Round) + ")\n\n```json\n" +
		strings.TrimRight(string(findings), "\n") + "\n```\n"
	return []byte(document), nil
}

// riskRemedy names the public remedy when the close owner refused because
// the chain's register still holds open severe or unproven findings, read
// through the register's own typed reads for this goal: each needs a
// person's accept-risk, then the continuation completes the review. It
// grants nothing and claims no closure; any other refusal is left as the
// owner reported it.
func (inv *intentInvocation) riskRemedy(closed *intentResult, root, goalID, rootJob string, continuation []string) bool {
	ids, err := dispatchcore.CritiqueOpenFindingIDs(root, rootJob)
	if err != nil {
		return false
	}
	var risks, commands []string
	for _, id := range ids {
		finding, err := dispatchcore.CritiqueRegisterDecisionFinding(root, rootJob, id, goalID)
		if err != nil || (finding.RigorClass != string(critiqueModel.Severe) && finding.RigorClass != string(critiqueModel.Unproven)) {
			continue
		}
		risks = append(risks, id)
		commands = append(commands, shellCommand([]string{"metasystem", "accept-risk", goalID, "--finding", id, "--review", rootJob, "--repo", root, "--reason", "TEXT"}))
	}
	if len(risks) == 0 {
		return false
	}
	data, _ := closed.Data.(map[string]any)
	if data == nil {
		data = map[string]any{}
	}
	data["riskFindings"] = risks
	closed.Data = data
	closed.Outcome, closed.code = intentRefused, max(closed.code, 1)
	closed.text = []string{"The review remains incomplete; no risk was accepted and no result was published."}
	closed.Summary = fmt.Sprintf("review %s is not complete: %d severe or unproven finding(s) are open (%s), and only a person may accept their risk",
		rootJob, len(risks), strings.Join(risks, ", "))
	closed.Decision = fmt.Sprintf("if a person chooses to accept each risk, they give the reason: %s; then run %s",
		strings.Join(commands, "; "), shellCommand(continuation))
	return true
}
