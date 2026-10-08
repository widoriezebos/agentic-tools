package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/textui"
)

// A goal's work is the named units the unit runner holds for the goal's own
// worktree. Selection reads them through the runner and never picks by time:
// exactly one eligible item is taken, none gives the stage-appropriate
// result, and several are listed with the command that names each.

// goalWork is the goal's named work and the worktree it lives in. A goal
// without a worktree has no work yet.
func (inv *intentInvocation) goalWork(id string) ([]launch.NamedWork, *intentResult) {
	projection, _, problem := inv.projection()
	if problem != nil {
		return nil, problem
	}
	if file, _ := goalRecord(projection, id); file == nil {
		return nil, &intentResult{Outcome: intentRefused, code: 1, Targets: inv.targets(id),
			Summary: fmt.Sprintf("no goal %s on the accepted ledger; nothing was done", shellCommand([]string{id})),
			next:    inv.publicArgv("goal", "list", "--all"), nextReason: "list the goals by id"}
	}
	worktree, problem := inv.goalWorktree(id)
	if problem != nil {
		if problem.Outcome == intentFailed {
			return nil, problem
		}
		return nil, nil
	}
	work, err := inv.unitRunner().NamedWork(worktree, id)
	if err != nil {
		return nil, &intentResult{Outcome: intentFailed, code: 1, Targets: inv.targets(id),
			Summary: fmt.Sprintf("goal %s's work cannot be read: %v; nothing was done", id, err), next: inv.publicArgv("system", "check"),
			nextReason: "diagnose the saved work"}
	}
	return work, nil
}

// workStage is a work item's stage in plain words.
func workStage(work launch.NamedWork, readers ...func(string, string, string) (branch.BranchReadResult, error)) string {
	inspect := branch.InspectBranchRead
	if len(readers) > 0 {
		inspect = readers[0]
	}
	switch {
	case work.Record == nil:
		return "starting"
	case work.Running():
		return "running"
	}
	outcome := ""
	if rounds := work.Record.Rounds; len(rounds) > 0 {
		round := rounds[len(rounds)-1]
		if round.Transferred {
			return "transferred; its destinations remain required"
		}
		if round.Stop != nil && round.Stop.Decision == "stop" {
			return "stopped: " + round.Stop.Class + "; " + round.Stop.Handoff
		}
		outcome = round.Outcome
	}
	if launch.UnitReviewReadyOutcomes[outcome] {
		if subject := currentSubject(work); subject != nil && subject.Commit != "" {
			if read, err := inspect(work.Record.Worktree, work.Record.Goal, subject.Commit); err == nil && read.State == "review refused" {
				return "review refused: " + read.DispatchRefusal
			} else if err == nil && read.State == "collected" && read.Published {
				return "reviewed; its read is collected and published (attestation " + shortSHA(read.AttestationCommit) + ")"
			} else if err == nil && read.State == "collected" {
				return "reviewed; its read is collected but not yet published (attestation " + shortSHA(read.AttestationCommit) + ")"
			} else if err == nil && read.State == "examining" {
				return "committed and under independent examination"
			}
		}
		return "built, ready for review"
	}
	return "stopped (" + outcome + "), needs a revision"
}

func workAttempt(work launch.NamedWork) int {
	if work.Record == nil {
		return 0
	}
	return len(work.Record.Rounds)
}

// namedWorkOnly narrows the work to the --work name when one is given.
func (inv *intentInvocation) namedWorkOnly(id string, work []launch.NamedWork) ([]launch.NamedWork, *intentResult) {
	name := inv.input.text("work")
	if name == "" {
		return work, nil
	}
	for _, one := range work {
		if one.Unit == name {
			return []launch.NamedWork{one}, nil
		}
	}
	names := make([]string, 0, len(work))
	for _, one := range work {
		names = append(names, one.Unit)
	}
	summary := fmt.Sprintf("goal %s has no work named %s; nothing was done", id, shellCommand([]string{name}))
	if len(names) > 0 {
		return nil, &intentResult{Outcome: intentRefused, code: 1, Targets: inv.targets(id), Summary: summary + "; its work is " + strings.Join(names, ", "),
			next: append(inv.typedArgvLess("work"), "--work", names[0]), nextReason: "or another of its work"}
	}
	return nil, &intentResult{Outcome: intentRefused, code: 1, Targets: inv.targets(id), Summary: summary,
		next: inv.publicArgv("work", "build", id, "--work", name, "--brief", "FILE", "--check", "COMMAND..."), nextReason: "start that work"}
}

func workTargets(id string, work launch.NamedWork) []intentTarget {
	return []intentTarget{{Kind: "goal", ID: id}, {Kind: "work", ID: work.Unit}}
}

