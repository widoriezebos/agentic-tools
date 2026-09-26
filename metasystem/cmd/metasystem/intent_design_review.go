package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/validate"
)

// A design's critique is one bounded chain per goal and design document.
// A changed design continues that chain once the author has decided every
// finding of the reviewed round; a failed round is examined once more with
// --retry N. Each continuation is retained before it is requested: its frozen
// operation id, the decisions and subject it binds, and afterwards the child
// round the follow-up owner actually minted. A replay reads that child, or
// finds the operation's own record when the response was lost; it never asks
// for another round to learn the answer.

type designReviewRequest struct {
	Kind            string `json:"kind"`
	Root            string `json:"root"`
	AfterRound      int64  `json:"afterRound"`
	OperationID     string `json:"operationId"`
	DecisionsSHA256 string `json:"decisionsSha256,omitempty"`
	SubjectSHA256   string `json:"subjectSha256"`
	Brief           string `json:"brief"`
	Child           string `json:"child,omitempty"`
}

type designReviewEntry struct {
	Goal     string                `json:"goal"`
	Design   string                `json:"design"`
	Subjects map[string]string     `json:"subjects"` // examined round -> subject digest
	Requests []designReviewRequest `json:"requests"`
}

func (inv *intentInvocation) designReviewEntryPath(recordID string) string {
	return filepath.Join(inv.layout.InstallationRoot, "artifacts", "agents", "intent-review", "design-"+strings.ToLower(recordID), "chain.json")
}

func (inv *intentInvocation) readDesignReviewEntry(recordID string) designReviewEntry {
	var entry designReviewEntry
	if data, err := os.ReadFile(inv.designReviewEntryPath(recordID)); err == nil {
		json.Unmarshal(data, &entry)
	}
	if entry.Subjects == nil {
		entry.Subjects = map[string]string{}
	}
	return entry
}

func (inv *intentInvocation) writeDesignReviewEntry(recordID string, entry designReviewEntry) error {
	data, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return err
	}
	path := inv.designReviewEntryPath(recordID)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	_, err = atomicfile.WriteText(path, string(data)+"\n", inv.layout.InstallationRoot)
	return err
}

// designReviewPlan is what reviewDesign resolved before any dispatch.
type designReviewPlan struct {
	targets          []intentTarget
	goalID, recordID string
	design           string // canonical absolute path
	subject          string // current design digest
	brief            string
}

