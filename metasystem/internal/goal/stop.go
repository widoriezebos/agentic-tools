package goal

// Breach-stop has two durable pieces. The accepted goal record owns the
// launch fence. The local stop batch owns cancellation progress so a crashed
// custodian can continue without guessing which jobs remain.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
)

type StopBatchState string

const (
	StopBatchOpen          StopBatchState = "OPEN"
	StopBatchComplete      StopBatchState = "COMPLETE"
	StopBatchIndeterminate StopBatchState = "INDETERMINATE"
)

// StopBatch is the resumable local cancellation ledger for one closed fence.
type StopBatch struct {
	StopID               string              `json:"stopId"`
	GoalID               string              `json:"goalId"`
	GoalRevision         uint64              `json:"goalRevision"`
	FenceEpoch           uint64              `json:"fenceEpoch"`
	CapabilityGeneration uint64              `json:"capabilityGeneration"`
	Machine              string              `json:"machineId"`
	ClaimEpoch           int64               `json:"claimEpoch"`
	Reason               string              `json:"reason"`
	State                StopBatchState      `json:"state"`
	OpenedAt             string              `json:"openedAt"`
	UpdatedAt            string              `json:"updatedAt"`
	CompletedAt          string              `json:"completedAt,omitempty"`
	Pass                 uint64              `json:"pass"`
	Pending              []string            `json:"pendingJobs"`
	Terminal             []string            `json:"terminalJobs"`
	Foreign              []string            `json:"foreignJobs,omitempty"`
	Observed             []StopJob           `json:"observedJobs"`
	CancelOutcomes       []StopOutcome       `json:"cancelOutcomes"`
	FiringEvidence       *StopFiringEvidence `json:"firingEvidence,omitempty"`
	Failure              string              `json:"failure,omitempty"`
}

// StopFiringEvidence preserves the elapsed decision that closed the fence.
// A nil value identifies a batch written before firing evidence was recorded.
type StopFiringEvidence struct {
	ElapsedUsed    string `json:"elapsedUsed"`
	AdmissionLimit string `json:"admissionLimit"`
	BreachBoundary string `json:"breachBoundary"`
	GracePercent   uint64 `json:"gracePercent"`
}

// StopJob is one exact job generation seen by the fixed-point scan.
type StopJob struct {
	JobID          string `json:"jobId"`
	OperationID    string `json:"operationId"`
	Machine        string `json:"machineId"`
	ClaimEpoch     int64  `json:"claimEpoch"`
	Status         string `json:"status"`
	Disposition    string `json:"disposition"`
	LastObservedAt string `json:"lastObservedAt"`
}

// StopOutcome records the durable result for a generation that no longer
// needs a local cancellation attempt.
type StopOutcome struct {
	JobID       string `json:"jobId"`
	OperationID string `json:"operationId"`
	Outcome     string `json:"outcome"`
	ObservedAt  string `json:"observedAt"`
}

func stopBatchPath(root, stopID string) (string, error) {
	if !safeStopID(stopID) {
		return "", fmt.Errorf("invalid stop id %q", stopID)
	}
	return filepath.Join(root, "artifacts", "agents", "goal-stops", stopID+".json"), nil
}

func safeStopID(value string) bool {
	if value == "" || len(value) > 180 {
		return false
	}
	for _, r := range value {
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') &&
			(r < '0' || r > '9') && r != '-' && r != '_' && r != '.' {
			return false
		}
	}
	return true
}

