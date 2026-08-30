package dispatch

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

type ClaimOutcome string

const (
	ClaimWON                     ClaimOutcome = "WON"
	ClaimInProgress              ClaimOutcome = "IN-PROGRESS"
	ClaimBound                   ClaimOutcome = "BOUND"
	ClaimReconciling             ClaimOutcome = "RECONCILING"
	ClaimRefusedOpIDMismatch     ClaimOutcome = "REFUSED-OPID-MISMATCH"
	ClaimRefusedUnprovableLegacy ClaimOutcome = "REFUSED-UNPROVABLE-LEGACY"
	ClaimRefusedSessionBusy      ClaimOutcome = "REFUSED-SESSION-BUSY"
	ClaimRefusedUnprovable       ClaimOutcome = "REFUSED-UNPROVABLE"
)

const (
	ClaimWaitAttempts = 40
	ClaimWaitReads    = ClaimWaitAttempts + 1
	ClaimWaitInterval = 15 * time.Second
)

type ClaimResult struct {
	Outcome  ClaimOutcome   `json:"outcome"`
	Evidence map[string]any `json:"evidence"`
}

type ClaimLaunchParams struct {
	Root                 string
	OpID                 string
	Request              LaunchFingerprintRequest
	MainID               string
	ClaimEpoch           string
	GoalID               string
	GoalRevision         uint64
	MachineID            string
	ApprovedRef          string
	DefaultCapMinutes    int64
	Wait                 bool
	OccupancyPreparation *SessionOccupancyPreparation
}

type claimReservationProvenance struct {
	mainID       any
	claimEpoch   any
	goalID       any
	goalRevision any
	machineID    any
	approvedRef  any
}

type ClaimProcessVerifier interface {
	Verify(pid int64, instanceTag string) identity.Verification
}

type ClaimLaunchDependencies struct {
	Now             func() time.Time
	Sleep           func(time.Duration)
	CreatorPID      int64
	IdentityReader  identity.StartReader
	ProcessVerifier ClaimProcessVerifier
	Occupancy       SessionOccupancyReader
	Nonce           func() (string, error)
	// Reconcile runs the nonce-global adoption engine after a same-operation
	// read reaches RECONCILING. The command boundary binds production census;
	// tests bind a deterministic process table.
	Reconcile func(root, job string) (ReconciliationResult, error)
}

type unavailableClaimProcessVerifier struct{}

func (unavailableClaimProcessVerifier) Verify(int64, string) identity.Verification {
	return identity.Verification{Outcome: identity.VerificationIndeterminate, Presence: identity.Unknown}
}

// ClaimLaunch resolves an operation id before consulting session occupancy.
// Waiting is optional so callers can either observe IN-PROGRESS or use the
// pinned ten-minute waiter. The waiter counts its first immediate read as
// attempt one and never consults a wall-clock deadline.
func ClaimLaunch(params ClaimLaunchParams, dependencies ClaimLaunchDependencies) (ClaimResult, error) {
	if !validJobID.MatchString(params.OpID) {
		return ClaimResult{}, fmt.Errorf("claim-launch opid must be a valid job id")
	}
	fingerprint, err := CanonicalizeLaunchFingerprint(params.Root, params.Request, params.DefaultCapMinutes)
	if err != nil {
		return ClaimResult{}, err
	}
	claimEpoch, err := nullableEpoch(params.ClaimEpoch)
	if err != nil {
		return ClaimResult{}, err
	}
	goalRevision, err := nullableGoalRevision(params.GoalID, params.GoalRevision)
	if err != nil {
		return ClaimResult{}, err
	}
	provenance := claimReservationProvenance{
		mainID: nullableString(params.MainID), claimEpoch: claimEpoch, goalID: nullableString(params.GoalID),
		goalRevision: goalRevision, machineID: nullableString(params.MachineID), approvedRef: nullableString(params.ApprovedRef),
	}
	dependencies = claimDependenciesWithDefaults(dependencies)
	if !params.Wait {
		result, err := claimLaunchAttempt(params, fingerprint, provenance, dependencies, 1, false)
		return finishClaimReconciliation(params, fingerprint, dependencies, result, err)
	}
	for attempt := 1; attempt <= ClaimWaitReads; attempt++ {
		lastRead := attempt == ClaimWaitReads
		result, err := claimLaunchAttempt(params, fingerprint, provenance, dependencies, attempt, lastRead)
		if err != nil || result.Outcome != ClaimInProgress {
			return finishClaimReconciliation(params, fingerprint, dependencies, result, err)
		}
		if lastRead {
			return finishClaimReconciliation(params, fingerprint, dependencies, result, err)
		}
		dependencies.Sleep(ClaimWaitInterval)
	}
	panic("claim-launch waiter exhausted an unreachable loop")
}

