package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/project"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/readsubject"
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
	Fold     *goal.DesignExit      `json:"fold,omitempty"`
	Goal     string                `json:"goal"`
	Design   string                `json:"design"`
	Subjects map[string]string     `json:"subjects"` // examined round -> subject digest
	Requests []designReviewRequest `json:"requests"`
	Exit     *goal.DesignExit      `json:"exit,omitempty"`
}

func (inv *intentInvocation) designReviewEntryPath(recordID string) string {
	return inv.layout.InstallationRoot.Path("artifacts", "agents", "intent-review", "design-"+strings.ToLower(recordID), "chain.json")
}

func (inv *intentInvocation) readDesignReviewEntry(recordID string) (designReviewEntry, error) {
	var entry designReviewEntry
	data, err := os.ReadFile(inv.designReviewEntryPath(recordID))
	if err == nil {
		err = json.Unmarshal(data, &entry)
	} else if os.IsNotExist(err) {
		err = nil
	}
	if entry.Subjects == nil {
		entry.Subjects = map[string]string{}
	}
	return entry, err
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
	_, err = atomicfile.WriteText(path, string(data)+"\n", inv.layout.InstallationRoot.Path())
	return err
}

// designReviewPlan is what reviewDesign resolved before any dispatch.
type designReviewPlan struct {
	root             string // an explicitly selected canonical job alias
	targets          []intentTarget
	goalID, recordID string
	design           string // canonical absolute path
	subject          string // current design digest
	brief            string
	inputs           map[string]string // the generated brief and outputs, by path, not yet written
}

// writeMissingInputs writes the generated inputs a continuing chain needs
// where none are written yet; one already written is the one its chain
// admitted and is kept as it is.
func (plan designReviewPlan) writeMissingInputs() error {
	missing := map[string]string{}
	for path, content := range plan.inputs {
		if _, err := os.Stat(path); errors.Is(err, fs.ErrNotExist) {
			missing[path] = content
		}
	}
	if len(missing) == 0 {
		return nil
	}
	return writeIntentInputs(filepath.Dir(plan.brief), missing)
}

