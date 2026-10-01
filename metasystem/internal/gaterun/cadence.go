package gaterun

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
)

const CadenceForcedInterval = 6 * time.Hour

type CadenceTrunk struct{ Commit, Tree string }

// CadenceRevalidation contains one newest observation at the revalidated
// execution identity for every deep-only group.
type CadenceRevalidation struct{ Groups []proofrun.GroupResult }

type CadenceAuthority struct {
	GoalID             string
	ObligationRevision uint64
}

type CadenceTickResult struct {
	ClaimOutcome goal.CadenceClaimOutcome
	Trigger      goal.CadenceTrigger
	Status       *goal.CadenceStatus
	Executed     bool
	Published    bool
	ForceGroups  bool
}

// GoalCadenceLedger adapts the shared goal transaction to the injectable tick
// boundary used by the landing owner.
type GoalCadenceLedger struct {
	Endpoint goal.Endpoint
	Actor    goal.Actor
	MintULID func() (string, error)
}

func (ledger GoalCadenceLedger) request(now time.Time) (goal.VerbRequest, error) {
	mint := ledger.MintULID
	if mint == nil {
		mint = goal.NewOperationULID
	}
	ulid, err := mint()
	return goal.VerbRequest{Endpoint: ledger.Endpoint, Actor: ledger.Actor, Ulid: ulid, Now: now}, err
}

func (ledger GoalCadenceLedger) Claim(now time.Time, key goal.CadenceClaimKey, lease time.Duration) (goal.CadenceClaimResult, error) {
	request, err := ledger.request(now)
	if err != nil {
		return goal.CadenceClaimResult{}, err
	}
	return goal.ClaimCadence(request, key, lease)
}

func (ledger GoalCadenceLedger) Publish(now time.Time, claim string, status goal.CadenceStatus, groups []goal.TrunkRedRecordGroup) error {
	request, err := ledger.request(now)
	if err != nil {
		return err
	}
	_, err = goal.PublishCadence(request, goal.CadencePublishArgs{ClaimOpid: claim, Status: status, Groups: groups})
	return err
}

// cadenceForcedWindow is the forced-window rule: no cadence status yet, or a
// window that has run CadenceForcedInterval, forces a run in a window that
// starts now; otherwise the latest window holds.
func cadenceForcedWindow(now time.Time, latest *goal.CadenceStatus) (forced bool, window string, err error) {
	if latest == nil {
		return true, now.Format(time.RFC3339), nil
	}
	start, err := time.Parse(time.RFC3339, latest.ForcedWindowStart)
	if err != nil {
		return false, "", err
	}
	if !now.Before(start.Add(CadenceForcedInterval)) {
		return true, now.Format(time.RFC3339), nil
	}
	return false, start.UTC().Format(time.RFC3339), nil
}

// CadenceDueByClock says whether validation is due by what local state alone
// shows: the forced window ran out (or none was ever validated), or the
// validation weight is over its threshold. The landing keeper wakes the
// agent on it; an identity change of the trunk's deep-only groups needs a
// fetch and a revalidation, which the validation itself judges.
func CadenceDueByClock(now time.Time, latest *goal.CadenceStatus, weightDue bool) (bool, error) {
	forced, _, err := cadenceForcedWindow(now.UTC(), latest)
	if err != nil {
		return false, err
	}
	return forced || weightDue, nil
}

func cadenceTrigger(now time.Time, latest *goal.CadenceStatus, weightDue bool, ids []string, probes map[string]proofrun.GroupResult) (goal.CadenceTrigger, bool, string, bool, error) {
	forced, window, err := cadenceForcedWindow(now, latest)
	if err != nil {
		return "", false, "", false, err
	}
	if forced {
		return goal.CadenceTriggerForcedWindow, true, window, true, nil
	}
	latestGroups := map[string]goal.CadenceGroupStatus{}
	for _, group := range latest.Groups {
		latestGroups[group.Group] = group
	}
	for _, id := range ids {
		probe, prior := probes[id], latestGroups[id]
		if prior.Group == "" || prior.ExecutionIdentity != probe.ExecutionIdentity || !cadenceGroupGreen(prior.Status) || !cadenceGroupGreen(probe.Status) {
			return goal.CadenceTriggerIdentityChanged, false, window, true, nil
		}
	}
	if weightDue {
		return goal.CadenceTriggerWeightDue, false, window, true, nil
	}
	return "", false, window, false, nil
}

