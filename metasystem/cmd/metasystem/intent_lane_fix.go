package main

import (
	"fmt"
	"slices"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batchowner"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
)

func landingFixFor(root, checkout, goal string, caller int64) *plain.Fix {
	home, err := batchowner.LandingLaneHome()
	if err != nil {
		return nil
	}
	registered, present, err := lane.Read(home)
	if err != nil || !present || registered.Install != root || checkout == "" {
		return nil
	}
	top, _ := goalBranchGit(checkout, "rev-parse", "--show-toplevel")
	return landingFixForRegistered(root, top, goal, caller, registered)
}

func landingFixForRegistered(root, checkout, goal string, caller int64, registered lane.Record) *plain.Fix {
	checkpoint := landingFixCheckpoint(root, checkout, registered)
	if checkpoint == nil || !landingFixActor(root, caller) || !slices.Contains(checkpoint.Members, goal) {
		return nil
	}
	running, _, _, err := plain.ReadRunning(root, plain.ProveSeams{})
	result, present, readErr := plain.LastResult(root)
	if running.Gate {
		result, present, readErr = plain.LastGate(root)
	}
	if err != nil || readErr != nil || !present || running.Attempt == "" || result.Attempt != running.Attempt || result.Commit != running.Commit || result.Result != plain.Red || result.Cause == nil || result.Cause.Kind != "own" || result.Cause.Goal != goal {
		return nil
	}
	fix, err := plain.FixForAttempt(root, running.Attempt)
	if fix == nil && err == nil {
		fix = &plain.Fix{Goal: goal, Attempt: running.Attempt, Parent: running.Commit, Round: 1}
		for _, unit := range result.Failed {
			fix.Units = append(fix.Units, unit.Unit)
		}
	}
	if err != nil || fix == nil || fix.Attempt != running.Attempt || fix.Goal != goal || fix.Round != 1 || fix.State == "closed" || fix.Parent != running.Commit && fix.Commit != running.Commit {
		return nil
	}
	head, err := goalBranchGit(checkout, "rev-parse", "HEAD")
	ref, _ := goalBranchGit(checkout, "symbolic-ref", "-q", "HEAD")
	if err != nil || ref != "" || head != fix.Parent && head != fix.Commit {
		return nil
	}
	return fix
}

// reviewLaneFix reads the committed repair without moving a goal branch or main.
func (inv *intentInvocation) reviewLaneFix(targets []intentTarget, root string, fix plain.Fix, context ...string) intentResult {
	args := append([]string{"--root", root, "--goal", fix.Goal, "--unit", fix.Commit}, context...)
	if inv.input.has("brief") && len(context) == 0 {
		args = append(args, "--brief", inv.callerPath(inv.input.text("brief")))
	}
	if inv.input.has("retry") {
		args = append(args, "--retry", inv.input.text("retry"))
	}
	if inv.input.has("model") {
		args = append(args, "--model", inv.input.text("model"))
	}
	read, code, err := inv.delivery().branchRead(args)
	result := intentResult{Targets: targets, Outcome: intentInProgress, Summary: "the lane fix's committed review is in progress", next: inv.sameCommand()}
	if err != nil {
		result.Outcome, result.code, result.Summary = intentRefused, max(code, 1), err.Error()
	} else if read.RootJob != "" {
		fix.Read = read.RootJob
		result = inv.collectReview(targets, delegateOutcome{JobID: read.RootJob})
		if data, ok := result.Data.(map[string]any); ok {
			fix.Verdict, _ = data["verdict"].(string)
		}
	}
	if err := plain.WriteFix(root, fix); err != nil {
		return intentResult{Targets: targets, Outcome: intentFailed, code: 1, Summary: fmt.Sprintf("the lane fix's read could not be retained: %v", err)}
	}
	if data, ok := result.Data.(map[string]any); ok {
		data["commit"] = fix.Commit
	}
	return result
}

// Only a repair's build and commit use batch custody instead of a goal claim.
func (inv *intentInvocation) laneClaimCheck(conn intentConnectionOwners, root, goalID string, endpoint goal.Endpoint) func() error {
	return func() error {
		caller, err := inv.owners.resolver.ResolveLayout(inv.cwd)
		if err == nil && conn.laneFix(root, caller.GitRoot, goalID) != nil {
			return nil
		}
		return conn.claimCheck(root, goalID, endpoint)()
	}
}
