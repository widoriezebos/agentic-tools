package branch

import (
	"fmt"
	"slices"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

// Resolved includes successful optional drops without treating them as reads.
func (u UnitStatus) Resolved() bool {
	return u.Drop != nil || u.ReadState == "read clean" || u.ReadState == "read transferred"
}
func dropsFromPage(data []byte) ([]goal.UnitDrop, error) {
	if !strings.Contains(string(data), "- UnitDrop:") {
		return nil, nil
	}
	page, problems := goal.ParseFile(data)
	if len(problems) != 0 {
		return nil, fmt.Errorf("the goal's drop outcomes cannot be read: %v", problems)
	}
	return page.UnitDrops, nil
}
func committedDrops(repo, endpoint, id string) ([]goal.UnitDrop, error) {
	data, err := gitOutput(repo, "show", endpoint+":metasystem/plans/goals/"+id+".md")
	if err != nil {
		return nil, nil
	}
	return dropsFromPage(data)
}
func applyDrops(status *Status, repo, endpoint, id string, deps statusDependencies) error {
	for position, commit := range status.Commits {
		if commit.Kind != Drop {
			continue
		}
		drops, err := deps.drops(repo, endpoint, id)
		if err != nil {
			return operationRefusal(LandUnprovenCode, "%v\nrun: metasystem work status %s --work %s", err, id, commit.Unit)
		}
		info, err := deps.kind(repo, commit.ID, id)
		if err != nil {
			return err
		}
		for _, drop := range drops {
			if drop.Commit != commit.ID || drop.Unit != commit.Unit || drop.Operation != info.Operation {
				continue
			}
			tree, err := deps.dropTree(repo, commit.ID)
			if err != nil {
				return err
			}
			expected := drop.Tree
			if drop.CommitTree != "" {
				expected = drop.CommitTree
			}
			if tree != expected {
				return fmt.Errorf("the drop's resulting tree differs from its published outcome")
			}
			covered := 0
			for _, unit := range status.Units {
				if unit.Unit == drop.Unit && slices.Contains(drop.Covered, unit.Commit) && slices.ContainsFunc(status.Commits[:position], func(c Commit) bool { return c.ID == unit.Commit }) {
					covered++
				}
			}
			if covered != len(drop.Covered) {
				continue
			}
			for index, unit := range status.Units {
				if unit.Unit == drop.Unit && slices.Contains(drop.Covered, unit.Commit) {
					status.Units[index].PriorReadState = unit.ReadState
					status.Units[index].ReadState, status.Units[index].Drop = "dropped", &drop
				}
			}
		}
	}
	return nil
}

func (defaultBranchReadRepository) DropRead(repo, endpoint, tip, id, commit string) (BranchReadResult, bool, error) {
	status, err := InspectStatus(repo, endpoint, tip, id)
	if err != nil {
		return BranchReadResult{}, false, err
	}
	for _, unit := range status.Units {
		if unit.Drop != nil && (unit.Commit == commit || unit.Drop.Commit == commit) {
			return BranchReadResult{State: "dropped", GateRunID: unit.Drop.Proof, Published: true}, true, nil
		}
	}
	return BranchReadResult{}, false, nil
}
func droppedRead(repository BranchReadRepository, req BranchReadRequest) (BranchReadResult, bool, error) {
	owner, ok := repository.(interface {
		DropRead(string, string, string, string, string) (BranchReadResult, bool, error)
	})
	if !ok {
		return BranchReadResult{}, false, nil
	}
	return owner.DropRead(req.Repo, req.EndpointTip, req.BranchTip, req.GoalID, req.UnitCommit)
}