// reviewDesignChain decides a design review against the document's
// existing critique chain; nil means no chain exists and the first review
// is dispatched as before.
func (inv *intentInvocation) reviewDesignChain(plan designReviewPlan) *intentResult {
	entry, err := inv.readDesignReviewEntry(plan.recordID)
	chains, chainErr := dispatchcore.ReadDesignCritiqueChains(inv.layout.InstallationRoot.Path(), plan.goalID, plan.design)
	if plan.root != "" {
		selected := chains[:0]
		for _, chain := range chains {
			if chain.Root == plan.root {
				selected = append(selected, chain)
			}
		}
		chains = selected
	}
	if err != nil || chainErr != nil || len(chains) == 0 && len(entry.Subjects) != 0 {
		return &intentResult{Targets: plan.targets, Outcome: intentFailed, code: 1, Summary: "the design history is unknown; no new critique was requested", Details: []string{fmt.Sprint(err, chainErr)}, next: inv.sameCommand(), nextReason: "restore the retained chain entry, job index and frozen subject for this design"}
	}
	var open []dispatchcore.DesignCritiqueChain
	for _, chain := range chains {
		if !chain.Closed {
			open = append(open, chain)
		}
	}
	if len(open) > 0 {
		chains = open
	}
	switch {
	case len(chains) == 0:
		if inv.input.has("dispositions") || inv.input.has("retry") || inv.input.has("after") {
			return &intentResult{Targets: plan.targets, Outcome: intentRefused, code: 2,
				Summary: "this design has no earlier review to continue; nothing was done",
				next:    inv.typedArgvLess("dispositions", "retry", "after"), nextReason: "its first review"}
		}
		return nil
	case len(chains) > 1:
		lines := []string{}
		for _, chain := range chains {
			lines = append(lines, fmt.Sprintf("  chain %s, newest examination %d", chain.Root, chain.NewestRound))
		}
		return &intentResult{Targets: plan.targets, Outcome: intentRefused, code: 2, text: lines,
			Summary:    fmt.Sprintf("design %s has %d separate critiques and none is picked for you; nothing was done", plan.recordID, len(chains)),
			next:       []string{"metasystem", "work", "review", dispatchJobPrefix + chains[0].Root, "--dispositions", "FILE"},
			nextReason: "decides and closes a critique that no longer applies; the others are listed above"}
	}
	chain := chains[0]
	if entry.Subjects[strconv.FormatInt(chain.NewestRound, 10)] == "" {
		if subject, present, err := readsubject.ReadRoundSubject(inv.layout.InstallationRoot.Path("artifacts", "agents"), chain.Root, chain.NewestRound); err == nil && present {
			entry.Subjects[strconv.FormatInt(chain.NewestRound, 10)] = subject.ContentDigest
			entry.Goal, entry.Design = plan.goalID, plan.design
			if err := inv.writeDesignReviewEntry(plan.recordID, entry); err != nil {
				return &intentResult{Targets: plan.targets, Outcome: intentFailed, code: 1, Summary: "the recovered design history could not be retained", Details: []string{err.Error()}, next: inv.sameCommand()}
			}
		}
	}
	if resumed := inv.finishDesignAcceptance(plan, chain, false); resumed != nil {
		return resumed
	}
	if recordText(chain.Newest, "status") == "completed" && !chain.Closed {
		required, err := dispatchcore.DesignEvidenceRequired(inv.layout.InstallationRoot.Path(), chain.Root, chain.NewestRound)
		if err == nil && required {
			_, err = inv.delivery().examinationRead(inv.layout.InstallationRoot.Path(), chain.NewestJob)
			if err == nil {
				_, err = dispatchcore.CritiqueRegisterAdvance(inv.layout.InstallationRoot.Path(), chain.Root, chain.NewestJob)
			}
		}
		if err != nil {
			if inv.input.has("retry") {
				return inv.retryDesignExamination(plan, chain, entry)
			}
			return inv.unknownDesignExamination(plan, chain, err)
		}
		if required && inv.input.has("retry") {
			return inv.collectDesignExamination(plan, chain, entry.Subjects[strconv.FormatInt(chain.NewestRound, 10)])
		}
	}
	entry.Goal, entry.Design = plan.goalID, plan.design
	status := recordText(chain.Newest, "status")
	switch {
	case chain.Closed && inv.input.has("dispositions") && inv.closedByTheseDecisions(chain):
		// The same answer again is the close it already made; the design's
		// Dispositions section, if the first close could not write it, is
		// written now.
		return inv.designCritiqueClosed(plan, chain, intentResult{Targets: append(plan.targets, jobTarget(chain.Root)), Outcome: intentUnchanged})
	case chain.Closed && (inv.input.has("dispositions") || inv.input.has("retry")):
		return &intentResult{Targets: plan.targets, Outcome: intentRefused, code: 1,
			Summary:  fmt.Sprintf("design %s's critique is closed and is never continued; nothing was requested", plan.recordID),
			Decision: "nothing to do; the critique is complete", Details: []string{"critique " + chain.Root}}
	case inv.input.has("retry"):
		return inv.retryDesignExamination(plan, chain, entry)
	case inv.input.has("dispositions"):
		return inv.continueDesignChain(plan, chain, entry)
	case chain.Closed:
		if entry.Subjects[strconv.FormatInt(chain.NewestRound, 10)] != plan.subject {
			return &intentResult{Targets: plan.targets, Outcome: intentRefused, code: 1,
				Summary: fmt.Sprintf("design %s changed after its critique closed, and a change starts no new critique; nothing was done", plan.recordID),
				next:    inv.publicArgv("design", "review", relativeOrSame(inv.layout.GitRoot, plan.design), "--scope", "FILE", "--reason", "TEXT", "--by", "NAME"), nextReason: "a person records an explicit scope change; the closed critique keeps its consumed allowance"}
		}
		return &intentResult{Targets: append(plan.targets, jobTarget(chain.Root)), Outcome: intentUnchanged, Summary: "the design's retained critique is closed; no new examination was requested"}
	case !dispatchcore.TerminalStatus(status):
		return &intentResult{Targets: append(plan.targets, jobTarget(chain.NewestJob)), Outcome: intentInProgress,
			Summary: fmt.Sprintf("examination %d of design %s is %s", chain.NewestRound, plan.recordID, status), next: inv.sameCommand(), nextReason: "the same review shows its result"}
	case status == "timeout":
		return inv.unknownDesignExamination(plan, chain, fmt.Errorf("the design examination reached its deadline; no automatic retry is admitted"))
	case status != "completed":
		return &intentResult{Targets: append(plan.targets, jobTarget(chain.NewestJob)), Outcome: intentFailed, code: 1,
			Summary:    fmt.Sprintf("examination %d of design %s ended %s without findings to decide", chain.NewestRound, plan.recordID, status),
			next:       inv.publicArgv("design", "review", relativeOrSame(inv.layout.GitRoot, plan.design), "--retry", strconv.FormatInt(chain.NewestRound, 10)),
			nextReason: "examine the design once more in the same chain, under its round limit, once the old process is proven stopped"}
	}
	examined := entry.Subjects[strconv.FormatInt(chain.NewestRound, 10)]
	if examined == plan.subject {
		return inv.collectDesignExamination(plan, chain, examined)
	}
	if examined == "" {
		return &intentResult{Targets: plan.targets, Outcome: intentRefused, code: 1,
			Summary:  fmt.Sprintf("the design version review %d read isn't recorded here; nothing was done", chain.NewestRound),
			Decision: "put back the design as review " + strconv.FormatInt(chain.NewestRound, 10) + " read it, then run metasystem design review " + relativeOrSame(inv.layout.GitRoot, plan.design) + " to see its findings",
			Details:  []string{"critique " + chain.Root}}
	}
	// The design changed since the newest examination: its findings need
	// the author's decisions before the chain continues.
	returnPath := inv.returnPathAt(inv.layout.InstallationRoot.Path(), chain.Root, chain.NewestRound)
	digest, _, _ := reviewReturnDigest(returnPath)
	findings, _, _ := readIntentFindings(returnPath)
	binding := reviewBinding{Goal: plan.goalID, Work: "design:" + plan.recordID, Attempt: int(chain.NewestRound), Subject: examined,
		Examination: chain.Root, Round: chain.NewestRound, Return: digest}
	template := filepath.Join(filepath.Dir(returnPath), "decisions.md")
	if _, err := os.Stat(template); err != nil {
		os.WriteFile(template, []byte(decisionsDocument(binding, findings)), 0o600)
	}
	return &intentResult{Targets: plan.targets, Outcome: intentInProgress, code: 1, Data: map[string]any{"template": template, "examination": chain.NewestRound},
		Summary: fmt.Sprintf("design %s changed since review %d; decide its %d finding(s) first", plan.recordID, chain.NewestRound, len(findings)),
		next:    append(inv.sameCommand(), "--dispositions", template), nextReason: "after deciding every finding in " + template}
}

