package batch

import (
	"fmt"
	"os/exec"
	"slices"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	goalbranch "github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
)

type BranchReadRequest struct {
	Repo, EndpointTip, BranchTip, GoalID, Through string
	Last                                          bool
}

type BranchBuild struct {
	Units       []string               `json:"units"`
	Commit      string                 `json:"commit"`
	Digest      string                 `json:"digest"`
	Folds       []goalbranch.Commit    `json:"folds,omitempty"`
	FoldPaths   []string               `json:"foldPaths,omitempty"`
	CoAuthors   []string               `json:"coAuthors,omitempty"`
	Attestation goalbranch.Attestation `json:"attestation"`
}

type BranchMember struct {
	GoalID, Tip string        `json:"-"`
	Last        bool          `json:"-"`
	Builds      []BranchBuild `json:"builds"`
}

// BindBranchMember records the stable identities needed to reassemble,
// receipt, land, and recover one goal as a single batch member.
func BindBranchMember(unit Unit, member BranchMember) Unit {
	unit.BranchTip = member.Tip
	unit.GoalLast = member.Last
	unit.Builds = append([]BranchBuild(nil), member.Builds...)
	for _, build := range member.Builds {
		unit.CommitIDs = append(unit.CommitIDs, build.Commit)
		if len(build.Units) != 0 {
			unit.LastUnit = build.Units[len(build.Units)-1]
		}
	}
	return unit
}

func branchMemberOf(unit Unit) (BranchMember, bool) {
	if len(unit.Builds) == 0 {
		return BranchMember{}, false
	}
	return BranchMember{GoalID: unit.GoalID, Tip: unit.BranchTip, Last: unit.GoalLast, Builds: append([]BranchBuild(nil), unit.Builds...)}, true
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
			index, ok := byCommit[commit.ID]
			if !ok {
				break
			}
			if index == next {
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

func requireCriticRootSource(unit string, attestation goalbranch.Attestation) error {
	if attestation.Source.Kind != "critic-root" {
		return refuseBatch("BATCH_JOIN_UNREAD", "build "+unit+" was read outside a critic-root job")
	}
	return nil
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
	member := BranchMember{GoalID: request.GoalID, Tip: request.BranchTip, Last: request.Last, Builds: groupBranchBuilds(status, count)}
	for index := range member.Builds {
		build := &member.Builds[index]
		unit := status.Units[index]
		attestation, err := goalbranch.ValidateAttestationAt(request.Repo, request.BranchTip, request.EndpointTip, request.GoalID, unit.Unit, build.Commit)
		if err != nil {
			return BranchMember{}, refuseBatch("BATCH_JOIN_UNREAD", "build "+unit.Unit+" has no valid branch attestation: "+err.Error())
		}
		if err := requireCriticRootSource(unit.Unit, attestation); err != nil {
			return BranchMember{}, err
		}
		build.Attestation = attestation
		paths := map[string]bool{}
		for _, fold := range build.Folds {
			entries, err := goalbranch.RawEntries(request.Repo, fold.ID)
			if err != nil {
				return BranchMember{}, err
			}
			for _, entry := range entries {
				paths[entry.Path] = true
			}
		}
		for path := range paths {
			build.FoldPaths = append(build.FoldPaths, path)
		}
		slices.Sort(build.FoldPaths)
		command := exec.Command("git", "-C", request.Repo, "show", "-s", "--format=%B", build.Commit)
		command.Env = gittree.ScrubbedEnviron()
		message, err := command.Output()
		if err != nil {
			return BranchMember{}, err
		}
		for _, line := range strings.Split(string(message), "\n") {
			if value, ok := strings.CutPrefix(strings.TrimSpace(line), "Co-Authored-By:"); ok && strings.TrimSpace(value) != "" {
				build.CoAuthors = append(build.CoAuthors, strings.TrimSpace(value))
			}
		}
	}
	return member, nil
}