// reviewDesignChain decides a design review against the document's
// existing critique chain; nil means no chain exists and the first review
// is dispatched as before.
func (inv *intentInvocation) reviewDesignChain(plan designReviewPlan) *intentResult {
	chains := dispatchcore.DesignCritiqueChains(inv.layout.InstallationRoot, plan.goalID, plan.design)
	switch {
	case len(chains) == 0:
		if inv.input.has("dispositions") || inv.input.has("retry") || inv.input.has("after") {
			return &intentResult{Targets: plan.targets, Outcome: intentRefused, code: 2,
				Summary: "this design has no earlier examination to continue; review it first without --dispositions, --retry or --after; nothing was done"}
		}
		return nil
	case len(chains) > 1:
		lines := []string{}
		for _, chain := range chains {
			lines = append(lines, fmt.Sprintf("  chain %s, newest examination %d", chain.Root, chain.NewestRound))
		}
		return &intentResult{Targets: plan.targets, Outcome: intentRefused, code: 2, text: lines,
			Summary:  fmt.Sprintf("design %s has %d critique chains; none is chosen by time; nothing was done", plan.recordID, len(chains)),
			Decision: "the author decides and closes each chain that no longer applies: metasystem review job ROOT --dispositions FILE"}
	}
	chain := chains[0]
	entry := inv.readDesignReviewEntry(plan.recordID)
	entry.Goal, entry.Design = plan.goalID, plan.design
	status := recordText(chain.Newest, "status")
	switch {
	case chain.Closed && (inv.input.has("dispositions") || inv.input.has("retry")):
		return &intentResult{Targets: plan.targets, Outcome: intentRefused, code: 1,
			Summary: fmt.Sprintf("design %s's critique %s is closed; it is not continued; nothing was requested", plan.recordID, chain.Root)}
	case inv.input.has("retry"):
		return inv.retryDesignExamination(plan, chain, entry)
	case inv.input.has("dispositions"):
		return inv.continueDesignChain(plan, chain, entry)
	case chain.Closed:
		if entry.Subjects[strconv.FormatInt(chain.NewestRound, 10)] != plan.subject {
			return &intentResult{Targets: plan.targets, Outcome: intentRefused, code: 1,
				Summary:  fmt.Sprintf("design %s's critique is complete and closed; a changed design does not start a new critique by itself; nothing was done", plan.recordID),
				Decision: "a person decides whether the change needs another bounded critique (a new review budget or a re-scoped goal)"}
		}
		return nil
	case !dispatchcore.TerminalStatus(status):
		return &intentResult{Targets: append(plan.targets, jobTarget(chain.NewestJob)), Outcome: intentInProgress,
			Summary: fmt.Sprintf("examination %d of design %s is %s", chain.NewestRound, plan.recordID, status), next: inv.sameCommand(), nextReason: "the same review shows its result"}
	case status != "completed":
		return &intentResult{Targets: append(plan.targets, jobTarget(chain.NewestJob)), Outcome: intentFailed, code: 1,
			Summary:    fmt.Sprintf("examination %d of design %s ended %s without findings to decide", chain.NewestRound, plan.recordID, status),
			next:       inv.publicArgv("review", "design", relativeOrSame(inv.layout.GitRoot, plan.design), "--retry", strconv.FormatInt(chain.NewestRound, 10)),
			nextReason: "examine the design once more in the same chain, under its round limit, once the old process is proven stopped"}
	}
	examined := entry.Subjects[strconv.FormatInt(chain.NewestRound, 10)]
	if examined == plan.subject {
		return inv.collectDesignExamination(plan, chain, examined)
	}
	if examined == "" {
		return &intentResult{Targets: plan.targets, Outcome: intentRefused, code: 1,
			Summary:  fmt.Sprintf("the design version examination %d of critique %s read is not recorded here, so a changed design cannot be bound to it; no new critique is bought; nothing was done", chain.NewestRound, chain.Root),
			Decision: "review the same design version to see its findings and decide them with review design FILE --dispositions FILE"}
	}
	// The design changed since the newest examination: its findings need
	// the author's decisions before the chain continues.
	returnPath := inv.returnPathAt(inv.layout.InstallationRoot, chain.Root, chain.NewestRound)
	digest, _, _ := reviewReturnDigest(returnPath)
	findings, _, _ := readIntentFindings(returnPath)
	binding := reviewBinding{Goal: plan.goalID, Work: "design:" + plan.recordID, Attempt: int(chain.NewestRound), Subject: examined,
		Examination: chain.Root, Round: chain.NewestRound, Return: digest}
	template := filepath.Join(filepath.Dir(returnPath), "decisions.md")
	if _, err := os.Stat(template); err != nil {
		os.WriteFile(template, []byte(decisionsDocument(binding, findings)), 0o600)
	}
	return &intentResult{Targets: plan.targets, Outcome: intentInProgress, code: 1, Data: map[string]any{"template": template, "examination": chain.NewestRound},
		Summary:  fmt.Sprintf("design %s changed since examination %d; its %d finding(s) need the author's decisions before the same critique continues", plan.recordID, chain.NewestRound, len(findings)),
		Decision: fmt.Sprintf("decide every finding in %s, then run %s", template, shellCommand(append(inv.sameCommand(), "--dispositions", template)))}
}