// goalUnitStages is the unit list the status verb and the seat prompt share.
func (inv *intentInvocation) goalUnitStages(id string) (work []launch.NamedWork, manual []manualWorkItem, views []map[string]any, lines []string, units []steward.UnitStage, problem *intentResult) {
	work, problem = inv.goalWork(id)
	if problem == nil {
		work, manual = inv.rangeWork(id, work)
		if name := inv.input.text("work"); name != "" {
			manual = slices.DeleteFunc(manual, func(item manualWorkItem) bool { return item.Unit != name })
			if len(manual) > 0 {
				work = nil
			}
		}
		if len(manual) == 0 {
			work, problem = inv.namedWorkOnly(id, work)
		}
	}
	if problem != nil {
		return
	}
	views, lines, units = []map[string]any{}, []string{}, []steward.UnitStage{}
	for _, one := range work {
		stage := workStage(one, inv.work().inspectRead)
		view := map[string]any{"work": one.Unit, "stage": stage, "attempt": workAttempt(one)}
		if one.Record != nil {
			view["state"] = one.Record.State
			if len(one.Record.Rounds) > 0 {
				r := one.Record.Rounds[len(one.Record.Rounds)-1]
				view["stop"], view["reads"] = r.Stop, r.Reads
			}
		}
		views = append(views, view)
		next, _ := inv.workContinuation(id, one, true)
		lines = append(lines, fmt.Sprintf("  %s: %s, attempt %d; next: %s", one.Unit, stage, workAttempt(one), shellCommand(next)))
		at := ""
		if one.Record != nil {
			for _, round := range one.Record.Rounds {
				for _, step := range round.Steps {
					if step.StartedAt > at {
						at = step.StartedAt
					}
				}
			}
		}
		units = append(units, steward.UnitStage{Unit: one.Unit, Stage: stage, Line: lines[len(lines)-1], At: at})
	}
	for _, item := range manual {
		stage := item.stage(inv.work().inspectRead)
		views = append(views, map[string]any{"work": item.Unit, "stage": stage, "commit": item.Commit, "source": "goal branch"})
		next, _ := inv.manualContinuation(id, item)
		lines = append(lines, fmt.Sprintf("  %s: %s, version %s; next: %s", item.Unit, stage, shortSHA(item.Commit), shellCommand(next)))
		units = append(units, steward.UnitStage{Unit: item.Unit, Stage: stage, Line: lines[len(lines)-1]})
	}
	return
}

func runIntentStatusGoal(inv *intentInvocation, id string) int {
	if problem := inv.selectRoot(); problem != nil {
		return inv.render(*problem)
	}
	work, manual, views, lines, units, problem := inv.goalUnitStages(id)
	if problem != nil {
		return inv.render(*problem)
	}
	if len(manual) > 0 && len(work) == 0 {
		result := intentResult{Outcome: intentConfirmed, Targets: inv.targets(id), text: lines, Data: map[string]any{"goal": id, "work": views}}
		if len(manual) == 1 {
			result.Summary = fmt.Sprintf("goal %s: work %s is %s", id, manual[0].Unit, units[0].Stage)
			result.next, result.nextReason = inv.manualContinuation(id, manual[0])
		} else {
			result.Summary = fmt.Sprintf("goal %s has %d work items", id, len(manual))
		}
		return inv.renderGoalUnitStatus(result, len(units))
	}
	designs := []map[string]any{}
	var designNext []string
	var designReason string
	if !inv.input.has("work") {
		for _, view := range inv.goalDesignAttempts(id) {
			designs = append(designs, map[string]any{"document": view.Document, "attempt": view.Attempt.Attempt, "stage": designStage(view)})
			lines = append(lines, fmt.Sprintf("  design %s: %s, attempt %d", view.Document, designStage(view), view.Attempt.Attempt))
			switch {
			case view.SupervisorLost:
				designNext, designReason = inv.publicArgv("design", "stop", id, "--out", view.Document), "the author's supervisor is gone; stop the attempt"
			case !view.Record.State.Terminal() && view.Record.State != "":
				designNext, designReason = inv.publicArgv("work", "wait", id), "the design author is writing"
			case view.Attempt.Outcome == "published":
				designNext, designReason = inv.publicArgv("design", "review", view.Document, "--goal", id), "an independent critique examines the draft"
			}
		}
	}
	result := intentResult{Outcome: intentConfirmed, Targets: inv.targets(id), text: lines, Data: map[string]any{"goal": id, "work": views, "designs": designs}}
	// The goal's own card line (D14-r2, R23).
	if line, ok := inv.hostBoardView(inv.boardNow()).GoalLine(id, inv.boardNow(), time.Local); ok {
		result.text = append(result.text, "  "+line)
		result.Data.(map[string]any)["board"] = line
	}
	switch {
	case len(work) == 0 && len(designs) > 0:
		result.Summary = fmt.Sprintf("goal %s has %d design document(s) and no built work yet", id, len(designs))
		result.next, result.nextReason = designNext, designReason
	case len(work) == 0:
		result.Summary = fmt.Sprintf("goal %s has no work yet", id)
		result.next, result.nextReason = inv.publicArgv("work", "build", id, "--brief", "FILE", "--check", "COMMAND..."), "start the goal's first work"
	case len(work) == 1 && len(manual) == 0:
		result.Summary = fmt.Sprintf("goal %s: work %s is %s", id, work[0].Unit, units[0].Stage)
		result.next, result.nextReason = inv.workContinuation(id, work[0], inv.input.has("work"))
	default:
		result.Summary = fmt.Sprintf("goal %s has %d work items", id, len(work)+len(manual))
	}
	return inv.renderGoalUnitStatus(result, len(units))
}

// renderGoalUnitStatus keeps the shared unit lines whole in the status page.
func (inv *intentInvocation) renderGoalUnitStatus(result intentResult, unitCount int) int {
	if unitCount == 0 {
		return inv.render(result)
	}
	result.view = func(page *textui.Page) {
		page.Headline(result.Summary)
		section := page.Section("", "")
		for _, line := range result.text[:unitCount] {
			section.Fixed(strings.TrimSpace(line))
		}
		page.Legacy(result.text[unitCount:]...)
	}
	return inv.render(result)
}

// manualWorkItem is work the goal branch range holds as a Goal-Unit commit
// without a build whose recorded subject is that commit: written by hand, or
// corrected by hand after a build. The range is its only record.
type manualWorkItem struct {
	Unit, Commit, Worktree, Goal string
	// ReadsWaived says the goal lands its work without a read (tier 1).
	ReadsWaived bool
}

