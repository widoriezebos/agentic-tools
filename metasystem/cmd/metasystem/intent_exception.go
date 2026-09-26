package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing"
)

// Exceptional landing is a person's explicit act: the goal's whole-project
// landing candidate is computed by the landing owner, the person's exception
// is recorded by the carry owner (with its enrolled proof, lifetime, one
// exception and replacement rules), and the existing carried landing
// transaction (land.sh --carried, with its proof token, reservation and
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
				return intentResult{Outcome: intentRefused, code: 2, Targets: targets, Summary: fmt.Sprintf("--using-exception lands under a recorded exception and takes no --%s; nothing was done", other)}
			}
		}
	} else {
		if problem := inv.requireInputs("exception", "reason", "by"); problem != nil {
			return *problem
		}
		if inv.input.has("transfer") && !inv.input.has("replace-exception") {
			return intentResult{Outcome: intentRefused, code: 2, Targets: targets, Summary: "--transfer moves a replaced exception from another seat and needs --replace-exception ID; nothing was done"}
		}
	}
	expires := 2 * time.Hour
	if inv.input.has("expires") {
		value, err := time.ParseDuration(inv.input.text("expires"))
		if err != nil || value <= 0 || value > 4*time.Hour {
			return intentResult{Outcome: intentRefused, code: 2, Targets: targets, Summary: "--expires is a positive duration of at most 4h, such as 2h; nothing was done"}
		}
		expires = value
	}
	if inv.input.has("through") {
		return intentResult{Outcome: intentRefused, code: 2, Targets: targets, Summary: "an exception covers the goal's whole landing candidate; --through is not taken with it; nothing was done"}
	}
	if len(validateOnly) > 0 && validateOnly[0] {
		return intentResult{}
	}
	root := inv.layout.InstallationRoot
	opid := using
	data := map[string]any{"goal": goalID}
	// Everything the landing touches is the main installation: a request
	// from a goal's linked checkout delivers into the checkout it belongs to.
	primary := goalBranchHolderRoot(root)
	subjects := filepath.Join(primary, "artifacts", "agents", "intent-land", goalID)
	top, err := (gittree.Workspace{Dir: primary}).TopLevel()
	if err != nil {
		return intentResult{Outcome: intentFailed, code: 1, Targets: targets, Summary: "the main checkout cannot be read: " + err.Error() + "; nothing was recorded"}
	}
	if head, err := inv.work().git(primary, "symbolic-ref", "-q", "HEAD"); err != nil || strings.TrimSpace(string(head)) != "refs/heads/main" {
		return intentResult{Outcome: intentRefused, code: 2, Targets: targets, Summary: fmt.Sprintf("the main checkout %s is not on main; an exception lands main; nothing was recorded", primary)}
	}
	if _, problem := inv.carriedRefresh(primary, targets, data); problem != nil {
		return *problem
	}
	var subject landing.CarriedSubject
	if opid == "" {
		composed, problem := inv.composeCarriedSubject(root, primary, goalID, targets, data)
		if problem != nil {
			return *problem
		}
		if err := landing.RetainCarriedSubject(subjects, composed); err != nil {
			return intentResult{Outcome: intentFailed, code: 1, Targets: targets, Data: data, Summary: "the composed candidate cannot be retained: " + err.Error() + "; nothing was recorded"}
		}
		tree := composed.Workspace
		data["candidate"], data["tree"], data["endpoint"] = tree, tree, composed.Endpoint
		if opid = inv.recordedException(goalID, tree); opid == "" {
			args := []string{"goal", "carry", "--root", inv.stateRoot, "--id", goalID, "--by", inv.input.text("by"), "--tree", tree,
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
			ran, problem := inv.engineVerb(args...)
			if problem != nil {
				return *problem
			}
			if recorded := ownerVerbResult(ran, targets, "", data); recorded.Outcome != intentConfirmed {
				recorded.Summary = "the exception was not recorded: " + recorded.Summary
				return recorded
			}
			if opid = inv.recordedException(goalID, tree); opid == "" {
				return intentResult{Outcome: intentFailed, code: 1, Targets: targets, Data: data,
					Summary: "the carry owner confirmed the exception but it cannot be read back for this candidate", next: inv.sameCommand(), nextReason: "the same request rejoins the recorded exception"}
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
			return inv.carriedStopped(goalID, opid, targets, data, "its composition cannot be bound: "+err.Error())
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
		return inv.carriedStopped(goalID, opid, targets, data, "its carry status cannot be read: "+err.Error())
	}
	data["word"], data["consumption"] = status.Word, status.Consumption
	acknowledgePlans := false
	if status.Word == "ok" && status.Consumption == "none" {
		if subject.Workspace == "" {
			bound, present, err := landing.BoundCarriedSubject(subjects, opid)
			if err != nil {
				return inv.carriedStopped(goalID, opid, targets, data, "its composition cannot be read: "+err.Error())
			}
			if !present {
				// A word whose binding was lost keeps the one composition
				// retained for its workspace; land never chooses between two.
				retained, retainedErr := landing.RetainedCarriedSubject(subjects, goalID, status.Workspace)
				var refusal *landing.CarriedRefusal
				if retainedErr == nil {
					if err := landing.BindCarriedSubject(subjects, opid, retained); err != nil {
						return inv.carriedStopped(goalID, opid, targets, data, "its composition cannot be bound: "+err.Error())
					}
					bound, present = retained, true
				} else if !errors.As(retainedErr, &refusal) || refusal.Code != "carried-subject-missing" {
					return inv.carriedUnidentified(goalID, opid, primary, targets, data, retainedErr)
				}
			}
			if !present {
				// A word recorded elsewhere (a channel answer) is adopted
				// only when the goal composes to exactly its workspace now.
				composed, problem := inv.composeCarriedSubject(root, primary, goalID, targets, data)
				if problem != nil {
					return inv.carriedStopped(goalID, opid, targets, data, "its candidate cannot be composed: "+problem.Summary)
				}
				if composed.Workspace != status.Workspace {
					return inv.carriedStopped(goalID, opid, targets, data, fmt.Sprintf("the goal now composes workspace %s, not the exception's %s", composed.Workspace, status.Workspace))
				}
				bindErr := landing.RetainCarriedSubject(subjects, composed)
				if bindErr == nil {
					bindErr = landing.BindCarriedSubject(subjects, opid, composed)
				}
				if bindErr != nil {
					return inv.carriedStopped(goalID, opid, targets, data, "its composition cannot be bound: "+bindErr.Error())
				}
				bound = composed
			}
			subject = bound
		}
		if subject.Workspace != status.Workspace {
			return inv.carriedStopped(goalID, opid, targets, data, fmt.Sprintf("its bound composition has workspace %s, not the word's %s", subject.Workspace, status.Workspace))
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
				result := inv.carriedStopped(goalID, opid, targets, data, "main moved after its candidate was composed: "+err.Error())
				result.next, result.nextReason = inv.carriedReplacement(goalID, opid, status, "main moved after the exception was recorded")
				return result
			}
			return inv.carriedStopped(goalID, opid, targets, data, "its candidate was not staged: "+err.Error())
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
		return inv.carriedStopped(goalID, opid, targets, data, err.Error())
	}
	if _, err := os.Stat(message); err != nil {
		text := fmt.Sprintf("Land %s under a recorded exception\n\nException: %s\n", goalID, opid)
		if err := os.WriteFile(message, []byte(text), 0o644); err != nil {
			return inv.carriedStopped(goalID, opid, targets, data, err.Error())
		}
	}
	argv := []string{filepath.Join(primary, "scripts", "agents", "land.sh"), "-m", message, "--goal", goalID, "--carried", opid, "--staged-only"}
	if acknowledgePlans {
		argv = append(argv, "--allow-new-plan")
	}
	ran := inv.delivery().process(intentProcess{argv: argv, dir: top})
	result := ownerVerbResult(ran, targets, fmt.Sprintf("goal %s landed under exception %s", goalID, opid), data)
	if result.Outcome != intentConfirmed {
		stopped := inv.carriedStopped(goalID, opid, targets, result.Data.(map[string]any), "the carried landing did not complete: "+result.Summary)
		refusal := string(ran.stderr)
		if strings.Contains(refusal, "carry-battery-unverified") && strings.Contains(refusal, errRetainedCandidateEngineAbsent.Error()) {
			// The carried transaction verifies retained proof; it never
			// runs tests. The person proves the staged candidate with the
			// public test command, then continues under the same word.
			continuation := stopped.next
			stopped.Summary = fmt.Sprintf("the exception %s is recorded and its candidate is staged in %s, but no test run has proved that candidate yet", opid, primary)
			stopped.next = []string{"metasystem", "test", "--goal", goalID, "--repo", primary}
			stopped.nextReason = "prove the staged candidate, then continue under the same exception: " + shellCommand(continuation)
			stopped.Data.(map[string]any)["afterProof"] = continuation
		}
		return stopped
	}
	return result
}

// carriedReplacement is the public replacement of a recorded word, from
// the word's own recorded refusal and person: the one explicit decision
// that authorizes the goal's composition on the current main.
func (inv *intentInvocation) carriedReplacement(goalID, opid string, status landing.CarryStatus, reason string) ([]string, string) {
	return inv.publicArgv("land", goalID, "--exception", status.Past, "--replace-exception", opid,
			"--reason", reason, "--by", strings.TrimPrefix(status.By, "human:")),
		"a person replaces the exception for the candidate composed on the current main"
}

// carriedUnidentified stops a word whose composition cannot be identified
// before anything is bound, staged or delivered. When more than one
// retained composition has its workspace, land does not guess between
// them: the remedy is the person's explicit replacement.
func (inv *intentInvocation) carriedUnidentified(goalID, opid, primary string, targets []intentTarget, data map[string]any, err error) intentResult {
	result := inv.carriedStopped(goalID, opid, targets, data, "its composition cannot be identified: "+err.Error())
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
// and the same exception continues the landing.
func (inv *intentInvocation) carriedStopped(goalID, opid string, targets []intentTarget, data map[string]any, why string) intentResult {
	return intentResult{Outcome: intentPartial, code: 1, Targets: targets, Data: data,
		Summary: fmt.Sprintf("the exception %s is recorded, but %s", opid, why),
		next:    inv.publicArgv("land", goalID, "--using-exception", opid), nextReason: "continue the carried landing under the same exception; no new exception is recorded"}
}

// carriedRefresh fetches the code origin and the goal ledger the carried
// transaction reads, and returns the accepted ledger tip. It never creates
// the origin remote.
func (inv *intentInvocation) carriedRefresh(primary string, targets []intentTarget, data map[string]any) (string, *intentResult) {
	if _, err := inv.work().git(primary, "fetch", "--quiet", "origin", "+refs/heads/main:refs/remotes/origin/main"); err != nil {
		return "", &intentResult{Outcome: intentRefused, code: 1, Targets: targets, Data: data, Summary: "the code origin cannot be fetched: " + err.Error() + "; nothing more was done"}
	}
	ran, problem := inv.engineVerb("goal", "fetch", "--root", primary)
	if problem != nil {
		return "", problem
	}
	output := string(ran.stdout)
	_, tip, _ := strings.Cut(output, "tip=")
	tip, _, _ = strings.Cut(tip, " ")
	if ran.code != 0 || len(strings.TrimSpace(tip)) != 40 {
		fetched := ownerVerbResult(ran, targets, "", data)
		return "", &intentResult{Outcome: intentRefused, code: 1, Targets: targets, Data: data, Summary: "the goal ledger cannot be fetched: " + fetched.Summary + "; nothing more was done"}
	}
	return strings.TrimSpace(tip), nil
}

// composeCarriedSubject composes the goal's canonical landing candidate on
// the live code endpoint and names it with that endpoint's product.
func (inv *intentInvocation) composeCarriedSubject(root, primary, goalID string, targets []intentTarget, data map[string]any) (landing.CarriedSubject, *intentResult) {
	candidate, code, err := inv.delivery().landCandidate([]string{"--root", root, "--goal", goalID, "--last"})
	if err != nil {
		return landing.CarriedSubject{}, &intentResult{Outcome: intentRefused, code: max(code, 1), Targets: targets, Data: data, Summary: "the landing candidate cannot be composed: " + err.Error() + "; nothing was recorded"}
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
		return landing.CarriedSubject{}, &intentResult{Outcome: intentFailed, code: 1, Targets: targets, Data: data, Summary: "the candidate's workspace cannot be read: " + err.Error() + "; nothing was recorded"}
	}
	return landing.CarriedSubject{Goal: goalID, Endpoint: candidate.Result.Endpoint, EndpointWorkspace: endpointWorkspace, Workspace: workspace}, nil
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