func validateStopBatch(batch StopBatch) error {
	if !safeStopID(batch.StopID) || !validId(batch.GoalID) || batch.GoalRevision == 0 ||
		batch.FenceEpoch == 0 || batch.CapabilityGeneration == 0 || batch.Machine == "" || batch.ClaimEpoch < 1 {
		return fmt.Errorf("stop batch has incomplete authority coordinates")
	}
	if batch.Reason != StopReasonElapsedLimit && batch.Reason != StopReasonCorruptOverLimit {
		return fmt.Errorf("stop batch has invalid reason %q", batch.Reason)
	}
	if batch.State != StopBatchOpen && batch.State != StopBatchComplete && batch.State != StopBatchIndeterminate {
		return fmt.Errorf("stop batch has invalid state %q", batch.State)
	}
	if !validStamp(batch.OpenedAt) || !validStamp(batch.UpdatedAt) {
		return fmt.Errorf("stop batch has invalid timestamps")
	}
	if batch.State == StopBatchComplete {
		if !validStamp(batch.CompletedAt) || len(batch.Pending) != 0 || batch.Failure != "" {
			return fmt.Errorf("complete stop batch contradicts its completion evidence")
		}
	} else if batch.CompletedAt != "" {
		return fmt.Errorf("non-complete stop batch carries completedAt")
	}
	if batch.FiringEvidence != nil {
		if batch.Reason != StopReasonElapsedLimit {
			return fmt.Errorf("only an elapsed-limit stop may carry firing evidence")
		}
		if err := validateStopFiringEvidence(*batch.FiringEvidence); err != nil {
			return err
		}
	}
	for _, observed := range batch.Observed {
		if !safeStopID(observed.JobID) || !safeStopID(observed.OperationID) || observed.Machine == "" ||
			observed.ClaimEpoch < 1 || observed.Status == "" || observed.Disposition == "" || !validStamp(observed.LastObservedAt) {
			return fmt.Errorf("stop batch has an incomplete observed job generation")
		}
	}
	for _, outcome := range batch.CancelOutcomes {
		if !safeStopID(outcome.JobID) || !safeStopID(outcome.OperationID) || outcome.Outcome == "" || !validStamp(outcome.ObservedAt) {
			return fmt.Errorf("stop batch has an incomplete cancellation outcome")
		}
	}
	return nil
}

func validateStopFiringEvidence(evidence StopFiringEvidence) error {
	used, usedErr := time.ParseDuration(evidence.ElapsedUsed)
	boundary, boundaryErr := time.ParseDuration(evidence.BreachBoundary)
	admission, admissionOK := ParseWorkingDuration(evidence.AdmissionLimit)
	if usedErr != nil || boundaryErr != nil || !admissionOK || admission <= 0 || boundary <= 0 {
		return fmt.Errorf("stop batch has malformed elapsed firing evidence")
	}
	expected, err := (Budget{ElapsedLimit: evidence.AdmissionLimit}).ElapsedBreachDuration(evidence.GracePercent)
	if err != nil || expected != boundary || used < boundary {
		return fmt.Errorf("stop batch elapsed firing evidence contradicts its boundary")
	}
	return nil
}

func sameStopFiringEvidence(left, right *StopFiringEvidence) bool {
	if left == nil || right == nil {
		return left == right
	}
	return *left == *right
}

func normalizeStopBatch(batch *StopBatch) {
	if batch.Pending == nil {
		batch.Pending = []string{}
	}
	if batch.Terminal == nil {
		batch.Terminal = []string{}
	}
	if batch.Observed == nil {
		batch.Observed = []StopJob{}
	}
	if batch.CancelOutcomes == nil {
		batch.CancelOutcomes = []StopOutcome{}
	}
}

