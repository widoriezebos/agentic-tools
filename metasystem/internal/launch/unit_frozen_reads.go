package launch

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
)

func roundReadOwner(run string, n int) diskstore.Owner {
	return diskstore.Owner{Kind: diskstore.OwnerAttempt, Ref: fmt.Sprintf("unit:%s-r%d", run, n)}
}

// The build tree can contain new objects that were hashed in a private
// index. Materialize those objects in the reader's clone without adding
// objects, a ref or a linked worktree to the builder's repository.
func (runner *UnitRunner) frozenReadCheckout(record *UnitRunRecord, round *UnitRound, plan UnitPlan) (string, error) {
	if round.ReadCheckout != "" && !round.CheckoutRemoved {
		info, err := os.Lstat(filepath.Join(round.ReadStore, "checkout", ".git"))
		if err != nil {
			return "", err
		}
		if !info.IsDir() {
			return "", errors.New("the retained reader clone lost its Git directory")
		}
		return round.ReadCheckout, nil
	}
	source, err := runner.git().Run(plan.Worktree, nil, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	root := strings.TrimSpace(string(source))
	prefix, err := filepath.Rel(root, plan.Worktree)
	if err != nil || prefix == ".." || strings.HasPrefix(prefix, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("cannot place the reader at the unit's repository path")
	}
	owner := roundReadOwner(record.ID, round.Number)
	store, err := diskstore.CreateTempStore(fmt.Sprintf("metasystem-unit-read.%s-r%d", record.ID, round.Number), readContextClass, owner)
	if err != nil {
		return "", err
	}
	ready := false
	defer func() {
		if !ready {
			_ = diskstore.ReleaseTempStore(context.Background(), store, readContextClass, owner)
		}
	}()
	// The parent of the clone must never provide a fallback Git repository.
	for dir := filepath.Dir(store); dir != filepath.Dir(dir); dir = filepath.Dir(dir) {
		if _, err := os.Lstat(filepath.Join(dir, ".git")); err == nil {
			return "", errors.New("the temporary read store lies inside another Git checkout")
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
	}
	clone := filepath.Join(store, "checkout")
	if _, err := runner.git().Run(store, nil, "clone", "--shared", "--no-checkout", "--", root, clone); err != nil {
		return "", err
	}
	snapshot, err := runner.snapshotWorktree(root, true, filepath.Join(clone, ".git", "objects"))
	if err != nil {
		return "", err
	}
	if snapshot.TreeID != round.Result.Tree || strings.TrimSpace(snapshot.Head) != round.Result.Parent {
		return "", coded("UNIT_TREE_CHANGED", "", errors.New("the frozen build tree changed before the read"))
	}
	if _, err := runner.git().Run(clone, nil, "checkout", "--detach", round.Result.Parent); err != nil {
		return "", err
	}
	if _, err := runner.git().Run(clone, nil, "read-tree", "--reset", "-u", round.Result.Tree); err != nil {
		return "", err
	}
	// Resuming engines parse the retained run plan strictly. Install its
	// compatibility guard before any launch can enter the paired phase.
	retained, err := readUnitPlan(record.Plan, record.PlanDirectory)
	if err != nil {
		return "", err
	}
	if !retained.PairedReads {
		retained.PairedReads = true
		if err := writeUnitJSON(record.Plan, retained, runner.root()); err != nil {
			return "", err
		}
	}
	round.ReadStore, round.ReadCheckout, round.CheckoutRemoved = store, filepath.Join(clone, prefix), false
	if err := runner.save(*record); err != nil {
		return "", err
	}
	ready = true
	return round.ReadCheckout, nil
}

type readCheckoutHeld struct{ step *UnitStep }

func (e *readCheckoutHeld) Error() string {
	return fmt.Sprintf("read %s still holds its checkout", e.step.LaunchID)
}

func (runner *UnitRunner) releaseFrozenReads(record *UnitRunRecord, round *UnitRound) error {
	if round.ReadStore == "" || round.CheckoutRemoved {
		return nil
	}
	for _, step := range round.Steps {
		if !strings.HasPrefix(step.Name, "read") {
			continue
		}
		for _, id := range step.LaunchIDs {
			r, err := runner.Manager.Store.Read(id)
			if err != nil {
				return err
			}
			if !r.State.Terminal() || !runner.Manager.provenDead(r) {
				return &readCheckoutHeld{step: &step}
			}
		}
	}
	if _, err := os.Lstat(round.ReadStore); !errors.Is(err, os.ErrNotExist) {
		if err := diskstore.ReleaseTempStore(context.Background(), round.ReadStore, readContextClass, roundReadOwner(record.ID, round.Number)); err != nil {
			return err
		}
	}
	round.CheckoutRemoved = true
	return nil
}

func (runner *UnitRunner) discardRoundReads(record *UnitRunRecord, round *UnitRound, reason string) error {
	for i := range round.Steps {
		step := &round.Steps[i]
		if !strings.HasPrefix(step.Name, "read") {
			continue
		}
		step.Unattached = true
		if step.LaunchID != "" {
			r, err := runner.Manager.Store.Read(step.LaunchID)
			if err != nil && !errors.Is(err, os.ErrNotExist) {
				return err
			}
			if err == nil {
				if !slices.Contains(step.LaunchIDs, r.ID) {
					step.LaunchIDs = append(step.LaunchIDs, r.ID)
				}
				if !r.State.Terminal() {
					r, err = runner.Manager.Cancel(step.LaunchID)
					if err != nil {
						return err
					}
				}
				if len(step.Moved) > 0 && step.State == StepFailed {
					continue
				}
				runner.driver(record, round).endStep(i, r)
				if r.State == Completed {
					continue
				}
			}
		}
		step.State, step.Reason = StepSkipped, reason
	}
	return runner.collectLaunches(*record, *round)
}

// While the check runs this command still owns all round transitions. Each
// wait releases its locks and rechecks custody. No second writer is needed.
func (runner *UnitRunner) readBesideProof(sequence readSequence, first, proof int, deadline time.Time) error {
	round := sequence.driver.round
	for {
		more, err := sequence.startReadyReads(first)
		if IsCode(err, "UNIT_WAIT_RETRY") {
			return err
		}
		// A speculative read refusal is retried by the normal read route
		// after the proof settles and frees its reserved minutes.
		if err != nil || !more || !runner.Manager.Now().Before(deadline) {
			return nil
		}
		_, ended, err := runner.waitLaunch(round.Steps[proof].LaunchID, min(runner.Manager.Poll, deadline.Sub(runner.Manager.Now())))
		if err != nil || ended {
			return err
		}
	}
}

// Advance ready siblings without waiting for any reader. Shared outputs
// serialize writers and reruns require the completed first pass.
func (sequence readSequence) startReadyReads(first int) (bool, error) {
	round := sequence.driver.round
	active := false
	for i := first; i < len(round.Steps); i++ {
		step := &round.Steps[i]
		switch step.State {
		case StepPending, StepStarting:
			spec, err := sequence.stepSpec(*step)
			if err != nil {
				return false, err
			}
			if _, err := sequence.driver.startStep(i, spec); err != nil {
				if IsCode(err, "BUDGET_REFUSED") && step.State == StepStarting && len(step.LaunchIDs) == 0 {
					step.State, step.LaunchID = StepPending, ""
					if saveErr := sequence.driver.save(); saveErr != nil {
						return false, saveErr
					}
				}
				return false, err
			}
		case StepRunning:
			r, err := sequence.driver.manager.Status(step.LaunchID)
			if err != nil {
				return false, err
			}
			if r.State.Terminal() {
				sequence.driver.endStep(i, r)
				if err := sequence.driver.save(); err != nil {
					return false, err
				}
			}
		}
		if step.State == StepRunning {
			active = true
			if sequence.serial {
				return true, nil
			}
		}
	}
	if active {
		return true, nil
	}
	for _, step := range round.Steps[first:] {
		if step.State != StepPassed || step.Rerun {
			return false, nil
		}
	}
	n := len(round.Steps)
	if err := sequence.addReruns(first); err != nil {
		return false, err
	}
	if len(round.Steps) > n {
		return sequence.startReadyReads(n)
	}
	return false, nil
}

func (runner *UnitRunner) prepareRoundReadBrief(record *UnitRunRecord, round *UnitRound, plan UnitPlan, sequence *readSequence) error {
	packet, err := runner.warmRead(*record, *round)
	if err != nil {
		return err
	}
	brief, err := os.ReadFile(plan.Read.Brief)
	if err != nil {
		return err
	}
	// The prepared-copy field in an existing unit plan names its original
	// worktree. Give the reader the path it actually owns for this round.
	text := strings.Replace(string(brief), "\nPath: "+plan.Worktree+"\n", "\nPath: "+sequence.spec.WorkingDirectory+"\n", 1)
	// A plan may declare paths in the builder's own tree. Keep such
	// outputs outside both trees and let normal launch collection retain them.
	if round.ReadStore != "" {
		sequence.spec.Outputs = slices.Clone(sequence.spec.Outputs)
		var paths []string
		for i, path := range sequence.spec.Outputs {
			rel, err := filepath.Rel(plan.Worktree, path)
			if err == nil && filepath.IsLocal(rel) {
				dest := filepath.Join(round.ReadStore, "outputs", rel)
				paths = append(paths, path, dest)
				sequence.spec.Outputs[i] = dest
			}
		}
		if len(paths) > 0 {
			text = strings.NewReplacer(paths...).Replace(text)
		}
	}
	if text != string(brief) || packet != "" || len(round.ExcludedDropped) > 0 {
		if packet != "" {
			text = strings.Replace(text, "Not this read. A follow-up round of this unit is read again by the unit runner with this brief.", "Check the follow-up packet below before reading new changes.", 1) + packet
		}
		if len(round.ExcludedDropped) > 0 {
			text += "\n\nThe worktree held excluded differences from the branch in engine-written files; the engine kept them in " + filepath.Join(round.Directory, "excluded") + "/ and put them back before the check, so they are not part of this change: " + strings.Join(round.ExcludedDropped, ", ") + ".\n"
		}
		path := filepath.Join(round.Directory, "read-brief.md")
		if _, err := atomicfile.WriteText(path, text, runner.root()); err != nil {
			return err
		}
		sequence.spec.Brief = path
	}
	return nil
}
