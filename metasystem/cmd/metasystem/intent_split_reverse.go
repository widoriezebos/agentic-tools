package main

import (
	"fmt"
	"strings"

	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goalrevision"
)

func runIntentSplitReverse(inv *intentInvocation, id string) int {
	reason, problem := inv.textValue("reason")
	if problem != nil {
		return inv.render(*problem)
	}
	if strings.TrimSpace(reason) == "" {
		return inv.refuse(id, "reversing a split needs --reason TEXT; nothing was done", "give the reason for restoring the parent")
	}
	fmt.Fprintln(inv.stderr, "Reversing restores the parent and parks its children without execution approval; their lineage and history remain.")
	actor, proof, problem := inv.actingAs("split-reverse", id, actorHuman)
	if problem != nil {
		return inv.render(*problem)
	}
	args := append([]string{"--root", inv.stateRoot, "--id", id}, actor...)
	return inv.render(inv.goalAct(id, "split-reverse", inv.syncOwner("split-reverse", args, proof, false, func(req goal.VerbRequest, f *syncFlags) (goal.PublishResult, error) {
		projection, err := goal.Project(req.Endpoint, false, req.Now)
		if err != nil {
			return goal.PublishResult{}, err
		}
		parent := projection.Tree.Live[id]
		if parent == nil || parent.Split == nil {
			return goal.PublishResult{}, fmt.Errorf("goal %s has no split to reverse", id)
		}
		transaction := parent.Split.Transaction
		children := map[string]uint64{}
		for _, childID := range parent.Split.Children {
			child := projection.Tree.Live[childID]
			if child == nil {
				return goal.PublishResult{}, fmt.Errorf("child %s is no longer live; reconcile its work before reversing %s", childID, id)
			}
			held, err := goalrevision.Acquire(f.root, childID, child.Revision, "goal-split-reverse")
			if err != nil {
				return goal.PublishResult{}, err
			}
			defer held.Release()
			children[childID] = child.Revision
		}
		req.SplitCheck = func(child *goal.GoalFile) error {
			if child.Revision != children[child.Id] {
				return fmt.Errorf("child %s changed during reversal; inspect metasystem goal show %s and repeat", child.Id, child.Id)
			}
			started, err := dispatchcore.SplitChildWorkStarted(f.root, child.Id)
			if err != nil {
				return fmt.Errorf("child %s's work evidence is unreadable: %w; inspect metasystem goal show %s", child.Id, err, child.Id)
			}
			if started {
				return fmt.Errorf("child %s has started work; reconcile that work before reversing %s", child.Id, id)
			}
			return nil
		}
		return goal.ReverseSplit(req, id, transaction, reason)
	}, "id")))
}
