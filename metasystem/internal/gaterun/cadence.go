package gaterun

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/trunkredmap"
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

type CadenceRunRequest struct {
	Trunk            CadenceTrunk
	Authority        CadenceAuthority
	WeightGeneration uint64
	ForceAttempt     bool
	ForceGroups      bool
}

type CadenceRunResult struct {
	RunID  string
	Result proofrun.TestResult
}

type CadenceLedger interface {
	Claim(time.Time, goal.CadenceClaimKey, time.Duration) (goal.CadenceClaimResult, error)
	Publish(time.Time, string, goal.CadenceStatus, []goal.TrunkRedRecordGroup) error
}

type CadenceTickInput struct {
	Latest         *goal.CadenceStatus
	Weight         WeightState
	WeightDue      bool
	DeepOnlyGroups []string
	Lease          time.Duration
}

type CadenceDependencies struct {
	Clock            func() time.Time
	Fetch            func() (CadenceTrunk, error)
	Revalidate       func(CadenceTrunk) (CadenceRevalidation, error)
	Ledger           CadenceLedger
	ClaimAuthority   func(time.Time) (CadenceAuthority, error)
	ReleaseAuthority func(CadenceAuthority, time.Time) error
	Run              func(CadenceRunRequest) (CadenceRunResult, error)
	DischargeWeight  func(CadenceAuthority, string, uint64, time.Time) error
}

