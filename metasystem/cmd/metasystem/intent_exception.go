package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/landpath"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testrun"
)

// Exceptional landing is a person's explicit act: the goal's whole-project
// landing candidate is computed by the landing path, the person's exception
// is recorded by the carry owner (with its enrolled proof, lifetime, one
// exception and replacement rules), and the existing carried landing
// transaction (landpath.Land with Carried, with its proof token, reservation and
// consumption) delivers it. A repeated request rejoins the recorded
// exception for the same candidate instead of recording another.

var exceptionOptions = []string{"exception", "reason", "by", "expires", "replace-exception", "transfer", "upgrade-goals"}

// landExceptionInput refuses a malformed exceptional landing before any
// repository or owner is read.
func (inv *intentInvocation) landExceptionInput(goalID string) *intentResult {
	result := inv.landException(goalID, true)
	if result.Outcome == "" {
		return nil
	}
	return &result
}

func (inv *intentInvocation) landException(goalID string, validateOnly ...bool) intentResult {
	targets := inv.targets(goalID)
	using := inv.input.text("using-exception")
	if using != "" {
		for _, other := range exceptionOptions {
			if inv.input.has(other) {
				return intentResult{Outcome: intentRefused, code: 2, Targets: targets,
					Summary: fmt.Sprintf("--using-exception lands under an exception already recorded, so it takes no --%s; nothing was done", other),
					next:    withoutOption(inv.typedArgv(), other)}
			}
		}
	} else {
		if problem := inv.requireInputs("exception", "reason", "by"); problem != nil {
			return *problem
		}
		if inv.input.has("transfer") && !inv.input.has("replace-exception") {
			return intentResult{Outcome: intentRefused, code: 2, Targets: targets,
				Summary: "--transfer takes over another seat's exception and needs --replace-exception; nothing was done",
				next:    append(inv.typedArgv(), "--replace-exception", "ID"), nextReason: "naming the exception it replaces"}
		}
	}
	expires := 2 * time.Hour
	if inv.input.has("expires") {
		value, err := time.ParseDuration(inv.input.text("expires"))
		if err != nil || value <= 0 || value > 4*time.Hour {
			return intentResult{Outcome: intentRefused, code: 2, Targets: targets,
				Summary: fmt.Sprintf("--expires %s isn't a time of at most 4h, such as 2h; nothing was done", inv.input.text("expires")),
				next:    append(withoutOption(inv.typedArgv(), "expires"), "--expires", "2h")}
		}
		expires = value
	}
	if inv.input.has("through") {
		return intentResult{Outcome: intentRefused, code: 2, Targets: targets,
			Summary: "an exception covers all of the goal's work, so it takes no --through; nothing was done",
			next:    withoutOption(inv.typedArgv(), "through")}
	}
	if len(validateOnly) > 0 && validateOnly[0] {
		return intentResult{}
	}
	// An exception carries a landing past an agent refusal; it is not the
	// human's current-tip word, and the landing gate still binds (g1-s70 D2).
	if refused := inv.admitLanding(targets, goalID, inv.intentBranchTip(goalID)); refused != nil {
		return *refused
	}
	root := inv.layout.InstallationRoot
	opid := using
	data := map[string]any{"goal": goalID}
	// Everything the landing touches is the main installation: a request
	// from a goal's linked checkout delivers into the checkout it belongs to.
	primary := goalBranchHolderRoot(root)
	subjects := filepath.Join(primary, "artifacts", "agents", "intent-land", goalID)
	if _, err := (gittree.Workspace{Dir: primary}).TopLevel(); err != nil {
		return intentResult{Outcome: intentFailed, code: 1, Targets: targets, Summary: "git can't read the main checkout, so nothing was recorded",
			next: []string{"git", "-C", primary, "status"}, nextReason: "shows what git reports; then repeat this command",
			Details: []string{"the main checkout cannot be read: " + err.Error()}}
	}
	if head, err := inv.work().git(primary, "symbolic-ref", "-q", "HEAD"); err != nil || strings.TrimSpace(string(head)) != "refs/heads/main" {
		return intentResult{Outcome: intentRefused, code: 2, Targets: targets,
			Summary: "an exception lands main, and the main checkout is on another branch; nothing was recorded",
			next:    []string{"git", "-C", primary, "switch", "main"}, nextReason: "then repeat this command"}
	}
	if _, problem := inv.carriedRefresh(primary, targets, data); problem != nil {
		return *problem
	}
	// A pushed exception landing whose release was cut short is finished
	// first (disk-lifetimes Part B 3.6).
	inv.finishReleaseSets(filepath.Join(primary, "artifacts", "agents", "landing-intent", goalID))
	var subject landing.CarriedSubject
	if opid == "" {
		composed, composedTip, problem := inv.composeCarriedSubject(root, primary, goalID, targets, data)
		if problem != nil {
			return *problem
		}
		if err := landing.RetainCarriedSubject(subjects, composed); err != nil {
			return intentResult{Outcome: intentFailed, code: 1, Targets: targets, Data: data,
				Summary: "the goal's work couldn't be saved for the exception, so nothing was recorded",
				next:    inv.sameCommand(), nextReason: "tries again", Details: []string{"the composed candidate cannot be retained: " + err.Error()}}
		}
		tree := composed.Workspace
		data["candidate"], data["tree"], data["endpoint"] = tree, tree, composed.Endpoint
		if opid = inv.recordedException(goalID, tree); opid == "" {
			args := []string{"--root", inv.stateRoot, "--id", goalID, "--by", inv.input.text("by"), "--tree", tree,
				"--past", inv.input.text("exception"), "--why", inv.input.text("reason"), "--expires", expires.String()}
			if inv.input.has("replace-exception") {
				args = append(args, "--supersede", inv.input.text("replace-exception"))
			}
			if inv.input.switched("transfer") {
				args = append(args, "--transfer")
			}
			if inv.input.switched("upgrade-goals") {
				args = append(args, "--raise-format")
			}
			ran := inv.goalOwnerCall(inv.ownerCalls().goalCarry, args...)
			if recorded := ownerVerbResult(ran, targets, "", data); recorded.Outcome != intentConfirmed {
				recorded.Summary = "the exception was not recorded: " + recorded.Summary
				return recorded
			}
			if opid = inv.recordedException(goalID, tree); opid == "" {
				return intentResult{Outcome: intentFailed, code: 1, Targets: targets, Data: data,
					Summary: "the exception was recorded but can't be read back for this work yet", next: inv.sameCommand(),
					nextReason: "picks up the recorded exception; it records no second one"}
			}
			subject = composed
		} else {
			data["rejoined"] = true
			// A rejoined exception keeps the composition it was recorded
			// for; newer goal work or a newer endpoint never changes it.
			bound, present, err := landing.BoundCarriedSubject(subjects, opid)
			if err == nil && !present {
				bound, err = landing.RetainedCarriedSubject(subjects, goalID, tree)
			}
			if err != nil {
				return inv.carriedUnidentified(goalID, opid, primary, targets, data, err)
			}
			subject = bound
		}
		if err := landing.BindCarriedSubject(subjects, opid, subject); err != nil {
			return inv.carriedStopped(goalID, opid, targets, data, notTied, "its composition cannot be bound: "+err.Error())
		}
		if subject.Workspace == composed.Workspace {
			inv.noteExceptionRelease(primary, goalID, opid, composedTip, data)
		}
	}
	data["exception"] = opid
	// The word's own state decides the path: a consumed word resumes
	// through the carried transaction's recovery without staging, and an
	// unusable word keeps the carry owner's own refusal.
	tip, problem := inv.carriedRefresh(primary, targets, data)
	if problem != nil {
		return *problem
	}
	status, err := landing.ReadCarryStatus(primary, opid, goalID, tip, time.Now().UTC())
	if err != nil {
		return inv.carriedStopped(goalID, opid, targets, data, "its state can't be read from the goal records", "its carry status cannot be read: "+err.Error())
	}
	data["word"], data["consumption"] = status.Word, status.Consumption
	acknowledgePlans := false
	if status.Word == "ok" && status.Consumption == "none" {
		if subject.Workspace == "" {
			bound, present, err := landing.BoundCarriedSubject(subjects, opid)
			if err != nil {
				return inv.carriedStopped(goalID, opid, targets, data, notTied, "its composition cannot be read: "+err.Error())
			}
			if !present {
				// A word whose binding was lost keeps the one composition
				// retained for its workspace; land never chooses between two.
				retained, retainedErr := landing.RetainedCarriedSubject(subjects, goalID, status.Workspace)
				var refusal *landing.CarriedRefusal
				if retainedErr == nil {
					if err := landing.BindCarriedSubject(subjects, opid, retained); err != nil {
						return inv.carriedStopped(goalID, opid, targets, data, notTied, "its composition cannot be bound: "+err.Error())
					}
					bound, present = retained, true
				} else if !errors.As(retainedErr, &refusal) || refusal.Code != "carried-subject-missing" {
					return inv.carriedUnidentified(goalID, opid, primary, targets, data, retainedErr)
				}
			}
			if !present {
				// A word recorded elsewhere (a channel answer) is adopted
				// only when the goal composes to exactly its workspace now.
				composed, composedTip, problem := inv.composeCarriedSubject(root, primary, goalID, targets, data)
				if problem != nil {
					return inv.carriedStopped(goalID, opid, targets, data, strings.TrimSuffix(problem.Summary, ", so nothing was recorded"),
						append([]string{"its candidate cannot be composed"}, problem.Details...)...)
				}
				if composed.Workspace != status.Workspace {
					return inv.carriedStopped(goalID, opid, targets, data, "the goal's work has changed since it was recorded",
						fmt.Sprintf("the goal now composes workspace %s, not the exception's %s", composed.Workspace, status.Workspace))
				}
				bindErr := landing.RetainCarriedSubject(subjects, composed)
				if bindErr == nil {
					bindErr = landing.BindCarriedSubject(subjects, opid, composed)
				}
				if bindErr != nil {
					return inv.carriedStopped(goalID, opid, targets, data, notTied, "its composition cannot be bound: "+bindErr.Error())
				}
				inv.noteExceptionRelease(primary, goalID, opid, composedTip, data)
				bound = composed
			}
			subject = bound
		}
		if subject.Workspace != status.Workspace {
			return inv.carriedStopped(goalID, opid, targets, data, "the goal's work has changed since it was recorded",
				fmt.Sprintf("its bound composition has workspace %s, not the word's %s", subject.Workspace, status.Workspace))
		}
		stage := landing.CarriedStage{Root: primary, Upstream: "refs/remotes/origin/main", Subject: subject, Exception: status.Past, Opid: opid}
		err := landing.CarriedAdvance(primary, stage.Upstream, subject)
		var staged landing.CarriedStaged
		if err == nil {
			staged, err = landing.StageCarriedCandidate(stage)
		}
		if err != nil {
			var refusal *landing.CarriedRefusal
			if errors.As(err, &refusal) && refusal.Code == "carried-origin-moved" {
				result := inv.carriedStopped(goalID, opid, targets, data, "main moved since, so it no longer covers the goal's work on main", err.Error())
				result.next, result.nextReason = inv.carriedReplacement(goalID, opid, status, "main moved after the exception was recorded")
				return result
			}
			return inv.carriedStopped(goalID, opid, targets, data, "the goal's work couldn't be staged: "+oneLine(carriedWhy(err)),
				"its candidate was not staged: "+err.Error())
		}
		data["staged"], data["receipt"] = map[string]any{"applied": staged.Applied, "tree": staged.Tree}, staged.Receipt
		// The pre-commit guard keeps a peer's unexamined new plan out of a
		// hand-staged commit. Staging just validated, under the checkout
		// lock, that the index is exactly this exception's retained
		// candidate, so any plan it adds is the goal's own selected work.
		acknowledgePlans = true
	}
	message := filepath.Join(subjects, "exception-"+opid+".txt")
	if err := os.MkdirAll(filepath.Dir(message), 0o755); err != nil {
		return inv.carriedStopped(goalID, opid, targets, data, "its commit message couldn't be written", err.Error())
	}
	if _, err := os.Stat(message); err != nil {
		text := fmt.Sprintf("Land %s under a recorded exception\n\nException: %s\n", goalID, opid)
		if err := os.WriteFile(message, []byte(text), 0o644); err != nil {
			return inv.carriedStopped(goalID, opid, targets, data, "its commit message couldn't be written", err.Error())
		}
	}
	// The carried transaction runs in this process (landpath.Land); the
	// seat's lineage is read once here, at the entry, and named on it. Its
	// stop is the result's two lines; its step log the result's details.
	stop := &landpath.Stop{}
	ran := inv.delivery().landCarried(landpath.LandRequest{Root: primary, MessageFile: message, Goal: goalID, GoalSet: true,
		Carried: opid, StagedOnly: true, AllowNewPlan: acknowledgePlans, OwnerLineage: os.Getenv("METASYSTEM_OWNER_LINEAGE"), Stop: stop}, inv.pushGate(),
		inv.exceptionRelease(primary, goalID, opid))
	// A landing recovered from its recorded consumption pushed nothing in
	// this process: once the carried transaction completed, its set is
	// finished here when the pushed commit is on the remote-tracking ref. A
	// transaction that stopped leaves it to the next work land of the goal
	// and the sweeper's retry.
	var released intentLanded
	if ran.code == 0 {
		var recorded bool
		if released, recorded = inv.finishExceptionRelease(primary, goalID, opid); recorded {
			data["releaseSet"] = released.ReleaseSet
		}
	}
	result := ownerVerbResult(ran, targets, fmt.Sprintf("goal %s landed under exception %s%s", goalID, opid, releaseSummary(released.ReleaseSet)), data)
	// The landing's step log is detail; a landing that went well says so in
	// one line.
	log := append(nonEmptyLines(string(ran.stdout)), extraToldLines(string(ran.stderr), *stop)...)
	result.text = nil
	if result.Outcome != intentConfirmed {
		why := stop.Reason
		if why == "" {
			why = result.Summary
		}
		stopped := inv.carriedStopped(goalID, opid, targets, result.Data.(map[string]any), why, log...)
		switch {
		case len(stop.Run) > 0:
			stopped.next, stopped.nextReason = stop.Run, stop.Then
		case stop.Then != "" && !strings.Contains(stop.Then, "repeat this command"):
			stopped.next, stopped.nextReason, stopped.Decision = nil, "", stop.Then
		}
		refusal := string(ran.stdout) + string(ran.stderr)
		if strings.Contains(refusal, "carry-battery-unverified") && strings.Contains(refusal, testrun.ErrRetainedCandidateEngineAbsent.Error()) {
			// The carried transaction verifies retained proof; it never
			// runs tests. The person proves the staged candidate with the
			// public test command, then continues under the same word.
			continuation := inv.publicArgv("work", "land", goalID, "--using-exception", opid)
			stopped.Summary = fmt.Sprintf("exception %s is recorded and its work is staged, but no test run has passed for it yet", opid)
			stopped.next = []string{"metasystem", "test", "run", "--goal", goalID, "--repo", primary}
			stopped.nextReason = "then continue under the same exception: " + shellCommand(continuation)
			stopped.Decision = ""
			stopped.Data.(map[string]any)["afterProof"] = continuation
		}
		return stopped
	}
	result.Details = append(result.Details, log...)
	return result
}

