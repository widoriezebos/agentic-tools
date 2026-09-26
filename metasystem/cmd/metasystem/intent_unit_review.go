package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
)

// review unit RUN connects a built unit to the goal branch evidence landing
// consumes. The unit runner binds its latest completed round to a
// goal-branch subject under the run's lock; the branch commit owner makes
// the Goal-Unit commit (or amends the unit's earlier one) under the claim
// and worktree commit token, with the endpoint and claim of the selected
// installation; the branch push owner publishes it; commitReview, shared
// with review commit, requests the committed critic, requires the chain's
// recorded closure before collection, and publishes the collected Goal-Read.
// The preliminary read of the build is feedback and is never promoted;
// findings are never accepted here.

func (inv *intentInvocation) reviewUnit(run string) intentResult {
	targets := []intentTarget{{Kind: "unit", ID: run}}
	runner := inv.unitRunner()
	var result intentResult
	err := runner.ReviewSubject(run, func(review launch.UnitReview, retain func(launch.UnitSubject) error) error {
		result = inv.reviewUnitRound(runner, targets, review, retain)
		return nil
	})
	if err == nil {
		return result
	}
	message := err.Error()
	switch {
	case strings.HasPrefix(message, "UNIT_RUN_BUSY"):
		return intentResult{Targets: targets, Outcome: intentInProgress, code: 3, Summary: message,
			next: inv.sameCommand(), nextReason: "another call holds this run; the same command continues it"}
	case strings.HasPrefix(message, "UNIT_RUN_UNKNOWN"):
		return intentResult{Targets: targets, Outcome: intentRefused, code: 2, Summary: message}
	case strings.HasPrefix(message, "UNIT_REVIEW_NOT_READY") && strings.Contains(message, "still running"):
		record, _ := runner.Status(run)
		record.ID = run
		return intentResult{Targets: targets, Outcome: intentRefused, code: 1, Summary: message + "; nothing was committed",
			next: inv.workArgv(record, "wait"), nextReason: "the attempt must finish before its result is committed"}
	case strings.HasPrefix(message, "UNIT_REVIEW_NOT_READY"):
		record, _ := runner.Status(run)
		record.ID = run
		return intentResult{Targets: targets, Outcome: intentRefused, code: 1, Summary: message + "; nothing was committed",
			next: inv.workArgv(record, "revise", "--after", strconv.Itoa(len(record.Rounds)), "--brief", "FILE"), nextReason: "correct the attempt; a correction brief starts one new attempt"}
	}
	return intentResult{Targets: targets, Outcome: intentRefused, code: 1, Summary: message + "; nothing was committed"}
}

func splitNUL(data []byte) []string {
	var items []string
	for _, item := range strings.Split(string(data), "\x00") {
		if item != "" {
			items = append(items, item)
		}
	}
	return items
}

