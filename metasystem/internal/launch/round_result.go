package launch

import (
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
	plan, err := readUnitPlan(filepath.Join(review.Round.Directory, "plan.json"), review.Round.Directory)
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
			id, err := newID(runner.Manager.Now())
			if err != nil {
				return "", err
			}
			brief := filepath.Join(review.Round.Directory, "publication-check-"+id+".json")
			if err := writeUnitJSON(brief, PlainBrief{command.Argv, command.Dir, command.Env}, runner.root()); err != nil {
				return "", err
			}
			if err := review.AdmitChild(id); err != nil {
				return "", err
			}
			spec := StartSpec{ID: id, Kind: "proof", Goal: plan.Goal, Tag: plan.Unit,
				WorkingDirectory: worktree, Brief: brief, Round: review.Round.Number, MaxRounds: review.Record.MaxRounds}
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
			startedAt := runner.Manager.Now().UTC().Format(time.RFC3339Nano)
			launch, err := runner.Manager.Start(spec)
			if launch.ID == "" && err != nil && !IsCode(err, "UNIT_WAIT_RETRY") {
				if current, readErr := runner.Manager.Store.Read(id); readErr == nil {
					launch = current
				} else if os.IsNotExist(readErr) {
					launch = Record{ID: id, State: Failed, StartedAt: startedAt, FinishedAt: runner.Manager.Now().UTC().Format(time.RFC3339Nano)}
				}
			}
			if launch.State.Terminal() && runner.CollectLaunch != nil {
				if collectErr := runner.CollectLaunch(review.Record, launch, failedStepCause(launch)); collectErr != nil {
					return "", collectErr
				}
			}
			if IsCode(err, "UNIT_WAIT_RETRY") {
				current, statusErr := runner.Manager.Status(id)
				return "", &PublicationPending{Running: statusErr != nil || !current.State.Terminal(), Cause: err}
			}
			if err != nil && (launch.State != Failed || launch.ExitCode == nil || *launch.ExitCode == 0) {
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
		if err == nil && terminal && runner.CollectLaunch != nil {
			if err := runner.CollectLaunch(review.Record, completed, failedStepCause(completed)); err != nil {
				return "", err
			}
		}
		if err == nil && terminal && completed.State == Failed && completed.ExitCode != nil && *completed.ExitCode != 0 {
			return "", coded("PUBLICATION_CHECK_RED", "launch="+launchID, fmt.Errorf("the publication check %s did not pass (launch %s)", command.Name, launchID))
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