func (item manualWorkItem) read() (branch.BranchReadResult, error) {
	return branch.InspectBranchRead(item.Worktree, item.Goal, item.Commit)
}

func (item manualWorkItem) stage(readers ...func(string, string, string) (branch.BranchReadResult, error)) string {
	inspect := branch.InspectBranchRead
	if len(readers) > 0 {
		inspect = readers[0]
	}
	read, err := inspect(item.Worktree, item.Goal, item.Commit)
	switch {
	case err == nil && read.State == "review refused":
		return "review refused: " + read.DispatchRefusal
	case err == nil && read.State == "collected" && read.Published:
		return "reviewed; its read is collected and published (attestation " + shortSHA(read.AttestationCommit) + ")"
	case err == nil && read.State == "collected":
		return "reviewed; its read is collected but not yet published (attestation " + shortSHA(read.AttestationCommit) + ")"
	case err == nil && read.State == "examining":
		return "committed and under independent examination"
	case item.ReadsWaived:
		return "committed, ready to land without a read"
	}
	return "committed, ready for review"
}

func (inv *intentInvocation) manualContinuation(id string, item manualWorkItem) ([]string, string) {
	if item.ReadsWaived {
		return inv.goalNextStep(id)
	}
	read, err := inv.work().inspectRead(item.Worktree, item.Goal, item.Commit)
	if err == nil && read.State == "collected" && read.Published {
		return inv.goalNextStep(id)
	}
	return inv.publicArgv(append(reviewGoalWords(id), "--work", item.Unit)...), "an independent review examines this version, or continues its examination"
}

// rangeWork splits the goal's work between builds and the goal branch
// range: a unit the range holds is the build's work only while the build's
// recorded subject is that very commit; otherwise the committed version is
// shown from the range, so stale build bookkeeping never overshadows it.
func (inv *intentInvocation) rangeWork(id string, work []launch.NamedWork) ([]launch.NamedWork, []manualWorkItem) {
	worktree, problem := inv.goalWorktree(id)
	if problem != nil {
		return work, nil
	}
	conn := inv.connection()
	endpoint, err := conn.endpoint(inv.layout.InstallationRoot.Path())
	if err != nil {
		return work, nil
	}
	base, err := conn.endpointTip(inv.layout.InstallationRoot.Path(), endpoint)
	if err != nil {
		return work, nil
	}
	tip, err := inv.work().git(worktree, "rev-parse", "--verify", "HEAD^{commit}")
	if err != nil {
		return work, nil
	}
	commits, err := branch.ValidateRangeWithGit(inv.goalWorktreeInstallation(worktree), base, strings.TrimSpace(string(tip)), id, inv.work().git)
	if err != nil {
		return work, nil
	}
	waived := false
	if projection, _, problem := inv.projection(); problem == nil && projection.Tree != nil {
		waived = goal.ReadsWaived(projection.Tree.Live[id])
	}
	var manual []manualWorkItem
	for _, commit := range commits {
		if commit.Kind != branch.Unit || len(commit.Units) != 1 {
			continue
		}
		name := commit.Units[0]
		index := slices.IndexFunc(work, func(one launch.NamedWork) bool { return one.Unit == name })
		if index >= 0 {
			if subject := currentSubject(work[index]); subject != nil && subject.Commit == commit.ID {
				continue
			}
			if subject := currentSubject(work[index]); subject == nil || subject.Commit == "" {
				continue
			}
			work = slices.Delete(work, index, index+1)
		}
		manual = append(manual, manualWorkItem{Unit: name, Commit: commit.ID, Worktree: worktree, Goal: id, ReadsWaived: waived})
	}
	return work, manual
}

// workContinuation is the public command that moves one work item on.
func (inv *intentInvocation) workContinuation(id string, work launch.NamedWork, named bool) ([]string, string) {
	suffix := []string{}
	if named {
		suffix = []string{"--work", work.Unit}
	}
	read := branch.BranchReadResult{}
	gapPerson := work.Record != nil && work.Record.MaxRounds > 0 && workAttempt(work) >= work.Record.MaxRounds
	if lastOutcome(work) == "build-gap" {
		policy, err := inv.unitRunner().ReviewPolicy()
		gapPerson = gapPerson || err != nil || policy == "person"
	}
	if subject := currentSubject(work); builtWork(work) && subject != nil && subject.Commit != "" {
		read, _ = inv.work().inspectRead(work.Record.Worktree, work.Record.Goal, subject.Commit)
	}

	switch {
	case work.Record != nil && len(work.Record.Rounds) > 0 && work.Record.Rounds[len(work.Record.Rounds)-1].Stop != nil && work.Record.Rounds[len(work.Record.Rounds)-1].Stop.Loop == "unit-build" && lastOutcome(work) != "build-size" && lastOutcome(work) != "build-gap":
		round := work.Record.Rounds[len(work.Record.Rounds)-1]
		if round.Cause == "environment" || round.Cause == "deadline" {
			return inv.workArgv(*work.Record, "review", "--reason", "TEXT", "--by", "NAME"), "a person reruns the retained failed step after repairing its environment"
		}
		return inv.workArgv(*work.Record, "revise", "--after", fmt.Sprint(workAttempt(work)), "--brief", "FILE", "--reason", "TEXT", "--by", "NAME"), "a person decides how to continue the failed step"
	case lastOutcome(work) == "build-gap" && gapPerson:
		return inv.workArgv(*work.Record, "revise", "--after", fmt.Sprint(workAttempt(work)), "--brief", "FILE", "--reason", "TEXT", "--by", "NAME"), "a person decides whether to admit another gap correction"
	case lastOutcome(work) == "build-size":
		return inv.workArgv(*work.Record, "review", "--reason", "TEXT", "--by", "NAME"), "a person decides whether to accept the retained change size"
	case work.Running():
		return inv.publicArgv(append([]string{"work", "wait", id}, suffix...)...), "the work is running; this waits for it"
	case read.State == "collected" && !read.Published:
		return inv.publicArgv(append(reviewGoalWords(id), suffix...)...), "the work's read is collected but not published; the same review publishes it"
	case read.State == "collected":
		return inv.goalNextStep(id)
	case launch.UnitReviewReadyOutcomes[lastOutcome(work)]:
		return inv.publicArgv(append(reviewGoalWords(id), suffix...)...), "the result is built; an independent review examines it"
	}
	return inv.publicArgv(append(append([]string{"work", "revise", id}, suffix...), "--after", fmt.Sprint(workAttempt(work)), "--brief", "FILE")...), "the attempt stopped; a correction brief starts one new attempt"
}

