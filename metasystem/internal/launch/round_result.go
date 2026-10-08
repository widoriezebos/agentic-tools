package launch

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// RoundResult names the builder's immutable bytes and the declared proof.
type RoundResult struct {
	Tree          string `json:"tree"`
	Parent        string `json:"parent"`
	PatchDigest   string `json:"patchDigest"`
	ProofIdentity string `json:"proofIdentity"`
}

// ProveRoundResult runs the retained cheap checks on the publication tree.
// Output is retained with the round; checks may not change the candidate.
func (runner *UnitRunner) ProveRoundResult(review UnitReview, worktree string, subject *UnitSubject, retain func(UnitSubject) error) (string, error) {
	planPath := filepath.Join(review.Round.Directory, "plan.json")
	planRoot := review.Round.Directory
	if _, err := os.Stat(planPath); os.IsNotExist(err) {
		planPath = review.Record.Plan
		planRoot = review.Record.PlanDirectory
	}
	plan, err := readUnitPlan(planPath, planRoot)
	if err != nil {
		return "", err
	}
	if len(proofCommands(plan)) == 0 {
		return "", fmt.Errorf("the retained round has no cheap check for its replayed result")
	}
	before, err := runner.snapshotWorktree(worktree, false)
	if err != nil {
		return "", err
	}
	if subject.GateSnapshot == nil {
		subject.GateSnapshot = &before

	} else if before != *subject.GateSnapshot {
		return "", fmt.Errorf("the publication checks changed the result tree")
	}
	subject.GateWorktree = worktree
	if err := retain(*subject); err != nil {
		return "", err
	}
	for index, command := range proofCommands(plan) {
		var launchID string
		if index < len(subject.GateLaunches) {
			launchID = subject.GateLaunches[index]
		} else {
			rel, err := filepath.Rel(review.Record.Worktree, command.Dir)
			if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				return "", fmt.Errorf("the retained check %s runs outside the result tree", command.Name)
			}
			command.Dir = filepath.Join(worktree, rel)
			file, err := os.CreateTemp(review.Round.Directory, "publication-check-*.json")
			if err != nil {
				return "", err
			}
			body, err := json.Marshal(PlainBrief{command.Argv, command.Dir, command.Env})
			if err == nil {
				_, err = file.Write(body)
			}
			closeErr := file.Close()
			if err != nil || closeErr != nil {
				return "", fmt.Errorf("the publication check could not be recorded: %v %v", err, closeErr)
			}
			id, err := newID(runner.Manager.Now())
			if err != nil {
				return "", err
			}
			if err := review.AdmitChild(id); err != nil {
				return "", err
			}
			spec := StartSpec{ID: id, Kind: "proof", Goal: plan.Goal, Tag: plan.Unit,
				WorkingDirectory: worktree, Brief: file.Name(), Round: review.Round.Number, MaxRounds: review.Record.MaxRounds}
			spec.wait = review.Wait
			if runner.BeforeModelLaunch != nil {
				if err := runner.BeforeModelLaunch(review.Record, spec); err != nil {
					return "", err
				}
			}
			subject.GateLaunches = append(subject.GateLaunches, id)
			if err := retain(*subject); err != nil {
				return "", err
			}
			launch, err := runner.Manager.Start(spec)
			if IsCode(err, "UNIT_WAIT_RETRY") {
				current, statusErr := runner.Manager.Status(id)
				return "", &PublicationPending{Running: statusErr != nil || !current.State.Terminal(), Cause: err}
			}
			if err != nil {
				return "", fmt.Errorf("the publication check %s did not start or pass (launch %s): %w", command.Name, id, err)
			}
			launchID = launch.ID
		}
		var completed Record
		var terminal bool
		err = review.Wait(func() error {
			var err error
			completed, terminal, err = runner.Manager.Wait(launchID, time.Duration(runner.Manager.Settings.WaitCapSeconds)*time.Second)
			return err
		})
		if IsCode(err, "UNIT_WAIT_RETRY") || err == nil && !terminal {
			current, statusErr := runner.Manager.Status(launchID)
			return "", &PublicationPending{Running: statusErr != nil || !current.State.Terminal(), Cause: err}
		}
		if err != nil || !terminal || completed.State != Completed || completed.ExitCode == nil || *completed.ExitCode != 0 {
			return "", fmt.Errorf("the publication check %s did not pass (launch %s): %v", command.Name, launchID, err)
		}
	}
	after, err := runner.snapshotWorktree(worktree, false)
	if err != nil || before != after {
		return "", fmt.Errorf("the publication checks changed the result tree: %v", err)
	}
	return strings.Join(subject.GateLaunches, ","), nil
}

// PublicationPending retains the check to resume after a command wait ends.
type PublicationPending struct {
	Running bool
	Cause   error
}

func (e *PublicationPending) Error() string {
	return "the publication check is still being collected; repeat the same command"
}
func (e *PublicationPending) Unwrap() error { return e.Cause }