func cadenceProbeMap(ids []string, groups []proofrun.GroupResult) (map[string]proofrun.GroupResult, error) {
	wanted, probes := map[string]bool{}, map[string]proofrun.GroupResult{}
	for _, id := range ids {
		if id == "" || wanted[id] {
			return nil, fmt.Errorf("deep-only cadence inventory is invalid")
		}
		wanted[id] = true
	}
	for _, group := range groups {
		if !wanted[group.ID] || probes[group.ID].ID != "" || len(group.ExecutionIdentity) != 64 {
			return nil, fmt.Errorf("cadence revalidation inventory is incomplete or duplicated")
		}
		if _, err := hex.DecodeString(group.ExecutionIdentity); err != nil {
			return nil, fmt.Errorf("cadence group %s has an invalid execution identity", group.ID)
		}
		probes[group.ID] = group
	}
	if len(probes) != len(wanted) {
		return nil, fmt.Errorf("cadence revalidation omitted a deep-only group")
	}
	return probes, nil
}

func cadenceStatus(trunk CadenceTrunk, trigger goal.CadenceTrigger, runID string, run proofrun.TestResult, started, ended time.Time, key goal.CadenceClaimKey, ids []string, probes map[string]proofrun.GroupResult) goal.CadenceStatus {
	observed := map[string]proofrun.GroupResult{}
	for _, group := range run.Groups {
		observed[group.ID] = group
	}
	groups := make([]goal.CadenceGroupStatus, 0, len(ids))
	for _, id := range ids {
		group := probes[id]
		candidate, ok := observed[id]
		if ok {
			group = candidate
		} else if trigger != goal.CadenceTriggerRevalidation {
			group.Status, group.ReuseAttempt = "unavailable", ""
			group.NotRunReason = "cadence result omitted the deep-only group"
		}
		if cadenceGroupGreen(probes[id].Status) && group.ExecutionIdentity != probes[id].ExecutionIdentity {
			group.Status, group.ReuseAttempt, group.ExecutionIdentity = "invalid", "", probes[id].ExecutionIdentity
		}
		if trigger == goal.CadenceTriggerRevalidation {
			group.Status = "reused"
		}
		groups = append(groups, goal.CadenceGroupStatus{Group: id, ExecutionIdentity: group.ExecutionIdentity, Status: group.Status,
			EvidenceDigest: cadenceEvidenceDigest(group), ReuseSource: group.ReuseAttempt})
	}
	return goal.CadenceStatus{TrunkCommit: trunk.Commit, TrunkTree: trunk.Tree, Trigger: trigger, RunID: runID, AttemptID: run.AttemptID,
		StartedAt: started.Format(time.RFC3339), EndedAt: ended.Format(time.RFC3339), WeightGeneration: key.WeightGeneration,
		ForcedWindowStart: key.ForcedWindowStart, Groups: groups}
}

func cadenceRunGreen(result proofrun.TestResult) bool {
	if len(result.Groups) == 0 {
		return false
	}
	for _, group := range result.Groups {
		if !cadenceGroupGreen(group.Status) || !group.CollectionComplete {
			return false
		}
	}
	return true
}

func cadenceGroupGreen(status string) bool { return status == "passed" || status == "reused" }

func cadenceEvidenceDigest(group proofrun.GroupResult) string {
	encoded, _ := json.Marshal(group)
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:])
}

func cadenceObjectID(value string) bool {
	if len(value) != 40 && len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}
