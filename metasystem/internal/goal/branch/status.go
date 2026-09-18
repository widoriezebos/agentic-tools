package branch

import (
	"fmt"
	"strings"
)

const ParkUnpushedCode = "GOAL_PARK_UNPUSHED"

type UnitStatus struct {
	Unit, Commit, Digest, ReadState string
	Units                           []string
}

type Status struct {
	Tip     string
	Commits []Commit
	Units   []UnitStatus
	Prefix  int
}

func InspectStatus(repo, endpointTip, tip, goalID string) (Status, error) {
	commits, err := ValidateRange(repo, endpointTip, tip, goalID)
	if err != nil {
		return Status{}, err
	}
	result := Status{Tip: tip, Commits: commits}
	unitIndex := map[string]int{}
	for _, commit := range commits {
		switch commit.Kind {
		case Unit:
			if _, exists := unitIndex[commit.ID]; exists {
				return Status{}, operationRefusal(RangeCode, "goal branch repeats unit commit %s", commit.ID)
			}
			unitIndex[commit.ID] = len(result.Units)
			result.Units = append(result.Units, UnitStatus{
				Unit: commit.Unit, Units: append([]string(nil), commit.Units...), Commit: commit.ID, Digest: commit.Digest, ReadState: "built",
			})
		case Read:
			info, err := KindOf(repo, commit.ID, goalID)
			if err != nil {
				return Status{}, err
			}
			index, exists := unitIndex[info.CommitID]
			if !exists || result.Units[index].Unit != commit.Unit {
				continue
			}
			result.Units[index].ReadState = "needs read"
			if _, err := ValidateAttestationAt(repo, tip, endpointTip, goalID, commit.Unit, info.CommitID); err == nil {
				result.Units[index].ReadState = "read clean"
			}
		}
	}
	for _, unit := range result.Units {
		if unit.ReadState != "read clean" {
			break
		}
		result.Prefix++
	}
	return result, nil
}

type ParkBranchState struct {
	Branch  bool
	Summary string
}

type ParkBranchRemoteReader func() (endpointTip, originTip string, originPresent bool, err error)

func nextNamesUnitCommit(repo, goalID, next string) bool {
	for _, field := range strings.Fields(next) {
		field = strings.Trim(field, "()[]{}<>,.;:\"'")
		if !hex40(field) {
			continue
		}
		kind, err := KindOf(repo, field, goalID)
		if err == nil && kind.Kind == Unit {
			return true
		}
	}
	return false
}

func ShouldSweep(repo, goalID, next string) (bool, error) {
	_, localPresent, err := localBranchTip(repo, goalBranchRef(goalID))
	if err != nil {
		return false, err
	}
	return localPresent || nextNamesUnitCommit(repo, goalID, next), nil
}

func CheckParkBranch(repo, goalID, next string, readRemote ParkBranchRemoteReader) (ParkBranchState, error) {
	localTip, localPresent, err := localBranchTip(repo, goalBranchRef(goalID))
	if err != nil {
		return ParkBranchState{}, err
	}
	if !localPresent {
		if nextNamesUnitCommit(repo, goalID, next) {
			return ParkBranchState{}, operationRefusal(ParkUnpushedCode, "this checkout has no goal/%s; fetch it and check it out", goalID)
		}
		return ParkBranchState{}, nil
	}
	endpointTip, originTip, originPresent, err := readRemote()
	if err != nil {
		return ParkBranchState{}, err
	}
	if !originPresent || localTip != originTip {
		remote := "<absent>"
		if originPresent {
			remote = originTip
		}
		return ParkBranchState{}, operationRefusal(ParkUnpushedCode, "local goal/%s is %s while origin is %s; push the branch before parking", goalID, localTip, remote)
	}
	status, err := InspectStatus(repo, endpointTip, originTip, goalID)
	if err != nil {
		return ParkBranchState{}, err
	}
	if len(status.Units) == 0 {
		return ParkBranchState{Branch: true, Summary: fmt.Sprintf("goal/%s at %s has no unit", goalID, originTip)}, nil
	}
	last := status.Units[len(status.Units)-1]
	return ParkBranchState{Branch: true, Summary: fmt.Sprintf("goal/%s last unit %s commit %s is %s", goalID, last.Unit, last.Commit, last.ReadState)}, nil
}
