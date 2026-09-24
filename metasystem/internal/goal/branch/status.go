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

type statusDependencies struct {
	validatedRange func(repo, endpointTip, tip, goalID string) ([]Commit, error)
	kind           func(repo, commit, goalID string) (KindInfo, error)
	attestation    func(repo, snapshot, endpointTip, goalID, unit, commit string) (Attestation, error)
	localTip       func(repo, ref string) (string, bool, error)
}

func defaultStatusDependencies() statusDependencies {
	return statusDependencies{ValidateRange, KindOf, ValidateAttestationAt, localBranchTip}
}

func InspectStatus(repo, endpointTip, tip, goalID string) (Status, error) {
	return inspectStatus(repo, endpointTip, tip, goalID, defaultStatusDependencies())
}

func inspectStatus(repo, endpointTip, tip, goalID string, deps statusDependencies) (Status, error) {
	commits, err := deps.validatedRange(repo, endpointTip, tip, goalID)
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
			info, err := deps.kind(repo, commit.ID, goalID)
			if err != nil {
				return Status{}, err
			}
			index, exists := unitIndex[info.CommitID]
			if !exists || result.Units[index].Unit != commit.Unit {
				continue
			}
			result.Units[index].ReadState = "needs read"
			if _, err := deps.attestation(repo, tip, endpointTip, goalID, commit.Unit, info.CommitID); err == nil {
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

func nextNamesUnitCommit(repo, goalID, next string, deps statusDependencies) bool {
	for _, field := range strings.Fields(next) {
		field = strings.Trim(field, "()[]{}<>,.;:\"'")
		if !hex40(field) {
			continue
		}
		kind, err := deps.kind(repo, field, goalID)
		if err == nil && kind.Kind == Unit {
			return true
		}
	}
	return false
}

func ShouldSweep(repo, goalID, next string) (bool, error) {
	return shouldSweep(repo, goalID, next, defaultStatusDependencies())
}

// ShouldSweepWithLocalTip applies the branch policy to a caller's raw local ref.
func ShouldSweepWithLocalTip(repo, goalID, next string, localTip func(repo, ref string) (string, bool, error)) (bool, error) {
	deps := defaultStatusDependencies()
	deps.localTip = localTip
	return shouldSweep(repo, goalID, next, deps)
}

func shouldSweep(repo, goalID, next string, deps statusDependencies) (bool, error) {
	_, localPresent, err := deps.localTip(repo, goalBranchRef(goalID))
	if err != nil {
		return false, err
	}
	return localPresent || nextNamesUnitCommit(repo, goalID, next, deps), nil
}

func CheckParkBranch(repo, goalID, next string, readRemote ParkBranchRemoteReader) (ParkBranchState, error) {
	return checkParkBranch(repo, goalID, next, readRemote, defaultStatusDependencies())
}

// CheckParkBranchWithLocalTip uses the ordinary branch policy with a caller's
// raw local-ref reader. All other status readers retain their defaults.
func CheckParkBranchWithLocalTip(repo, goalID, next string, readRemote ParkBranchRemoteReader, localTip func(repo, ref string) (string, bool, error)) (ParkBranchState, error) {
	deps := defaultStatusDependencies()
	deps.localTip = localTip
	return checkParkBranch(repo, goalID, next, readRemote, deps)
}

func checkParkBranch(repo, goalID, next string, readRemote ParkBranchRemoteReader, deps statusDependencies) (ParkBranchState, error) {
	localTip, localPresent, err := deps.localTip(repo, goalBranchRef(goalID))
	if err != nil {
		return ParkBranchState{}, err
	}
	if !localPresent {
		if nextNamesUnitCommit(repo, goalID, next, deps) {
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
	status, err := inspectStatus(repo, endpointTip, originTip, goalID, deps)
	if err != nil {
		return ParkBranchState{}, err
	}
	if len(status.Units) == 0 {
		return ParkBranchState{Branch: true, Summary: fmt.Sprintf("goal/%s at %s has no unit", goalID, originTip)}, nil
	}
	last := status.Units[len(status.Units)-1]
	return ParkBranchState{Branch: true, Summary: fmt.Sprintf("goal/%s last unit %s commit %s is %s", goalID, last.Unit, last.Commit, last.ReadState)}, nil
}
