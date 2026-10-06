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
	gitOutput      func(repo string, args ...string) ([]byte, error)
}

func defaultStatusDependencies() statusDependencies {
	return statusDependencies{ValidateRange, KindOf, ValidateAttestationAt, localBranchTip, gitOutput}
}

func statusDependenciesWithRaw(read func(string, ...string) ([]byte, error)) statusDependencies {
	deps := defaultStatusDependencies()
	deps.gitOutput = read
	deps.kind = func(repo, commit, goalID string) (KindInfo, error) {
		return KindOfWithRaw(repo, commit, goalID, read)
	}
	return deps
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
				return Status{}, operationRefusal(RangeCode, "goal %s's branch holds build %s twice\nrun: metasystem work status %s", goalID, commit.ID, goalID)
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
	return ShouldSweepWithRaw(repo, goalID, next, localTip, gitOutput)
}

// ShouldSweepWithRaw applies the sweep policy to a caller's local ref and Git.
func ShouldSweepWithRaw(repo, goalID, next string, localTip func(repo, ref string) (string, bool, error), read func(string, ...string) ([]byte, error)) (bool, error) {
	deps := statusDependenciesWithRaw(read)
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

func checkParkBranch(repo, goalID, next string, readRemote ParkBranchRemoteReader, deps statusDependencies) (ParkBranchState, error) {
	localTip, localPresent, err := deps.localTip(repo, goalBranchRef(goalID))
	if err != nil {
		return ParkBranchState{}, err
	}
	if !localPresent {
		if nextNamesUnitCommit(repo, goalID, next, deps) {
			return ParkBranchState{}, operationRefusal(ParkUnpushedCode, "this checkout has no goal/%s branch, so its work can't be kept while parked\nrun: git fetch origin goal/%s:goal/%s, then metasystem goal pause %s", goalID, goalID, goalID, goalID)
		}
		return ParkBranchState{}, nil
	}
	endpointTip, originTip, originPresent, err := readRemote()
	if err != nil {
		return ParkBranchState{}, err
	}
	if !originPresent || localTip != originTip {
		remote := "no copy"
		if originPresent {
			remote = originTip
		}
		return ParkBranchState{}, operationRefusal(ParkUnpushedCode, "goal/%s here is %s but origin has %s; push it before parking\nrun: git push origin goal/%s, then metasystem goal pause %s", goalID, localTip, remote, goalID, goalID)
	}
	// Parking keeps pushed work recoverable without judging its reads.
	commits, err := deps.gitOutput(repo, "rev-list", "--first-parent", endpointTip+".."+originTip)
	if err != nil {
		return ParkBranchState{}, err
	}
	for _, commit := range strings.Fields(string(commits)) {
		kind, err := deps.kind(repo, commit, goalID)
		if err != nil {
			return ParkBranchState{}, err
		}
		if kind.Kind == Unit {
			return ParkBranchState{Branch: true, Summary: fmt.Sprintf("goal/%s at %s is pushed; last unit %s commit %s", goalID, originTip, kind.Unit, commit)}, nil
		}
	}
	return ParkBranchState{Branch: true, Summary: fmt.Sprintf("goal/%s at %s has no unit", goalID, originTip)}, nil
}