func lastOutcome(work launch.NamedWork) string {
	if work.Record == nil || len(work.Record.Rounds) == 0 {
		return ""
	}
	return work.Record.Rounds[len(work.Record.Rounds)-1].Outcome
}

// runIntentWaitWork waits for the goal's running work: the named item, or
// the only one running.
func runIntentWaitWork(inv *intentInvocation, id string) int {
	timeout, problem := inv.waitTimeout()
	if problem != nil {
		return inv.render(*problem)
	}
	if problem := inv.selectRoot(); problem != nil {
		return inv.render(*problem)
	}
	work, problem := inv.goalWork(id)
	if problem == nil {
		work, problem = inv.namedWorkOnly(id, work)
	}
	if problem != nil {
		return inv.render(*problem)
	}
	named := inv.input.has("work")
	var running []launch.NamedWork
	for _, one := range work {
		if one.Running() || named {
			running = append(running, one)
		}
	}
	switch len(running) {
	case 1:
		one := running[0]
		if one.Run == "" {
			return inv.render(intentResult{Outcome: intentInProgress, Targets: workTargets(id, one),
				Summary: fmt.Sprintf("work %s of goal %s is still being reserved by its build", one.Unit, id),
				next:    inv.sameCommand(), nextReason: "the same wait continues once the build recorded its run"})
		}
		again := inv.publicArgv("work", "wait", id)
		if named || len(work) > 1 {
			again = append(again, "--work", one.Unit)
		}
		return inv.render(inv.waitUnit(one.Run, timeout, workTargets(id, one), again))
	case 0:
		if designed, ok := inv.waitDesign(id, timeout); ok && !named {
			return inv.render(designed)
		}
		result := intentResult{Outcome: intentUnchanged, Targets: inv.targets(id), Data: map[string]any{"goal": id},
			Summary: fmt.Sprintf("goal %s has no running work; nothing to wait for", id)}
		if inv.goalQueuedToLand(id) {
			result.next, result.nextReason = inv.publicArgv("work", "wait", id, "--for", "landing"), "the goal is queued to land; this waits for its landing"
		} else {
			result.next, result.nextReason = inv.publicArgv("status", id), "the goal's work and what each needs next"
		}
		return inv.render(result)
	}
	names := make([]string, 0, len(running))
	for _, one := range running {
		names = append(names, one.Unit)
	}
	return inv.render(intentResult{Outcome: intentRefused, code: 2, Targets: inv.targets(id), Data: map[string]any{"candidates": names},
		Summary: fmt.Sprintf("goal %s has %d running work items (%s); nothing was done", id, len(running), strings.Join(names, ", ")),
		next:    inv.publicArgv("work", "wait", id, "--work", names[0]), nextReason: "or another of them"})
}

// goalQueuedToLand reports whether the goal waits in a landing slot.
func (inv *intentInvocation) goalQueuedToLand(id string) bool {
	projection, _, problem := inv.projection()
	if problem != nil {
		return false
	}
	file := projection.Tree.Live[id]
	return file != nil && file.State == goal.StateClaimed && file.Claimed != nil && file.Landing != nil
}

// selectWork picks the one work item a stage acts on: the --work name, else
// the only item in the first non-empty eligible set. Several eligible items
// are refused with each item's command; none returns nil with no problem.
func (inv *intentInvocation) selectWork(id, verb string, work []launch.NamedWork, eligible ...func(launch.NamedWork) bool) (*launch.NamedWork, *intentResult) {
	if inv.input.has("work") {
		named, problem := inv.namedWorkOnly(id, work)
		if problem != nil {
			return nil, problem
		}
		return &named[0], nil
	}
	for _, keep := range eligible {
		var matched []launch.NamedWork
		for _, one := range work {
			if keep(one) {
				matched = append(matched, one)
			}
		}
		switch len(matched) {
		case 0:
			continue
		case 1:
			return &matched[0], nil
		}
		names, lines := []string{}, []string{}
		for _, one := range matched {
			names = append(names, one.Unit)
			lines = append(lines, fmt.Sprintf("  %s (%s): %s", one.Unit, workStage(one), shellCommand(inv.publicArgv(append(workVerbWords(verb), id, "--work", one.Unit)...))))
		}
		return nil, &intentResult{Outcome: intentRefused, code: 2, Targets: inv.targets(id), text: lines, Data: map[string]any{"candidates": names},
			Summary: fmt.Sprintf("goal %s has %d work items this could mean (%s); nothing was done", id, len(matched), strings.Join(names, ", ")),
			next:    inv.publicArgv(append(workVerbWords(verb), id, "--work", names[0])...), nextReason: "or another one listed above"}
	}
	return nil, nil
}