func (inv *intentInvocation) reviewUnitRound(runner *launch.UnitRunner, targets []intentTarget, review launch.UnitReview, retain func(launch.UnitSubject) error) intentResult {
	record := review.Record
	goalID, unit := record.Goal, record.Unit
	if named := inv.input.text("goal"); named != "" && named != goalID {
		return intentResult{Targets: targets, Outcome: intentRefused, code: 2,
			Summary: fmt.Sprintf("unit run %s builds goal %s, not %s; nothing was committed", record.ID, goalID, named)}
	}
	targets = append(targets, intentTarget{Kind: "goal", ID: goalID})
	worktree, original := record.Worktree, inv.layout.InstallationRoot
	install := inv.goalWorktreeInstallation(worktree)
	data := map[string]any{"run": record.ID, "unit": unit, "goal": goalID, "round": review.Round.Number,
		"outcome": review.Round.Outcome, "worktree": worktree, "expectedParent": review.Head}
	refuse := func(format string, args ...any) intentResult {
		return intentResult{Targets: targets, Outcome: intentRefused, code: 1, Data: data, Summary: fmt.Sprintf(format, args...)}
	}
	conn, git := inv.connection(), inv.work().git
	// The selected installation's endpoint and claim authorize every
	// effect; the goal worktree's own resolution must agree, because the
	// read and publication owners resolve there.
	endpoint, err := conn.endpoint(original)
	if err != nil {
		return refuse("the goal branch endpoint is unavailable: %v; nothing was committed", err)
	}
	if local, err := conn.endpoint(install); err != nil || local.Remote != endpoint.Remote || local.Branch != endpoint.Branch {
		return refuse("goal worktree %s resolves goal branch endpoint %s %s (%v), not the selected installation's %s %s; nothing was committed",
			install, local.Remote, local.Branch, err, endpoint.Remote, endpoint.Branch)
	}
	check := conn.claimCheck(original, goalID, endpoint)
	if err := branch.CheckCommitAccess(goalID, check); err != nil {
		return refuse("%v; nothing was committed", err)
	}
	endpointTip := ""
	tip := func() (string, error) {
		if endpointTip == "" {
			var tipErr error
			endpointTip, tipErr = conn.endpointTip(original, endpoint)
			if tipErr != nil {
				return "", tipErr
			}
		}
		return endpointTip, nil
	}
	subject := review.Subject
	if subject == nil || subject.Commit == "" {
		base, err := tip()
		if err != nil {
			return refuse("cannot resolve the landing endpoint's tip: %v; nothing was committed", err)
		}
		head, current, err := runner.WorktreeResult(worktree)
		if err != nil {
			return refuse("cannot read goal worktree %s: %v; nothing was committed", worktree, err)
		}
		if subject != nil && subject.StagedTree != "" && head != review.Head {
			// A commit may have been made without being recorded; only
			// the exact bound result is adopted.
			commit, conflict := resolveUnitCommit(git, install, base, goalID, unit, *subject, head)
			if conflict != nil {
				data["subject"] = subject
				return refuse("UNIT_SUBJECT_CONFLICT: %v; nothing was committed again", conflict)
			}
			subject.Commit, subject.Tip = commit, head
		} else {
			diff, err := runner.WorktreeDiff(worktree, review.Base)
			if err != nil {
				return refuse("cannot read goal worktree %s: %v; nothing was committed", worktree, err)
			}
			if head != review.Head || !bytes.Equal(diff, review.Diff) || (!review.Legacy && current != review.Result) {
				return refuse("UNIT_RESULT_CHANGED: goal worktree %s no longer holds round %d's result (HEAD %.12s, expected %.12s); nothing was staged",
					worktree, review.Round.Number, head, review.Head)
			}
			paths, err := launch.UnitResultPaths(current)
			if err != nil || len(paths) == 0 {
				return refuse("round %d of run %s has no committable result (%v)", review.Round.Number, record.ID, err)
			}
			staged, err := git(worktree, "diff", "--cached", "--name-only", "-z")
			if err != nil {
				return refuse("cannot read the goal worktree's index: %v", err)
			}
			for _, path := range splitNUL(staged) {
				if !slices.Contains(paths, path) {
					return refuse("UNIT_RESULT_CHANGED: %q is staged but is not part of round %d's result; nothing was staged", path, review.Round.Number)
				}
			}
			if subject == nil {
				operation, err := conn.operationID()
				if err != nil {
					return refuse("%v", err)
				}
				subject = &launch.UnitSubject{Round: review.Round.Number, Operation: operation}
			}
			subject.ExpectedParent, subject.ResultDigest, subject.DiffDigest, subject.Paths =
				review.Head, launch.UnitResultDigest(current), review.DiffDigest, paths
			if review.Prior != nil {
				parent, err := git(worktree, "rev-parse", review.Prior.Commit+"^")
				if err != nil {
					return refuse("cannot read the parent of unit %s's earlier commit %s: %v", unit, review.Prior.Commit, err)
				}
				subject.Amends, subject.AmendsParent = review.Prior.Commit, strings.TrimSpace(string(parent))
			}
			if _, err := git(worktree, append([]string{"--literal-pathspecs", "add", "-A", "--"}, paths...)...); err != nil {
				return refuse("cannot stage round %d's result: %v", review.Round.Number, err)
			}
			stagedResult, err := git(worktree, "diff", "--cached", "--raw", "-z", "--no-abbrev", "HEAD", "--", ".")
			if err != nil || string(stagedResult) != current {
				return refuse("UNIT_RESULT_CHANGED: the staged tree is not round %d's result; the frozen paths are staged and nothing was committed", review.Round.Number)
			}
			tree, err := git(worktree, "write-tree")
			if err != nil {
				return refuse("cannot write the staged tree: %v", err)
			}
			subject.StagedTree = strings.TrimSpace(string(tree))
			if err := retain(*subject); err != nil {
				return refuse("cannot retain the unit subject before committing: %v; nothing was committed", err)
			}
			var installed string
			commitErr := conn.commitToken(install, func() error {
				var err error
				installed, err = conn.commit(branch.CommitRequest{Repo: install, Remote: endpoint.Remote, EndpointTip: base,
					GoalID: goalID, Units: []string{unit}, OpID: subject.Operation + "-unit", Kind: branch.Unit,
					Amend: subject.Amends != "", CheckClaim: check, Transport: conn.transport})
				return err
			})
			if commitErr != nil {
				after, _ := git(worktree, "rev-parse", "HEAD")
				installed = strings.TrimSpace(string(after))
			}
			commit, conflict := resolveUnitCommit(git, install, base, goalID, unit, *subject, installed)
			if conflict != nil {
				data["subject"] = subject
				summary := fmt.Sprintf("the branch commit owner's result for unit %s does not bind round %d: %v", unit, review.Round.Number, conflict)
				if commitErr != nil {
					summary = fmt.Sprintf("the branch commit owner did not commit unit %s: %v", unit, commitErr)
				}
				return intentResult{Targets: targets, Outcome: intentFailed, code: 1, Data: data, Summary: summary,
					next: inv.sameCommand(), nextReason: "the bound subject is kept; the same command reconciles or commits it once"}
			}
			subject.Commit, subject.Tip = commit, installed
		}
		if err := retain(*subject); err != nil {
			data["commit"] = subject.Commit
			return intentResult{Targets: targets, Outcome: intentPartial, code: 1, Data: data,
				Summary: fmt.Sprintf("unit commit %s exists but could not be recorded with run %s: %v", subject.Commit, record.ID, err),
				next:    inv.sameCommand(), nextReason: "the same command reconciles the exact commit"}
		}
	}
	data["commit"], data["tip"], data["operation"] = subject.Commit, subject.Tip, subject.Operation
	if subject.Amends != "" {
		data["amends"] = subject.Amends
	}
	targets = append(targets, intentTarget{Kind: "commit", ID: subject.Commit})
	if subject.Published == "" {
		base, err := tip()
		if err == nil {
			var pushed branch.PushResult
			pushed, err = conn.push(branch.PushRequest{Repo: install, Remote: endpoint.Remote, EndpointTip: base,
				GoalID: goalID, OpID: subject.Operation + "-push", CheckClaim: check, Transport: conn.transport})
			if err == nil {
				subject.Published = pushed.Tip
				err = retain(*subject)
			}
		}
		if err != nil {
			return intentResult{Targets: targets, Outcome: intentPartial, code: 1, Data: data,
				Summary: fmt.Sprintf("unit %s is committed as %s but not published: %v", unit, subject.Commit, err),
				next:    inv.sameCommand(), nextReason: "the same command publishes this commit; it never makes another"}
		}
	}
	data["published"] = subject.Published
	args := []string{"--root", install, "--goal", goalID, "--unit", subject.Commit}
	if review.BuildBrief != "" {
		args = append(args, "--brief", review.BuildBrief)
	}
	if inv.input.has("model") {
		args = append(args, "--model", inv.input.text("model"))
	}
	if install != original {
		args = append(args, "--selected-installation", original)
	}
	if inv.reviewWork != nil {
		inv.reviewWork.attempt, inv.reviewWork.retain, inv.reviewWork.subject = review.Round.Number, retain, subject
	}
	if inv.reviewWork != nil && inv.reviewWork.retry > 0 {
		args = append(args, "--retry", strconv.FormatInt(inv.reviewWork.retry, 10))
	}
	result := inv.commitReview(targets, install, goalID, subject.Commit, args)
	if merged, ok := result.Data.(map[string]any); ok {
		for key, value := range data {
			if _, taken := merged[key]; !taken {
				merged[key] = value
			}
		}
	} else if result.Data == nil {
		result.Data = data
	}
	if result.Outcome == intentConfirmed || result.Outcome == intentUnchanged {
		result.next, result.nextReason = inv.publicArgv("land", goalID), "the unit's read is published on the goal branch; landing admits it by its own rules"
	}
	return result
}

