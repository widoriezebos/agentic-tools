package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
)

// recordUnitStopOverride publishes the person's impact statement before the
// command admits its effect. Failed commands retain that admission evidence
// but cannot close any finding's question.
func (inv *intentInvocation) recordUnitStopOverride(id, kind, reason, impact, who string, worktrees ...string) error {
	at := inv.unitStopNow()
	operation, err := goal.NewOperationULID()
	if err != nil {
		return err
	}
	record := struct {
		Goal      string    `json:"goal"`
		Kind      string    `json:"kind"`
		Reason    string    `json:"reason"`
		Impact    string    `json:"impact"`
		Who       string    `json:"who"`
		At        time.Time `json:"at"`
		Worktrees []string  `json:"worktrees,omitempty"`
	}{id, kind, reason, impact, who, at.UTC(), worktrees}
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(inv.layout.InstallationRoot.Path(), "artifacts", "agents", "channel", "unit-stop-overrides", operation+".json")
	if _, err := atomicfile.WriteFile(path, append(data, '\n'), 0o600, ""); err != nil {
		return err
	}
	_, err = fmt.Fprintln(inv.stderr, impact+" Reason: "+reason)
	return err
}

// A conclusion retains its repository's worktree paths before branch cleanup
// can remove them. Repeats include those paths to finish the shared unit store.
func (inv *intentInvocation) goalReviewWorktrees(id string) ([]string, bool, error) {
	recorded := false
	paths := []string{inv.layout.GitRoot}
	registered, err := inv.registeredWorktrees()
	for path := range registered {
		if !slices.Contains(paths, path) {
			paths = append(paths, path)
		}
	}
	records, scanErr := filepath.Glob(filepath.Join(inv.layout.InstallationRoot.Path(), "artifacts", "agents", "channel", "unit-stop-overrides", "*.json"))
	if scanErr != nil {
		return paths, recorded, scanErr
	}
	for _, path := range records {
		data, readErr := os.ReadFile(path)
		var record struct {
			Goal, Kind string
			Worktrees  []string
		}
		if readErr == nil {
			readErr = json.Unmarshal(data, &record)
		}
		if readErr != nil {
			err = errors.Join(err, readErr)
			continue
		}
		if record.Goal == id && record.Kind == "goal-done" {
			recorded = true
			for _, tree := range record.Worktrees {
				if !slices.Contains(paths, tree) {
					paths = append(paths, tree)
				}
			}
		}
	}
	return paths, recorded, err
}

// Cleanup stays discoverable while its records are open or cannot be read.
func (inv *intentInvocation) goalReviewCleanupPending(id string) bool {
	worktrees, _, treeErr := inv.goalReviewWorktrees(id)
	root := inv.layout.InstallationRoot.Path()
	chains, chainErr := dispatchcore.GoalReviewCleanupPending(root, id)
	units, unitErr := inv.work().units(inv.layout).GoalReviewCleanupPending(id, worktrees)
	questions, unreadable := channel.WalkOpenQuestions(root)
	if treeErr != nil || chainErr != nil || unitErr != nil || len(unreadable) > 0 || chains || units {
		return true
	}
	for _, question := range questions {
		if question.Goal == id && question.UnitStop != nil {
			return true
		}
	}
	return false
}

func (inv *intentInvocation) unitStopNow() time.Time {
	if inv.owners.commandNow != nil {
		if at, err := inv.owners.commandNow(inv.layout.InstallationRoot.Path()); err == nil {
			return at
		}
	}
	return time.Now()
}

// recordUnitStopActForReview joins a successful risk act to its finding's
// exact review subject. Other findings and other reviews remain open.
func recordUnitStopActForReview(root, id, review, finding, reason string, at time.Time) error {
	questions, unreadable := channel.WalkOpenQuestions(root)
	if len(unreadable) > 0 {
		return fmt.Errorf("risk was recorded, but its questions could not be read: %v", unreadable)
	}
	for _, q := range questions {
		stop := q.UnitStop
		if q.Goal != id || stop == nil || stop.Review != review || stop.Finding != finding {
			continue
		}
		act := channel.UnitStopAct{ID: "accept-risk:" + q.ID, Goal: id, Loop: stop.Loop, Subject: stop.Subject, Attempt: stop.Attempt, Findings: []string{finding}, Kind: "goal-accept-risk", Reason: reason, At: at}
		if err := channel.RecordUnitStopAct(root, act); err != nil {
			return err
		}
	}
	return nil
}