// carriedWhy is a carried refusal's words without its code.
func carriedWhy(err error) string {
	var refusal *landing.CarriedRefusal
	if errors.As(err, &refusal) && refusal.Detail != "" {
		return refusal.Detail
	}
	return err.Error()
}

// notTied is why an exception stopped when its files could not be tied to it.
const notTied = "its files couldn't be tied to it"

// extraToldLines are the lines a landing wrote for the person besides its
// stop's two.
func extraToldLines(told string, stop landpath.Stop) []string {
	var extra []string
	for _, line := range nonEmptyLines(told) {
		if line == stop.Reason || strings.HasPrefix(line, "run: ") || strings.HasPrefix(line, "needed first: ") {
			continue
		}
		extra = append(extra, line)
	}
	return extra
}

// carriedReplacement is the public replacement of a recorded word, from
// the word's own recorded refusal and person: the one explicit decision
// that authorizes the goal's composition on the current main.
func (inv *intentInvocation) carriedReplacement(goalID, opid string, status landing.CarryStatus, reason string) ([]string, string) {
	return inv.publicArgv("work", "land", goalID, "--exception", status.Past, "--replace-exception", opid,
			"--reason", reason, "--by", strings.TrimPrefix(status.By, "human:")),
		"a person replaces the exception for the candidate composed on the current main"
}

