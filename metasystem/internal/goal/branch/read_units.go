package branch

import (
	"fmt"
	"slices"
	"strings"
)

func (att Attestation) CoversUnit(unit UnitStatus) bool {
	names := unit.Units
	if len(names) == 0 {
		names = strings.Split(unit.Unit, "+")
	}
	for _, name := range names {
		if !slices.Contains(att.CoveredUnits, name) {
			return false
		}
	}
	return len(names) > 0
}

// A tip read keeps the last build's trailer names while its subject and
// covered units describe the complete examination, including later records.
func branchReadSubject(repository BranchReadRepository, request BranchReadRequest) (KindInfo, error) {
	commits, err := repository.Range(request.Repo, request.EndpointTip, request.BranchTip, request.GoalID)
	if err != nil {
		return KindInfo{}, err
	}
	var info KindInfo
	found := false
	for _, commit := range commits {
		if commit.Kind == Unit {
			info = KindInfo{Kind: Unit, Unit: commit.Unit, Units: commit.Units, CommitID: commit.ID}
		}
		if commit.ID == request.UnitCommit {
			if commit.Kind == Unit {
				return info, nil
			}
			found = true
			break
		}
	}
	if !found || len(info.Units) == 0 {
		return KindInfo{}, operationRefusal(ReadInvalidCode, "commit %s is not a build on goal %s's branch\nrun: metasystem work status %s", request.UnitCommit, request.GoalID, request.GoalID)
	}
	if request.UnitCommit != request.BranchTip || len(request.Units) == 0 {
		_, path, _, pathErr := branchReadPathsWithRepository(repository, request.Repo, request.GoalID, request.UnitCommit)
		if pathErr != nil {
			return KindInfo{}, pathErr
		}
		record, readErr := loadBranchReadRecord(path)
		if readErr != nil || record.RootJob == "" || len(record.CoveredUnits) == 0 {
			return KindInfo{}, fmt.Errorf("a new read naming units must examine the branch tip")
		}
	}
	return info, nil
}
