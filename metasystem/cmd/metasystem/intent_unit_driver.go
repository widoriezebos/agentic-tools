package main

import (
	"errors"
	"fmt"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/helm"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"slices"
	"strconv"
	"strings"
)

func (inv *intentInvocation) driveUnitReviews() error {
	defer inv.leaveStores()
	if problem := inv.selectRoot(); problem != nil {
		return errors.New(problem.Summary)
	}
	projection, _, problem := inv.projection()
	if problem != nil {
		return errors.New(problem.Summary)
	}
	connection := inv.connection()
	endpoint, err := connection.endpoint(inv.layout.InstallationRoot.Path())
	if err != nil {
		return err
	}
	for _, id := range goal.OrderedOpenGoalIDs(projection.Tree.Live) {
		if err := branch.CheckCommitAccess(id, connection.claimCheck(inv.layout.InstallationRoot.Path(), id, endpoint)); err != nil {
			continue
		}
		work, problem := inv.goalWork(id)
		if problem != nil {
			return errors.New(problem.Summary)
		}
		for _, one := range work {
			if one.Record == nil || one.Running() || len(one.Record.Rounds) == 0 {
				continue
			}
			record := *one.Record
			if act := record.ReviewAct; act != nil && act.Key == launch.ReviewActKey(record) {
				if act.State == "judgement" {
					continue
				}
				observed := launch.UnitReviewAct{Round: len(record.Rounds)}
				if err := inv.observeUnitPublication(record, &observed); err != nil {
					return err
				}
				if observed.State == "satisfied" {
					continue
				}
			}
			review := *inv
			defer review.leaveStores()
			review.command, _ = findIntentCommand("work review")
			review.input = intentInput{args: []string{id}, values: map[string][]string{"work": {record.Unit}}}
			review.raw = []string{id, "--work", record.Unit}
			review.reviewWork = &reviewWorkContext{goal: id, work: record.Unit, run: record.ID}
			if act := record.ReviewAct; act != nil && act.Key == launch.ReviewActKey(record) {
				if i := slices.Index(act.Command, "--retry"); i >= 0 && i+1 < len(act.Command) {
					review.reviewWork.retry, _ = strconv.ParseInt(act.Command[i+1], 10, 64)
				}
			}
			if err := review.driveUnitReview(record); err != nil {
				return fmt.Errorf("run %s: %w", record.ID, err)
			}
		}
	}
	return nil
}
func (inv *intentInvocation) automaticReviewAdmission() error {
	policy, _, code, err := inv.work().config("seat.driver", intentConfPath(inv.layout))
	if err != nil || code != 0 {
		return fmt.Errorf("the seat driver policy cannot be read: %v", err)
	}
	if policy != "auto" {
		return fmt.Errorf("the seat waits for a person to review this unit")
	}
	if state := helm.Active(inv.stateRoot); state.Active || state.Diagnostic != "" {
		return fmt.Errorf("automatic review waits for the seat's helm to be available")
	}
	return nil
}
func (inv *intentInvocation) driveUnitReview(record launch.UnitRunRecord) error {
	runner := inv.unitRunner()
	act := launch.UnitReviewAct{Key: launch.ReviewActKey(record), Round: len(record.Rounds), State: "prepared",
		Command: inv.workArgv(record, "review"), Summary: "the completed unit is ready for review"}
	if err := runner.RetainReviewAct(record.ID, act); err != nil {
		return err
	}
	if err := inv.observeUnitPublication(record, &act); err != nil {
		return err
	}
	if act.State == "satisfied" {
		return runner.RetainReviewAct(record.ID, act)
	}
	if err := inv.automaticReviewAdmission(); err != nil {
		act.Summary = err.Error()
		return runner.RetainReviewAct(record.ID, act)
	}
	connection := inv.connection()
	claim := connection.claimCheck
	connection.claimCheck = func(root, id string, endpoint goal.Endpoint) func() error {
		check := claim(root, id, endpoint)
		return func() error {
			if err := inv.automaticReviewAdmission(); err != nil {
				return err
			}
			return check()
		}
	}
	inv.owners.connection = connection
	delivery := *inv.delivery()
	read, publish := delivery.branchRead, delivery.publishRead
	delivery.branchRead = func(args []string) (branch.BranchReadResult, int, error) {
		if err := inv.automaticReviewAdmission(); err != nil {
			return branch.BranchReadResult{}, 1, err
		}
		return read(args)
	}
	delivery.publishRead = func(root, id, commit string) (branch.PublishReadResult, error) {
		if err := inv.automaticReviewAdmission(); err != nil {
			return branch.PublishReadResult{}, err
		}
		return publish(root, id, commit)
	}
	close := delivery.closeOwner
	delivery.closeOwner = func(root string, args []string) intentProcessResult {
		if err := inv.automaticReviewAdmission(); err != nil {
			return intentProcessResult{code: 1, stderr: []byte(err.Error())}
		}
		return close(root, args)
	}
	inv.owners.delivery = &delivery
	result := inv.reviewUnit(record.ID)
	current, err := runner.Status(record.ID)
	if err != nil {
		return err
	}
	if len(current.Rounds) != len(record.Rounds) {
		return errors.New("the unit changed during review; observe this run again")
	}
	act.Key, act.Round = launch.ReviewActKey(current), len(current.Rounds)
	act.State, act.Summary, act.Details = "pending", result.Summary, result.Details
	act.Command = inv.workArgv(current, "review")
	if len(result.next) > 0 {
		act.Command = result.next
	}
	if data, ok := result.Data.(map[string]any); ok {
		act.Template, _ = data["template"].(string)
	}
	if err := inv.observeUnitPublication(current, &act); err != nil {
		return err
	}
	if act.State != "satisfied" && act.Template != "" {
		round := current.Rounds[len(current.Rounds)-1]
		if round.Stop != nil && round.Stop.Decision == "continue" {
			act.State = "judgement"
			act.Command = inv.workArgv(current, "revise", "--after", strconv.Itoa(round.Number), "--brief", "FILE", "--dispositions", act.Template)
			act.Summary = "the worker must decide the findings and prepare the correction brief"
		} else if !strings.Contains(shellCommand(act.Command), "--retry") {
			act.State = "judgement"
		}
	}
	return runner.RetainReviewAct(current.ID, act)
}
func (inv *intentInvocation) observeUnitPublication(current launch.UnitRunRecord, act *launch.UnitReviewAct) error {
	// Publication is evidence from the branch owner, never the command headline.
	for _, subject := range current.Subjects {
		if subject.Round != act.Round || subject.Published == "" {
			continue
		}
		read, err := inv.work().inspectRead(inv.goalWorktreeInstallation(current.Worktree), current.Goal, subject.Commit)
		if err != nil {
			return err
		}
		if read.Published && read.AttestationCommit != "" {
			act.State, act.Effect, act.Command, act.Summary, act.Details = "satisfied", read.AttestationCommit, nil, "the unit’s reviewed result is published", nil
		}
	}
	return nil
}
