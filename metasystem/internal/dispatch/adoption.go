package dispatch

import (
	"fmt"
	"sort"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/census"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

type ReconciliationOutcome string

const (
	ReconciliationAdopted          ReconciliationOutcome = "ADOPTED"
	ReconciliationCreatorAbandoned ReconciliationOutcome = "CREATOR-ABANDONED"
	ReconciliationDeferred         ReconciliationOutcome = "DEFERRED"
	ReconciliationRecordAdvanced   ReconciliationOutcome = "RECORD-ADVANCED"
)

type ReconciliationResult struct {
	Outcome      ReconciliationOutcome
	Reason       string
	RecordStatus string
	PrimaryPID   int64
	Leaderless   bool
	TaggedCount  int
	UnknownCount int
}

type ReconciliationDependencies struct {
	Now     func() time.Time
	Scanner census.TaggedProcessScanner
	Creator identity.Prober
	Emit    func(string)
}

// ReconcileReservation resolves an identityless reservation from a nonce-wide
// process census. The slow census runs outside all record and session locks;
// the chosen action is revalidated and published under the shared transaction.
func ReconcileReservation(root, job string, dependencies ReconciliationDependencies) (ReconciliationResult, error) {
	if dependencies.Now == nil {
		dependencies.Now = time.Now
	}
	if dependencies.Scanner == nil {
		dependencies.Scanner = unavailableTaggedProcessScanner{}
	}
	if dependencies.Creator == nil {
		dependencies.Creator = identity.KernelProber{}
	}

	var snapshot map[string]any
	err := withRecordSessionLock(root, job, func(recordPath string, _ *SessionIndexTransaction) error {
		record, readErr := readObject(recordPath)
		if readErr != nil {
			return readErr
		}
		snapshot = record
		return nil
	})
	if err != nil {
		return ReconciliationResult{}, fmt.Errorf("dispatch: read reservation for reconciliation: %w", err)
	}
	status := asString(snapshot["status"])
	if status != "pending-setup" && status != "pending" {
		return ReconciliationResult{Outcome: ReconciliationRecordAdvanced, RecordStatus: status}, nil
	}
	if pid, hasPID := numInt(snapshot["pid"]); hasPID && pid > 0 {
		return ReconciliationResult{Outcome: ReconciliationRecordAdvanced, RecordStatus: status, PrimaryPID: pid}, nil
	}
	tag := asString(snapshot["instanceTag"])
	if tag == "" {
		return ReconciliationResult{}, fmt.Errorf("dispatch: identityless reservation %s has no instance tag", job)
	}

	var reservationCreatedAt time.Time
	if parsed, parseErr := parseRecordTime(asString(snapshot["createdAt"])); parseErr == nil {
		reservationCreatedAt = parsed
	}
	observed := dependencies.Scanner.ScanTag(tag, reservationCreatedAt)
	action := chooseReconciliation(snapshot, observed, dependencies.Creator)
	action.TaggedCount = len(observed.Tagged)
	action.UnknownCount = observed.UnknownWithinUniverse()
	now := dependencies.Now().UTC().Truncate(time.Second)

	err = withRecordSessionLock(root, job, func(recordPath string, transaction *SessionIndexTransaction) error {
		record, readErr := readObject(recordPath)
		if readErr != nil {
			return readErr
		}
		currentStatus := asString(record["status"])
		pid, hasPID := numInt(record["pid"])
		if asString(record["instanceTag"]) != tag ||
			(currentStatus != "pending-setup" && currentStatus != "pending") ||
			(hasPID && pid > 0) {
			action = ReconciliationResult{
				Outcome: ReconciliationRecordAdvanced, RecordStatus: currentStatus, PrimaryPID: pid,
			}
			return nil
		}

		evidence := reconciliationEvidence(action, observed, now)
		record["reconciliation"] = evidence
		record["reconciliationHandoff"] = evidence
		switch action.Outcome {
		case ReconciliationDeferred:
			// The reservation remains busy and retryable. Persisting the
			// unknown list prevents a later reader from mistaking deferral for
			// an empty census.
		case ReconciliationAdopted:
			primary, others := adoptedProcesses(observed.Tagged)
			if action.PrimaryPID != primary.PID {
				return fmt.Errorf("dispatch: reconciliation primary changed while applying")
			}
			for key, value := range exactIdentityFields(primary.Identity.Ref()) {
				record[key] = value
			}
			record["pgid"] = primary.PGID
			record["status"] = "pending"
			record["phase"] = "reconciliation"
			record["leaderless"] = action.Leaderless
			proof := exactIdentityFields(primary.Identity.Ref())
			proof["pgid"] = primary.PGID
			proof["instanceTag"] = tag
			proof["provenAt"] = now.Format(time.RFC3339)
			proof["source"] = "nonce-global-adoption"
			record["ownershipProof"] = proof
			custody := make([]any, 0, len(others))
			for _, process := range others {
				entry := exactIdentityFields(process.Identity.Ref())
				entry["pgid"] = process.PGID
				entry["instanceTag"] = tag
				custody = append(custody, entry)
			}
			record["custodyProcesses"] = custody
			action.RecordStatus = "pending"
		case ReconciliationCreatorAbandoned:
			record["status"] = "failed"
			record["phase"] = "reconciliation"
			record["error"] = "creator-abandoned"
			record["endedAt"] = now.Format(time.RFC3339)
			action.RecordStatus = "failed"
		}
		if err := writeRecord(recordPath, record); err != nil {
			return err
		}
		return transaction.syncRecord(job, record)
	})
	if err != nil {
		return ReconciliationResult{}, fmt.Errorf("dispatch: publish reconciliation for %s: %w", job, err)
	}
	if action.Outcome == ReconciliationDeferred && dependencies.Emit != nil {
		dependencies.Emit(fmt.Sprintf("REAP-DEFERRED job=%s reconciliation=%s tagged=%d unknown=%d foreign=%d excludedByAge=%d",
			job, action.Reason, action.TaggedCount, action.UnknownCount,
			observed.ForeignCount(), observed.ExcludedByAgeCount()))
	}
	return action, nil
}

type unavailableTaggedProcessScanner struct{}

func (unavailableTaggedProcessScanner) ScanTag(string, time.Time) census.TaggedProcessCensus {
	return census.TaggedProcessCensus{EnumerationError: "tag-position scanner is unavailable"}
}

func chooseReconciliation(record map[string]any, observed census.TaggedProcessCensus, creator identity.Prober) ReconciliationResult {
	if !observed.Complete() {
		reason := "unknown-process-observations"
		if observed.EnumerationError != "" {
			reason = "process-enumeration-unavailable"
		}
		return ReconciliationResult{Outcome: ReconciliationDeferred, Reason: reason}
	}
	leaders := 0
	var leader census.TaggedProcess
	for _, process := range observed.Tagged {
		if process.PID == process.PGID {
			leaders++
			leader = process
		}
	}
	if leaders > 1 {
		return ReconciliationResult{Outcome: ReconciliationDeferred, Reason: "multiple-tagged-leaders"}
	}
	if leaders == 1 {
		return ReconciliationResult{
			Outcome: ReconciliationAdopted, Reason: "exactly-one-tagged-leader", PrimaryPID: leader.PID,
		}
	}
	if len(observed.Tagged) > 0 {
		ordered := append([]census.TaggedProcess(nil), observed.Tagged...)
		sort.SliceStable(ordered, func(i, j int) bool {
			if ordered[i].Identity.StartedAt.Equal(ordered[j].Identity.StartedAt) {
				return ordered[i].PID < ordered[j].PID
			}
			return ordered[i].Identity.StartedAt.Before(ordered[j].Identity.StartedAt)
		})
		return ReconciliationResult{
			Outcome: ReconciliationAdopted, Reason: "eldest-tagged-survivor", PrimaryPID: ordered[0].PID, Leaderless: true,
		}
	}
	creatorObject, ok := record["creatorLiveness"].(map[string]any)
	if !ok {
		return ReconciliationResult{Outcome: ReconciliationDeferred, Reason: "creator-breadcrumb-missing"}
	}
	ref, valid := identityRefFromObject(creatorObject)
	if !valid || !ref.NativeExact() {
		return ReconciliationResult{Outcome: ReconciliationDeferred, Reason: "creator-breadcrumb-unprovable"}
	}
	switch recordedRefLiveness(creator, ref) {
	case identity.Dead:
		return ReconciliationResult{Outcome: ReconciliationCreatorAbandoned, Reason: "creator-identity-proven-dead"}
	case identity.Alive:
		return ReconciliationResult{Outcome: ReconciliationDeferred, Reason: "complete-census-absence-creator-alive"}
	default:
		return ReconciliationResult{Outcome: ReconciliationDeferred, Reason: "creator-identity-unreadable"}
	}
}

func adoptedProcesses(processes []census.TaggedProcess) (census.TaggedProcess, []census.TaggedProcess) {
	ordered := append([]census.TaggedProcess(nil), processes...)
	sort.SliceStable(ordered, func(i, j int) bool {
		leftLeader := ordered[i].PID == ordered[i].PGID
		rightLeader := ordered[j].PID == ordered[j].PGID
		if leftLeader != rightLeader {
			return leftLeader
		}
		if ordered[i].Identity.StartedAt.Equal(ordered[j].Identity.StartedAt) {
			return ordered[i].PID < ordered[j].PID
		}
		return ordered[i].Identity.StartedAt.Before(ordered[j].Identity.StartedAt)
	})
	return ordered[0], ordered[1:]
}

func reconciliationEvidence(result ReconciliationResult, observed census.TaggedProcessCensus, now time.Time) map[string]any {
	unknown := make([]any, 0, len(observed.Indeterminate))
	for _, process := range observed.Indeterminate {
		unknown = append(unknown, map[string]any{
			"pid": process.PID, "pgid": process.PGID, "reason": process.Reason,
			"universe": process.Universe.String(),
		})
	}
	return map[string]any{
		"resolvedAt":       now.Format(time.RFC3339),
		"outcome":          string(result.Outcome),
		"reason":           result.Reason,
		"taggedCount":      len(observed.Tagged),
		"unknownProcesses": unknown,
		"enumerationError": nullableString(observed.EnumerationError),
		"leaderless":       result.Leaderless,
	}
}
