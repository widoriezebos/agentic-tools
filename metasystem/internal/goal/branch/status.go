package branch

import (
	"fmt"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"slices"
	"strings"
)

const ParkUnpushedCode = "GOAL_PARK_UNPUSHED"

type UnitStatus struct {
	Whole                           bool
	Unit, Commit, Digest, ReadState string
	Units                           []string
	PriorReadState, ScopeOperation  string
}

type Status struct {
	Tip               string
	Commits           []Commit
	Units             []UnitStatus
	Prefix            int
	ReviewObligations []goal.ReviewObligation
	Scope             *goal.GoalFile
}

type statusDependencies struct {
	validatedRange      func(repo, endpointTip, tip, goalID string) ([]Commit, error)
	kind                func(repo, commit, goalID string) (KindInfo, error)
	attestation         func(repo, snapshot, endpointTip, goalID, unit, commit string) (Attestation, error)
	localTip            func(repo, ref string) (string, bool, error)
	gitOutput           func(repo string, args ...string) ([]byte, error)
	transferObligations func(repo, endpoint, goalID string) []goal.ReviewObligation
}

func defaultStatusDependencies() statusDependencies {
	return statusDependencies{validatedRange: ValidateRange, kind: KindOf, attestation: ValidateAttestationAt, localTip: localBranchTip, gitOutput: gitOutput,
		transferObligations: func(repo, endpoint, goalID string) []goal.ReviewObligation {
			data, err := gitOutput(repo, "show", endpoint+":metasystem/plans/goals/"+goalID+".md")
			if err != nil {
				return nil
			}
			return transferObligationsFromPage(data)
		}}
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

// ApplyScope projects the current ledger's exclusion onto fresh branch status, preserving its prior read.
func ApplyScope(status *Status, file *goal.GoalFile) {
	status.Scope = file
	if file == nil {
		return
	}
	for i, unit := range status.Units {
		for _, drop := range file.UnitDrops {
			if file.ExcludesScope(unit.Unit, "result:"+drop.Commit) && drop.Unit == unit.Unit && slices.Contains(drop.Covered, unit.Commit) {
				status.Units[i].PriorReadState, status.Units[i].ReadState, status.Units[i].ScopeOperation = unit.ReadState, "dropped", drop.Operation
			}
		}
	}
	status.Prefix = 0
	for status.Prefix < len(status.Units) && slices.Contains([]string{"read clean", "read transferred", "dropped"}, status.Units[status.Prefix].ReadState) {
		status.Prefix++
	}
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
				Unit: commit.Unit, Units: append([]string(nil), commit.Units...), Commit: commit.ID, Digest: commit.Digest, ReadState: "built", Whole: commit.Whole,
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
	if deps.transferObligations != nil {
		result.ReviewObligations = deps.transferObligations(repo, endpointTip, goalID)
		applyTransferCoverage(&result, func(unit UnitStatus) (Attestation, error) {
			return deps.attestation(repo, tip, endpointTip, goalID, unit.Unit, unit.Commit)
		})
	}
	for _, unit := range result.Units {
		if unit.ReadState != "read clean" && unit.ReadState != "read transferred" {
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

func transferObligationsFromPage(data []byte) []goal.ReviewObligation {
	if !strings.Contains(string(data), "- ReviewObligation:") {
		return nil
	}
	page, problems := goal.ParseFile(data)
	if len(problems) != 0 {
		return nil
	}
	return page.ReviewObligations
}

func applyTransferCoverage(status *Status, read func(UnitStatus) (Attestation, error)) {
	for index, source := range status.Units {
		found, complete := false, true
		for _, obligation := range status.ReviewObligations {
			if obligation.SourceUnit != source.Unit || obligation.TargetUnit == "" {
				continue
			}
			found = true
			if obligation.State != "discharged" || obligation.CoverageRead == "" || obligation.CoverageCommit == "" || obligation.SourceCommit != source.Commit {
				complete = false
				break
			}
			destinationFound := false
			for _, destination := range status.Units {
				if destination.Unit != obligation.TargetUnit || destination.Commit != obligation.CoverageCommit || destination.ReadState != "read clean" {
					continue
				}
				att, err := read(destination)
				if err != nil {
					continue
				}
				coverage := coverageOf(att)
				if coverage.ReadID != obligation.CoverageRead || !slices.Contains(coverage.Findings, obligation.OriginalFinding) || !slices.Contains(coverage.SourceCommits, source.Commit) {
					continue
				}
				destinationFound = true
			}
			if !destinationFound {
				complete = false
				break
			}
		}
		if found && complete {
			status.Units[index].ReadState = "read transferred"
		}
	}
}