func builtWork(work launch.NamedWork) bool {
	return !work.Running() && launch.UnitReviewReadyOutcomes[lastOutcome(work)]
}

// currentSubject is the committed subject of the work's newest attempt.
func currentSubject(work launch.NamedWork) *launch.UnitSubject {
	if work.Record == nil {
		return nil
	}
	attempt := len(work.Record.Rounds)
	for index := range work.Record.Subjects {
		if work.Record.Subjects[index].Round == attempt {
			return &work.Record.Subjects[index]
		}
	}
	return nil
}

// unreviewedWork is built work whose newest result has no collected read:
// the work review G examines when no name is given.
func unreviewedWork(work launch.NamedWork) bool {
	if lastOutcome(work) == "build-size" {
		return true
	}
	if !builtWork(work) {
		return false
	}
	subject := currentSubject(work)
	if subject == nil || subject.Commit == "" {
		return true
	}
	read, err := branch.InspectBranchRead(work.Record.Worktree, work.Record.Goal, subject.Commit)
	return err != nil || read.State != "collected"
}

// unpublishedRead is built work whose newest result has a collected read
// the goal branch's remote does not yet hold.
func unpublishedRead(work launch.NamedWork) bool {
	if !builtWork(work) {
		return false
	}
	subject := currentSubject(work)
	if subject == nil || subject.Commit == "" {
		return false
	}
	read, err := branch.InspectBranchRead(work.Record.Worktree, work.Record.Goal, subject.Commit)
	return err == nil && read.State == "collected" && !read.Published
}

func failedWork(work launch.NamedWork) bool {
	return !work.Running() && work.Record != nil && !launch.UnitReviewReadyOutcomes[lastOutcome(work)]
}

func finishedWork(work launch.NamedWork) bool { return !work.Running() }

