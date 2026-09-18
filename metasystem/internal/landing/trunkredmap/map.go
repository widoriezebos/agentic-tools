package trunkredmap

import (
	"slices"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
)

// ResultToRedGroups is the shared proof-result adapter used by batch and
// cadence publication.
func ResultToRedGroups(result proofrun.TestResult) []batch.RedGroup {
	var groups []batch.RedGroup
	for _, group := range result.Groups {
		if group.Status == "passed" || group.Status == "reused" {
			continue
		}
		red := batch.RedGroup{ID: group.ID, Status: group.Status, NotRunReason: group.NotRunReason,
			LogPath: group.LogPath, LogDigest: group.LogDigest, InputManifest: slices.Clone(group.InputManifest)}
		for _, observed := range group.Observed {
			if observed.Status == "failed" {
				red.Failures = append(red.Failures, batch.Failure{Report: observed.Report, Classname: observed.Classname,
					Name: observed.Name, Status: observed.Status, Reason: observed.Reason})
			}
		}
		groups = append(groups, red)
	}
	return groups
}

// RedGroupsToRecordGroups preserves the batch trunk-red identity and failure
// shape at the shared goal-ledger boundary.
func RedGroupsToRecordGroups(groups []batch.RedGroup) []goal.TrunkRedRecordGroup {
	converted := make([]goal.TrunkRedRecordGroup, 0, len(groups))
	for _, group := range groups {
		record := goal.TrunkRedRecordGroup{Identity: batch.TrunkRedID(group), Group: group.ID, Status: group.Status,
			NotRunReason: group.NotRunReason, LogPath: group.LogPath, LogDigest: group.LogDigest}
		for _, failure := range group.Failures {
			record.Failures = append(record.Failures, goal.TrunkRedFailure{Report: failure.Report, Classname: failure.Classname,
				Name: failure.Name, Status: failure.Status, Reason: failure.Reason})
		}
		if record.Failures == nil {
			record.Failures = []goal.TrunkRedFailure{}
		}
		converted = append(converted, record)
	}
	return converted
}
