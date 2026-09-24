package goal

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

type CadenceTrigger string

const (
	CadenceTriggerIdentityChanged CadenceTrigger = "identity-changed"
	CadenceTriggerWeightDue       CadenceTrigger = "weight-due"
	CadenceTriggerForcedWindow    CadenceTrigger = "forced-window"
	CadenceTriggerRevalidation    CadenceTrigger = "tip-revalidation"
)

// CadenceClaimKey identifies one cadence decision across every owner machine.
type CadenceClaimKey struct {
	TrunkTree         string `json:"trunkTree"`
	WeightGeneration  uint64 `json:"weightGeneration"`
	ForcedWindowStart string `json:"forcedWindowStart"`
}

type CadenceGroupStatus struct {
	Group             string `json:"group"`
	ExecutionIdentity string `json:"executionIdentity"`
	Status            string `json:"status"`
	EvidenceDigest    string `json:"evidenceDigest"`
	ReuseSource       string `json:"reuseSource,omitempty"`
}

// CadenceStatus is the latest terminal observation for the deep-only groups.
type CadenceStatus struct {
	TrunkCommit       string               `json:"trunkCommit"`
	TrunkTree         string               `json:"trunkTree"`
	Trigger           CadenceTrigger       `json:"trigger"`
	RunID             string               `json:"runId"`
	AttemptID         string               `json:"attemptId"`
	StartedAt         string               `json:"startedAt"`
	EndedAt           string               `json:"endedAt"`
	WeightGeneration  uint64               `json:"weightGeneration"`
	ForcedWindowStart string               `json:"forcedWindowStart"`
	Groups            []CadenceGroupStatus `json:"groups"`
	Opid              string               `json:"opid"`
}

func (status CadenceStatus) Key() CadenceClaimKey {
	return CadenceClaimKey{TrunkTree: status.TrunkTree, WeightGeneration: status.WeightGeneration, ForcedWindowStart: status.ForcedWindowStart}
}

// Green reports whether every recorded deep-only group has usable evidence.
func (status CadenceStatus) Green() bool {
	if len(status.Groups) == 0 {
		return false
	}
	for _, group := range status.Groups {
		if group.Status != "passed" && group.Status != "reused" {
			return false
		}
	}
	return true
}

type CadenceClaim struct {
	Key          CadenceClaimKey `json:"key"`
	OwnerMachine string          `json:"ownerMachine"`
	Opid         string          `json:"opid"`
	ClaimedAt    string          `json:"claimedAt"`
	LeaseUntil   string          `json:"leaseUntil"`
}

type CadenceClaimOutcome string

const (
	CadenceClaimAcquired CadenceClaimOutcome = "acquired"
	CadenceClaimJoined   CadenceClaimOutcome = "joined"
	CadenceClaimComplete CadenceClaimOutcome = "complete"
	CadenceClaimOccupied CadenceClaimOutcome = "occupied"
)

type CadenceClaimResult struct {
	Outcome CadenceClaimOutcome
	Claim   *CadenceClaim
	Status  *CadenceStatus
	Tip     string
}

type CadencePublishArgs struct {
	ClaimOpid string
	Status    CadenceStatus
	Groups    []TrunkRedRecordGroup
}

// ClaimCadence acquires, joins, or reads the terminal result for a shared key.
// VerbRequest.Now is the only clock used to decide lease liveness.
func ClaimCadence(r VerbRequest, key CadenceClaimKey, lease time.Duration) (CadenceClaimResult, error) {
	if err := validateCadenceKey(key); err != nil {
		return CadenceClaimResult{}, err
	}
	if lease <= 0 {
		return CadenceClaimResult{}, fmt.Errorf("cadence claim lease must be positive")
	}
	now := r.Now.UTC().Truncate(time.Second)
	claim := &CadenceClaim{Key: key, OwnerMachine: r.Actor.Machine, Opid: r.opid(),
		ClaimedAt: now.Format(time.RFC3339), LeaseUntil: now.Add(lease).Format(time.RFC3339)}
	result, err := RecordTrunkRed(r, TrunkRedRecordArgs{CadenceClaim: claim})
	if err != nil {
		return CadenceClaimResult{}, err
	}
	tree, err := loadTreeFor(r.Endpoint, result.Tip)
	if err != nil {
		return CadenceClaimResult{}, err
	}
	return classifyCadenceClaim(tree, key, r.opid(), r.Now, result.Tip)
}