// runIntentReviewGoal reviews the goal's built work through the unit review:
// the result is committed on the goal branch as the candidate and examined
// by an independent critic.
func runIntentReviewGoal(inv *intentInvocation, id string) int {
	if inv.input.has("finding") {
		return runIntentReviewDischarge(inv, id)
	}
	for _, only := range []string{"test", "review", "implementation-chain", "artifact", "result", "critic", "by", "lineage", "fixture-human-authority"} {
		if inv.input.has("reason") && (only == "by" || only == "fixture-human-authority") {
			continue
		}
		if inv.input.has("dispositions") && (only == "by" || only == "fixture-human-authority") {
			continue
		}
		if inv.input.has(only) {
			return inv.render(intentResult{Outcome: intentRefused, code: 2,
				Summary: fmt.Sprintf("--%s only goes with --finding, which resolves a review finding; nothing was done", only),
				next:    inv.typedArgvWith("--finding", "FINDING"), nextReason: "names the finding it resolves"})
		}
	}
	for _, other := range []string{"tool-calls", "effort", "goal"} {
		if inv.input.has(other) {
			return inv.render(intentResult{Outcome: intentRefused, code: 2,
				Summary: fmt.Sprintf("--%s is not for reviewing a goal's work; nothing was done", other),
				next:    inv.typedArgvLess(other), nextReason: "a goal's review takes --work, --dispositions and --model"})
		}
	}
	var retry int64
	if inv.input.has("retry") {
		value, err := strconv.ParseInt(inv.input.text("retry"), 10, 64)
		if err != nil || value < 1 {
			return inv.render(intentResult{Outcome: intentRefused, code: 2,
				Summary: fmt.Sprintf("--retry takes the number of the review round that failed, not %s; nothing was done", shellCommand([]string{inv.input.text("retry")})),
				next:    append(inv.typedArgvLess("retry"), "--retry", "1"), nextReason: "the round number that metasystem status " + id + " shows"})
		}
		if inv.input.has("dispositions") {
			return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "--retry reviews again and --dispositions decides findings; give one; nothing was done",
				next: inv.typedArgvLess("dispositions"), nextReason: "reviews again; or drop --retry to decide the findings"})
		}
		retry = value
	}
	if problem := inv.selectRoot(); problem != nil {
		return inv.render(*problem)
	}
	work, problem := inv.goalWork(id)
	if problem != nil {
		return inv.render(*problem)
	}
	// Work committed on the goal branch without a build (written by hand)
	// is reviewed as its committed version, by the same committed review.
	built, manual := inv.rangeWork(id, work)
	if name := inv.input.text("work"); name != "" {
		manual = slices.DeleteFunc(manual, func(item manualWorkItem) bool { return item.Unit != name })
	}
	manualChosen := false
	if !inv.input.has("work") {
		// Every item awaiting review, whichever producer made it: one is
		// inferred, several are named with their exact commands.
		var awaiting []string
		var awaitingManual []manualWorkItem
		for _, one := range built {
			if unreviewedWork(one) || unpublishedRead(one) {
				awaiting = append(awaiting, one.Unit)
			}
		}
		for _, item := range manual {
			if read, err := item.read(); err != nil || read.State != "collected" || !read.Published {
				awaiting, awaitingManual = append(awaiting, item.Unit), append(awaitingManual, item)
			}
		}
		if len(awaiting) > 1 {
			lines := []string{}
			for _, name := range awaiting {
				lines = append(lines, "  "+shellCommand(inv.publicArgv(append(reviewGoalWords(id), "--work", name)...)))
			}
			return inv.render(intentResult{Outcome: intentRefused, code: 2, Targets: inv.targets(id), text: lines, Data: map[string]any{"candidates": awaiting},
				Summary: fmt.Sprintf("goal %s has %d work items awaiting review (%s); nothing was started", id, len(awaiting), strings.Join(awaiting, ", ")),
				next:    inv.publicArgv(append(reviewGoalWords(id), "--work", awaiting[0])...), nextReason: "or another one listed above"})
		}
		if len(awaitingManual) == 1 {
			manual, manualChosen = awaitingManual, true
		} else if len(awaiting) == 1 {
			manual = nil
		}
	}
	if len(manual) == 1 && (inv.input.has("work") || len(built) == 0 || manualChosen) {
		item := manual[0]
		install := inv.goalWorktreeInstallation(item.Worktree)
		args := []string{"--root", install, "--goal", id, "--unit", item.Commit}
		if inv.input.has("brief") {
			args = append(args, "--brief", inv.callerPath(inv.input.text("brief")))
		}
		if retry > 0 {
			args = append(args, "--retry", strconv.FormatInt(retry, 10))
		}
		if inv.input.has("model") {
			args = append(args, "--model", inv.input.text("model"))
		}
		if install != inv.layout.InstallationRoot.Path() {
			args = append(args, "--selected-installation", inv.layout.InstallationRoot.Path())
		}
		targets := []intentTarget{{Kind: "goal", ID: id}, {Kind: "work", ID: item.Unit}, {Kind: "commit", ID: item.Commit}}
		result := inv.commitReview(targets, install, id, item.Commit, args)
		if result.Outcome == intentConfirmed || result.Outcome == intentUnchanged {
			result.next, result.nextReason = inv.goalNextStep(id)
		}
		return inv.render(result)
	}
	selected, problem := inv.selectWork(id, "review", built, unreviewedWork, unpublishedRead, builtWork)
	if problem != nil {
		return inv.render(*problem)
	}
	if selected == nil {
		result := intentResult{Outcome: intentUnchanged, Targets: inv.targets(id), Data: map[string]any{"goal": id},
			Summary: fmt.Sprintf("goal %s has no work waiting for review; nothing was started", id)}
		result.next, result.nextReason = inv.publicArgv("status", id), "the goal's work and what each needs next"
		return inv.render(result)
	}
	if selected.Running() || selected.Run == "" {
		return inv.render(intentResult{Outcome: intentInProgress, Targets: workTargets(id, *selected),
			Summary: fmt.Sprintf("work %s of goal %s is still running; it is reviewed once built", selected.Unit, id),
			next:    inv.publicArgv("work", "wait", id, "--work", selected.Unit), nextReason: "wait for the build to finish"})
	}
	heldBuild := selected.Record != nil && len(selected.Record.Rounds) > 0 && selected.Record.Rounds[len(selected.Record.Rounds)-1].Stop != nil && selected.Record.Rounds[len(selected.Record.Rounds)-1].Stop.Loop == "unit-build"
	if !builtWork(*selected) && lastOutcome(*selected) != "build-size" && lastOutcome(*selected) != "build-gap" && !heldBuild {
		return inv.render(intentResult{Outcome: intentRefused, code: 1, Targets: workTargets(id, *selected),
			Summary: fmt.Sprintf("work %s of goal %s did not pass its checks (%s), so there is nothing to review; nothing was started", selected.Unit, id, lastOutcome(*selected)),
			next:    inv.publicArgv("work", "revise", id, "--work", selected.Unit, "--after", fmt.Sprint(workAttempt(*selected)), "--brief", "FILE"), nextReason: "a correction brief starts one new attempt"})
	}
	inv.reviewWork = &reviewWorkContext{goal: id, work: selected.Unit, repair: inv.command.name == "repair"}
	inv.reviewWork.retry = retry
	result := inv.reviewUnit(selected.Run)
	if inv.reviewWork.retry > 0 && result.Outcome == intentInProgress {
		result.next, result.nextReason = inv.publicArgv(append(reviewGoalWords(id), "--work", selected.Unit)...), "the retried examination continues; the same review collects it"
	}
	result.Targets = append(workTargets(id, *selected), result.Targets...)
	return inv.render(result)
}

// runIntentReviewDischarge discharges one finding's obligation through the
// review-obligation owner, inferring the examination only when the goal's
// work records exactly one.
func runIntentReviewDischarge(inv *intentInvocation, id string) int {
	for _, other := range []string{"work", "dispositions", "model"} {
		if inv.input.has(other) {
			return inv.render(intentResult{Outcome: intentRefused, code: 2,
				Summary: fmt.Sprintf("--finding resolves a review finding and takes no --%s; nothing was done", other),
				next:    inv.typedArgvLess(other), nextReason: "without --" + other})
		}
	}
	if !inv.input.has("review") {
		if problem := inv.selectRoot(); problem != nil {
			return inv.render(*problem)
		}
		chain, problem := inv.uniqueExamination(id, "review")
		if problem != nil {
			return inv.render(*problem)
		}
		inv.input.values["review"] = []string{chain}
	}
	return runIntentResolve(inv)
}