// carriedUnidentified stops a word whose composition cannot be identified
// before anything is bound, staged or delivered. When more than one
// retained composition has its workspace, land does not guess between
// them: the remedy is the person's explicit replacement.
func (inv *intentInvocation) carriedUnidentified(goalID, opid, primary string, targets []intentTarget, data map[string]any, err error) intentResult {
	result := inv.carriedStopped(goalID, opid, targets, data, notTied, "its composition cannot be identified: "+err.Error())
	var refusal *landing.CarriedRefusal
	if !errors.As(err, &refusal) || refusal.Code != "carried-subject-ambiguous" {
		return result
	}
	tip, problem := inv.carriedRefresh(primary, targets, data)
	if problem != nil {
		return *problem
	}
	status, statusErr := landing.ReadCarryStatus(primary, opid, goalID, tip, time.Now().UTC())
	if statusErr != nil || status.Past == "" || status.By == "" {
		return result
	}
	result.next, result.nextReason = inv.carriedReplacement(goalID, opid, status, "the recorded exception's composition is ambiguous")
	return result
}

// carriedStopped is a stop after the exception is recorded: the word stays,
// and the same exception continues the landing. why is line 1's plain cause;
// details are what only --verbose shows.
func (inv *intentInvocation) carriedStopped(goalID, opid string, targets []intentTarget, data map[string]any, why string, details ...string) intentResult {
	return intentResult{Outcome: intentPartial, code: 1, Targets: targets, Data: data,
		Summary: fmt.Sprintf("exception %s is recorded, but %s", opid, why),
		next:    inv.publicArgv("work", "land", goalID, "--using-exception", opid), nextReason: "continues under the same exception once that is fixed",
		Details: details}
}