type CadenceTickResult struct {
	ClaimOutcome goal.CadenceClaimOutcome
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

// RunCadenceTick owns trigger choice and one claimed cadence transition. It
// never waits for a joined claim; a later owner tick reads its terminal result.
func RunCadenceTick(input CadenceTickInput, deps CadenceDependencies) (result CadenceTickResult, err error) {
	if deps.Clock == nil || deps.Fetch == nil || deps.Revalidate == nil || deps.Ledger == nil || input.Lease <= 0 || len(input.DeepOnlyGroups) == 0 {
		return result, fmt.Errorf("cadence tick dependencies are incomplete")
	}
	started := deps.Clock().UTC().Truncate(time.Second)
	trunk, err := deps.Fetch()
	if err != nil {
		return result, err
	}
	if !cadenceObjectID(trunk.Commit) || !cadenceObjectID(trunk.Tree) {
		return result, fmt.Errorf("fetched cadence trunk is incomplete")
	}
	revalidation, err := deps.Revalidate(trunk)
	if err != nil {
		return result, err
	}
	probes, err := cadenceProbeMap(input.DeepOnlyGroups, revalidation.Groups)
	if err != nil {
		return result, err
	}
	trigger, forceGroups, window, due, err := cadenceTrigger(started, input.Latest, input.WeightDue, input.DeepOnlyGroups, probes)
	if err != nil {
		return result, err
	}
	if !due && input.Latest != nil && input.Latest.TrunkCommit == trunk.Commit && input.Latest.TrunkTree == trunk.Tree {
		return result, nil
	}
	key := goal.CadenceClaimKey{TrunkTree: trunk.Tree, WeightGeneration: input.Weight.Generation, ForcedWindowStart: window}
	claim, err := deps.Ledger.Claim(started, key, input.Lease)
	if err != nil {
		return result, err
	}
	result.ClaimOutcome = claim.Outcome
	if claim.Outcome == goal.CadenceClaimComplete {
		result.Status = claim.Status
		return result, nil
	}
	if claim.Outcome != goal.CadenceClaimAcquired {
		return result, nil
	}
	if claim.Claim == nil {
		return result, fmt.Errorf("acquired cadence claim has no record")
	}
	if !due {
		status := cadenceStatus(trunk, goal.CadenceTriggerRevalidation, "", proofrun.TestResult{}, started, started, key, input.DeepOnlyGroups, probes)
		err = deps.Ledger.Publish(started, claim.Claim.Opid, status, nil)
		if err == nil {
			result.Status, result.Published = &status, true
		}
		return result, err
	}
	result.ForceGroups = forceGroups
	if deps.ClaimAuthority == nil || deps.ReleaseAuthority == nil || deps.Run == nil {
		return publishCadenceFailure(result, deps, claim.Claim.Opid, trunk, trigger, key, started, input.DeepOnlyGroups, probes, "standing cadence authority is unavailable")
	}
	authority, authorityErr := deps.ClaimAuthority(started)
	if authorityErr != nil || authority.GoalID == "" || authority.ObligationRevision == 0 {
		return publishCadenceFailure(result, deps, claim.Claim.Opid, trunk, trigger, key, started, input.DeepOnlyGroups, probes, "standing cadence authority is unavailable")
	}
	run, runErr := deps.Run(CadenceRunRequest{Trunk: trunk, Authority: authority, WeightGeneration: input.Weight.Generation, ForceAttempt: true, ForceGroups: forceGroups})
	result.Executed = true
	ended := deps.Clock().UTC().Truncate(time.Second)
	if ended.Before(started) {
		ended = started
	}
	if runErr != nil || run.RunID == "" || run.Result.AttemptID == "" {
		result, err = publishCadenceFailure(result, deps, claim.Claim.Opid, trunk, trigger, key, started, input.DeepOnlyGroups, probes, "cadence runner did not return a terminal result")
	} else {
		status := cadenceStatus(trunk, trigger, run.RunID, run.Result, started, ended, key, input.DeepOnlyGroups, probes)
		red := trunkredmap.RedGroupsToRecordGroups(trunkredmap.ResultToRedGroups(run.Result))
		if cadenceRunGreen(run.Result) && input.WeightDue {
			if deps.DischargeWeight == nil || deps.DischargeWeight(authority, run.RunID, input.Weight.Generation, ended) != nil {
				failed := unavailableCadenceResult(input.DeepOnlyGroups, probes, "authorized weight discharge was refused")
				failed.AttemptID = run.Result.AttemptID
				status = cadenceStatus(trunk, trigger, run.RunID, failed, started, ended, key, input.DeepOnlyGroups, probes)
				red = trunkredmap.RedGroupsToRecordGroups(trunkredmap.ResultToRedGroups(failed))
			}
		}
		err = deps.Ledger.Publish(ended, claim.Claim.Opid, status, red)
		if err == nil {
			result.Status, result.Published = &status, true
		}
	}
	releaseErr := deps.ReleaseAuthority(authority, ended)
	if err == nil {
		err = releaseErr
	}
	return result, err
}

func cadenceTrigger(now time.Time, latest *goal.CadenceStatus, weightDue bool, ids []string, probes map[string]proofrun.GroupResult) (goal.CadenceTrigger, bool, string, bool, error) {
	window := now.Format(time.RFC3339)
	forced := latest == nil
	if latest != nil {
		start, err := time.Parse(time.RFC3339, latest.ForcedWindowStart)
		if err != nil {
			return "", false, "", false, err
		}
		window = start.UTC().Format(time.RFC3339)
		if !now.Before(start.Add(CadenceForcedInterval)) {
			forced, window = true, now.Format(time.RFC3339)
		}
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
		if group.ExecutionIdentity != probes[id].ExecutionIdentity {
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

func publishCadenceFailure(result CadenceTickResult, deps CadenceDependencies, claim string, trunk CadenceTrunk, trigger goal.CadenceTrigger, key goal.CadenceClaimKey, started time.Time, ids []string, probes map[string]proofrun.GroupResult, reason string) (CadenceTickResult, error) {
	failed := unavailableCadenceResult(ids, probes, reason)
	status := cadenceStatus(trunk, trigger, "cadence-unavailable", failed, started, started, key, ids, probes)
	red := trunkredmap.RedGroupsToRecordGroups(trunkredmap.ResultToRedGroups(failed))
	err := deps.Ledger.Publish(started, claim, status, red)
	if err == nil {
		result.Status, result.Published = &status, true
	}
	return result, err
}

func unavailableCadenceResult(ids []string, probes map[string]proofrun.GroupResult, reason string) proofrun.TestResult {
	result := proofrun.TestResult{AttemptID: "cadence-unavailable"}
	for _, id := range ids {
		result.Groups = append(result.Groups, proofrun.GroupResult{ID: id, Kind: "cadence", InputManifest: []string{"cadence-authority"},
			ExecutionIdentity: probes[id].ExecutionIdentity, Status: "unavailable", NotRunReason: reason})
	}
	return result
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