func unitStopActor(actor []string) string {
	for i, v := range actor {
		if v == "--by" && i+1 < len(actor) {
			return actor[i+1]
		}
	}
	return ""
}

// Build holds ask for the retained round's remedy. Successful continuation
// closes only that hold, including when collection resumes after a restart.
func (inv *intentInvocation) syncBuildHolds(record launch.UnitRunRecord) error {
	root := inv.layout.InstallationRoot.Path()
	for index, round := range record.Rounds {
		for _, step := range round.Steps {
			if step.RetryLaunch != "" {
				if err := channel.RecordUnitStopAct(root, channel.UnitStopAct{ID: step.RetryLaunch + ":retry", Goal: record.Goal, Loop: "unit-build", Subject: record.Goal + "/" + record.Unit + "/" + record.ID, Attempt: round.Number, Findings: []string{step.RetryLaunch}, Kind: "work-review-retry", Reason: step.RetryReason, At: inv.unitStopNow()}); err != nil {
					return err
				}
			}
		}
		kind := round.Outcome
		if round.SizeAcceptedBy != "" {
			kind = "build-size"
		}
		failure := round.Stop != nil && round.Stop.Loop == "unit-build" && kind != "build-gap" && kind != "build-size"
		if kind != "build-gap" && kind != "build-size" && !failure {
			continue
		}
		subject := record.Goal + "/" + record.Unit + "/" + record.ID
		act := "work-review-size"
		next := inv.workArgv(record, "review", "--reason", "TEXT", "--by", "NAME")
		if kind == "build-gap" {
			act = "work-revise"
			next, _ = inv.workContinuation(record.Goal, launch.NamedWork{Unit: record.Unit, Record: &record}, true)
		}
		if failure {
			act = "work-revise"
			next, _ = inv.workContinuation(record.Goal, launch.NamedWork{Unit: record.Unit, Record: &record}, true)
			if round.Cause == "environment" || round.Cause == "deadline" {
				act = "work-review-retry"
			}
			if act == "work-review-retry" {
				for _, step := range round.Steps {
					if step.State == launch.StepFailed {
						kind = step.LaunchID
						break
					}
				}
			}
		}
		supersededSize := kind == "build-size" && round.SizeAcceptedBy == "" && index+1 < len(record.Rounds)
		if supersededSize {
			act = "work-revise"
		}
		if index+1 < len(record.Rounds) || round.SizeAcceptedBy != "" {
			if err := channel.RecordUnitStopAct(root, channel.UnitStopAct{ID: record.ID + ":" + kind + ":" + fmt.Sprint(round.Number), Goal: record.Goal, Loop: "unit-build", Subject: subject, Attempt: round.Number, Findings: []string{kind}, Kind: act, UnitClosed: supersededSize, Reason: "the retained build hold was continued", At: inv.unitStopNow()}); err != nil {
				return err
			}
			continue
		}
		_, err := channel.Ask(channel.AskRequest{RepoRoot: root, Goal: record.Goal, Kind: "other", Facts: []string{round.Stop.Class, fmt.Sprintf("%d changed lines against %d declared", round.BuildLines, round.DeclaredLines), "Retained builder message: " + round.GapMessage}, Recommendation: "Run the requested act to continue this retained round.", UnitStop: &channel.UnitStopQuestion{Loop: "unit-build", Subject: subject, Attempt: round.Number, Finding: kind, Needs: shellCommand(next), AcceptableActs: []string{act}}, Now: inv.unitStopNow()})
		if err != nil {
			return err
		}
	}
	return nil
}