// resolveUnitCommit finds the unit's Goal-Unit commit on the branch the
// commit owner installed at tip, through the branch range owner, and adopts
// it only when it binds the retained subject: for a first commit, tip is
// the commit on the expected parent with exactly the staged tree; for an
// amend, the replacement sits on the amended commit's parent, the replayed
// suffix carries the earlier suffix's commits in order without the reads of
// the replaced subject, and the tip's tree is the staged tree except for
// the dropped reads' paths.
func resolveUnitCommit(git func(string, ...string) ([]byte, error), install, endpointTip, goalID, unit string, subject launch.UnitSubject, tip string) (string, error) {
	if tip == "" || tip == subject.ExpectedParent {
		return "", fmt.Errorf("no unit commit was installed")
	}
	commits, err := branch.ValidateRange(install, endpointTip, tip, goalID)
	if err != nil {
		return "", err
	}
	found := ""
	for _, commit := range commits {
		if commit.Kind == branch.Unit && slices.Equal(commit.Units, []string{unit}) {
			if found != "" {
				return "", fmt.Errorf("goal/%s names unit %s twice", goalID, unit)
			}
			found = commit.ID
		}
	}
	if found == "" {
		return "", fmt.Errorf("goal/%s at %.12s has no commit of unit %s", goalID, tip, unit)
	}
	line := func(args ...string) (string, error) {
		out, err := git(install, args...)
		return strings.TrimSpace(string(out)), err
	}
	parent, err := line("rev-parse", found+"^")
	if err != nil {
		return "", err
	}
	if subject.Amends == "" {
		tree, err := line("rev-parse", found+"^{tree}")
		if err != nil {
			return "", err
		}
		if found != tip || parent != subject.ExpectedParent || tree != subject.StagedTree {
			return "", fmt.Errorf("unit commit %.12s is not the staged tree %.12s on %.12s", found, subject.StagedTree, subject.ExpectedParent)
		}
		return found, nil
	}
	if found == subject.Amends || parent != subject.AmendsParent {
		return "", fmt.Errorf("unit %s at %.12s does not replace %.12s on its parent %.12s", unit, found, subject.Amends, subject.AmendsParent)
	}
	var kept, dropped []string
	old, err := line("rev-list", "--reverse", subject.Amends+".."+subject.ExpectedParent)
	if err != nil {
		return "", err
	}
	for _, commit := range strings.Fields(old) {
		kind, err := branch.KindOf(install, commit, goalID)
		if err != nil {
			return "", err
		}
		if kind.Kind == branch.Read && kind.CommitID == subject.Amends {
			dropped = append(dropped, commit)
		} else {
			kept = append(kept, commit)
		}
	}
	replayed, err := line("rev-list", "--reverse", found+".."+tip)
	if err != nil {
		return "", err
	}
	if len(strings.Fields(replayed)) != len(kept) {
		return "", fmt.Errorf("the branch after %.12s replays %d commits, not the %d kept after %.12s", found, len(strings.Fields(replayed)), len(kept), subject.Amends)
	}
	for index, commit := range strings.Fields(replayed) {
		was, err1 := line("log", "-1", "--format=%B", kept[index])
		now, err2 := line("log", "-1", "--format=%B", commit)
		if err1 != nil || err2 != nil || was != now {
			return "", fmt.Errorf("replayed commit %.12s is not %.12s", commit, kept[index])
		}
	}
	allowed := map[string]bool{}
	for _, commit := range dropped {
		names, err := git(install, "diff-tree", "-r", "-z", "--no-commit-id", "--name-only", commit+"^", commit)
		if err != nil {
			return "", err
		}
		for _, name := range splitNUL(names) {
			allowed[name] = true
		}
	}
	differs, err := git(install, "diff-tree", "-r", "-z", "--name-only", subject.StagedTree, tip+"^{tree}")
	if err != nil {
		return "", err
	}
	for _, name := range splitNUL(differs) {
		if !allowed[name] {
			return "", fmt.Errorf("the amended branch's %q differs from the staged result", name)
		}
	}
	return found, nil
}