func finishClaimReconciliation(params ClaimLaunchParams, fingerprint LaunchFingerprint, dependencies ClaimLaunchDependencies, result ClaimResult, err error) (ClaimResult, error) {
	if err != nil || result.Outcome != ClaimReconciling || dependencies.Reconcile == nil {
		return result, err
	}
	reconciled, err := dependencies.Reconcile(params.Root, params.OpID)
	if err != nil {
		return ClaimResult{}, err
	}
	evidence := map[string]any{
		"resolution":     "same-opid-reconciliation",
		"reconciliation": string(reconciled.Outcome),
		"reason":         reconciled.Reason,
		"recordedStatus": reconciled.RecordStatus,
		"pid":            reconciled.PrimaryPID,
		"leaderless":     reconciled.Leaderless,
		"taggedCount":    reconciled.TaggedCount,
		"unknownCount":   reconciled.UnknownCount,
	}
	switch reconciled.Outcome {
	case ReconciliationAdopted:
		return claimResult(ClaimBound, fingerprint, evidence), nil
	case ReconciliationCreatorAbandoned:
		return claimResult(ClaimOutcome("REPLAYED-failed"), fingerprint, evidence), nil
	case ReconciliationDeferred:
		return claimResult(ClaimReconciling, fingerprint, evidence), nil
	default:
		return result, nil
	}
}

func claimDependenciesWithDefaults(dependencies ClaimLaunchDependencies) ClaimLaunchDependencies {
	if dependencies.Now == nil {
		dependencies.Now = time.Now
	}
	if dependencies.Sleep == nil {
		dependencies.Sleep = time.Sleep
	}
	if dependencies.CreatorPID < 1 {
		dependencies.CreatorPID = int64(os.Getpid())
	}
	if dependencies.IdentityReader == nil {
		dependencies.IdentityReader = identity.KernelProber{}
	}
	if dependencies.ProcessVerifier == nil {
		dependencies.ProcessVerifier = unavailableClaimProcessVerifier{}
	}
	if dependencies.Occupancy == nil {
		dependencies.Occupancy = IndexedSessionOccupancyReader{}
	}
	if dependencies.Nonce == nil {
		dependencies.Nonce = claimNonce
	}
	return dependencies
}

func claimNonce() (string, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("claim-launch cannot mint the reservation nonce: %w", err)
	}
	return hex.EncodeToString(raw), nil
}