// continueDesignChain requests, or rejoins, the one follow-up examination of
// the changed design the bound decisions ask for.
func (inv *intentInvocation) continueDesignChain(plan designReviewPlan, chain dispatchcore.DesignCritiqueChain, entry designReviewEntry) *intentResult {
	path := inv.flagPath("dispositions")
	content, err := os.ReadFile(path)
	if err != nil {
		return &intentResult{Targets: plan.targets, Outcome: intentRefused, code: 1, Summary: "cannot read the decisions file: " + err.Error()}
	}
	bound, err := readReviewBinding(content)
	if err == nil && (bound.Goal != plan.goalID || bound.Work != "design:"+plan.recordID || bound.Examination != chain.Root) {
		err = fmt.Errorf("the decisions file answers %s of %s, not design %s's critique %s", bound.Work, bound.Goal, plan.recordID, chain.Root)
	}
	if err == nil && inv.input.has("after") && inv.input.text("after") != strconv.FormatInt(bound.Round, 10) {
		err = fmt.Errorf("--after %s is not the examination %d the decisions file answers", inv.input.text("after"), bound.Round)
	}
	if err != nil {
		return &intentResult{Targets: plan.targets, Outcome: intentRefused, code: 1, Summary: err.Error() + "; nothing was requested"}
	}
	returnPath := inv.returnPathAt(inv.layout.InstallationRoot, chain.Root, bound.Round)
	if digest, _, readErr := reviewReturnDigest(returnPath); readErr != nil || digest != bound.Return || entry.Subjects[strconv.FormatInt(bound.Round, 10)] != bound.Subject {
		return &intentResult{Targets: plan.targets, Outcome: intentRefused, code: 1,
			Summary: fmt.Sprintf("the decisions file does not answer the recorded examination %d of design %s; nothing was requested", bound.Round, plan.recordID)}
	}
	if violations := validate.CritiqueClosed(returnPath, path); len(violations) > 0 {
		return &intentResult{Targets: plan.targets, Outcome: intentRefused, code: 1, text: violations,
			Summary: "the decisions file is not a complete decision of the examined findings; nothing was requested"}
	}
	if bound.Subject == plan.subject {
		return inv.closeDesignCritique(plan, chain, returnPath, path)
	}
	decisions := digestText(content)
	operation := fmt.Sprintf("design-%s-after-%d-%s", strings.ToLower(plan.recordID), bound.Round, decisions[:12])
	// The follow-up examines the changed design with the author's decisions
	// on the reviewed findings; both are frozen in its brief.
	brief, err := os.ReadFile(plan.brief)
	if err != nil {
		return &intentResult{Targets: plan.targets, Outcome: intentFailed, code: 1, Summary: "the review brief cannot be read: " + err.Error()}
	}
	followUp := filepath.Join(filepath.Dir(inv.designReviewEntryPath(plan.recordID)), operation+"-brief.md")
	if _, statErr := os.Stat(followUp); statErr != nil {
		text := string(brief) + fmt.Sprintf("\n## The author's decisions on examination %d\n\nThe design changed since examination %d. Judge whether each accepted finding is addressed in the new version.\n\n", bound.Round, bound.Round) + string(content)
		if err := os.MkdirAll(filepath.Dir(followUp), 0o755); err != nil {
			return &intentResult{Targets: plan.targets, Outcome: intentFailed, code: 1, Summary: err.Error()}
		}
		if _, err := atomicfile.WriteText(followUp, text, inv.layout.InstallationRoot); err != nil {
			return &intentResult{Targets: plan.targets, Outcome: intentFailed, code: 1, Summary: err.Error()}
		}
	}
	return inv.designFollowUp(plan, chain, entry, designReviewRequest{Kind: "continue", Root: chain.Root, AfterRound: bound.Round, OperationID: operation,
		DecisionsSHA256: decisions, SubjectSHA256: plan.subject, Brief: followUp})
}

// retryDesignExamination requests, or rejoins, one more examination of the
// same subject after examination N failed; the dispatch owner admits it only
// for a terminal round without a return whose process is proven dead.
func (inv *intentInvocation) retryDesignExamination(plan designReviewPlan, chain dispatchcore.DesignCritiqueChain, entry designReviewEntry) *intentResult {
	round, err := strconv.ParseInt(inv.input.text("retry"), 10, 64)
	if err != nil || round < 1 {
		return &intentResult{Targets: plan.targets, Outcome: intentRefused, code: 2, Summary: "--retry takes the failed examination's number; nothing was done"}
	}
	operation := fmt.Sprintf("design-%s-retry-%d", strings.ToLower(plan.recordID), round)
	for _, request := range entry.Requests {
		if request.OperationID == operation {
			return inv.designFollowUp(plan, chain, entry, request)
		}
	}
	if round != chain.NewestRound {
		return &intentResult{Targets: plan.targets, Outcome: intentRefused, code: 1,
			Summary: fmt.Sprintf("examination %d is not design %s's newest examination (%d); nothing was requested", round, plan.recordID, chain.NewestRound)}
	}
	return inv.designFollowUp(plan, chain, entry, designReviewRequest{Kind: "retry", Root: chain.Root, AfterRound: round, OperationID: operation,
		SubjectSHA256: entry.Subjects[strconv.FormatInt(round, 10)], Brief: plan.brief})
}