// continueDesignChain requests, or rejoins, the one follow-up examination of
// the changed design the bound decisions ask for.
func (inv *intentInvocation) continueDesignChain(plan designReviewPlan, chain dispatchcore.DesignCritiqueChain, entry designReviewEntry) *intentResult {
	path := inv.flagPath("dispositions")
	content, err := os.ReadFile(path)
	if err != nil {
		return &intentResult{Targets: plan.targets, Outcome: intentRefused, code: 1, Summary: fileProblem("decisions file", path, err) + "; nothing was requested",
			next: inv.sameCommand(), nextReason: "once --dispositions names a readable file"}
	}
	bound, err := readReviewBinding(content)
	if err == nil && (bound.Goal != plan.goalID || bound.Work != "design:"+plan.recordID || bound.Examination != chain.Root) {
		err = fmt.Errorf("the decisions file answers %s of %s, not design %s's critique %s", bound.Work, bound.Goal, plan.recordID, chain.Root)
	}
	if err == nil && inv.input.has("after") && inv.input.text("after") != strconv.FormatInt(bound.Round, 10) {
		err = fmt.Errorf("--after %s is not the examination %d the decisions file answers", inv.input.text("after"), bound.Round)
	}
	if err != nil {
		return &intentResult{Targets: plan.targets, Outcome: intentRefused, code: 1, Summary: err.Error() + "; nothing was requested",
			next: inv.typedArgvLess("dispositions", "after"), nextReason: "shows the current findings and writes their decisions file"}
	}
	if bound.Round < chain.NewestRound {
		for _, request := range entry.Requests {
			if request.Kind == "continue" && request.AfterRound == bound.Round && request.DecisionsSHA256 == digestText(content) && request.SubjectSHA256 == plan.subject {
				return inv.designFollowUp(plan, chain, entry, request)
			}
		}
		return &intentResult{Targets: plan.targets, Outcome: intentRefused, code: 1,
			Summary: fmt.Sprintf("review %d already has examination %d; its decisions are frozen; nothing was changed", bound.Round, chain.NewestRound),
			next:    inv.typedArgvLess("dispositions", "after"), nextReason: "shows the newest examination and its own decisions file"}
	}
	returnPath := inv.returnPathAt(inv.layout.InstallationRoot.Path(), chain.Root, bound.Round)
	if digest, _, readErr := reviewReturnDigest(returnPath); readErr != nil || digest != bound.Return || entry.Subjects[strconv.FormatInt(bound.Round, 10)] != bound.Subject {
		return &intentResult{Targets: plan.targets, Outcome: intentRefused, code: 1,
			Summary: fmt.Sprintf("the decisions file doesn't answer review %d of design %s as it was recorded; nothing was requested", bound.Round, plan.recordID),
			next:    inv.typedArgvLess("dispositions", "after"), nextReason: "shows the current findings and writes their decisions file"}
	}
	if violations := validate.CritiqueClosed(returnPath, path); len(violations) > 0 {
		return &intentResult{Targets: plan.targets, Outcome: intentRefused, code: 1, text: violations,
			Summary: "the decisions file leaves findings undecided (listed above); nothing was requested",
			next:    inv.sameCommand(), nextReason: "once every finding has a decision"}
	}
	if required, err := dispatchcore.DesignEvidenceRequired(inv.layout.InstallationRoot.Path(), chain.Root, bound.Round); err == nil && required {
		page, problem := os.ReadFile(plan.design)
		if problem == nil {
			_, problem = dispatchcore.DesignRevisionSections(inv.layout.InstallationRoot.Path(), chain.Root, bound.Round, string(page), content)
		}
		if problem != nil {
			return &intentResult{Targets: plan.targets, Outcome: intentFailed, code: 1, Summary: "the section evidence is unknown; nothing was closed or requested", Details: []string{problem.Error()}, next: inv.sameCommand(), nextReason: "once the mapping names unique headings in both page versions"}
		}
	}
	// The decisions are the round's own from here on: the close composes the
	// design's Dispositions section from every answered round's file.
	if err := retainRoundDecisions(returnPath, path, content); err != nil {
		return &intentResult{Targets: plan.targets, Outcome: intentFailed, code: 1, Summary: "the decisions can't be saved with the review, so nothing was requested",
			next: inv.sameCommand(), nextReason: "try again; --verbose shows the cause", Details: []string{err.Error()}}
	}
	// The final round is answered by the close whatever a fold changed: there
	// is no further round to examine the change, and the engine's own
	// classification of what is left is the exit (g1-s66 D4).
	final := bound.Round >= inv.designRoundLimit(chain.Root)
	modern := false
	if root, err := inv.jobRecord(chain.Root); err == nil {
		if decision, ok := root["designDecision"].(map[string]any); ok {
			final = recordText(decision, "decision") != "continue"
			modern = true
		}
	}
	unchanged := bound.Subject == plan.subject
	if modern && unchanged && !final {
		decisions, _ := validate.Dispositions(path)
		unchanged = false
		for _, decision := range decisions {
			if decision == "accepted" {
				unchanged = true
			}
		}
	}
	if unchanged || final {
		return inv.closeDesignCritique(plan, chain, returnPath, path, bound.Round, final)
	}
	if modern {
		if err := dispatchcore.CritiqueRegisterApplyDecisions(inv.layout.InstallationRoot.Path(), chain.Root, inv.registerDecisions(chain.Root, bound.Round+1)); err != nil {
			return inv.unknownDesignExamination(plan, chain, err)
		}
		policy, err := inv.unitRunner().ReviewPolicy()
		if err != nil || policy == "person" && inv.directPersonProof("design continuation") != nil {
			result := &intentResult{Targets: plan.targets, Outcome: intentInProgress, Summary: "the review policy holds the prepared design continuation", next: inv.sameCommand(), nextReason: "release that hold to resume"}
			if err != nil {
				result.Details = []string{err.Error()}
				result.nextReason = "repair the review.stop setting, then run the same command to resume"
			}
			return result
		}
	}
	decisions := digestText(content)
	operation := fmt.Sprintf("design-%s-after-%d-%s", strings.ToLower(plan.recordID), bound.Round, decisions[:12])
	// The follow-up examines the changed design with the author's decisions
	// on the reviewed findings; both are frozen in its brief.
	if err := plan.writeMissingInputs(); err != nil {
		return &intentResult{Targets: plan.targets, Outcome: intentFailed, code: 1, Summary: "the review's inputs can't be written, so nothing was requested",
			next: inv.sameCommand(), nextReason: "try again; --verbose shows the cause", Details: []string{err.Error()}}
	}
	brief, err := os.ReadFile(plan.brief)
	if err != nil {
		return &intentResult{Targets: plan.targets, Outcome: intentFailed, code: 1, Summary: fileProblem("review brief", plan.brief, err) + "; nothing was requested",
			next: inv.sameCommand(), nextReason: "try again once it is readable", Details: []string{err.Error()}}
	}
	followUp := filepath.Join(filepath.Dir(inv.designReviewEntryPath(plan.recordID)), operation+"-brief.md")
	if _, statErr := os.Stat(followUp); statErr != nil {
		// Drafts the decisions cite are frozen as the page's were.
		drafts := inv.freezeDesignDrafts(inv.layout.GitRoot, filepath.Dir(plan.brief), "", nil, content)
		text := string(brief) + fmt.Sprintf("\n## The author's decisions on examination %d\n\nJudge the whole submitted page, including whether each accepted finding is addressed and each refutation is supported.\n\n", bound.Round) + string(content) + drafts.section()
		if err := writeIntentInputs(filepath.Dir(plan.brief), drafts.files); err != nil {
			return &intentResult{Targets: plan.targets, Outcome: intentFailed, code: 1, Summary: "the review's inputs can't be written, so nothing was requested",
				next: inv.sameCommand(), nextReason: "try again; --verbose shows the cause", Details: []string{err.Error()}}
		}
		if err := os.MkdirAll(filepath.Dir(followUp), 0o755); err != nil {
			return &intentResult{Targets: plan.targets, Outcome: intentFailed, code: 1, Summary: "the follow-up review's brief can't be written, so nothing was requested",
				next: inv.sameCommand(), nextReason: "try again; --verbose shows the cause", Details: []string{err.Error()}}
		}
		if _, err := atomicfile.WriteText(followUp, text, inv.layout.InstallationRoot.Path()); err != nil {
			return &intentResult{Targets: plan.targets, Outcome: intentFailed, code: 1, Summary: "the follow-up review's brief can't be written, so nothing was requested",
				next: inv.sameCommand(), nextReason: "try again; --verbose shows the cause", Details: []string{err.Error()}}
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
		return &intentResult{Targets: plan.targets, Outcome: intentRefused, code: 2, Summary: "--retry takes the number of the review that failed; nothing was done",
			next: append(inv.typedArgvLess("retry"), "--retry", strconv.FormatInt(chain.NewestRound, 10)), nextReason: "the newest review"}
	}
	operation := fmt.Sprintf("design-%s-retry-%d", strings.ToLower(plan.recordID), round)
	for _, request := range entry.Requests {
		if request.OperationID == operation {
			return inv.designFollowUp(plan, chain, entry, request)
		}
	}
	if round != chain.NewestRound {
		return &intentResult{Targets: plan.targets, Outcome: intentRefused, code: 1,
			Summary: fmt.Sprintf("review %d is not design %s's newest review (%d); nothing was requested", round, plan.recordID, chain.NewestRound),
			next:    append(inv.typedArgvLess("retry"), "--retry", strconv.FormatInt(chain.NewestRound, 10)), nextReason: "retries the newest review"}
	}
	if status := recordText(chain.Newest, "status"); status == "completed" || status == "failed" {
		if entry.Subjects[strconv.FormatInt(round, 10)] != plan.subject {
			return inv.unknownDesignExamination(plan, chain, fmt.Errorf("restore the frozen candidate before retrying its examination"))
		}
		if err := dispatchcore.ReserveUnknownExaminationRetry(inv.layout.InstallationRoot.Path(), chain.NewestJob); err != nil {
			root, _ := inv.jobRecord(chain.Root)
			if recordText(root, "unknownExaminationRetryFrom") != chain.NewestJob {
				return inv.unknownDesignExamination(plan, chain, err)
			}
		}
	} else if err := dispatchcore.ExaminationRetryAdmissible(inv.layout.InstallationRoot.Path(), chain.Newest); err != nil {
		return inv.unknownDesignExamination(plan, chain, err)
	}
	if err := plan.writeMissingInputs(); err != nil {
		return &intentResult{Targets: plan.targets, Outcome: intentFailed, code: 1, Summary: "the review's inputs can't be written, so nothing was requested",
			next: inv.sameCommand(), nextReason: "try again; --verbose shows the cause", Details: []string{err.Error()}}
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
			return &intentResult{Targets: plan.targets, Outcome: intentFailed, code: 1, Summary: "the review request can't be saved, so nothing was requested",
				next: inv.sameCommand(), nextReason: "try again; --verbose shows the cause", Details: []string{err.Error()}}
		}
	}
	if request.Child == "" {
		// A lost response: the operation's own record names the child.
		if job, record, err := dispatchcore.FindOperationRecord(inv.layout.InstallationRoot.Path(), request.OperationID); err == nil && job != "" {
			if recordText(record, "parentJob") == "" || !designChildOf(inv, record, request.Root) {
				return &intentResult{Targets: plan.targets, Outcome: intentFailed, code: 1,
					Summary:  "this request is recorded on a job that belongs to another critique; nothing was requested",
					Decision: "nothing to do from here; the job records disagree and need a look (--verbose names them)",
					Details:  []string{fmt.Sprintf("operation %s is recorded on job %s, which is not a round of critique %s", request.OperationID, job, request.Root)}}
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
	if record, err := inv.jobRecord(request.Child); err == nil {
		if round := recordRound(record); round > 0 {
			entry.Subjects[strconv.FormatInt(round, 10)] = request.SubjectSHA256
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
	// A continuation says which round it asked for, and when that round is
	// the chain's last: its answer is the close.
	if record, err := inv.jobRecord(request.Child); err == nil && request.Kind == "continue" {
		round := recordRound(record)
		said := fmt.Sprintf("round %d of critique %s requested", round, request.Root)
		if round >= inv.designRoundLimit(request.Root) {
			said += ", the final round"
		}
		result.Summary = said + "; " + result.Summary
	}
	return &result
}

// designChildOf reports whether a job record is a round of the critique
// root, by its chain.
func designChildOf(inv *intentInvocation, record map[string]any, root string) bool {
	chainRoot, err := dispatchcore.ChainRootOf(inv.layout.InstallationRoot.Path(), recordText(record, "jobId"))
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
	if claimed, granted := inv.acquireClaim(goalID); !granted {
		claimed.Summary = fmt.Sprintf("the design critique claims goal %s first, and the claim was not granted: %s; no critique started", goalID, strings.TrimSpace(claimed.Summary))
		return &claimed
	}
	return nil
}

// recordFirstDesignExamination retains the design version the first
// examination of a new chain reads.
func (inv *intentInvocation) recordFirstDesignExamination(plan designReviewPlan) {
	chains := dispatchcore.DesignCritiqueChains(inv.layout.InstallationRoot.Path(), plan.goalID, plan.design)
	if len(chains) != 1 {
		return
	}
	entry, err := inv.readDesignReviewEntry(plan.recordID)
	if err != nil {
		return
	}
	if _, known := entry.Subjects["1"]; known {
		return
	}
	entry.Goal, entry.Design, entry.Subjects["1"] = plan.goalID, plan.design, plan.subject
	inv.writeDesignReviewEntry(plan.recordID, entry)
}

// closeDesignCritique completes a critique whose decisions answer the
// design version still in place, or its final round. Before the final round,
// an accepted material finding needs a changed design, which the next review
// examines, and it is never closed as clean. Every answered round's decisions
// reach the register first (the refuted, the out-of-scope, and an earlier
// round's accepted findings the follow-up did not raise again); the whole
// close owner then closes the chain by its own classification, and the
// close's last act appends the design's Dispositions section.
func (inv *intentInvocation) closeDesignCritique(plan designReviewPlan, chain dispatchcore.DesignCritiqueChain, returnPath, dispositions string, round int64, final bool) *intentResult {
	findings, _, err := readIntentFindings(returnPath)
	if err != nil {
		return &intentResult{Targets: plan.targets, Outcome: intentFailed, code: 1, Summary: "the review's findings can't be read, so nothing was closed",
			next: inv.sameCommand(), nextReason: "try again; --verbose shows the cause", Details: []string{err.Error()}}
	}
	decisions, violations := validate.Dispositions(dispositions)
	if len(violations) > 0 {
		return &intentResult{Targets: plan.targets, Outcome: intentRefused, code: 1, text: violations,
			Summary: "the decisions file leaves findings undecided (listed above); nothing was closed",
			next:    inv.sameCommand(), nextReason: "once every finding has a decision"}
	}
	var accepted []string
	for _, finding := range findings {
		if finding.Material && decisions[finding.ID] == "accepted" {
			accepted = append(accepted, finding.ID)
		}
	}
	if len(accepted) > 0 && !final {
		return &intentResult{Targets: plan.targets, Outcome: intentRefused, code: 1, Data: map[string]any{"accepted": accepted},
			Summary: fmt.Sprintf("design %s is unchanged, but you accepted finding(s) %s; nothing was closed", plan.recordID, strings.Join(accepted, ", ")),
			next:    inv.sameCommand(), nextReason: "after changing the design to address them; the critique then reviews the new version"}
	}
	if required, err := dispatchcore.DesignEvidenceRequired(inv.layout.InstallationRoot.Path(), chain.Root, round); final && err == nil && required {
		if err := inv.prepareDesignFold(plan, chain, decisions); err != nil {
			return &intentResult{Targets: plan.targets, Outcome: intentFailed, code: 1, Summary: "the final design revision is incomplete: " + err.Error(), next: inv.sameCommand(), nextReason: "correct the retained proposal, then collect the same revision"}
		}
	}
	if published := inv.finishDesignAcceptance(plan, chain, true); published != nil {
		return published
	}
	if err := dispatchcore.CritiqueRegisterApplyDecisions(inv.layout.InstallationRoot.Path(), chain.Root, inv.registerDecisions(chain.Root, round)); err != nil {
		return withCauseRef(err, intentResult{Targets: plan.targets, Outcome: intentRefused, code: 1,
			Summary: "the decisions can't be recorded, so nothing was closed",
			next:    inv.sameCommand(), nextReason: "try again; --verbose shows the cause", Details: []string{fmt.Sprintf("critique %s: %v", chain.Root, err)}})
	}
	closed := inv.closeChain(chain.Root)
	closed.Targets = append(append([]intentTarget{}, plan.targets...), closed.Targets...)
	if closed.Outcome != intentConfirmed && closed.Outcome != intentUnchanged {
		return &closed
	}
	return inv.designCritiqueClosed(plan, chain, closed)
}

// designCritiqueClosed says how the closed chain exited, in the engine's
// words, and appends the design's Dispositions section as the close's last
// act. A section that could not be written leaves the chain closed; the same
// command writes it.
func (inv *intentInvocation) designCritiqueClosed(plan designReviewPlan, chain dispatchcore.DesignCritiqueChain, closed intentResult) *intentResult {
	obligations := inv.deferredObligations(chain.Root)
	closed.Summary = fmt.Sprintf("design %s's critique is complete: every finding is decided and chain %s is closed", plan.recordID, chain.Root)
	if len(obligations) > 0 {
		noun := "obligations"
		if len(obligations) == 1 {
			noun = "obligation"
		}
		closed.Summary = fmt.Sprintf("design %s's critique closed at round %d on %d fixture %s; chain %s is closed and each obligation is published on goal %s",
			plan.recordID, chain.NewestRound, len(obligations), noun, chain.Root, plan.goalID)
		closed.text = append(closed.text, obligations...)
	}
	appended, err := appendDesignDispositions(plan.design, chain.Root, inv.answeredDesignRounds(chain.Root))
	data, _ := closed.Data.(map[string]any)
	if data == nil {
		data = map[string]any{}
	}
	data["closedAt"], data["obligations"], data["dispositions"] = chain.NewestRound, obligations, appended
	closed.Data = data
	if err != nil {
		closed.Outcome, closed.code = intentFailed, 1
		closed.Summary += fmt.Sprintf("; the design's Dispositions section could not be written: %v", err)
		closed.next, closed.nextReason = inv.sameCommand(), "the same answer writes the section onto the closed chain's design"
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
	data, _ := result.Data.(map[string]any)
	findings, present := data["findings"].([]intentFinding)
	if recordText(chain.Newest, "status") != "completed" || result.Outcome == intentFailed || !present {
		return &result
	}
	returnPath := inv.returnPathAt(inv.layout.InstallationRoot.Path(), chain.Root, chain.NewestRound)
	digest, _, err := reviewReturnDigest(returnPath)
	if err != nil {
		return &result
	}
	binding := reviewBinding{Goal: plan.goalID, Work: "design:" + plan.recordID, Attempt: int(chain.NewestRound), Subject: examined,
		Examination: chain.Root, Round: chain.NewestRound, Return: digest}
	template := filepath.Join(filepath.Dir(returnPath), "decisions.md")
	if _, statErr := os.Stat(template); statErr != nil {
		os.WriteFile(template, []byte(decisionsDocument(binding, findings)), 0o600)
	}
	data["template"], data["examination"] = template, chain.NewestRound
	result.Data = data
	result.next, result.nextReason = append(inv.typedArgvLess("retry", "after", "dispositions"), "--dispositions", template), "after deciding every finding in "+template
	return &result
}

// designDrafts is what a design review freezes from the seat's checkout: the
// design page as it is now, and every file the texts cite that HEAD does not
// hold yet (a draft brief, say). Each copy is content-addressed under the
// review's frozen folder and written with the review's other inputs; the
// brief names it on a line the brief-authority admission reads (item 59).
type designDrafts struct {
	pageCopy string
	lines    []string
	files    map[string]string // copy path -> bytes
}

// freezeDesignDrafts freezes the page (git-relative pageRel, bytes page),
// when page is not nil, and the drafts the texts cite. A draft is frozen only
// when HEAD lacks it and the checkout holds it as a regular file; anything
// else is left to the admission, which refuses a path it cannot find. When
// HEAD can't be read, no cited draft is frozen and the admission refuses as
// before.
func (inv *intentInvocation) freezeDesignDrafts(git, dir, pageRel string, page []byte, texts ...[]byte) designDrafts {
	draftPaths := dispatchcore.BriefDraftPaths
	if inv.owners.delivery != nil && inv.owners.delivery.draftPaths != nil {
		draftPaths = inv.owners.delivery.draftPaths
	}
	drafts := designDrafts{files: map[string]string{}}
	frozen := map[string]bool{}
	freeze := func(rel string, content []byte) string {
		digest := sha256.Sum256(content)
		sum := hex.EncodeToString(digest[:])
		copyPath := filepath.Join(dir, "frozen", sum[:16], filepath.Base(filepath.FromSlash(rel)))
		drafts.files[copyPath] = string(content)
		drafts.lines = append(drafts.lines, dispatchcore.FrozenInputLine(rel, sum, copyPath))
		frozen[rel] = true
		return copyPath
	}
	if page != nil {
		drafts.pageCopy = freeze(pageRel, page)
	}
	for _, text := range texts {
		cited, err := draftPaths(text, git)
		if err != nil {
			continue
		}
		for _, rel := range cited {
			if frozen[rel] {
				continue
			}
			path := filepath.Join(git, filepath.FromSlash(rel))
			if info, err := os.Lstat(path); err != nil || !info.Mode().IsRegular() {
				continue
			}
			if content, err := os.ReadFile(path); err == nil {
				freeze(rel, content)
			}
		}
	}
	return drafts
}

// section is the brief section naming the frozen copies.
func (drafts designDrafts) section() string {
	if len(drafts.lines) == 0 {
		return ""
	}
	return "\n## Frozen drafts\n\nThese files are frozen as the seat's checkout held them when the review was asked. Read each from its copy,\nnever from the checkout, which may have moved since:\n\n" +
		strings.Join(drafts.lines, "\n") + "\n"
}

// closeDesignJob resolves a public job alias to its current design page,
// retaining the canonical root and using the design publication owner.
func (inv *intentInvocation) closeDesignJob(job, goalID string) intentResult {
	fail := func(err error) intentResult {
		return intentResult{Targets: []intentTarget{jobTarget(job)}, Outcome: intentFailed, code: 1, Summary: "the design history is unknown; nothing was closed", Details: []string{err.Error()}, next: inv.sameCommand(), nextReason: "restore this design's retained identity, page and frozen subjects"}
	}
	state, err := inv.owners.resolver.RootForInstallation(inv.layout.InstallationRoot)
	if err != nil {
		return fail(err)
	}
	pages, err := project.Read(project.Roots{Checkout: inv.layout.GitRoot, Installation: inv.layout.InstallationRoot, StateRoot: state})
	if err != nil {
		return fail(err)
	}
	for _, page := range pages.List(project.KindDesign, project.ListOptions{Goal: goalID}) {
		path := filepath.Join(inv.layout.GitRoot, filepath.FromSlash(page.Path))
		chains, err := dispatchcore.ReadDesignCritiqueChains(inv.layout.InstallationRoot.Path(), goalID, path)
		for _, chain := range chains {
			if chain.Root == job {
				if err != nil {
					return fail(err)
				}
				root, _ := inv.jobRecord(job)
				decision, _ := root["designDecision"].(map[string]any)
				if chain.Closed || !inv.input.has("dispositions") || recordText(decision, "decision") == "close" || recordText(decision, "decision") == "stop" {
					data, err := os.ReadFile(path)
					if err != nil {
						return fail(err)
					}
					plan := designReviewPlan{root: job, targets: []intentTarget{{Kind: "design", ID: path}}, goalID: goalID, recordID: page.ID, design: path, subject: digestText(data)}
					if result := inv.reviewDesignChain(plan); result != nil {
						return *result
					}
					return fail(fmt.Errorf("critique %s lost its canonical root", job))
				}
				closer := *inv
				args := []string{"design", "review", path, "--dispositions", inv.flagPath("dispositions")}
				if calls := inv.input.text("tool-calls"); calls != "" {
					args = append(args, "--tool-calls", calls)
				}
				closer.command, _ = findIntentAction("design", "review")
				closer.raw = args[2:]
				return closer.reviewDesign(path, job)
			}
		}
	}
	return fail(fmt.Errorf("critique %s has no current design with its retained identity", job))
}