func claimLaunchAttempt(params ClaimLaunchParams, fingerprint LaunchFingerprint, provenance claimReservationProvenance, dependencies ClaimLaunchDependencies, attempt int, reconcileIdentityless bool) (result ClaimResult, err error) {
	found := false
	err = withRecordSessionLock(params.Root, params.OpID, func(recordPath string, _ *SessionIndexTransaction) error {
		record, readErr := readObject(recordPath)
		if readErr == nil {
			found = true
			result, readErr = resolveSameOpID(params.OpID, recordPath, record, fingerprint, dependencies, attempt, reconcileIdentityless)
			return readErr
		}
		if !os.IsNotExist(readErr) {
			return fmt.Errorf("claim-launch cannot read standing opid %s: %w", params.OpID, readErr)
		}
		return nil
	})
	if err != nil || found {
		return result, err
	}

	var prepared SessionOccupancyPreparation
	if params.OccupancyPreparation != nil {
		prepared = *params.OccupancyPreparation
	} else {
		prepared, err = dependencies.Occupancy.Prepare(params.Root, fingerprint.Request.SessionKey)
		if err != nil {
			return ClaimResult{}, err
		}
	}
	err = dependencies.Occupancy.Resolve(params.Root, fingerprint.Request.SessionKey, params.OpID, prepared, func(occupancy SessionOccupancy, transaction *SessionIndexTransaction) error {
		return withRecordLock(params.Root, params.OpID, func(recordPath string) error {
			record, readErr := readObject(recordPath)
			if readErr == nil {
				result, readErr = resolveSameOpID(params.OpID, recordPath, record, fingerprint, dependencies, attempt, reconcileIdentityless)
				return readErr
			}
			if !os.IsNotExist(readErr) {
				return fmt.Errorf("claim-launch cannot read standing opid %s: %w", params.OpID, readErr)
			}

			if occupancy.Unprovable != nil {
				result = claimResult(ClaimRefusedUnprovable, fingerprint, map[string]any{
					"resolution":      "busy-gate-unprovable",
					"occupantOpid":    occupancy.Unprovable.OpID,
					"occupantStatus":  occupancy.Unprovable.Status,
					"occupancyReason": occupancy.Unprovable.Reason,
				})
				return nil
			}
			if occupancy.Busy != nil {
				result = claimResult(ClaimRefusedSessionBusy, fingerprint, map[string]any{
					"resolution":         "busy-gate",
					"occupantOpid":       occupancy.Busy.OpID,
					"occupantStatus":     occupancy.Busy.Status,
					"occupantProofLevel": occupancy.Busy.ProofLevel,
					"occupancyReason":    occupancy.Busy.Reason,
				})
				return nil
			}

			creator, state, creatorErr := dependencies.IdentityReader.ReadStart(dependencies.CreatorPID)
			if creatorErr != nil || state != identity.Alive || creator.Pid != dependencies.CreatorPID || !creator.Ref().NativeExact() {
				result = claimResult(ClaimRefusedUnprovable, fingerprint, map[string]any{
					"resolution": "creator-liveness-unprovable",
					"creatorPid": dependencies.CreatorPID,
				})
				return nil
			}
			nonce, nonceErr := dependencies.Nonce()
			if nonceErr != nil {
				return nonceErr
			}
			if nonce == "" || strings.ContainsAny(nonce, " /\\\t\r\n") {
				return fmt.Errorf("claim-launch nonce source returned an invalid tag suffix")
			}
			createdAt := dependencies.Now().UTC().Truncate(time.Second)
			instanceTag, tagErr := reservationInstanceTag(params.OpID, nonce)
			if tagErr != nil {
				return tagErr
			}
			creatorBreadcrumb := exactIdentityFields(creator.Ref())
			creatorBreadcrumb["recordedAt"] = createdAt.Format(time.RFC3339)
			record = claimReservationRecord(params.OpID, fingerprint, provenance, instanceTag, creatorBreadcrumb, occupancy.FreeEvidence, createdAt)
			record["sessionOccupancyHealing"] = healingObject(occupancy.Healing)
			generation, publishErr := transaction.publishBusy(classifySessionRecord(params.OpID, record))
			if publishErr != nil {
				return publishErr
			}
			if generation > 0 {
				record["sessionOccupancyClaimGeneration"] = generation
			}
			if writeErr := writeRecord(recordPath, record); writeErr != nil {
				return writeErr
			}
			result = claimResult(ClaimWON, fingerprint, map[string]any{
				"resolution":              "reservation-created",
				"recordPath":              recordPath,
				"instanceTag":             instanceTag,
				"creatorPid":              dependencies.CreatorPID,
				"reservationDeadline":     record["reservationDeadline"],
				"deadlinePurpose":         "wake-only",
				"freeSessionEvidence":     occupancy.FreeEvidence,
				"sessionOccupancyHealing": healingObject(occupancy.Healing),
			})
			return nil
		})
	})
	return result, err
}