// runIntentRevise corrects one work item through the unit runner's retained
// revision request.
func runIntentRevise(inv *intentInvocation) int {
	if len(inv.input.args) != 1 {
		retry := inv.typedArgvFor("GOAL")
		if !inv.input.has("brief") {
			retry = append(retry, "--brief", "FILE")
		}
		return inv.render(intentResult{Outcome: intentRefused, code: 2,
			Summary: "work revise needs one goal and a correction brief; nothing was done", next: retry, nextReason: "FILE says what to correct"})
	}
	ref, problem := inv.resolveWorkRef(inv.input.args[0], inv.command.accepts)
	if problem != nil {
		return inv.render(*problem)
	}
	switch ref.kind {
	case refRun:
		for _, other := range []string{"work", "after", "dispositions"} {
			if inv.input.has(other) {
				return inv.render(intentResult{Outcome: intentRefused, code: 2, Targets: []intentTarget{{Kind: "run", ID: ref.qualified()}},
					Summary: fmt.Sprintf("revising run %s takes only --brief, not --%s; nothing was done", ref.qualified(), other),
					next:    inv.typedArgvLess(other), nextReason: "the run keeps its own plan, checks and round limit"})
			}
		}
		hook := inv.delivery().foldUnitHook
		if hook == nil {
			hook = runIntentReviseRun
		}
		return hook(inv, ref.id)
	case refJ2:
		for _, other := range []string{"work", "after"} {
			if inv.input.has(other) {
				return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: fmt.Sprintf("revising %s takes no --%s; nothing was done", ref.qualified(), other),
					next: inv.typedArgvLess(other), nextReason: "without --" + other})
			}
		}
		if result := inv.selectRoot(); result != nil {
			return inv.render(*result)
		}
		return inv.render(inv.foldReview(ref.id))
	}
	if !inv.input.has("brief") {
		return inv.render(intentResult{Outcome: intentRefused, code: 2,
			Summary: "work revise needs a correction brief; nothing was done", next: inv.typedArgvWith("--brief", "FILE"), nextReason: "FILE says what to correct"})
	}
	id := ref.id
	after := 0
	if inv.input.has("after") {
		value, err := strconv.Atoi(inv.input.text("after"))
		if err != nil || value <= 0 {
			return inv.render(intentResult{Outcome: intentRefused, code: 2,
				Summary: fmt.Sprintf("--after must be an attempt number such as 2, not %s; nothing was done", shellCommand([]string{inv.input.text("after")})),
				next:    inv.typedArgvLess("after"), nextReason: "follows the newest attempt"})
		}
		after = value
	}
	if problem := inv.selectRoot(); problem != nil {
		return inv.render(*problem)
	}
	// The goal is checked before the brief is read.
	projection, _, problem := inv.projection()
	if problem != nil {
		return inv.render(*problem)
	}
	if file, _ := goalRecord(projection, id); file == nil {
		return unknownGoal(inv, id)
	}
	briefPath := inv.callerPath(inv.input.text("brief"))
	brief, err := os.ReadFile(briefPath)
	if err != nil || len(bytes.TrimSpace(brief)) == 0 {
		summary := "the brief at " + briefPath + " is empty"
		if err != nil {
			summary = fileProblem("brief", briefPath, err)
		}
		return inv.render(intentResult{Outcome: intentRefused, code: 1, Summary: summary + "; nothing was done",
			next: inv.sameCommand(), nextReason: "once --brief names a file that says what to correct"})
	}
	work, problem := inv.goalWork(id)
	if problem != nil {
		return inv.render(*problem)
	}
	selected, problem := inv.selectWork(id, "revise", work, failedWork, finishedWork)
	if problem != nil {
		return inv.render(*problem)
	}
	if selected == nil {
		return inv.render(intentResult{Outcome: intentRefused, code: 1, Targets: inv.targets(id),
			Summary: fmt.Sprintf("goal %s has no finished work to correct; nothing was done", id),
			next:    inv.publicArgv("status", id), nextReason: "the goal's work and what each needs next"})
	}
	targets := workTargets(id, *selected)
	if selected.Run == "" {
		return inv.render(intentResult{Outcome: intentInProgress, Targets: targets,
			Summary: fmt.Sprintf("work %s of goal %s is still being reserved by its build; nothing was done", selected.Unit, id),
			next:    inv.publicArgv("work", "wait", id, "--work", selected.Unit), nextReason: "wait for the build to record its run"})
	}
	person, reason, impact := "", inv.input.text("reason"), ""
	if strings.TrimSpace(reason) != "" || inv.input.has("by") {
		actor, proof, problem := inv.actingAs("revise", id, actorHuman)
		if problem != nil {
			return inv.render(*problem)
		}
		_ = proof
		for index, value := range actor {
			if value == "--by" && index+1 < len(actor) {
				person = actor[index+1]
			}
		}
		if person == "" || strings.TrimSpace(reason) == "" {
			return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "a reasoned revision needs the person's name and reason", next: inv.typedArgvWith("--reason", "TEXT"), nextReason: "give the reason for this revision"})
		}
		impact = "Impact: this admits one correction despite the recorded read.\nIts findings and automatic allowance remain. The result needs another read.\nCancel the admitted run to undo the request."
		if err := inv.recordUnitStopOverride(id, "work-revise", reason, impact, person); err != nil {
			return inv.render(intentResult{Outcome: intentFailed, code: 1, Summary: "the revision impact could not be recorded", Details: []string{err.Error()}, next: inv.sameCommand(), nextReason: "records the impact before starting"})
		}
	}
	var decisions []byte
	if person == "" && inv.input.has("dispositions") {
		decisions, problem = inv.reviseDecisions(id, *selected, after)
		if problem != nil {
			return inv.render(*problem)
		}
	}
	if problem := inv.buildEngineAdmission(); problem != nil {
		return inv.render(*problem)
	}
	runner := inv.unitRunner()
	revised, err := runner.Revise(launch.UnitRevisionRequest{Run: selected.Run, After: after, Brief: brief, Decisions: decisions, Person: person, Reason: reason, Impact: impact})
	if person != "" && revised.Revision.Attempt > 0 {
		findings := revised.Revision.Findings
		if len(findings) > 0 {
			subject := selected.Record.Rounds[len(selected.Record.Rounds)-1].Stop
			if subject != nil {
				actErr := channel.RecordUnitStopAct(inv.layout.InstallationRoot.Path(), channel.UnitStopAct{ID: revised.Record.ID + ":" + fmt.Sprint(revised.Revision.Attempt), Goal: id, Loop: subject.Loop, Subject: subject.Subject, Attempt: subject.Attempt, Findings: findings, Kind: "work-revise", Reason: reason, At: inv.unitStopNow()})
				if actErr != nil {
					return inv.render(intentResult{Outcome: intentFailed, code: 1, Summary: "the revision was admitted, but its questions need reconciliation", Details: []string{actErr.Error()}, next: inv.publicArgv("work", "revise", id, "--work", selected.Unit, "--brief", inv.callerPath(inv.input.text("brief")), "--reason", reason, "--by", person), nextReason: "reconciles the same admission"})
				}
			}
		}
	}
	again := inv.publicArgv("work", "revise", id, "--work", selected.Unit, "--after", fmt.Sprint(max(after, revised.Revision.After)), "--brief", inv.callerPath(inv.input.text("brief")))
	if inv.input.has("dispositions") {
		again = append(again, "--dispositions", inv.flagPath("dispositions"))
	}
	if err != nil {
		plain, details := launchAccount(err)
		switch {
		case launch.IsCode(err, "UNIT_REVISE_UNDECIDED"):
			return inv.render(intentResult{Outcome: intentRefused, code: 1, Targets: targets, Summary: plain, Details: details,
				next: again, nextReason: "add each material finding's fixed, refuted or follow-up row under Decisions on round N in the brief"})
		case errors.Is(err, launch.ErrUnitRevisionStale):
			return inv.render(intentResult{Outcome: intentRefused, code: 1, Targets: targets, Details: details,
				Summary:    "the correction follows an attempt that is no longer the newest; nothing was launched",
				Data:       map[string]any{"current": revised.Current},
				next:       inv.publicArgv("work", "revise", id, "--work", selected.Unit, "--after", fmt.Sprint(revised.Current), "--brief", inv.callerPath(inv.input.text("brief"))),
				nextReason: "correct the newest attempt instead; this deliberately starts one new attempt"})
		case launch.IsCode(err, "UNIT_REVISION_CONFLICT"):
			return inv.render(intentResult{Outcome: intentRefused, code: 1, Targets: targets, Summary: plain + "; nothing was launched", Details: details,
				next: inv.publicArgv("status", id, "--work", selected.Unit), nextReason: "the attempt that request created, and what it needs next"})
		case launch.IsCode(err, "UNIT_RUN_NOT_AWAITING"):
			return inv.render(intentResult{Outcome: intentInProgress, Targets: targets, Summary: plain + "; nothing was launched", Details: details,
				next: inv.publicArgv("work", "wait", id, "--work", selected.Unit), nextReason: "wait for the running attempt to finish"})
		case launch.IsCode(err, "UNIT_ROUND_LIMIT"):
			return inv.render(intentResult{Outcome: intentRefused, code: 1, Targets: targets, Summary: plain + "; nothing was launched", Details: details,
				next: inv.publicArgv("goal", "budget", id, "BOX"), nextReason: "metasystem goal budget gives the goal a larger budget, such as 1d/10/720m/1/5"})
		case launch.IsCode(err, "UNIT_ROUND_CAP"), launch.IsCode(err, "UNIT_ROUND_DIVERGENT"):
			return inv.render(inv.unitOutcome(runner, revised.UnitResult, err, targets, again))
		}
	}
	outcome := inv.unitOutcome(runner, revised.UnitResult, err, targets, again)
	if data, ok := outcome.Data.(map[string]any); ok {
		data["revision"] = map[string]any{"after": revised.Revision.After, "attempt": revised.Revision.Attempt, "rejoined": revised.Rejoined, "current": revised.Current}
	}
	if revised.Rejoined && revised.Current > revised.Revision.Attempt {
		outcome.text = append(outcome.text, fmt.Sprintf("This request already created attempt %d; the work has since reached attempt %d. Nothing was launched.", revised.Revision.Attempt, revised.Current))
	}
	return inv.render(outcome)
}

// uniqueExamination is the one examination root the goal's work records.
func (inv *intentInvocation) uniqueExamination(id, verb string) (string, *intentResult) {
	work, problem := inv.goalWork(id)
	if problem != nil {
		return "", problem
	}
	var examinations []string
	for _, one := range work {
		if one.Record == nil {
			continue
		}
		for _, subject := range one.Record.Subjects {
			if subject.Examination != "" && !slices.Contains(examinations, subject.Examination) {
				examinations = append(examinations, subject.Examination)
			}
		}
	}
	if len(examinations) != 1 {
		return "", &intentResult{Outcome: intentRefused, code: 2, Targets: inv.targets(id), Data: map[string]any{"candidates": examinations},
			Summary: fmt.Sprintf("goal %s has %d reviews (%s), so which one the finding belongs to is unclear; nothing was done", id, len(examinations), strings.Join(examinations, ", ")),
			next:    inv.typedArgvWith("--review", firstOr(examinations, "REVIEW")), nextReason: "or another review it has"}
	}
	return examinations[0], nil
}