// carriedRefresh fetches the code origin and the goal ledger the carried
// transaction reads, and returns the accepted ledger tip. It never creates
// the origin remote.
func (inv *intentInvocation) carriedRefresh(primary string, targets []intentTarget, data map[string]any) (string, *intentResult) {
	if _, err := inv.work().git(primary, "fetch", "--quiet", "origin", "+refs/heads/main:refs/remotes/origin/main"); err != nil {
		return "", &intentResult{Outcome: intentRefused, code: 1, Targets: targets, Data: data,
			Summary: "origin can't be fetched, so nothing more was done", next: []string{"git", "-C", primary, "fetch", "origin"},
			nextReason: "shows why; then repeat this command", Details: []string{"the code origin cannot be fetched: " + err.Error()}}
	}
	ran := ownerCall(func(stdout, stderr io.Writer) int { return inv.ownerCalls().goalFetch(stdout, stderr, primary) })
	output := string(ran.stdout)
	_, tip, _ := strings.Cut(output, "tip=")
	tip, _, _ = strings.Cut(tip, " ")
	if ran.code != 0 || len(strings.TrimSpace(tip)) != 40 {
		fetched := ownerVerbResult(ran, targets, "", data)
		return "", &intentResult{Outcome: intentRefused, code: 1, Targets: targets, Data: data,
			Summary: "the goal records can't be fetched, so nothing more was done", next: inv.publicArgv("goal", "sync"),
			nextReason: "then repeat this command", Details: append([]string{"the goal ledger cannot be fetched: " + fetched.Summary}, fetched.text...)}
	}
	return strings.TrimSpace(tip), nil
}