// designFollowUp is the retained follow-up: rejoin its child, adopt the
// operation's own record after a lost response, or request it once.
func (inv *intentInvocation) designFollowUp(plan designReviewPlan, chain dispatchcore.DesignCritiqueChain, entry designReviewEntry, request designReviewRequest) *intentResult {
	index := -1
	for position, retained := range entry.Requests {
		if retained.OperationID == request.OperationID {
			index, request = position, retained
		}
	}
	if index < 0 {
		entry.Requests = append(entry.Requests, request)
		index = len(entry.Requests) - 1
		if err := inv.writeDesignReviewEntry(plan.recordID, entry); err != nil {
			return &intentResult{Targets: plan.targets, Outcome: intentFailed, code: 1, Summary: "the review request cannot be retained: " + err.Error()}
		}
	}
	if request.Child == "" {
		// A lost response: the operation's own record names the child.
		if job, record, err := dispatchcore.FindOperationRecord(inv.layout.InstallationRoot, request.OperationID); err == nil && job != "" {
			if recordText(record, "parentJob") == "" || !designChildOf(inv, record, request.Root) {
				return &intentResult{Targets: plan.targets, Outcome: intentFailed, code: 1,
					Summary: fmt.Sprintf("operation %s is recorded on job %s, which is not a round of critique %s; nothing was adopted", request.OperationID, job, request.Root)}
			}
			request.Child = job
		}
	}
	if request.Child == "" {
		outcome, problem := inv.delegate(plan.targets, []string{"--follow-up", request.Root, "--brief", request.Brief, "--op", request.OperationID})
		if problem != nil {
			return problem
		}
		request.Child = outcome.JobID
	}
	entry.Requests[index] = request
	if request.Kind == "continue" {
		if record, err := inv.jobRecord(request.Child); err == nil {
			if round := recordRound(record); round > 0 {
				entry.Subjects[strconv.FormatInt(round, 10)] = request.SubjectSHA256
			}
		}
	}
	if err := inv.writeDesignReviewEntry(plan.recordID, entry); err != nil {
		return &intentResult{Targets: plan.targets, Outcome: intentFailed, code: 1, Summary: "the examination " + request.Child + " was requested but cannot be retained: " + err.Error(),
			next: inv.sameCommand(), nextReason: "the same request finds the examination by its operation and retains it"}
	}
	result := inv.collectReview(plan.targets, delegateOutcome{Outcome: "WON", JobID: request.Child})
	if data, ok := result.Data.(map[string]any); ok {
		data["operation"], data["afterExamination"] = request.OperationID, request.AfterRound
	}
	return &result
}

// designChildOf reports whether a job record is a round of the critique
// root, by its chain.
func designChildOf(inv *intentInvocation, record map[string]any, root string) bool {
	chainRoot, err := dispatchcore.ChainRootOf(inv.layout.InstallationRoot, recordText(record, "jobId"))
	return err == nil && chainRoot == root
}

// acquireDesignCritiqueClaim claims an approved goal nobody holds before its
// first paid design critique, as build does; a goal already held keeps its
// holder and the dispatch owner judges it.
func (inv *intentInvocation) acquireDesignCritiqueClaim(goalID string) *intentResult {
	if problem := inv.selectRoot(); problem != nil {
		return problem
	}
	projection, _, problem := inv.projection()
	if problem != nil {
		return problem
	}
	file, _ := goalRecord(projection, goalID)
	if file == nil || file.State == goal.StateClaimed {
		return nil
	}
	if claimed := inv.acquireClaim(goalID); claimed.Outcome != intentConfirmed {
		claimed.Summary = fmt.Sprintf("the design critique claims goal %s first, and the claim was not granted: %s; no critique started", goalID, strings.TrimSpace(claimed.Summary))
		return &claimed
	}
	return nil
}

