package steward

import (
	"fmt"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/lock"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/narratordigest"
)

// ClearHealthAlert retries ended healing from the current observation. Only
// a caller with a proven person's name acknowledges an unexamined ledger tip.
// Arbitration precedes both record locks, as it does in the steward tick.
func ClearHealthAlert(repoRoot, episodeID string, invoker AlertInvoker, acknowledgedBy string, now time.Time) (episode AlertEpisode, changed bool, rearmed []HealthRole, acknowledged bool, err error) {
	if !validEpisodeID(episodeID) {
		return episode, false, nil, false, fmt.Errorf("alert episode id is invalid")
	}
	arbitration, err := AcquireArbitration(repoRoot)
	if err != nil {
		return
	}
	defer arbitration.Release()
	alerts, err := lockAlerts(repoRoot, lock.Exclusive)
	if err != nil {
		return
	}
	defer unlockAlerts(alerts)
	episode, err = loadAlertEpisode(alertPath(repoRoot, episodeID))
	if err != nil {
		return
	}
	if episode.Owner != "" && episode.Owner != PatternOwner("health-standing-red") {
		episode, changed, err = clearAlertEpisode(repoRoot, episode, invoker, now)
		return
	}
	health, err := lock.File(healthLockPath(repoRoot), 0o644, lock.Exclusive)
	if err != nil {
		return
	}
	defer health.Release()
	record, err := loadHealthRecord(HealthRecordPath(repoRoot))
	if err != nil {
		err = fmt.Errorf("%s: %w", HealthRecordPath(repoRoot), err)
		return
	}
	// Validate attention before writing anything, even when the caller cannot
	// acknowledge it. Unreadable current state must leave the alert open.
	var attention ledgerAttentionState
	for _, role := range record.Verdict.Roles {
		if role.Role == RoleLedgerAttention && role.Status == HealthDead {
			var exists bool
			attention, exists, err = loadLedgerAttentionState(repoRoot)
			if err == nil && !exists {
				err = fmt.Errorf("no ledger-attention state is recorded")
			}
			if err != nil {
				err = fmt.Errorf("%s: %w", ledgerAttentionStatePath(repoRoot), err)
				return
			}
		}
		namedStanding := role.Standing && episode.Owner == PatternOwner("health-standing-red") && episode.ScopeID == string(role.Role)
		if role.Status == HealthDead && (role.FailureEscalation == AutoHealEnded || namedStanding) {
			rearmed = append(rearmed, role.Role)
			delete(record.State.FailureCounts, role.Role)
			delete(record.State.FailureCauses, role.Role)
		}
	}
	if acknowledgedBy != "" && attention.RemoteTip != "" && attention.RemoteTip != attention.ExaminedTip {
		if err = narratordigest.Append(repoRoot, []narratordigest.Entry{{Kind: "highlight",
			Text:       fmt.Sprintf("A person (%s) acknowledged the shared ledger's move to %s.", acknowledgedBy, attention.RemoteTip),
			SourceType: "ledger-acknowledgment", SourceID: eventSourceID(attention.RemoteTip, attention.TopologyEpoch),
		}}, now); err != nil {
			return
		}
		attention.ExaminedTip, attention.MovedAt = attention.RemoteTip, ""
		if err = saveLedgerAttentionState(repoRoot, attention); err != nil {
			return
		}
		acknowledged = true
	}
	if len(rearmed) > 0 {
		record.Verdict.State = record.State
		record.Verdict.Roles = standingRoles(record.State, record.Verdict.Roles)
		record.Verdict.Aggregate, record.Verdict.ShouldAlert = healthSummary(record.Verdict.Roles)
		record.Verdict.FindingDigest = healthFindingDigest(record.Verdict.Roles)
		if err = saveHealthRecord(repoRoot, HealthRecordPath(repoRoot), record); err != nil {
			return
		}
	}
	episode, changed, err = clearAlertEpisode(repoRoot, episode, invoker, now)
	return
}