// composeCarriedSubject composes the goal's canonical landing candidate on
// the live code endpoint and names it with that endpoint's product.
// The goal branch tip it was composed from is returned too, read before
// and after the composition; a tip that moved meanwhile, or could not be
// read, is empty.
func (inv *intentInvocation) composeCarriedSubject(root, primary, goalID string, targets []intentTarget, data map[string]any) (landing.CarriedSubject, string, *intentResult) {
	before := inv.intentBranchTip(goalID)
	candidate, code, err := inv.delivery().landCandidate([]string{"--root", root, "--goal", goalID, "--last"})
	if err != nil {
		return landing.CarriedSubject{}, "", withCauseRef(err, intentResult{Outcome: intentRefused, code: max(code, 1), Targets: targets, Data: data,
			Summary: "the goal's work can't be put together on top of main, so nothing was recorded",
			next:    inv.sameCommand(), nextReason: "once what --verbose shows (a conflict with main, say) is fixed",
			Details: []string{"the landing candidate cannot be composed: " + err.Error()}})
	}
	workspace, err := landing.ProjectWorkspaceTree(primary, candidate.Result.Candidate)
	var endpointWorkspace string
	if err == nil {
		var tree []byte
		if tree, err = inv.work().git(primary, "rev-parse", "--verify", candidate.Result.Endpoint+"^{tree}"); err == nil {
			endpointWorkspace, err = landing.ProjectWorkspaceTree(primary, strings.TrimSpace(string(tree)))
		}
	}
	if err != nil {
		return landing.CarriedSubject{}, "", &intentResult{Outcome: intentFailed, code: 1, Targets: targets, Data: data,
			Summary: "the goal's files can't be read, so nothing was recorded", next: inv.sameCommand(), nextReason: "tries again",
			Details: []string{"the candidate's workspace cannot be read: " + err.Error()}}
	}
	tip := ""
	if after := inv.intentBranchTip(goalID); before != "" && after == before {
		tip = before
	}
	return landing.CarriedSubject{Goal: goalID, Endpoint: candidate.Result.Endpoint, EndpointWorkspace: endpointWorkspace, Workspace: workspace}, tip, nil
}