// ReadStopBatch reads one exact batch with no permissive JSON tail or fields.
func ReadStopBatch(root, stopID string) (StopBatch, error) {
	path, err := stopBatchPath(root, stopID)
	if err != nil {
		return StopBatch{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return StopBatch{}, err
	}
	var batch StopBatch
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&batch); err != nil {
		return StopBatch{}, fmt.Errorf("stop batch %s is unreadable: %w", stopID, err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return StopBatch{}, fmt.Errorf("stop batch %s has trailing JSON", stopID)
	}
	if batch.StopID != stopID {
		return StopBatch{}, fmt.Errorf("stop batch %s names stopId %q", stopID, batch.StopID)
	}
	if err := validateStopBatch(batch); err != nil {
		return StopBatch{}, fmt.Errorf("stop batch %s: %w", stopID, err)
	}
	return batch, nil
}

// WriteStopBatch atomically publishes progress. COMPLETE is absorbing.
func WriteStopBatch(root string, batch StopBatch) error {
	normalizeStopBatch(&batch)
	if err := validateStopBatch(batch); err != nil {
		return err
	}
	path, err := stopBatchPath(root, batch.StopID)
	if err != nil {
		return err
	}
	if previous, readErr := ReadStopBatch(root, batch.StopID); readErr == nil {
		if !sameStopFiringEvidence(previous.FiringEvidence, batch.FiringEvidence) {
			return fmt.Errorf("stop batch %s firing evidence is immutable", batch.StopID)
		}
		if previous.State == StopBatchComplete {
			oldBytes, _ := json.Marshal(previous)
			newBytes, _ := json.Marshal(batch)
			if !bytes.Equal(oldBytes, newBytes) {
				return fmt.Errorf("stop batch %s is COMPLETE and immutable", batch.StopID)
			}
			return nil
		}
	} else if !os.IsNotExist(readErr) {
		return readErr
	}
	encoded, err := json.MarshalIndent(batch, "", "  ")
	if err != nil {
		return err
	}
	durable, err := atomicfile.WriteText(path, string(encoded)+"\n", root)
	if err != nil {
		return err
	}
	if !durable {
		return fmt.Errorf("stop batch %s was published but its crash durability is unknown", batch.StopID)
	}
	return nil
}

// VerifyStopBatchComplete is called from inside resume's goal mutation.
func VerifyStopBatchComplete(root, goalID string, capability StopCapability, fence StopFence) error {
	batch, err := ReadStopBatch(root, fence.StopID)
	if err != nil {
		return fmt.Errorf("cannot prove stop batch %s complete: %w", fence.StopID, err)
	}
	if batch.GoalID != goalID || batch.GoalRevision != capability.Revision || batch.FenceEpoch != fence.Epoch ||
		batch.CapabilityGeneration != capability.Generation || batch.Machine != capability.Machine ||
		batch.ClaimEpoch != capability.ClaimEpoch || batch.Reason != fence.Reason {
		return fmt.Errorf("stop batch %s does not bind the exact stopped authority for goal %s revision %d", fence.StopID, goalID, capability.Revision)
	}
	if batch.State != StopBatchComplete {
		return fmt.Errorf("stop batch %s is %s, not COMPLETE", fence.StopID, batch.State)
	}
	return nil
}

// CloseStopRequest carries the exact claim-scoped authority being consumed.
type CloseStopRequest struct {
	VerbRequest
	GoalID     string
	StopID     string
	Reason     string
	Capability StopCapability
}

// CloseStop atomically closes one revision's launch fence. Cancellation runs
// only after this transaction returns and the goal-revision lock is released.
func CloseStop(r CloseStopRequest) (PublishResult, error) {
	return Publish(r.Endpoint, closeStopRequest(r))
}

func closeStopRequest(r CloseStopRequest) PublishRequest {
	args := map[string]string{
		"stopId": r.StopID, "reason": r.Reason,
		"capabilityGeneration": strconv.FormatUint(r.Capability.Generation, 10),
		"goalRevision":         strconv.FormatUint(r.Capability.Revision, 10),
		"capabilityMachine":    r.Capability.Machine,
		"claimEpoch":           strconv.FormatInt(r.Capability.ClaimEpoch, 10),
		"fenceEpoch":           strconv.FormatUint(r.Capability.FenceEpoch, 10),
	}
	return PublishRequest{
		Opid: r.opid(), Machine: r.Actor.Machine, Lineage: r.Actor.Lineage,
		Intent:  Intent{Verb: "breach-stop", Targets: []string{r.GoalID}, Args: intentArgs(r.VerbRequest, args)},
		Message: "goal breach-stop " + r.GoalID,
		Mutate: func(tip string) ([]Change, error) {
			t, err := loadTree(r.Endpoint.Root, tip)
			if err != nil {
				return nil, err
			}
			f, ok := t.Live[r.GoalID]
			if !ok || f.State != StateClaimed || f.Claimed == nil || f.StopCapability == nil {
				return nil, fmt.Errorf("goal %s has no stoppable claimed revision", r.GoalID)
			}
			if opidLanded(f, r.VerbRequest) {
				return nil, AlreadyApplied{}
			}
			capability := f.StopCapability
			if f.StopFence != nil {
				fence := f.StopFence
				if fence.StopID == r.StopID && fence.Revision == r.Capability.Revision &&
					capability.Generation == r.Capability.Generation && capability.Revision == r.Capability.Revision &&
					capability.Machine == r.Capability.Machine && capability.ClaimEpoch == r.Capability.ClaimEpoch &&
					capability.FenceEpoch == r.Capability.FenceEpoch+1 {
					return nil, NothingToDo{Reason: "the exact launch fence is already closed"}
				}
				return nil, fmt.Errorf("goal %s revision %d is already fenced by %s", r.GoalID, capability.Revision, fence.StopID)
			}
			if *capability != r.Capability || r.Actor.Machine != capability.Machine ||
				r.ClaimEpoch != capability.ClaimEpoch {
				return nil, fmt.Errorf("stop capability does not bind goal %s revision %d machine %s claim epoch %d",
					r.GoalID, capability.Revision, capability.Machine, capability.ClaimEpoch)
			}
			if r.Reason != StopReasonElapsedLimit && r.Reason != StopReasonCorruptOverLimit {
				return nil, fmt.Errorf("%s is not a live-stop reason", r.Reason)
			}
			if !safeStopID(r.StopID) {
				return nil, fmt.Errorf("invalid stop id %q", r.StopID)
			}
			capability.FenceEpoch++
			f.StopFence = &StopFence{
				StopID: r.StopID, Revision: capability.Revision, Epoch: capability.FenceEpoch,
				CapabilityGeneration: capability.Generation, ClosedAt: r.stamp(), Reason: r.Reason,
			}
			touch(f, r.VerbRequest, "breach-stop", []string{r.GoalID})
			return []Change{{Path: livePath(r.GoalID), Content: RenderFile(f)}}, nil
		},
		Validate: func(commit string) error { return ValidateCommit(r.Endpoint.Root, commit) },
	}
}

// CloseStopPublishRequest exposes the canonical fence mutation only to
// recovery policy that has freshly re-established its live authority.
func CloseStopPublishRequest(r CloseStopRequest) PublishRequest {
	return closeStopRequest(r)
}

// ResumeRequest is the human-only fresh-revision transition.
type ResumeRequest struct {
	VerbRequest
	GoalID    string
	Budget    Budget
	Authority *humanauthority.Proof
}

// Resume verifies the exact stop batch during the transaction, installs the
// complete fresh tuple, creates a new revision, and clears the old fence.
func Resume(r ResumeRequest) (PublishResult, error) {
	if r.Actor.Human == "" || r.Authority == nil || !r.Authority.AuthorizesResume(r.Endpoint.Root) {
		return PublishResult{}, fmt.Errorf("goal resume requires freshly observed enrolled-human authority or a recorded temporary relay whose human provenance is not verified")
	}
	if err := r.Budget.Validate(); err != nil {
		return PublishResult{}, fmt.Errorf("invalid fresh budget: %w", err)
	}
	return Publish(r.Endpoint, resumeRequest(r))
}

func resumeRequest(r ResumeRequest) PublishRequest {
	args := budgetIntentArgs(r.Budget)
	temporaryAuthority := r.Authority.TemporaryResumeFor(r.Endpoint.Root)
	return PublishRequest{
		Opid: r.opid(), Machine: r.Actor.Machine, Lineage: r.Actor.Lineage,
		Intent:  Intent{Verb: "resume", Targets: []string{r.GoalID}, Args: intentArgs(r.VerbRequest, args)},
		Message: "goal resume " + r.GoalID,
		Mutate: func(tip string) ([]Change, error) {
			t, err := loadTree(r.Endpoint.Root, tip)
			if err != nil {
				return nil, err
			}
			f, ok := t.Live[r.GoalID]
			if !ok || f.State != StateClaimed || f.Claimed == nil || f.StopCapability == nil || f.StopFence == nil {
				return nil, fmt.Errorf("goal %s has no breach-stopped claimed revision", r.GoalID)
			}
			if opidLanded(f, r.VerbRequest) {
				return nil, AlreadyApplied{}
			}
			if temporaryAuthority {
				if err := repeatedRelayedActError(t.Root, f, "resume", r.Authority.Departure); err != nil {
					return nil, err
				}
			}
			fence := *f.StopFence
			if err := VerifyStopBatchComplete(r.Endpoint.Root, r.GoalID, *f.StopCapability, fence); err != nil {
				return nil, err
			}
			machine, lineage, claimEpoch := f.Claimed.Machine, f.Claimed.Lineage, f.StopCapability.ClaimEpoch
			budget := r.Budget
			approval, err := goalNormApproval(r.Endpoint.Root, t, f, budget, r.ApprovedRef)
			if err != nil {
				return nil, err
			}
			f.Budget = &budget
			f.NormApproval = approval
			touch(f, r.VerbRequest, "resume", []string{r.GoalID})
			f.History[len(f.History)-1].ApprovedRef = r.ApprovedRef
			if temporaryAuthority {
				f.History[len(f.History)-1].recordTemporaryRelay(r.Authority.ReviewBy, r.Authority.Departure, r.Authority.TemporaryHumanWord)
			}
			if err := bindClaim(f, machine, lineage, r.stamp(), f.Revision, claimEpoch); err != nil {
				return nil, err
			}
			return []Change{{Path: livePath(r.GoalID), Content: RenderFile(f)}}, nil
		},
		Validate: func(commit string) error { return ValidateCommit(r.Endpoint.Root, commit) },
	}
}