func resolveSameOpID(opid, recordPath string, record map[string]any, fingerprint LaunchFingerprint, dependencies ClaimLaunchDependencies, attempt int, reconcileIdentityless bool) (ClaimResult, error) {
	base := map[string]any{
		"resolution": "same-opid",
		"recordPath": recordPath,
	}
	if asString(record["jobId"]) != opid {
		base["reason"] = "standing-record-job-id-does-not-match-opid"
		return claimResult(ClaimRefusedUnprovable, fingerprint, base), nil
	}
	recordedDigest, hasDigest := record["fingerprint"].(string)
	recordedVersion, hasVersion := numInt(record["fingerprintVersion"])
	if !hasDigest || recordedDigest == "" || !hasVersion {
		base["reason"] = "standing-record-has-no-versioned-fingerprint"
		return claimResult(ClaimRefusedUnprovableLegacy, fingerprint, base), nil
	}
	base["recordedFingerprint"] = recordedDigest
	base["recordedFingerprintVersion"] = recordedVersion
	if recordedVersion != int64(fingerprint.Version) || recordedDigest != fingerprint.Digest {
		base["reason"] = "fingerprint-equality-gate-failed"
		return claimResult(ClaimRefusedOpIDMismatch, fingerprint, base), nil
	}
	if asString(record["proofLevel"]) != "proven" {
		base["reason"] = "standing-record-does-not-carry-proven-custody"
		return claimResult(ClaimRefusedUnprovable, fingerprint, base), nil
	}
	status := asString(record["status"])
	base["recordedStatus"] = status
	if status == "reconciled-proven-absent" {
		base["recordedOutcome"] = recordedOutcome(record)
		return claimResult(ClaimOutcome("REPLAYED-"+status), fingerprint, base), nil
	}
	if TerminalStatus(status) {
		base["recordedOutcome"] = recordedOutcome(record)
		return claimResult(ClaimOutcome("REPLAYED-"+status), fingerprint, base), nil
	}
	pid, hasPID := numInt(record["pid"])
	if !hasPID || pid < 1 {
		if status != "pending-setup" && status != "pending" {
			base["reason"] = "non-reservation-record-has-no-process-identity"
			return claimResult(ClaimRefusedUnprovable, fingerprint, base), nil
		}
		if reconcileIdentityless {
			handoff, err := recordReconciliationHandoff(recordPath, record, dependencies.Now(), "waiter-attempts-exhausted")
			if err != nil {
				return ClaimResult{}, err
			}
			base["reconciliation"] = handoff
			base["waitAttempt"] = attempt
			base["waitAttempts"] = attempt
			base["waitRetries"] = ClaimWaitAttempts
			base["waitIntervalSeconds"] = int64(ClaimWaitInterval / time.Second)
			return claimResult(ClaimReconciling, fingerprint, base), nil
		}
		base["waitAttempt"] = attempt
		base["waitAttempts"] = ClaimWaitReads
		base["waitRetries"] = ClaimWaitAttempts
		base["waitIntervalSeconds"] = int64(ClaimWaitInterval / time.Second)
		base["reservationDeadline"] = record["reservationDeadline"]
		base["deadlinePurpose"] = "wake-only"
		return claimResult(ClaimInProgress, fingerprint, base), nil
	}
	recordedIdentity, validIdentity := identityRefFromObject(record)
	if !validIdentity || !recordedIdentity.NativeExact() {
		base["reason"] = "recorded-process-identity-is-not-platform-exact"
		return claimResult(ClaimRefusedUnprovable, fingerprint, base), nil
	}
	instanceTag := asString(record["instanceTag"])
	if instanceTag == "" {
		base["reason"] = "recorded-process-has-no-instance-tag"
		return claimResult(ClaimRefusedUnprovable, fingerprint, base), nil
	}
	verification := dependencies.ProcessVerifier.Verify(pid, instanceTag)
	base["pid"] = pid
	base["verification"] = string(verification.Outcome)
	observed := verification.Identity.Ref()
	if verification.Presence == identity.Alive && observed.NativeExact() && !sameRecordedIdentity(recordedIdentity, observed) {
		handoff, err := recordReconciliationHandoff(recordPath, record, dependencies.Now(), "recorded-pid-recycled")
		if err != nil {
			return ClaimResult{}, err
		}
		base["reconciliation"] = handoff
		return claimResult(ClaimReconciling, fingerprint, base), nil
	}
	switch verification.Outcome {
	case identity.VerificationDead:
		handoff, err := recordReconciliationHandoff(recordPath, record, dependencies.Now(), "recorded-process-dead")
		if err != nil {
			return ClaimResult{}, err
		}
		base["reconciliation"] = handoff
		return claimResult(ClaimReconciling, fingerprint, base), nil
	case identity.VerificationVerified, identity.VerificationNotOurs:
		if verification.Outcome == identity.VerificationVerified && observed.NativeExact() && sameRecordedIdentity(recordedIdentity, observed) {
			base["identityMode"] = recordedIdentity.ModeName()
			return claimResult(ClaimBound, fingerprint, base), nil
		}
		base["reason"] = "live-recorded-process-lacks-positioned-instance-tag"
		return claimResult(ClaimRefusedUnprovable, fingerprint, base), nil
	default:
		base["reason"] = "recorded-process-verification-indeterminate"
		return claimResult(ClaimRefusedUnprovable, fingerprint, base), nil
	}
}