// noteExceptionRelease records the exception's release set at its
// selection; a set that cannot be recorded is reported and the landing goes
// on, releasing nothing.
func (inv *intentInvocation) noteExceptionRelease(primary, goalID, opid, tip string, data map[string]any) {
	if err := inv.recordExceptionRelease(primary, goalID, opid, tip); err != nil {
		data["releaseError"] = err.Error()
	}
}

// recordedException is the open exception already recorded for this goal
// and exact candidate tree, read through the carry owner's own records.
func (inv *intentInvocation) recordedException(goalID, tree string) string {
	projection, _, problem := inv.projection()
	if problem != nil {
		return ""
	}
	machine, err := goal.ResolveMachine(inv.stateRoot)
	if err != nil {
		return ""
	}
	projected, err := landing.ProjectWorkspaceTree(inv.stateRoot, tree)
	if err != nil {
		return ""
	}
	words, err := goal.OpenCarryWords(inv.stateRoot, projection.Tree, "refs/remotes/origin/main", machine, time.Now().UTC())
	if err != nil {
		return ""
	}
	for _, word := range words {
		// An explicit replacement never rejoins the word it replaces.
		if inv.input.has("replace-exception") && word.History.Opid == inv.input.text("replace-exception") {
			continue
		}
		if word.Goal == goalID && word.Workspace == projected && (!inv.input.has("exception") || word.Past == inv.input.text("exception")) {
			return word.History.Opid
		}
	}
	return ""
}
