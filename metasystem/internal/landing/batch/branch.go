package batch

import (
	"fmt"

	goalbranch "github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
)

type BranchReadRequest struct {
	Repo, EndpointTip, BranchTip, GoalID, Through string
	Last                                          bool
}

type BranchBuild struct {
	Units       []string
	Commit      string
	Digest      string
	Folds       []goalbranch.Commit
	Attestation goalbranch.Attestation
}

type BranchMember struct {
	GoalID, Tip string
	Builds      []BranchBuild
}

func groupBranchBuilds(status goalbranch.Status, count int) []BranchBuild {
	groups := make([]BranchBuild, count)
	byCommit := map[string]int{}
	byUnits := map[string]int{}
	for index := range groups {
		unit := status.Units[index]
		groups[index] = BranchBuild{Units: append([]string(nil), unit.Units...), Commit: unit.Commit, Digest: unit.Digest}
		byCommit[unit.Commit] = index
		byUnits[unit.Unit] = index
	}
	next := 0
	for _, commit := range status.Commits {
		if commit.Kind == goalbranch.Unit {
			if index, ok := byCommit[commit.ID]; ok && index == next {
				next++
			}
			continue
		}
		target := -1
		if commit.Kind == goalbranch.Read {
			if index, ok := byUnits[commit.Unit]; ok {
				target = index
			}
		} else if next < count {
			target = next
		} else if count > 0 {
			target = count - 1
		}
		if target >= 0 {
			groups[target].Folds = append(groups[target].Folds, commit)
		}
	}
	return groups
}

func ReadGoalBranch(request BranchReadRequest) (BranchMember, error) {
	if request.Repo == "" || request.EndpointTip == "" || request.BranchTip == "" || request.GoalID == "" || request.Last == (request.Through != "") {
		return BranchMember{}, fmt.Errorf("branch join needs a repository, endpoint, branch tip, goal, and exactly one of last or through")
	}
	status, err := goalbranch.InspectStatus(request.Repo, request.EndpointTip, request.BranchTip, request.GoalID)
	if err != nil {
		return BranchMember{}, err
	}
	count := status.Prefix
	if request.Last {
		if count == 0 || count != len(status.Units) {
			return BranchMember{}, refuseBatch("BATCH_JOIN_UNREAD", "goal "+request.GoalID+" is not read clean through its branch tip")
		}
	} else {
		count = 0
		for index := 0; index < status.Prefix; index++ {
			if status.Units[index].Commit == request.Through {
				count = index + 1
				break
			}
		}
		if count == 0 {
			return BranchMember{}, refuseBatch("BATCH_JOIN_UNREAD", "through commit "+request.Through+" is outside the read-clean prefix")
		}
	}
	member := BranchMember{GoalID: request.GoalID, Tip: request.BranchTip, Builds: groupBranchBuilds(status, count)}
	for index := range member.Builds {
		build := &member.Builds[index]
		unit := status.Units[index]
		attestation, err := goalbranch.ValidateAttestationAt(request.Repo, request.BranchTip, request.EndpointTip, request.GoalID, unit.Unit, build.Commit)
		if err != nil {
			return BranchMember{}, refuseBatch("BATCH_JOIN_UNREAD", "build "+unit.Unit+" has no valid branch attestation: "+err.Error())
		}
		if attestation.Source.Kind != "critic-root" {
			return BranchMember{}, refuseBatch("BATCH_JOIN_UNREAD", "build "+unit.Unit+" was read outside a critic-root job")
		}
		build.Attestation = attestation
	}
	return member, nil
}
