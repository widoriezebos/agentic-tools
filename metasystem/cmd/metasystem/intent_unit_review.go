package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

// review run RUN connects a built unit to the goal branch evidence landing
// consumes. The unit runner binds its latest completed round to a
// goal-branch subject under the run's lock; the branch commit owner makes
// the Goal-Unit commit (or amends the unit's earlier one) under the claim
// and worktree commit token, with the endpoint and claim of the selected
// installation; the branch push owner publishes it; commitReview, shared
// with review commit, records the unit's read and publishes its Goal-Read.
// A committed critic requires the chain's recorded closure before collection.
// A clean read of the build by another model than the builder's is the
// unit's read. Any other read is feedback and requests the committed critic.

func (inv *intentInvocation) reviewUnit(run string) intentResult {
	targets := []intentTarget{{Kind: "unit", ID: run}}
	runner := inv.unitRunner()
	if current, err := runner.Status(run); err == nil && len(current.Rounds) > 0 {
		round := current.Rounds[len(current.Rounds)-1]
		retryRequested := round.Stop != nil && round.Stop.Loop == "unit-build" && (round.Cause == "environment" || round.Cause == "deadline")
		for _, step := range round.Steps {
			if step.RetryReason != "" && step.RetryReason == inv.input.text("reason") {
				retryRequested = true
			}
		}
		if retryRequested && strings.TrimSpace(inv.input.text("reason")) != "" {
			actor, _, problem := inv.actingAs("work review retry", current.Goal, actorHuman)
			if problem != nil {
				return *problem
			}
			person, reason := unitStopActor(actor), inv.input.text("reason")
			impact := "Impact: rerun the retained failed step with the same inputs and fresh output, without another build.\nThe environment may still fail; automatic correction limits remain unchanged.\nStop this run to end the continuation; retained evidence remains."
			resumed, err := runner.RetryFailedStep(run, person, reason, func() error {
				return inv.recordUnitStopOverride(current.Goal, "work-review-retry", reason, impact, person)
			})
			if err == nil {
				resumed, err = runner.Continue(launch.UnitRequest{Resume: run})
			}
			return inv.unitOutcome(runner, resumed, err, targets, inv.workArgv(current, "wait"))
		}
		if round.Stop != nil && round.Stop.Loop == "unit-build" && round.Outcome != "build-size" || round.Outcome == "build-gap" || round.Outcome == "build-size" && strings.TrimSpace(inv.input.text("reason")) == "" {
			return inv.unitOutcome(runner, launch.UnitResult{Record: current}, nil, targets, inv.workArgv(current, "wait"))
		}
		if round.Outcome == "build-size" {
			actor, _, problem := inv.actingAs("work review size", current.Goal, actorHuman)
			if problem != nil {
				return *problem
			}
			impact := fmt.Sprintf("Impact: accept %d changed lines against %d declared and resume the pending checks.\nThe larger change still needs checks and review; later automatic limits remain.\nStop this run to undo the continuation; its retained change remains.", round.BuildLines, round.DeclaredLines)
			person := unitStopActor(actor)
			var impactErr error
			resumed, err := runner.AcceptBuildSize(run, person, func() error {
				impactErr = inv.recordUnitStopOverride(current.Goal, "work-review-size", inv.input.text("reason"), impact, person)
				return impactErr
			})
			if impactErr != nil {
				return intentResult{Outcome: intentFailed, code: 1, Summary: "the size decision impact could not be recorded", Details: []string{impactErr.Error()}, next: inv.sameCommand()}
			}
			if err != nil {
				return inv.unitOutcome(runner, resumed, err, targets, inv.workArgv(current, "wait"))
			}
			resumed, err = runner.Continue(launch.UnitRequest{Resume: run})
			return inv.unitOutcome(runner, resumed, err, targets, inv.workArgv(current, "wait"))
		}
		if round.Stop != nil && strings.HasPrefix(round.Stop.Handoff, "stopped ") && round.Stop.Handoff != "stopped unreadable-policy" && len(current.Subjects) == 0 && round.ReadModel != "" {
			if round.UnknownRetries == 0 {
				fresh, err := runner.RetryUnknownRead(run)
				if err != nil || fresh.Capped {
					return inv.unitOutcome(runner, fresh, err, targets, inv.workArgv(current, "wait"))
				}
				current = fresh.Record
				round = current.Rounds[len(current.Rounds)-1]
			}
			if round.Stop != nil && strings.HasPrefix(round.Stop.Handoff, "stopped ") {
				return intentResult{Targets: targets, Outcome: intentRefused, code: 1, Data: unitData(current, runner.Manager), Summary: "the fresh examination still has unknown inputs; " + round.Stop.Handoff + " holds this unit", next: inv.workArgv(current, "revise", "--brief", "FILE", "--reason", "TEXT", "--by", "NAME")}
			}
		}
	}
	var result intentResult
	err := runner.ReviewSubject(run, func(review launch.UnitReview, retain func(launch.UnitSubject) error) error {
		result = inv.reviewUnitRound(runner, targets, review, retain)
		return nil
	})
	if err == nil {
		return result
	}
	message, details := err.Error(), launchDetails(err)
	if details == nil {
		details = []string{message}
	}
	switch {
	case launch.IsCode(err, "UNIT_RUN_BUSY"):
		return intentResult{Targets: targets, Outcome: intentInProgress, code: 3, Summary: "another command is working on this run right now",
			next: inv.sameCommand(), nextReason: "the same command continues once it is done", Details: details}
	case launch.IsCode(err, "UNIT_RUN_UNKNOWN"):
		return intentResult{Targets: targets, Outcome: intentRefused, code: 2, Summary: fmt.Sprintf("no run %s is recorded; nothing was done", run),
			next: inv.publicArgv("work", "status", "--all"), nextReason: "lists the runs with their references", Details: details}
	case launch.IsCode(err, "UNIT_REVIEW_NOT_READY") && errors.Is(err, launch.ErrRunStillRunning):
		record, _ := runner.Status(run)
		record.ID = run
		return intentResult{Targets: targets, Outcome: intentRefused, code: 1, Summary: "the build is still running; nothing was committed",
			next: inv.workArgv(record, "wait"), nextReason: "the attempt must finish before its result is committed", Details: details}
	case launch.IsCode(err, "UNIT_REVIEW_NOT_READY"):
		record, _ := runner.Status(run)
		record.ID = run
		return intentResult{Targets: targets, Outcome: intentRefused, code: 1, Summary: unitRoundEnded(launch.ErrorDetail(err)) + ", so there is nothing to commit; nothing was committed",
			next: inv.workArgv(record, "revise", "--after", strconv.Itoa(len(record.Rounds)), "--brief", "FILE"), nextReason: "a correction brief starts one new attempt", Details: details}
	}
	return intentResult{Targets: targets, Outcome: intentRefused, code: 1, Summary: message + "; nothing was committed",
		next: inv.publicArgv("work", "status", unitRunPrefix+run), nextReason: "shows the run; --verbose shows the cause", Details: details}
}

// unitRoundEnded is how the newest build round ended, in plain words, read
// from the runner's UNIT_REVIEW_NOT_READY account.
func unitRoundEnded(message string) string {
	_, rest, _ := strings.Cut(message, "outcome=")
	outcome, _, _ := strings.Cut(rest, ":")
	switch outcome {
	case "build-failed":
		return "the build failed"
	case "proof-red":
		return "the build's checks failed"
	case "proof-wrote":
		return "the build's checks changed its files"
	case "":
		return "the build has no passing result"
	}
	return "the build ended " + outcome
}