// recordFirstDesignExamination retains the design version the first
// examination of a new chain reads.
func (inv *intentInvocation) recordFirstDesignExamination(plan designReviewPlan) {
	chains := dispatchcore.DesignCritiqueChains(inv.layout.InstallationRoot, plan.goalID, plan.design)
	if len(chains) != 1 {
		return
	}
	entry := inv.readDesignReviewEntry(plan.recordID)
	if _, known := entry.Subjects["1"]; known {
		return
	}
	entry.Goal, entry.Design, entry.Subjects["1"] = plan.goalID, plan.design, plan.subject
	inv.writeDesignReviewEntry(plan.recordID, entry)
}

// closeDesignCritique completes a critique whose decisions answer the
// design version still in place: with no accepted material finding there is
// nothing left to examine, so the whole close owner closes the chain. An
// accepted material finding needs a changed design, which the next review
// examines; it is never closed as clean.
func (inv *intentInvocation) closeDesignCritique(plan designReviewPlan, chain dispatchcore.DesignCritiqueChain, returnPath, dispositions string) *intentResult {
	findings, _, err := readIntentFindings(returnPath)
	if err != nil {
		return &intentResult{Targets: plan.targets, Outcome: intentFailed, code: 1, Summary: "the examination's findings cannot be read: " + err.Error()}
	}
	decisions, violations := validate.Dispositions(dispositions)
	if len(violations) > 0 {
		return &intentResult{Targets: plan.targets, Outcome: intentRefused, code: 1, text: violations,
			Summary: "the decisions file is not a complete decision of the findings; nothing was closed"}
	}
	var accepted []string
	for _, finding := range findings {
		if finding.Material && decisions[finding.ID] == "accepted" {
			accepted = append(accepted, finding.ID)
		}
	}
	if len(accepted) > 0 {
		return &intentResult{Targets: plan.targets, Outcome: intentRefused, code: 1, Data: map[string]any{"accepted": accepted},
			Summary:  fmt.Sprintf("design %s is unchanged, but material finding(s) %s are accepted; nothing was closed", plan.recordID, strings.Join(accepted, ", ")),
			Decision: "change the design to address them, then run the same review with the same decisions file: the critique examines the new version"}
	}
	closed := inv.closeChain(chain.Root)
	closed.Targets = append(append([]intentTarget{}, plan.targets...), closed.Targets...)
	if closed.Outcome == intentConfirmed || closed.Outcome == intentUnchanged {
		closed.Summary = fmt.Sprintf("design %s's critique is complete: every finding is decided and chain %s is closed", plan.recordID, chain.Root)
	}
	return &closed
}

// collectDesignExamination shows the newest examination of the design
// version still in place, from the chain the dispatch owner holds, rather
// than asking the dispatcher for an equal read. A finished examination
// comes with its decisions template: the decided findings of the unchanged
// design complete the critique through review design FILE --dispositions.
func (inv *intentInvocation) collectDesignExamination(plan designReviewPlan, chain dispatchcore.DesignCritiqueChain, examined string) *intentResult {
	result := inv.collectReview(plan.targets, delegateOutcome{Outcome: "REJOINED", JobID: chain.NewestJob})
	if recordText(chain.Newest, "status") != "completed" {
		return &result
	}
	returnPath := inv.returnPathAt(inv.layout.InstallationRoot, chain.Root, chain.NewestRound)
	digest, _, err := reviewReturnDigest(returnPath)
	if err != nil {
		return &result
	}
	findings, _, _ := readIntentFindings(returnPath)
	binding := reviewBinding{Goal: plan.goalID, Work: "design:" + plan.recordID, Attempt: int(chain.NewestRound), Subject: examined,
		Examination: chain.Root, Round: chain.NewestRound, Return: digest}
	template := filepath.Join(filepath.Dir(returnPath), "decisions.md")
	if _, statErr := os.Stat(template); statErr != nil {
		os.WriteFile(template, []byte(decisionsDocument(binding, findings)), 0o600)
	}
	data, _ := result.Data.(map[string]any)
	if data == nil {
		data = map[string]any{}
	}
	data["template"], data["examination"] = template, chain.NewestRound
	result.Data = data
	result.Decision = fmt.Sprintf("decide every finding in %s, then run %s", template, shellCommand(append(inv.sameCommand(), "--dispositions", template)))
	return &result
}