// PublishCadence atomically replaces the claim with terminal status and
// records every supplied red group through the trunk-red record mapping.
func PublishCadence(r VerbRequest, args CadencePublishArgs) (PublishResult, error) {
	status := cleanCadenceStatus(args.Status)
	status.Opid = r.opid()
	if err := validateCadenceStatus(&status); err != nil {
		return PublishResult{}, err
	}
	return RecordTrunkRed(r, TrunkRedRecordArgs{Attempt: status.AttemptID, BaseCommit: status.TrunkCommit,
		BaseTree: status.TrunkTree, SeenAt: status.EndedAt, Cadence: &status, CadenceClaimOpid: args.ClaimOpid, Groups: args.Groups})
}

func classifyCadenceClaim(tree *TreeGoals, key CadenceClaimKey, opid string, now time.Time, tip string) (CadenceClaimResult, error) {
	if tree.Cadence != nil && tree.Cadence.Key() == key {
		status := *tree.Cadence
		return CadenceClaimResult{Outcome: CadenceClaimComplete, Status: &status, Tip: tip}, nil
	}
	if tree.CadenceClaim == nil {
		return CadenceClaimResult{}, fmt.Errorf("cadence claim transaction left no claim or terminal result")
	}
	claim := *tree.CadenceClaim
	leaseUntil, err := time.Parse(time.RFC3339, claim.LeaseUntil)
	if err != nil {
		return CadenceClaimResult{}, err
	}
	if !now.UTC().Before(leaseUntil) {
		return CadenceClaimResult{}, fmt.Errorf("cadence claim transaction left an expired claim")
	}
	outcome := CadenceClaimOccupied
	if claim.Key == key {
		outcome = CadenceClaimJoined
		if claim.Opid == opid {
			outcome = CadenceClaimAcquired
		}
	}
	return CadenceClaimResult{Outcome: outcome, Claim: &claim, Tip: tip}, nil
}

func applyCadenceRecord(tree *TreeGoals, r VerbRequest, args TrunkRedRecordArgs) (handled, already bool, err error) {
	if args.CadenceClaim != nil {
		if args.Cadence != nil || len(args.Groups) != 0 || args.CadenceClaimOpid != "" {
			return true, false, fmt.Errorf("cadence claim cannot publish status or red groups")
		}
		if err := validateCadenceClaim(args.CadenceClaim); err != nil {
			return true, false, err
		}
		if tree.Cadence != nil && tree.Cadence.Key() == args.CadenceClaim.Key {
			return true, false, LostToCompetitor{Winner: tree.Cadence.Opid}
		}
		if current := tree.CadenceClaim; current != nil {
			if current.Opid == r.opid() {
				return true, true, nil
			}
			leaseUntil, parseErr := time.Parse(time.RFC3339, current.LeaseUntil)
			if parseErr != nil {
				return true, false, parseErr
			}
			if r.Now.UTC().Before(leaseUntil) {
				return true, false, LostToCompetitor{Winner: current.Opid}
			}
		}
		claim := *args.CadenceClaim
		tree.CadenceClaim = &claim
		return true, false, nil
	}
	if args.Cadence == nil {
		return false, false, nil
	}
	if args.Batch != "" || args.OwnerMachine != "" {
		return false, false, fmt.Errorf("cadence publication cannot assign a batch or owner")
	}
	if err := validateCadenceStatus(args.Cadence); err != nil {
		return false, false, err
	}
	if tree.Cadence != nil && tree.Cadence.Opid == r.opid() {
		return false, true, nil
	}
	if tree.CadenceClaim == nil || tree.CadenceClaim.Opid != args.CadenceClaimOpid {
		return false, false, fmt.Errorf("cadence publication does not own the active claim")
	}
	if tree.CadenceClaim.OwnerMachine != r.Actor.Machine || tree.CadenceClaim.Key != args.Cadence.Key() {
		return false, false, fmt.Errorf("cadence publication does not match the active claim")
	}
	status := cleanCadenceStatus(*args.Cadence)
	tree.Cadence = &status
	tree.CadenceClaim = nil
	return false, false, nil
}

func validateCadenceClaim(claim *CadenceClaim) error {
	if claim == nil || claim.OwnerMachine == "" || !validOpidShape(claim.Opid) || !validTrunkRedTime(claim.ClaimedAt) || !validTrunkRedTime(claim.LeaseUntil) {
		return fmt.Errorf("cadence claim is incomplete")
	}
	if err := validateCadenceKey(claim.Key); err != nil {
		return err
	}
	claimed, _ := time.Parse(time.RFC3339, claim.ClaimedAt)
	until, _ := time.Parse(time.RFC3339, claim.LeaseUntil)
	if !until.After(claimed) {
		return fmt.Errorf("cadence claim lease must end after it starts")
	}
	return nil
}