// unitRefusalCode is an owner's refusal code at the head of its account,
// such as UNIT_RESULT_CHANGED; the result keeps it as data.
var unitRefusalCode = regexp.MustCompile(`^[A-Z][A-Z0-9]+(_[A-Z0-9]+)+$`)

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
			Summary: fmt.Sprintf("run %s builds goal %s, not %s; nothing was committed", record.ID, goalID, named),
			next:    inv.typedArgvLess("goal"), nextReason: "reviews it for goal " + goalID}
	}
	targets = append(targets, intentTarget{Kind: "goal", ID: goalID})
	worktree, original := record.Worktree, inv.layout.InstallationRoot
	install := inv.goalWorktreeInstallation(worktree)
	data := map[string]any{"run": record.ID, "unit": unit, "goal": goalID, "round": review.Round.Number,
		"outcome": review.Round.Outcome, "worktree": worktree, "expectedParent": review.Head, "carried": []string{}}
	var carriedLines []string
	// refuse says what stopped the commit in plain words and runs next;
	// the owner's own account, codes included, is the detail.
	retry := inv.sameCommand()
	revise := inv.workArgv(record, "revise", "--after", strconv.Itoa(review.Round.Number), "--brief", "FILE")
	refuse := func(next []string, then, plain, format string, args ...any) intentResult {
		detail := fmt.Sprintf(format, args...)
		if code, _, found := strings.Cut(detail, ":"); found && unitRefusalCode.MatchString(code) {
			data["code"] = code
		}
		return intentResult{Targets: targets, Outcome: intentRefused, code: 1, Data: data, Summary: plain,
			next: next, nextReason: then, Details: []string{detail}}.withCause(argError(args))
	}
	conn, git := inv.connection(), inv.work().git
	fix := conn.laneFix(original.Path(), worktree, goalID)
	if fix != nil {
		commit := conn.commit
		conn.commit = func(req branch.CommitRequest) (string, error) { req.LaneFixUnits = fix.Units; return commit(req) }
	}
	// The selected installation's endpoint and claim authorize every
	// effect; the goal worktree's own resolution must agree, because the
	// read and publication owners resolve there.
	endpoint, err := conn.endpoint(original.Path())
	if err != nil {
		return refuse(retry, "try again; --verbose shows the cause", "the goal branch can't be reached, so nothing was committed", "the goal branch endpoint is unavailable: %v", err)
	}
	if local, err := conn.endpoint(install); err != nil || local.Remote != endpoint.Remote || local.Branch != endpoint.Branch {
		return refuse(inv.publicArgv("system", "check"), "shows both configurations", "the goal worktree points at another goal branch than this checkout, so nothing was committed",
			"goal worktree %s resolves goal branch endpoint %s %s (%v), not the selected installation's %s %s", install, local.Remote, local.Branch, err, endpoint.Remote, endpoint.Branch)
	}
	check := conn.claimCheck(original.Path(), goalID, endpoint)
	if err := branch.CheckCommitAccess(goalID, check); err != nil {
		if holder, next, reason, held := inv.heldElsewhere(goalID, err); held {
			return refuse(next, reason, fmt.Sprintf("seat %s holds goal %s and writes its branch, so nothing was committed", holder, goalID), "%v", err)
		}
		return refuse(inv.publicArgv("goal", "show", goalID), "shows who holds the goal", fmt.Sprintf("this session can't commit to goal %s's branch, so nothing was committed", goalID), "%v", err)
	}
	endpointTip := ""
	if fix != nil {
		endpointTip = fix.Parent
	}
	tip := func() (string, error) {
		if endpointTip == "" {
			tipErr := review.Wait(func() error {
				var err error
				endpointTip, err = conn.endpointTip(original.Path(), endpoint)
				return err
			})
			if tipErr != nil {
				return "", tipErr
			}
		}
		return endpointTip, nil
	}
	subject := review.Subject
	if subject == nil && review.Round.Stop != nil && review.Round.Stop.Decision == "stop" && len(review.Round.Reads) > 0 {
		read := review.Round.Reads[len(review.Round.Reads)-1]
		subject = &launch.UnitSubject{Round: review.Round.Number, Operation: record.ID + "-pending", ExpectedParent: review.Head, DiffDigest: review.DiffDigest, Examination: read.ID, ExaminationRound: int64(review.Round.Number), ExaminationReturnPath: read.Output}
		if err := retain(*subject); err != nil {
			return refuse(retry, "retains the stopped patch review", "the stopped review could not be retained", "%v", err)
		}
	}
	if inv.reviewWork != nil && subject != nil {
		inv.reviewWork.run, inv.reviewWork.attempt, inv.reviewWork.subject, inv.reviewWork.retain, inv.reviewWork.review = record.ID, review.Round.Number, subject, retain, &review
		// The stop arm needs a decisions file: a bare --retry of a stopped unit must reach its fresh examination.
		if subject.Drop != nil || subject.Commit == "" && subject.Examination != "" || inv.input.has("dispositions") && subject.Examination != "" && review.Round.Stop != nil && review.Round.Stop.Decision == "stop" {
			if stopped := inv.reviewStoppedUnit(targets, install, subject.Examination, subject.ExaminationReturnPath, inv.reviewWork); stopped != nil {
				return *stopped
			}
		}
	}
	if subject == nil || subject.Commit == "" {
		base, err := tip()
		if err != nil {
			return refuse(retry, "try again; --verbose shows the cause", "the goal branch's newest commit can't be read, so nothing was committed", "cannot resolve the landing endpoint's tip: %v", err)
		}
		head, current, err := runner.WorktreeResult(worktree)
		if err != nil {
			return refuse(retry, "try again; --verbose shows the cause", "the goal worktree can't be read, so nothing was committed", "cannot read goal worktree %s: %v", worktree, err)
		}
		if subject != nil && subject.StagedTree != "" && head != review.Head && head != subject.ExpectedParent {
			// A commit may have been made without being recorded; only
			// the exact bound result is adopted.
			commit, conflict := resolveUnitCommit(git, install, base, goalID, unit, *subject, head)
			if conflict != nil {
				data["subject"] = subject
				return refuse(inv.publicArgv("work", "status", unitRunPrefix+record.ID), "shows the run and its recorded commit",
					"a commit of this work exists that doesn't match its result, so nothing was committed again", "UNIT_SUBJECT_CONFLICT: %v", conflict)
			}
			subject.Commit, subject.Tip = commit, head
		} else {
			var frozenPatch []byte
			if review.Round.Result != nil {
				frozenPatch, err = os.ReadFile(filepath.Join(review.Round.Directory, "result.patch"))
				if err != nil || launch.UnitResultDigest(string(frozenPatch)) != review.Round.Result.PatchDigest {
					return refuse(retry, "recovers the retained patch", "the retained result patch cannot be verified", "%v", err)
				}
			}
			diff, err := runner.WorktreeDiff(worktree, review.Base)
			if err != nil {
				return refuse(retry, "try again; --verbose shows the cause", "the goal worktree can't be read, so nothing was committed", "cannot read goal worktree %s: %v", worktree, err)
			}
			movedEquivalent := false
			if head != review.Head && frozenPatch != nil {
				patch, patchErr := runner.WorktreeDiff(worktree, head)
				movedEquivalent = patchErr == nil && bytes.Equal(patch, frozenPatch) && current == review.Result
			}
			if !movedEquivalent && (head != review.Head || !bytes.Equal(diff, review.Diff) || (!review.Legacy && current != review.Result)) {
				return refuse(revise, "builds the result again; or put the worktree back as the build left it and repeat",
					fmt.Sprintf("the goal worktree changed after build round %d, so nothing was staged", review.Round.Number),
					"UNIT_RESULT_CHANGED: goal worktree %s no longer holds round %d's result (HEAD %.12s, expected %.12s)", worktree, review.Round.Number, head, review.Head)
			}
			paths, err := launch.UnitResultPaths(current)
			if err != nil || len(paths) == 0 {
				return refuse(revise, "a correction brief starts one new attempt", fmt.Sprintf("build round %d left nothing to commit", review.Round.Number),
					"round %d of run %s has no committable result (%v)", review.Round.Number, record.ID, err)
			}
			staged, err := git(worktree, "diff", "--cached", "--name-only", "-z")
			if err != nil {
				return refuse(retry, "try again; --verbose shows the cause", "the goal worktree's staged changes can't be read, so nothing was committed", "cannot read the goal worktree's index: %v", err)
			}
			for _, path := range splitNUL(staged) {
				if !slices.Contains(paths, path) {
					return refuse([]string{"git", "-C", worktree, "restore", "--staged", "--", path}, "unstages it; then repeat this command",
						fmt.Sprintf("%s is staged in the goal worktree but isn't part of the build's result; nothing was staged", path),
						"UNIT_RESULT_CHANGED: %q is staged but is not part of round %d's result", path, review.Round.Number)
				}
			}
			if subject == nil {
				operation, err := conn.operationID()
				if err != nil {
					return refuse(retry, "try again; --verbose shows the cause", "the commit can't be prepared, so nothing was committed", "%v", err)
				}
				subject = &launch.UnitSubject{Round: review.Round.Number, Operation: operation}
			}
			subject.ExpectedParent, subject.ResultDigest, subject.DiffDigest, subject.Paths =
				head, launch.UnitResultDigest(current), review.DiffDigest, paths
			if review.Prior != nil {
				parent, err := git(worktree, "rev-parse", review.Prior.Commit+"^")
				if err != nil {
					return refuse(retry, "try again; --verbose shows the cause", "this work's earlier commit can't be read, so nothing was committed",
						"cannot read the parent of unit %s's earlier commit %s: %v", unit, review.Prior.Commit, err)
				}
				subject.Amends, subject.AmendsParent = review.Prior.Commit, strings.TrimSpace(string(parent))
			}
			if _, err := git(worktree, append([]string{"--literal-pathspecs", "add", "-A", "--"}, paths...)...); err != nil {
				return refuse(retry, "try again; --verbose shows the cause", "the build's result can't be staged, so nothing was committed", "cannot stage round %d's result: %v", review.Round.Number, err)
			}
			stagedResult, err := git(worktree, "diff", "--cached", "--raw", "-z", "--no-abbrev", "HEAD", "--", ".")
			if err != nil || string(stagedResult) != current {
				return refuse(revise, "builds the result again", "what got staged isn't the build's result, so nothing was committed",
					"UNIT_RESULT_CHANGED: the staged tree is not round %d's result; the frozen paths are staged", review.Round.Number)
			}
			tree, err := git(worktree, "write-tree")
			if err != nil {
				return refuse(retry, "try again; --verbose shows the cause", "the staged result can't be written, so nothing was committed", "cannot write the staged tree: %v", err)
			}
			subject.StagedTree = strings.TrimSpace(string(tree))
			if err := retain(*subject); err != nil {
				return refuse(retry, "try again; --verbose shows the cause", "the commit can't be recorded before it is made, so nothing was committed", "cannot retain the unit subject before committing: %v", err)
			}
			var installed string
			var gateErr error
			commitErr := conn.commitToken(install, func() error {
				var err error
				installed, err = conn.commit(branch.CommitRequest{Repo: install, Remote: endpoint.Remote, EndpointTip: base,
					GoalID: goalID, Units: []string{unit}, OpID: subject.Operation + "-unit", Kind: branch.Unit,
					Amend: subject.Amends != "", Whole: review.Whole, CheckClaim: check, Transport: conn.transport,
					FrozenPatch: frozenPatch, ResumeWorktree: subject.GateWorktree,
					KeepWorktree: func() bool {
						for _, id := range subject.GateLaunches {
							current, err := runner.Manager.Status(id)
							if err != nil && !os.IsNotExist(err) || err == nil && !current.State.Terminal() {
								return true
							}
						}
						return false
					},
					BeforeCommit: func(dir, parent, tree string) error {
						if parent != review.Head || subject.Amends != "" || subject.GateSnapshot != nil {
							if subject.Amends != "" {
								if _, err := conn.rebaseGate(dir); err != nil {
									gateErr = err
									return err
								}
							}
							gate, err := runner.ProveRoundResult(review, dir, subject, retain)
							if err != nil {
								gateErr = err
								return err
							}
							subject.GateRunID = gate
						}
						if subject.Amends == "" {
							subject.ExpectedParent, subject.StagedTree = parent, tree
						}
						subject.Conflict = ""
						return retain(*subject)
					}})
				return err
			})

			var pending *launch.PublicationPending
			if errors.As(gateErr, &pending) {
				if !pending.Running {
					subject.GateWorktree = ""
					subject.GateSnapshot, subject.GateLaunches = nil, nil
					_ = retain(*subject)
				}
				return intentResult{Targets: targets, Outcome: intentInProgress, code: 3, Data: data, Summary: pending.Error(), next: retry, nextReason: "resumes the recorded publication check"}
			}
			if _, err := os.Stat(subject.GateWorktree); subject.GateWorktree == "" || os.IsNotExist(err) {
				subject.GateWorktree = ""
			}
			if commitErr != nil && subject.GateWorktree == "" {
				subject.GateSnapshot = nil
				subject.GateLaunches = nil
			}
			if err := retain(*subject); err != nil {
				return refuse(retry, "reconciles the retained subject", "the publication outcome could not be recorded", "%v", err)
			}
			var replayConflict *branch.OpError
			if errors.As(commitErr, &replayConflict) && replayConflict.Code == branch.ReplayConflictCode {
				subject.Conflict = commitErr.Error()
				if err := retain(*subject); err != nil {
					return refuse(retry, "retains the same conflict", "the publication conflict could not be recorded", "%v", err)
				}
				correction := append(slices.Clone(revise), "--reason", "resolve the retained publication conflict", "--by", "NAME")
				return refuse(correction, "a person admits a correction of this retained round with its original plan", "the retained change conflicts with the current branch; nothing was committed", "%s", subject.Conflict)
			}
			if commitErr != nil {
				after, _ := git(worktree, "rev-parse", "HEAD")
				installed = strings.TrimSpace(string(after))
			}
			commit, conflict := resolveUnitCommit(git, install, base, goalID, unit, *subject, installed)
			if conflict != nil {
				data["subject"] = subject
				if launch.IsCode(gateErr, "PUBLICATION_CHECK_RED") {
					correction := append(slices.Clone(revise), "--reason", "correct the retained publication check failure", "--by", "NAME")
					return refuse(correction, "a person admits a correction of this retained round", "the publication checks failed; nothing was committed", "%v", gateErr)
				}
				if gateErr != nil {
					next, reason := retry, "repeats the same publication after the cause is fixed"
					if launch.IsCode(gateErr, "BUDGET_REFUSED") || launch.IsCode(gateErr, "BUDGET_UNKNOWN") {
						next, reason = inv.publicArgv("goal", "budget", goalID, "BOX"), "a person sets the budget; then repeat this command"
					}
					return refuse(next, reason, gateErr.Error(), "%v", gateErr)
				}
				summary := fmt.Sprintf("the branch commit owner's result for unit %s does not bind round %d: %v", unit, review.Round.Number, conflict)
				if commitErr != nil {
					summary = fmt.Sprintf("the branch commit owner did not commit unit %s: %v", unit, commitErr)
				}
				return intentResult{Targets: targets, Outcome: intentFailed, code: 1, Data: data, Summary: summary,
					Details: launchDetails(commitErr),
					next:    inv.sameCommand(), nextReason: "the bound subject is kept; the same command reconciles or commits it once"}
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
	if fix != nil {
		fix.Commit, fix.State = subject.Commit, "reviewing"
		if err := plain.WriteFix(original.Path(), *fix); err != nil {
			return refuse(retry, "retains the fix commit", "the fix commit could not be recorded", "%v", err)
		}
		return inv.reviewLaneFix(targets, original.Path(), *fix, "--brief", inv.callerPath(chooseUnitValue(inv.input.text("brief"), review.BuildBrief)), "--build-brief-sha256", review.BuildBriefSHA256, "--join="+strconv.FormatBool(!inv.input.has("brief")))
	}
	data["commit"], data["tip"], data["operation"] = subject.Commit, subject.Tip, subject.Operation
	data["expectedParent"] = subject.ExpectedParent
	if subject.Amends != "" {
		data["amends"] = subject.Amends
	}
	targets = append(targets, intentTarget{Kind: "commit", ID: subject.Commit})
	if subject.Published == "" {
		base, err := tip()
		if err == nil && subject.Amends != "" {
			var carried branch.CarryResult
			err = conn.commitToken(install, func() error {
				var carryErr error
				carried, carryErr = conn.carry(branch.CarryRequest{Repo: install, Remote: endpoint.Remote, EndpointTip: base,
					GoalID: goalID, CheckClaim: check, Gate: conn.rebaseGate, SubjectCheck: conn.subjectCheck, Transport: conn.transport})
				return carryErr
			})
			data["carried"] = carried.Carried
			data["needsReview"], data["unknown"] = carried.NeedsReview, carried.Unknown
			for _, name := range carried.NeedsReview {
				carriedLines = append(carriedLines, "review needed: "+name+"; run: metasystem work review "+goalID+" --work "+name)
			}
			for _, name := range carried.Carried {
				carriedLines = append(carriedLines, "review carried: "+name)
			}
		}
		if err == nil {
			var pushed branch.PushResult
			err = review.Wait(func() error {
				var pushErr error
				pushed, pushErr = conn.push(branch.PushRequest{Repo: install, Remote: endpoint.Remote, EndpointTip: base,
					GoalID: goalID, OpID: subject.Operation + "-push", CheckClaim: check, Transport: conn.transport})
				return pushErr
			})
			if err == nil {
				// Carried review commits are part of the published branch tip.
				subject.Published, subject.Tip = pushed.Tip, pushed.Tip
				err = retain(*subject)
			}
		}
		if err != nil {
			return intentResult{Targets: targets, Outcome: intentPartial, code: 1, Data: data, text: carriedLines,
				Summary: fmt.Sprintf("unit %s is committed as %s but not published: %v", unit, subject.Commit, err),
				next:    inv.sameCommand(), nextReason: "the same command publishes this commit; it never makes another"}
		}
	}
	data["published"] = subject.Published
	args := []string{"--root", install, "--goal", goalID, "--unit", subject.Commit}
	if inv.input.has("model") {
		args = append(args, "--model", inv.input.text("model"))
	}
	if install != original.Path() {
		args = append(args, "--selected-installation", original.Path())
	}
	if inv.reviewWork == nil {
		retry, _ := strconv.ParseInt(inv.input.text("retry"), 10, 64)
		inv.reviewWork = &reviewWorkContext{goal: goalID, work: unit, run: record.ID, retry: retry}
	}
	if inv.reviewWork != nil {
		inv.reviewWork.attempt, inv.reviewWork.retain, inv.reviewWork.subject = review.Round.Number, retain, subject
		inv.reviewWork.run, inv.reviewWork.review = record.ID, &review
	}
	if inv.reviewWork != nil && inv.reviewWork.retry > 0 {
		args = append(args, "--retry", strconv.FormatInt(inv.reviewWork.retry, 10))
	}
	criticArgs := slices.Clone(args)
	if inv.input.has("brief") {
		criticArgs = append(criticArgs, "--brief", inv.callerPath(inv.input.text("brief")), "--build-brief-sha256", review.BuildBriefSHA256)
	} else if review.BuildBrief != "" {
		// The work's brief starts its read; an existing critic is joined.
		criticArgs = append(criticArgs, "--brief", review.BuildBrief, "--build-brief-sha256", review.BuildBriefSHA256, "--join")
	}
	var bundle *branch.UnitReadBundle
	var reason string
	read, inspectErr := inv.work().inspectRead(install, goalID, subject.Commit)
	switch {
	case inv.input.has("brief"):
		reason = "the supplied review brief requires a critic"
	case inspectErr != nil:
		reason = "the branch read record could not be inspected: " + inspectErr.Error()
	case read.RootJob != "":
		reason = "this build's review already started with a critic"
	default:
		path := filepath.Join(filepath.Dir(review.Round.Directory), fmt.Sprintf("unit-read-%d.json", review.Round.Number))
		body, err := os.ReadFile(path)
		switch {
		case err == nil:
			var saved branch.UnitReadBundle
			if err := json.Unmarshal(body, &saved); err != nil {
				reason = "the saved unit read bundle is damaged: " + err.Error()
			} else {
				bundle = &saved
			}
		case errors.Is(err, os.ErrNotExist):
			var revision uint64
			if projection, _, failed := inv.projection(); failed == nil && projection.Tree.Live[goalID] != nil {
				revision = projection.Tree.Live[goalID].Revision
			}
			bundle, reason = readPromotion(runner, review, *subject, revision, func(runtime, model string) (string, error) {
				return inv.work().resolveModel(intentConfPath(inv.layout), runtime, model)
			})
			if bundle != nil {
				body, err = json.MarshalIndent(bundle, "", "  ")
				if err == nil {
					// ReviewSubject holds the run's lock; a repeat reads these
					// same bytes even if the goal revision or aliases changed.
					err = os.WriteFile(path, append(body, '\n'), 0o600)
				}
				if err != nil {
					return intentResult{Targets: targets, Outcome: intentPartial, code: 1, Data: data,
						Summary: "the unit is published, but its clean read bundle could not be saved", Details: []string{err.Error()},
						next: retry, nextReason: "saves and records the same read"}
				}
			}
		default:
			reason = "the saved unit read bundle could not be read: " + err.Error()
		}
		if bundle != nil {
			args = append(args, "--unit-read", path)
			data["readLaunch"], data["readModel"] = bundle.ReadLaunch, bundle.ReadModel
		}
	}
	if bundle == nil {
		args = criticArgs
		data["readNotPromoted"] = reason
	}
	result := inv.commitReviewChecked(targets, install, goalID, subject.Commit, args, func(read branch.BranchReadResult) error {
		head, current, err := runner.WorktreeResult(worktree)
		if err != nil {
			return err
		}
		if current == "" && head != subject.Tip && read.AttestationCommit == "" && read.RootJob != "" {
			// A collected attestation can outlive a failed read-record save.
			// Only the same build, reviewer and check may cross this boundary.
			kind, kindErr := branch.KindOf(worktree, head, goalID)
			if kindErr == nil && kind.Kind == branch.Read && kind.CommitID == subject.Commit {
				base, baseErr := tip()
				if baseErr != nil {
					return baseErr
				}
				att, attErr := branch.ValidateAttestationAt(worktree, head, base, goalID, unit, subject.Commit)
				if attErr == nil && att.Source.Kind == "critic-root" && att.Source.RootJob == read.RootJob && att.Gate.RunID == read.GateRunID {
					return nil
				}
			}
		}
		if current != "" || head != subject.Tip && head != read.AttestationCommit {
			return fmt.Errorf("the worktree changed after the read; restore its result or revise before publishing")
		}
		return nil
	}, review.Wait, criticArgs)
	result.text = append(result.text, carriedLines...)
	if merged, ok := result.Data.(map[string]any); ok && merged["readNotPromoted"] != nil {
		bundle = nil
		delete(data, "readLaunch")
		delete(data, "readModel")
		if slices.Equal(result.next, inv.sameCommand()) {
			result.next = inv.workArgv(record, "review")
		}
	}
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
		if err := inv.retainPublication(subject, result, retain); err != nil {
			return intentResult{Targets: targets, Outcome: intentPartial, code: 1, Data: data, Summary: "the read is published, but its publication time could not be retained: " + err.Error(), next: retry, nextReason: "rechecks the published read; a missing publication time stays unavailable"}
		}
		if bundle != nil {
			result.Summary = fmt.Sprintf("the build's clean read %s by %s is the unit's read and is published", bundle.ReadLaunch, bundle.ReadModel)
		}
		if len(subject.TransferredTo) == 0 {
			result.next, result.nextReason = inv.goalNextStep(goalID)
		}
	}
	if bundle != nil && inv.input.has("model") {
		result.Summary += "; --model was not used because no critic started"
	}
	return result
}

// unitReadPromotion decides from retained facts alone. Models come from the
// launches, since a step's model may be an alias.
func unitReadPromotion(review launch.UnitReview, subject launch.UnitSubject, revision uint64, build, read launch.Record, report, readJSON []byte) (*branch.UnitReadBundle, string) {
	var reads []launch.UnitStep
	for _, step := range review.Round.Steps {
		if strings.HasPrefix(step.Name, "read") {
			reads = append(reads, step)
		}
	}
	if len(reads) != 1 {
		return nil, "the round must have exactly one read step"
	}
	step := reads[0]
	if revision == 0 || len(report) == 0 || len(readJSON) == 0 || read.State != launch.Completed || read.Kind != "read" || !read.VerdictIsCounting() {
		return nil, "the read's report, completed launch or goal revision is unavailable"
	}
	if read.Read != nil && read.Read.Material != 0 || read.Read == nil && len(read.AdapterData["unitStopInputs"]) > 0 {
		return nil, "the structured read is unavailable or has material findings"
	}
	if step.State != launch.StepPassed || !branch.UnitReadVerdictIsLand(step.Verdict, string(report)) {
		return nil, "the read did not pass with VERDICT: land"
	}
	if step.VerdictCounts == nil || !*step.VerdictCounts {
		return nil, "the read's verdict does not count"
	}
	var buildModel, readModel string
	_ = json.Unmarshal(build.AdapterData["model"], &buildModel)
	_ = json.Unmarshal(read.AdapterData["model"], &readModel)
	if buildModel == "" || readModel == "" || buildModel == readModel {
		return nil, "the read and build need different, non-empty resolved models"
	}
	parent := subject.ExpectedParent
	if subject.Amends != "" {
		parent = subject.AmendsParent
	}
	if review.Base != parent {
		return nil, "the run's base is not the commit's parent"
	}
	runtime := strings.TrimSuffix(strings.TrimSuffix(read.Adapter, "-exec"), "-headless")
	var canonical []byte
	var digest string
	if read.Read != nil {
		canonical, digest = read.Read.Canonical()
	}
	return &branch.UnitReadBundle{CanonicalRead: canonical, ReadDigest: digest, SchemaVersion: 1, Goal: review.Record.Goal, Commit: subject.Commit, UnitRun: review.Record.ID,
		Round: review.Round.Number, ReadLaunch: step.LaunchID, ReadRuntime: runtime, ReadModel: readModel, BuildModel: buildModel,
		ExaminedBase: review.Base, ExaminedTree: subject.StagedTree, GoalRevision: revision,
		VerdictLine: "VERDICT: land", Report: string(report), LaunchRecord: string(readJSON)}, ""
}

func readPromotion(runner *launch.UnitRunner, review launch.UnitReview, subject launch.UnitSubject, revision uint64, resolve func(runtime, model string) (string, error)) (*branch.UnitReadBundle, string) {
	if runner.InheritedFindings != nil {
		rows, err := runner.InheritedFindings(review.Record.Goal, review.Record.Unit)
		if err != nil {
			return nil, "the destination's inherited evidence cannot be read"
		}
		if len(rows) > 0 {
			return nil, "the destination needs a committed read of its complete inherited change"
		}
	}
	var build, read launch.Record
	var report, readJSON []byte
	if runner.Manager != nil {
		for _, step := range review.Round.Steps {
			if strings.HasPrefix(step.Name, "build") {
				build, _ = runner.Manager.Store.Read(step.LaunchID)
			} else if strings.HasPrefix(step.Name, "read") {
				read, _ = runner.Manager.Store.Read(step.LaunchID)
				for _, output := range read.Outputs {
					if strings.HasSuffix(output.Path, ".md") {
						report, _ = os.ReadFile(output.Path)
						break
					}
				}
				if dir, err := runner.Manager.Store.StateDir(step.LaunchID); err == nil {
					readJSON, _ = os.ReadFile(filepath.Join(dir, "record.json"))
				}
			}
		}
	}
	for _, record := range []*launch.Record{&build, &read} {
		var model string
		if json.Unmarshal(record.AdapterData["model"], &model) != nil {
			continue
		}
		runtime := strings.TrimSuffix(strings.TrimSuffix(record.Adapter, "-exec"), "-headless")
		model, err := resolve(runtime, model)
		if err != nil {
			return nil, "the launches' model ids could not be resolved"
		}
		record.AdapterData["model"], _ = json.Marshal(model)
	}
	return unitReadPromotion(review, subject, revision, build, read, report, readJSON)
}

// neverLaunchedCause is why a review's dispatch started nothing, in the
// dispatch's own words, and its remedy line when it gave one: the
// delegate's refusal detail when it reported one, else the refusal's own
// lines after the read owner's heading.
func neverLaunchedCause(err error) (cause, remedy string) {
	text := err.Error()
	var outcome *readDelegateOutcomeError
	if errors.As(err, &outcome) && strings.TrimSpace(outcome.Outcome.Detail) != "" {
		text = outcome.Outcome.Detail
	}
	var lines []string
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line != "" && line != branch.ReadDispatchFailed {
			lines = append(lines, line)
		}
	}
	if len(lines) == 0 {
		return "the dispatch refused without a reason", ""
	}
	cause = lines[0]
	for _, heading := range []string{"brief authority admission refused: ", "dispatch refused: "} {
		cause = strings.TrimPrefix(cause, heading)
	}
	if len(lines) > 1 {
		remedy = lines[1]
	}
	return cause, remedy
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

// closerAt is this invocation aimed at the installation at root, the checkout
// the review ran in, so the close owner writes the review's records there.
// root arrives as a plain path and is admitted as an installation before it
// replaces the selected one: a directory that holds no metasystem.conf is
// refused, never closed into.
func (inv *intentInvocation) closerAt(targets []intentTarget, root string) (*intentInvocation, *intentResult) {
	installation, err := stateroot.ParseInstallation(root)
	if err != nil {
		return nil, &intentResult{Targets: targets, Outcome: intentRefused, code: 1, Summary: "the review's checkout holds no MetaSystem installation, so nothing was closed",
			next: inv.publicArgv("system", "check"), nextReason: "names what is wrong here", Details: []string{err.Error()}}
	}
	closer := *inv
	closer.layout.InstallationRoot = installation
	return &closer, nil
}

// commitReview requests, collects and publishes the committed read of one
// Goal-Unit commit through the branch read owner and the collected-read
// publication owner, with its records in root. A terminal critic whose
// chain is not closed yields the author's close and collects nothing; a
// repeat publishes the same attestation without reading again. review
// commit and review run share it. A refused unit read uses the supplied
// critic arguments; if that start also fails, the same review continues it.
func (inv *intentInvocation) commitReview(targets []intentTarget, root, goalID, unit string, args []string, fallback ...[]string) (out intentResult) {
	if full, err := inv.work().git(root, "rev-parse", "--verify", unit+"^{commit}"); err == nil {
		unit = strings.TrimSpace(string(full))
	}
	if work := inv.workOfCommit(goalID, unit); work != nil {
		runner := inv.unitRunner()
		err := runner.ReviewSubject(work.Run, func(review launch.UnitReview, retain func(launch.UnitSubject) error) error {
			caller := *inv
			caller.reviewWork = &reviewWorkContext{goal: goalID, work: work.Unit, run: work.Run, attempt: review.Round.Number, subject: review.Subject, retain: retain, review: &review}
			out = caller.commitReviewChecked(targets, root, goalID, unit, args, func(read branch.BranchReadResult) error {
				head, dirty, err := runner.WorktreeResult(root)
				if err != nil {
					return err
				}
				if dirty != "" || head != review.Subject.Tip && head != read.AttestationCommit {
					return fmt.Errorf("the read's worktree changed; restore its result or revise this work before publishing")
				}
				return nil
			}, review.Wait, fallback...)
			return nil
		})
		if err != nil {
			return inv.treeFailure(err)
		}
		return out
	}
	return inv.commitReviewChecked(targets, root, goalID, unit, args, nil, nil, fallback...)
}

func (inv *intentInvocation) commitReviewChecked(targets []intentTarget, root, goalID, unit string, args []string, check func(branch.BranchReadResult) error, wait func(func() error) error, fallback ...[]string) (out intentResult) {
	owners := inv.delivery()
	projection, _, problem := inv.projection()
	if problem != nil {
		return *problem
	}
	file, _ := goalRecord(projection, goalID)
	if file != nil && slices.ContainsFunc(file.UnitDrops, func(drop goal.UnitDrop) bool {
		return file.ExcludesScope(drop.Unit, "result:"+drop.Commit) && slices.Contains(drop.Covered, unit)
	}) {
		result, code, err := owners.branchRead(args)
		if err != nil {
			return intentResult{Targets: targets, Outcome: intentFailed, code: code, Summary: err.Error(), next: inv.sameCommand()}
		}
		return intentResult{Targets: targets, Outcome: intentConfirmed, Summary: "the covered build is dropped under the person's scope exclusion; its prior read remains", Data: result}
	}
	var branchRead func([]string) (branch.BranchReadResult, int, error)
	installed, inspectErr := inv.work().inspectRead(root, goalID, unit)
	alreadyPublished := inspectErr == nil && installed.Published
	changed, discardRecord := false, false
	newVersion := "SHA"
	changedResult := func() intentResult {
		return intentResult{Targets: targets, Outcome: intentRefused, code: 1, Data: map[string]any{"cause": "environment"},
			Summary: "the worktree changed after the read; review a new version of this work before publishing",
			next:    inv.publicArgv("work", "review", "--commit", newVersion, "--goal", goalID), nextReason: "starts a new read of the changed version; commit any uncommitted changes first and use their new commit"}
	}
	if alreadyPublished {
		check, wait = nil, nil
		branchRead = func([]string) (branch.BranchReadResult, int, error) {
			installed.State = "already-collected"
			return installed, 0, nil
		}
	} else if check == nil {
		runner := inv.unitRunner()
		release, err := runner.ReserveMutation(root, goalID, "read-publication")
		if err != nil {
			return inv.treeFailure(err)
		}
		defer release()
		identity := sha256.Sum256([]byte(goalID + "\x00" + unit))
		path := filepath.Join(root, "artifacts", "agents", "read-publication", fmt.Sprintf("%x", identity))
		// Publication always compares against the reviewed commit. This record
		// keeps retirement across invocations without changing the critic's evidence.
		var record struct {
			ReviewedCommit string `json:"reviewedCommit"`
			Retired        bool   `json:"retired,omitempty"`
			ChangedTip     string `json:"changedTip,omitempty"`
			RootJob        string `json:"rootJob,omitempty"`
		}
		err = runner.MutationSection(root, func(_ *launch.UnitRunner) error {
			retained, err := os.ReadFile(path)
			if err == nil {
				// A plain saved tip is an active record from an older engine;
				// it never replaces the reviewed commit as the baseline.
				if len(retained) == 40 {
					if _, err := hex.DecodeString(string(retained)); err != nil {
						return err
					}
					record.ReviewedCommit = unit
					return nil
				}
				if err := json.Unmarshal(retained, &record); err != nil {
					return err
				}
				if record.ReviewedCommit != unit {
					return fmt.Errorf("the publication record does not match the reviewed commit")
				}
				return nil
			}
			if !os.IsNotExist(err) {
				return err
			}
			record.ReviewedCommit = unit
			data, err := json.Marshal(record)
			if err != nil {
				return err
			}
			durable, err := atomicfile.WriteText(path, string(data), root)
			if err == nil && !durable {
				err = fmt.Errorf("the publication record's durability is unknown")
			}
			return err
		})
		if err != nil {
			return inv.treeFailure(err)
		}
		defer func() {
			if !changed && !discardRecord {
				return
			}
			if err := runner.MutationSection(root, func(_ *launch.UnitRunner) error {
				if changed {
					data, err := json.Marshal(record)
					if err != nil {
						return err
					}
					durable, err := atomicfile.WriteText(path, string(data), root)
					if err == nil && !durable {
						err = fmt.Errorf("the retired read's durability is unknown")
					}
					return err
				}
				err := os.Remove(path)
				if os.IsNotExist(err) {
					return nil
				}
				return err
			}); err != nil {
				out = inv.treeFailure(err)
			}
		}()
		check = func(read branch.BranchReadResult) error {
			current, dirty, err := runner.WorktreeResult(root)
			if err != nil {
				return err
			}
			paths, err := launch.UnitResultPaths(dirty)
			if err != nil {
				return err
			}
			if !slices.ContainsFunc(paths, func(path string) bool { return branch.PathClass(path) != branch.ClassExcluded }) {
				dirty = ""
			}
			// Uncommitted changes hold publication without retiring the read:
			// once they are committed or removed, the same read is judged again.
			if dirty != "" && !record.Retired {
				return fmt.Errorf("the worktree has uncommitted changes; commit or remove them, then repeat this command")
			}
			refuse := record.Retired
			if dirty == "" && current != unit {
				git := inv.work().git
				base, err := git(root, "merge-base", unit, current)
				if err != nil {
					return err
				}
				if strings.TrimSpace(string(base)) != unit {
					refuse = true
				} else {
					paths, err := git(root, "diff-tree", "-r", "-z", "--no-commit-id", "--no-renames", "--name-only", unit+"^", unit)
					if err != nil {
						return err
					}
					reviewedPaths := splitNUL(paths)
					if !refuse {
						differs, err := git(root, "diff-tree", "-r", "-z", "--no-commit-id", "--no-renames", "--name-only", unit, current)
						if err != nil {
							return err
						}
						for _, path := range splitNUL(differs) {
							if slices.Contains(reviewedPaths, path) {
								refuse = true
								break
							}
						}
					}
					if refuse {
						// A later read record or unrelated unit is not the new
						// version of the files whose read was retired.
						args := append([]string{"log", "-1", "--format=%H", unit + ".." + current, "--"}, reviewedPaths...)
						latest, err := git(root, args...)
						if err != nil {
							return err
						}
						if commit := strings.TrimSpace(string(latest)); commit != "" {
							newVersion = commit
						}
					}
				}
			}
			if refuse {
				changed = true
				if !record.Retired {
					record.Retired, record.ChangedTip, record.RootJob = true, current, read.RootJob
				}
				return fmt.Errorf("the worktree changed after the read; review a new version of this work before publishing")
			}
			return nil
		}
		branchRead = func(args []string) (result branch.BranchReadResult, code int, err error) {
			err = runner.MutationSection(root, func(_ *launch.UnitRunner) error {
				installed, _ := inv.work().inspectRead(root, goalID, unit)
				if err := check(installed); err != nil {
					return err
				}
				var readErr error
				result, code, readErr = owners.branchRead(args)
				return readErr
			})
			return
		}
		wait = func(publish func() error) error {
			return runner.MutationSection(root, func(bound *launch.UnitRunner) error {
				// Collection installed this invocation's attestation; the reviewed
				// paths must still match before the push releases the lock.
				read, err := inv.work().inspectRead(root, goalID, unit)
				if err != nil {
					return err
				}
				if err := check(read); err != nil {
					return err
				}
				return bound.CommandWait(publish)
			})
		}
	} else {
		branchRead = func(args []string) (branch.BranchReadResult, int, error) {
			installed, _ := inv.work().inspectRead(root, goalID, unit)
			if err := check(installed); err != nil {
				return branch.BranchReadResult{}, 1, err
			}
			return owners.branchRead(args)
		}
	}
	result, code, err := branchRead(args)
	if changed {
		return changedResult()
	}
	if err != nil && slices.Contains(args, "--unit-read") && len(fallback) > 0 {
		reason := strings.SplitN(err.Error(), "\nrun:", 2)[0]
		installed, inspectErr := inv.work().inspectRead(root, goalID, unit)
		if inspectErr == nil && installed.RootJob == "" && installed.AttestationCommit != "" && installed.State == "collected" {
			result, code, err = installed, 0, nil
			result.State = "already-collected"
		} else {
			args = fallback[0]
			result, code, err = branchRead(args)
		}
		defer func() {
			data, _ := out.Data.(map[string]any)
			if data == nil {
				data = map[string]any{}
				out.Data = data
			}
			data["readNotPromoted"] = reason
		}()
	}
	if err == nil && result.RootJob != "" && inv.reviewWork != nil && slices.Contains([]string{"closed", "collected", "already-collected"}, result.State) {
		store := branch.CriticStore(root, result.RootJob)
		newest, readErr := inv.newestRoundAt(store, result.RootJob)
		if readErr == nil {
			readErr = retainWorkExamination(inv.reviewWork, result.RootJob, newest, inv.returnPathAt(store, result.RootJob, recordRound(newest)))
		}
		if readErr != nil {
			return intentResult{Outcome: intentFailed, code: 1, Summary: "the review decision could not be retained", Details: []string{readErr.Error()}, next: inv.sameCommand()}
		}
	}
	if err == nil && result.State == "closed" {
		// The critic's records are where it was dispatched: this
		// installation's store, or its primary checkout's when the critic
		// was dispatched there (branch.CriticStore, the read owner's rule).
		store := branch.CriticStore(root, result.RootJob)
		// The branch read calls a terminal critic closed; collection needs
		// the chain's own recorded closure, which only the close owner writes.
		if waiting := inv.criticClosure(targets, store, unit, goalID, result.RootJob); waiting != nil {
			if data, _ := waiting.Data.(map[string]any); data["failedRound"] != nil {
				// A failed examination has no findings to decide; its
				// continuation is the retry, never a decisions file.
				return *waiting
			}
			if inv.reviewWork == nil && inv.input.has("dispositions") {
				// An explicitly named commit's review closes with the author's
				// decisions through the whole close owner, then collects below.
				// A work's newest attempt records the examination first.
				inv.commitExamination(store, goalID, unit, result.RootJob)
				closer, refused := inv.closerAt(targets, store)
				if refused != nil {
					return *refused
				}
				closed := closer.closeChain(result.RootJob)
				if closed.Outcome != intentConfirmed && closed.Outcome != intentUnchanged {
					closed.Targets = append(targets, closed.Targets...)
					closed.Summary = "the examination finished, but its close did not complete: " + closed.Summary
					inv.riskRemedy(&closed, store, goalID, result.RootJob,
						append(inv.canonicalReviewArgv(targets, goalID, unit), "--dispositions", inv.input.text("dispositions")))
					return closed
				}
			} else if inv.reviewWork == nil {
				join, clean := inv.cleanExaminationJoin(store, goalID, unit, result.RootJob)
				if !clean {
					waiting.next = append(inv.canonicalReviewArgv(targets, goalID, unit), "--dispositions", "FILE")
					waiting.nextReason = "FILE decides every finding of " + result.RootJob
					// The decisions template names the findings, as the work
					// form's does, bound to the work when the commit is a
					// work's newest attempt.
					if binding, findings, template, ok := inv.commitExamination(store, goalID, unit, result.RootJob); ok && len(findings) > 0 {
						if _, err := os.Stat(template); err != nil {
							err = os.WriteFile(template, []byte(decisionsDocument(binding, findings)), 0o600)
							ok = err == nil
						}
						if ok {
							waiting.nextReason = "FILE decides every finding of " + result.RootJob + "; start from " + template
							if data, _ := waiting.Data.(map[string]any); data != nil {
								data["template"] = template
							}
						}
					}
					return *waiting
				}
				// A completed examination with no findings has nothing to
				// decide: its empty join closes through the whole close owner.
				closer, refused := inv.closerAt(targets, store)
				if refused != nil {
					return *refused
				}
				closer.input = intentInput{values: map[string][]string{"dispositions": {join}}}
				closed := closer.closeChain(result.RootJob)
				if closed.Outcome != intentConfirmed && closed.Outcome != intentUnchanged {
					closed.Targets = append(targets, closed.Targets...)
					closed.Summary = "the examination finished clean, but its close did not complete: " + closed.Summary
					inv.riskRemedy(&closed, store, goalID, result.RootJob, inv.canonicalReviewArgv(targets, goalID, unit))
					return closed
				}
			} else {
				// A goal's work review completes the author's decision and the
				// whole close here, then collects below.
				if pending := inv.closeWorkReview(targets, store, unit, result.RootJob); pending != nil {
					return *pending
				}
			}
		}
		if refuted := inv.refutedClose(targets, store, goalID, result.RootJob); refuted != nil {
			return *refuted
		}
		result, code, err = branchRead(append(args, "--collect"))
	}
	if err != nil {
		if changed {
			return changedResult()
		}
		var refusal *branch.OpError
		var neverLaunched *branch.ReadNeverLaunchedError
		switch {
		case errors.As(err, &refusal) && refusal.Code == branch.ReadDispatchPendingCode:
			return intentResult{Targets: targets, Outcome: intentInProgress, Summary: "whether the review's critic started isn't known yet; it is never started twice",
				Data: map[string]any{"code": refusal.Code}, next: inv.sameCommand(), nextReason: "finds out and continues", Details: []string{err.Error()}}
		case errors.As(err, &neverLaunched):
			cause, remedy := neverLaunchedCause(err)
			if remedy == "" {
				remedy = "nothing was started or kept, so the same command asks again"
			}
			return intentResult{Targets: targets, Outcome: intentRefused, code: max(code, 1), Summary: "no reviewer was started: " + cause,
				next: inv.sameCommand(), nextReason: remedy, Details: []string{err.Error()}}
		case errors.As(err, &refusal):
			return intentResult{Targets: targets, Outcome: intentRefused, code: max(code, 1), Summary: refusal.Message, Data: map[string]any{"code": refusal.Code},
				next: inv.sameCommand(), nextReason: "once that is settled", Details: []string{err.Error()}}
		}
		return intentResult{Targets: targets, Outcome: intentRefused, code: max(code, 1), Summary: "the review couldn't be requested",
			next: inv.sameCommand(), nextReason: "try again; --verbose shows the cause", Details: []string{err.Error()}}.withCause(err)
	}
	data := map[string]any{"state": result.State, "rootJob": result.RootJob, "gateRun": result.GateRunID, "attestation": result.AttestationCommit}
	if result.RootJob != "" {
		targets = append(targets, jobTarget(result.RootJob))
	}
	if result.State == "dropped" {
		return intentResult{Targets: targets, Outcome: intentConfirmed, Data: data, Summary: "the unit is dropped; no new read is needed"}
	}
	if result.State != "collected" && result.State != "already-collected" {
		return intentResult{Targets: targets, Outcome: intentInProgress, Data: data,
			Summary: "the review of this work is in progress",
			next:    inv.canonicalReviewArgv(targets, goalID, unit), nextReason: "continues this review and publishes its result when ready"}
	}
	if check != nil {
		if err := check(result); err != nil {
			if changed {
				return changedResult()
			}
			data["cause"] = "environment"
			return intentResult{Targets: targets, Outcome: intentRefused, code: 1, Data: data,
				Summary: err.Error(), next: inv.canonicalReviewArgv(targets, goalID, unit), nextReason: "publishes once the committed result is restored or corrected"}
		}
	}
	var published branch.PublishReadResult
	publish := func() error {
		if alreadyPublished {
			published = branch.PublishReadResult{Attestation: result.AttestationCommit, OpID: branch.PublishOperationID(result.GateRunID), State: "current"}
			return nil
		}
		var err error
		published, err = owners.publishRead(root, goalID, unit)
		return err
	}
	if wait != nil {
		err = wait(publish)
	} else {
		err = publish()
	}
	data["publication"] = published
	if err != nil {
		if changed {
			return changedResult()
		}
		return intentResult{Targets: targets, Outcome: intentPartial, code: 1, Data: data,
			Summary: fmt.Sprintf("the review is complete, but its result has not yet been published: %v", err),
			next:    inv.canonicalReviewArgv(targets, goalID, unit), nextReason: "publishes the same attestation under the same push operation; no critic or commit is repeated"}
	}
	discardRecord = true
	if result.TransferCoverage != nil {
		coverage := *result.TransferCoverage
		completed := inv.goalAct(goalID, "complete transferred findings", inv.syncOwner("complete-transfers", []string{"--root", inv.stateRoot, "--id", goalID}, nil, false, func(req goal.VerbRequest, _ *syncFlags) (goal.PublishResult, error) {
			return goal.CompleteTransfers(req, goalID, coverage.TargetUnit, coverage)
		}, "id"))
		if completed.Outcome != intentConfirmed && completed.Outcome != intentUnchanged {
			return completed
		}
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
// workOfCommit is the named work of goal goalID whose newest attempt is
// commit, if any: a commit-form review of that commit examines the work.
func (inv *intentInvocation) workOfCommit(goalID, commit string) *launch.NamedWork {
	worktree, problem := inv.goalWorktree(goalID)
	if problem != nil {
		return nil
	}
	works, err := inv.unitRunner().NamedWork(worktree, goalID)
	if err != nil {
		return nil
	}
	for _, work := range works {
		if work.Record == nil || work.Running() || len(work.Record.Rounds) == 0 {
			continue
		}
		newest := work.Record.Rounds[len(work.Record.Rounds)-1].Number
		for _, subject := range work.Record.Subjects {
			if subject.Round == newest && subject.Commit == commit {
				found := work
				return &found
			}
		}
	}
	return nil
}

// commitExamination is commit-form examination rootJob of commit, once it
// completed, as the binding of its decisions, its findings and the path of
// its decisions template. When the commit is a named work's newest attempt,
// the examination is recorded with that attempt, as the work form records
// it, so the work's revise binds to it, and the binding names the work.
func (inv *intentInvocation) commitExamination(store, goalID, commit, rootJob string) (reviewBinding, []intentFinding, string, bool) {
	newest, err := inv.newestRoundAt(store, rootJob)
	if err != nil || recordText(newest, "status") != "completed" {
		return reviewBinding{}, nil, "", false
	}
	round := recordRound(newest)
	returnPath := inv.returnPathAt(store, rootJob, round)
	digest, _, readErr := reviewReturnDigest(returnPath)
	findings, _, parseErr := readIntentFindings(returnPath)
	if readErr != nil || parseErr != nil {
		return reviewBinding{}, nil, "", false
	}
	binding := reviewBinding{Goal: goalID, Subject: commit, Examination: rootJob, Round: round, Return: digest}
	if full, err := goalBranchGit(store, "rev-parse", "--verify", commit+"^{commit}"); err == nil {
		if work := inv.workOfCommit(goalID, full); work != nil {
			// A subject that can't be recorded leaves the binding to the
			// commit alone; the work's revise then names what it lacks.
			_ = inv.unitRunner().ReviewSubject(work.Run, func(review launch.UnitReview, retain func(launch.UnitSubject) error) error {
				if review.Subject == nil || review.Subject.Commit != full {
					return nil
				}
				if review.Subject.Examination != rootJob || review.Subject.ExaminationRound != round || review.Subject.ExaminationReturnPath != returnPath {
					subject := *review.Subject
					subject.Examination, subject.ExaminationRound = rootJob, round
					subject.ExaminationReturnPath = returnPath
					if err := retain(subject); err != nil {
						return err
					}
				}
				binding.Work, binding.Attempt, binding.Subject = work.Unit, review.Round.Number, full
				return nil
			})
		}
	}
	return binding, findings, filepath.Join(filepath.Dir(returnPath), "decisions.md"), true
}

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

// retainPublication timestamps only a push made by this review call.
func (inv *intentInvocation) retainPublication(subject *launch.UnitSubject, result intentResult, retain func(launch.UnitSubject) error) error {
	data, _ := result.Data.(map[string]any)
	published, ok := data["publication"].(branch.PublishReadResult)
	if subject == nil || subject.PublishedAt != "" || !ok || published.State != "pushed" {
		return nil
	}
	subject.PublishedAt = inv.unitRunner().Manager.Now().UTC().Format(time.RFC3339Nano)
	return retain(*subject)
}
