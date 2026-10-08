package branch

import (
	"fmt"
	"slices"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

type rebasedDrop struct{ before, after goal.UnitDrop }

// Replaying with empty commits retained preserves the first-parent order.
// The published inverse bounds the mapping, including on a publication retry.
func rebaseDrops(req RebaseRequest, tip string, d rebaseDependencies) ([]rebasedDrop, map[string]bool, error) {
	commits, err := d.repository.facts.Range(req.Repo, req.EndpointTip, tip, req.GoalID)
	if err != nil {
		return nil, nil, err
	}
	covered := map[string]bool{}
	var carried []rebasedDrop
	for _, commit := range commits {
		if commit.Kind != Drop {
			continue
		}
		data, err := d.git(req.Repo, "show", req.EndpointTip+":metasystem/plans/goals/"+req.GoalID+".md")
		if err != nil {
			return nil, nil, err
		}
		drops, err := dropsFromPage(data)
		if err != nil {
			return nil, nil, err
		}
		info, err := d.repository.facts.Kind(req.Repo, commit.ID, req.GoalID)
		if err != nil {
			return nil, nil, err
		}
		for _, prior := range drops {
			if prior.Unit != commit.Unit || prior.Operation != info.Operation {
				continue
			}
			next := prior
			if prior.Commit != commit.ID {
				if req.RecordDrop == nil || req.Gate == nil {
					return nil, nil, fmt.Errorf("carrying a drop needs its checks and outcome publisher")
				}
				oldInfo, err := d.repository.facts.Kind(req.Repo, prior.Commit, req.GoalID)
				if err != nil || oldInfo.Kind != Drop || oldInfo.Unit != prior.Unit || oldInfo.Operation != prior.Operation {
					return nil, nil, fmt.Errorf("the published drop's original commit cannot be matched")
				}
				oldTree, err := d.git(req.Repo, "rev-parse", prior.Commit+"^{tree}")
				if err != nil || strings.TrimSpace(string(oldTree)) != prior.Tree {
					return nil, nil, fmt.Errorf("the published drop's original tree cannot be matched")
				}
				base, err := d.git(req.Repo, "merge-base", req.EndpointTip, prior.Commit)
				if err != nil {
					return nil, nil, err
				}
				before, err := d.git(req.Repo, "rev-list", "--reverse", "--first-parent", strings.TrimSpace(string(base))+".."+prior.Commit)
				if err != nil {
					return nil, nil, err
				}
				after, err := d.git(req.Repo, "rev-list", "--reverse", "--first-parent", req.EndpointTip+".."+commit.ID)
				if err != nil {
					return nil, nil, err
				}
				oldIDs, newIDs := strings.Fields(string(before)), strings.Fields(string(after))
				if len(oldIDs) != len(newIDs) {
					return nil, nil, fmt.Errorf("the replay changed the dropped unit's commit coverage")
				}
				next.Covered = slices.Clone(prior.Covered)
				for index, id := range prior.Covered {
					position := slices.Index(oldIDs, id)
					if position < 0 || position == len(oldIDs)-1 {
						return nil, nil, fmt.Errorf("the dropped commit is absent before its inverse")
					}
					old, err := d.repository.facts.Kind(req.Repo, id, req.GoalID)
					if err != nil || old.Kind != Unit || old.Unit != prior.Unit {
						return nil, nil, fmt.Errorf("the original covered commit belongs to another unit")
					}
					new, err := d.repository.facts.Kind(req.Repo, newIDs[position], req.GoalID)
					if err != nil || new.Kind != Unit || new.Unit != prior.Unit {
						return nil, nil, fmt.Errorf("the replayed covered commit belongs to another unit")
					}
					next.Covered[index] = newIDs[position]
				}
				next.Commit = commit.ID
				tree, err := d.git(req.Repo, "rev-parse", commit.ID+"^{tree}")
				if err != nil {
					return nil, nil, err
				}
				next.Tree = strings.TrimSpace(string(tree))
				proof, err := d.gate(ReadGateRequest{Repo: req.Repo, GoalID: req.GoalID, UnitCommit: commit.ID, Gate: req.Gate, Repository: defaultBranchReadRepository{}})
				if err != nil {
					return nil, nil, err
				}
				if proof.Tree != next.Tree {
					return nil, nil, fmt.Errorf("the rebased drop's checked tree changed")
				}
				next.Proof = proof.RunID
				carried = append(carried, rebasedDrop{prior, next})
			}
			for _, id := range next.Covered {
				covered[id] = true
			}
		}
	}
	return carried, covered, nil
}