func validateCadenceStatus(status *CadenceStatus) error {
	if status == nil || !validCadenceObjectID(status.TrunkCommit) ||
		!validTrunkRedTime(status.StartedAt) || !validTrunkRedTime(status.EndedAt) || !validOpidShape(status.Opid) || len(status.Groups) == 0 {
		return fmt.Errorf("cadence status is incomplete")
	}
	if status.Trigger != CadenceTriggerIdentityChanged && status.Trigger != CadenceTriggerWeightDue && status.Trigger != CadenceTriggerForcedWindow && status.Trigger != CadenceTriggerRevalidation {
		return fmt.Errorf("cadence status has unknown trigger %q", status.Trigger)
	}
	if status.Trigger == CadenceTriggerRevalidation {
		if status.RunID != "" || status.AttemptID != "" {
			return fmt.Errorf("cadence revalidation invents a run or attempt")
		}
	} else if status.RunID == "" || status.AttemptID == "" {
		return fmt.Errorf("cadence execution status has no run or attempt")
	}
	if err := validateCadenceKey(status.Key()); err != nil {
		return err
	}
	started, _ := time.Parse(time.RFC3339, status.StartedAt)
	ended, _ := time.Parse(time.RFC3339, status.EndedAt)
	if ended.Before(started) {
		return fmt.Errorf("cadence status ends before it starts")
	}
	seen := map[string]bool{}
	for _, group := range status.Groups {
		if group.Group == "" || seen[group.Group] || !validCadenceDigest(group.ExecutionIdentity) || !validCadenceDigest(group.EvidenceDigest) {
			return fmt.Errorf("cadence status group inventory is incomplete or duplicated")
		}
		seen[group.Group] = true
		switch group.Status {
		case "passed":
			if group.ReuseSource != "" {
				return fmt.Errorf("passed cadence group %s names a reuse source", group.Group)
			}
		case "reused":
			if group.ReuseSource == "" {
				return fmt.Errorf("reused cadence group %s has no source", group.Group)
			}
		case "failed", "invalid", "unavailable", "cancelled", "runaway", "dead", "not-run", "blocked":
			if group.ReuseSource != "" {
				return fmt.Errorf("non-green cadence group %s names a reuse source", group.Group)
			}
		default:
			return fmt.Errorf("cadence group %s has invalid status %q", group.Group, group.Status)
		}
	}
	if strings.ContainsAny(status.RunID+status.AttemptID, "\r\n") {
		return fmt.Errorf("cadence status identifiers contain a line break")
	}
	return nil
}

func validateCadenceKey(key CadenceClaimKey) error {
	if !validCadenceObjectID(key.TrunkTree) || !validTrunkRedTime(key.ForcedWindowStart) {
		return fmt.Errorf("cadence claim key is incomplete")
	}
	return nil
}

func validCadenceObjectID(value string) bool {
	if len(value) != 40 && len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func validCadenceDigest(value string) bool {
	if len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func cleanCadenceStatus(status CadenceStatus) CadenceStatus {
	status.Groups = append([]CadenceGroupStatus(nil), status.Groups...)
	sort.Slice(status.Groups, func(i, j int) bool { return status.Groups[i].Group < status.Groups[j].Group })
	return status
}

func renderTrunkRedState(entries []TrunkRedEntry, cadence *CadenceStatus, claim *CadenceClaim) []byte {
	copyEntries := make([]TrunkRedEntry, len(entries))
	for index := range entries {
		copyEntries[index] = cleanTrunkRedEntry(entries[index])
	}
	sort.Slice(copyEntries, func(i, j int) bool {
		if copyEntries[i].Opened == copyEntries[j].Opened {
			return copyEntries[i].ID < copyEntries[j].ID
		}
		return copyEntries[i].Opened < copyEntries[j].Opened
	})
	if cadence != nil {
		copyStatus := cleanCadenceStatus(*cadence)
		cadence = &copyStatus
	}
	data, _ := json.MarshalIndent(trunkRedFile{Schema: 1, Entries: copyEntries, Cadence: cadence, CadenceClaim: claim}, "", "  ")
	return append(data, '\n')
}

func trunkRedCadenceFields(data []byte) (*CadenceStatus, *CadenceClaim) {
	var file trunkRedFile
	_ = json.Unmarshal(data, &file)
	return file.Cadence, file.CadenceClaim
}