func claimReservationRecord(opid string, fingerprint LaunchFingerprint, provenance claimReservationProvenance, instanceTag string, creator map[string]any, freeEvidence []SessionOccupant, createdAt time.Time) map[string]any {
	request := fingerprint.Request
	roots := make([]any, 0, len(request.ProductRoots))
	for _, root := range request.ProductRoots {
		roots = append(roots, root)
	}
	occupancyEvidence := make([]any, 0, len(freeEvidence))
	for _, item := range freeEvidence {
		occupancyEvidence = append(occupancyEvidence, map[string]any{
			"opid": item.OpID, "status": item.Status, "proofLevel": item.ProofLevel, "reason": item.Reason,
		})
	}
	return map[string]any{
		"jobId":                      opid,
		"operationId":                opid,
		"mainId":                     provenance.mainID,
		"claimEpoch":                 provenance.claimEpoch,
		"goalId":                     provenance.goalID,
		"goalRevision":               provenance.goalRevision,
		"machineId":                  provenance.machineID,
		"approvedRef":                provenance.approvedRef,
		"proofLevel":                 "proven",
		"status":                     "pending-setup",
		"phase":                      "reservation",
		"error":                      nil,
		"sessionKey":                 request.SessionKey,
		"dispatchMode":               request.DispatchMode,
		"resumedSessionId":           request.ResumedSessionID,
		"runtime":                    request.Runtime,
		"canonicalModelKey":          request.CanonicalModelKey,
		"role":                       request.Role,
		"launchMode":                 request.LaunchMode,
		"permissionEnvelopeDigest":   request.PermissionEnvelopeDigest,
		"productRoots":               roots,
		"capMin":                     request.CapMinutes,
		"capRequest":                 map[string]any{"minutes": request.CapMinutes},
		"inputHash":                  request.InputHash,
		"fingerprintVersion":         fingerprint.Version,
		"fingerprint":                fingerprint.Digest,
		"createdAt":                  createdAt.Format(time.RFC3339),
		"reservationDeadline":        createdAt.Add(AbandonedSetupGrace).Format(time.RFC3339),
		"reservationDeadlinePurpose": "wake-only",
		"creatorLiveness":            creator,
		"instanceTag":                instanceTag,
		"pid":                        nil,
		"pidStartedAt":               nil,
		"pidStartedAtExactMicro":     nil,
		"pidStartTicks":              nil,
		"bootId":                     nil,
		"pgid":                       nil,
		"custodyProcesses":           []any{},
		"reconciliationHandoff":      nil,
		"sessionOccupancyEvidence":   occupancyEvidence,
	}
}

func recordReconciliationHandoff(recordPath string, record map[string]any, now time.Time, reason string) (map[string]any, error) {
	if existing, ok := record["reconciliationHandoff"].(map[string]any); ok {
		return existing, nil
	}
	handoff := map[string]any{
		"requestedAt":              now.UTC().Truncate(time.Second).Format(time.RFC3339),
		"reason":                   reason,
		"capability":               "nonce-global-adoption",
		"creatorBreadcrumbPresent": record["creatorLiveness"] != nil,
	}
	record["reconciliationHandoff"] = handoff
	if err := writeRecord(recordPath, record); err != nil {
		return nil, fmt.Errorf("claim-launch cannot record reconciliation hand-off: %w", err)
	}
	return handoff, nil
}

func recordedOutcome(record map[string]any) map[string]any {
	outcome := map[string]any{"status": record["status"]}
	for _, field := range []string{"error", "phase", "endedAt", "result", "usage"} {
		if value, present := record[field]; present {
			outcome[field] = value
		}
	}
	return outcome
}

func claimResult(outcome ClaimOutcome, fingerprint LaunchFingerprint, details map[string]any) ClaimResult {
	evidence := map[string]any{
		"fingerprintVersion": fingerprint.Version,
		"fingerprint":        fingerprint.Digest,
	}
	for key, value := range details {
		evidence[key] = value
	}
	return ClaimResult{Outcome: outcome, Evidence: evidence}
}

// ClaimOutcomeExitCode keeps domain decisions machine-readable while making
// shell composition fail closed on refusals and pause on non-terminal work.
func ClaimOutcomeExitCode(outcome ClaimOutcome) int {
	if strings.HasPrefix(string(outcome), "REFUSED-") {
		return 1
	}
	if outcome == ClaimInProgress || outcome == ClaimReconciling {
		return 3
	}
	return 0
}