// commitReview requests, collects and publishes the committed read of one
// Goal-Unit commit through the branch read owner and the collected-read
// publication owner, with its records in root. A terminal critic whose
// chain is not closed yields the author's close and collects nothing; a
// repeat publishes the same attestation without reading again. review
// commit and review unit share it.
func (inv *intentInvocation) commitReview(targets []intentTarget, root, goalID, unit string, args []string) intentResult {
	owners := inv.delivery()
	result, code, err := owners.branchRead(args)
	if err == nil && result.State == "closed" {
		// The branch read calls a terminal critic closed; collection needs
		// the chain's own recorded closure, which only the close owner writes.
		if waiting := inv.criticClosure(targets, root, unit, goalID, result.RootJob); waiting != nil {
			if data, _ := waiting.Data.(map[string]any); data["failedRound"] != nil {
				// A failed examination has no findings to decide; its
				// continuation is the retry, never a decisions file.
				return *waiting
			}
			if inv.reviewWork == nil && inv.input.has("dispositions") {
				// An explicitly named commit's review closes with the author's
				// decisions through the whole close owner, then collects below.
				closer := *inv
				closer.layout.InstallationRoot = root
				closed := closer.closeChain(result.RootJob)
				if closed.Outcome != intentConfirmed && closed.Outcome != intentUnchanged {
					closed.Targets = append(targets, closed.Targets...)
					closed.Summary = "the examination finished, but its close did not complete: " + closed.Summary
					inv.riskRemedy(&closed, root, goalID, result.RootJob,
						append(inv.canonicalReviewArgv(targets, goalID, unit), "--dispositions", inv.input.text("dispositions")))
					return closed
				}
			} else if inv.reviewWork == nil {
				join, clean := inv.cleanExaminationJoin(root, goalID, unit, result.RootJob)
				if !clean {
					waiting.Decision = fmt.Sprintf("the author decides every finding of %s in a dispositions file, then runs %s",
						result.RootJob, shellCommand(append(inv.canonicalReviewArgv(targets, goalID, unit), "--dispositions", "FILE")))
					return *waiting
				}
				// A completed examination with no findings has nothing to
				// decide: its empty join closes through the whole close owner.
				closer := *inv
				closer.layout.InstallationRoot = root
				closer.input = intentInput{values: map[string][]string{"dispositions": {join}}}
				closed := closer.closeChain(result.RootJob)
				if closed.Outcome != intentConfirmed && closed.Outcome != intentUnchanged {
					closed.Targets = append(targets, closed.Targets...)
					closed.Summary = "the examination finished clean, but its close did not complete: " + closed.Summary
					inv.riskRemedy(&closed, root, goalID, result.RootJob, inv.canonicalReviewArgv(targets, goalID, unit))
					return closed
				}
			} else {
				// A goal's work review completes the author's decision and the
				// whole close here, then collects below.
				if pending := inv.closeWorkReview(targets, root, unit, result.RootJob); pending != nil {
					return *pending
				}
			}
		}
		result, code, err = owners.branchRead(append(args, "--collect"))
	}
	if err != nil {
		var refusal *branch.OpError
		var neverLaunched *branch.ReadNeverLaunchedError
		switch {
		case errors.As(err, &refusal) && refusal.Code == branch.ReadDispatchPendingCode:
			return intentResult{Targets: targets, Outcome: intentInProgress, Summary: err.Error(),
				Data:     map[string]any{"code": refusal.Code},
				Decision: "the read's dispatch outcome is unknown; its record is reconciled from the job store, never dispatched again"}
		case errors.As(err, &neverLaunched):
			return intentResult{Targets: targets, Outcome: intentRefused, code: max(code, 1), Summary: err.Error(),
				next: inv.sameCommand(), nextReason: "no critic was started; the read may be requested again"}
		case errors.As(err, &refusal):
			return intentResult{Targets: targets, Outcome: intentRefused, code: max(code, 1), Summary: err.Error(), Data: map[string]any{"code": refusal.Code}}
		}
		return intentResult{Targets: targets, Outcome: intentRefused, code: max(code, 1), Summary: err.Error()}
	}
	data := map[string]any{"state": result.State, "rootJob": result.RootJob, "gateRun": result.GateRunID, "attestation": result.AttestationCommit}
	if result.RootJob != "" {
		targets = append(targets, jobTarget(result.RootJob))
	}
	if result.State != "collected" && result.State != "already-collected" {
		return intentResult{Targets: targets, Outcome: intentInProgress, Data: data,
			Summary: "the review of this work is in progress",
			next:    inv.canonicalReviewArgv(targets, goalID, unit), nextReason: "continues this review and publishes its result when ready"}
	}
	published, err := owners.publishRead(root, goalID, unit)
	data["publication"] = published
	if err != nil {
		return intentResult{Targets: targets, Outcome: intentPartial, code: 1, Data: data,
			Summary: fmt.Sprintf("the review is complete, but its result has not yet been published: %v", err),
			next:    inv.canonicalReviewArgv(targets, goalID, unit), nextReason: "publishes the same attestation under the same push operation; no critic or commit is repeated"}
	}
	outcome := intentConfirmed
	if result.State == "already-collected" && published.State == "current" {
		outcome = intentUnchanged
	}
	return intentResult{Targets: targets, Outcome: outcome, Data: data,
		Summary: "the review is complete and published"}
}

// cleanExaminationJoin is the empty decisions join of a critic chain whose
// newest round completed with a readable return and no findings, retained
// beside that return as the build review retains it. Any other round (still
// running, failed, malformed or with findings) is not clean.
func (inv *intentInvocation) cleanExaminationJoin(root, goalID, unit, rootJob string) (string, bool) {
	newest, err := inv.newestRoundAt(root, rootJob)
	if err != nil || recordText(newest, "status") != "completed" {
		return "", false
	}
	round := recordRound(newest)
	returnPath := inv.returnPathAt(root, rootJob, round)
	digest, _, readErr := reviewReturnDigest(returnPath)
	findings, _, parseErr := readIntentFindings(returnPath)
	if readErr != nil || parseErr != nil || len(findings) != 0 {
		return "", false
	}
	join := filepath.Join(filepath.Dir(returnPath), "decisions.md")
	if _, err := os.Stat(join); err == nil {
		return join, true
	}
	binding := reviewBinding{Goal: goalID, Subject: unit, Examination: rootJob, Round: round, Return: digest}
	if err := os.WriteFile(join, []byte(decisionsDocument(binding, nil)), 0o600); err != nil {
		return "", false
	}
	return join, true
}
