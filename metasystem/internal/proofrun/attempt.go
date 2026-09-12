package proofrun

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/hostload"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

const (
	LegacyAttemptSchemaVersion = 1
	AttemptSchemaVersion       = 2

	DispositionExecuted         = "executed"
	DispositionFailed           = "failed"
	DispositionLiveDuplicate    = "live-duplicate"
	DispositionReusableSuccess  = "reusable-success"
	DispositionRetryRequired    = "retry-required"
	DispositionAdmissionRefused = "admission-refused"

	ExitLiveDuplicate    = 75
	ExitReusableSuccess  = 76
	ExitRetryRequired    = 77
	ExitAdmissionRefused = 78

	TerminalSuccess   = "success"
	TerminalFailed    = "failed"
	TerminalCancelled = "cancelled"
	TerminalUnknown   = "unknown"
)

// ProofIdentity names every input that can change what one expensive proof
// establishes. Execution paths and process labels deliberately stay outside it.
type ProofIdentity struct {
	ScopeClass     string   `json:"scopeClass"`
	CommandClass   string   `json:"commandClass"`
	ManifestDigest string   `json:"manifestDigest"`
	Configuration  string   `json:"configurationDigest"`
	Sections       []string `json:"sections"`
	Platform       string   `json:"platform"`
	Toolchain      string   `json:"toolchain"`
	RatchetDigest  string   `json:"ratchetDigest,omitempty"`
	IdentityInputs []string `json:"identityInputs,omitempty"`
	BehaviorPolicy int      `json:"behaviorPolicyVersion"`
	IdentityDigest string   `json:"identityDigest"`
}

type RetryEvidence struct {
	SchemaVersion  int    `json:"schemaVersion"`
	PriorAttempt   string `json:"priorAttempt"`
	Cause          string `json:"cause"`
	EvidencePath   string `json:"evidencePath"`
	EvidenceSHA256 string `json:"evidenceSha256"`
	Rationale      string `json:"rationale"`
	Automatic      bool   `json:"automatic,omitempty"`
	// PriorLoad and PriorAttribution carry the prior attempt's host facts
	// into the decision that retries it, so a retry names the load it is
	// retrying against.
	PriorLoad        *AttemptLoad `json:"priorLoad,omitempty"`
	PriorAttribution string       `json:"priorAttribution,omitempty"`
}

type ReservationOwner struct {
	ControlRoot        string  `json:"controlRoot"`
	RunID              string  `json:"runId"`
	RunGeneration      int     `json:"runGeneration"`
	LaunchNonce        string  `json:"launchNonce"`
	GoalRevision       uint64  `json:"goalRevision"`
	ObligationRevision uint64  `json:"obligationRevision"`
	AttemptOrdinal     uint64  `json:"attemptOrdinal"`
	BudgetEpoch        *uint64 `json:"budgetEpoch"`
	Deadline           string  `json:"deadline"`
}

type AttemptTerminal struct {
	Result     string `json:"result"`
	ExitStatus int    `json:"exitStatus"`
	Reason     string `json:"reason,omitempty"`
	At         string `json:"at"`
	// Attribution is "load" when a non-success terminal was committed while
	// the end sample showed a crowded host (another proof launcher on the
	// host, or the minute load at the cores).
	Attribution string `json:"attribution,omitempty"`
}

type CoverageEvidence struct {
	SchemaVersion    int                `json:"schemaVersion"`
	Producer         ProcessIdentity    `json:"producer"`
	ProducerClass    string             `json:"producerClass"`
	AttemptID        string             `json:"attemptId"`
	PackageInventory []string           `json:"packageInventory"`
	Measurements     map[string]float64 `json:"measurements"`
	EngineDigest     string             `json:"engineDigest"`
	EngineManifest   string             `json:"engineManifest"`
	BehaviorPolicy   int                `json:"behaviorPolicyVersion"`
	Platform         string             `json:"platform"`
	Toolchain        string             `json:"toolchain"`
	RatchetDigest    string             `json:"ratchetDigest"`
	CompletedAt      string             `json:"completedAt"`
}

type PendingCoverage struct {
	Producer      ProcessIdentity   `json:"producer"`
	ProducerClass string            `json:"producerClass"`
	Expected      ProofIdentity     `json:"expected"`
	Evidence      *CoverageEvidence `json:"evidence,omitempty"`
}

// Attempt is the retained accounting and outcome record for one admitted
// proof. A nil Terminal result is accountable nonterminal work, never success.
type Attempt struct {
	SchemaVersion      int               `json:"schemaVersion"`
	AttemptID          string            `json:"attemptId"`
	GoalID             string            `json:"goalId"`
	GoalRevision       uint64            `json:"goalRevision"`
	AccountingRevision uint64            `json:"accountingRevision"`
	BudgetEpoch        *uint64           `json:"budgetEpoch"`
	ReservedMinutes    uint64            `json:"reservedMinutes"`
	ObservedMinutes    uint64            `json:"observedMinutes"`
	StartedAt          string            `json:"startedAt"`
	Deadline           string            `json:"deadline"`
	EndedAt            string            `json:"endedAt,omitempty"`
	ProofIdentity      ProofIdentity     `json:"proofIdentity"`
	ControlRoot        string            `json:"controlRoot"`
	ExecutionRoot      string            `json:"executionRoot"`
	Launcher           ProcessIdentity   `json:"launcher"`
	ProcessKeys        []string          `json:"processKeys"`
	Terminal           *AttemptTerminal  `json:"terminal,omitempty"`
	PreviousAttempt    string            `json:"previousAttempt,omitempty"`
	Retry              *RetryEvidence    `json:"retryEvidence,omitempty"`
	ReservationOwner   *ReservationOwner `json:"reservationOwner,omitempty"`
	CancellationIntent string            `json:"cancellationIntent,omitempty"`
	PendingCoverage    *PendingCoverage  `json:"pendingCoverage,omitempty"`
	PendingTestGroups  map[string]string `json:"pendingTestGroups,omitempty"`
	DeliveryReceipt    json.RawMessage   `json:"deliveryReceipt,omitempty"`
	// DeliveryReceiptBytes preserves the exact prepared schema-2 payload. A
	// json.RawMessage is re-indented when the enclosing attempt is persisted,
	// so it cannot be the recovery source for byte-exact publication.
	DeliveryReceiptBytes []byte      `json:"deliveryReceiptBytes,omitempty"`
	TestResult           *TestResult `json:"testResult,omitempty"`
	// Load is the host as this attempt saw it at its start and its end.
	Load *AttemptLoad `json:"load,omitempty"`
}

// CommittedDeliveryReceipt returns a defensive copy of the payload committed
// in the attempt's terminal transaction. Legacy attempts used RawMessage;
// schema-2 attempts use the byte-preserving field.
func CommittedDeliveryReceipt(attempt Attempt) json.RawMessage {
	payload := attempt.DeliveryReceiptBytes
	if len(payload) == 0 {
		payload = attempt.DeliveryReceipt
	}
	return append(json.RawMessage(nil), payload...)
}

type RetryDecision struct {
	SchemaVersion int    `json:"schemaVersion"`
	PriorAttempt  string `json:"priorAttempt"`
	Cause         string `json:"cause"`
	EvidencePath  string `json:"evidencePath"`
	Rationale     string `json:"rationale"`
}

type AdmissionRequest struct {
	ControlRoot         string
	ExecutionRoot       string
	GoalID              string
	GoalRevision        uint64
	AccountingRevision  uint64
	BudgetEpoch         *uint64
	ReservedMinutes     uint64
	Identity            ProofIdentity
	Launcher            ProcessIdentity
	RetryDecisionPath   string
	ReservationOwner    *ReservationOwner
	Now                 time.Time
	AttemptID           string
	ComponentIdentities map[string]string
	// ExecuteAfresh never answers reusable-success: the caller wants a fresh
	// execution (a cadence sweep, or a composer that could not compose from
	// the seat's newest observations); live duplicates and retry decisions
	// still apply.
	ExecuteAfresh bool
}

type LaunchResult struct {
	SchemaVersion int    `json:"schemaVersion"`
	Disposition   string `json:"disposition"`
	AttemptID     string `json:"attemptId,omitempty"`
	PriorAttempt  string `json:"priorAttemptId,omitempty"`
	EvidencePath  string `json:"evidencePath,omitempty"`
	ExitStatus    int    `json:"exitStatus"`
}

type MutationLock struct{ file *os.File }

func attemptsDir(root string) string {
	return filepath.Join(root, "artifacts", "agents", "proof-runs", "attempts")
}

func AttemptPath(root, id string) (string, error) {
	if !safeAttemptID(id) {
		return "", fmt.Errorf("invalid proof attempt id %q", id)
	}
	return filepath.Join(attemptsDir(root), id+".json"), nil
}

func safeAttemptID(value string) bool {
	if value == "" || len(value) > 160 {
		return false
	}
	for _, r := range value {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '-' {
			return false
		}
	}
	return true
}

// AcquireMutation serializes retained proof decisions across runtimes.
func AcquireMutation(root string) (*MutationLock, error) {
	path := filepath.Join(root, "artifacts", "agents", "locks", "proof-runs.lock")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	deadline := time.Now().Add(time.Second)
	for {
		err = syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			return &MutationLock{file: file}, nil
		}
		if !errors.Is(err, syscall.EWOULDBLOCK) || !time.Now().Before(deadline) {
			file.Close()
			return nil, fmt.Errorf("LOCK_BUSY rank=proof-mutation key=%s retry=retry-after-the-holder-releases", root)
		}
		time.Sleep(25 * time.Millisecond)
	}
}

func (lock *MutationLock) Release() error {
	if lock == nil || lock.file == nil {
		return nil
	}
	err := syscall.Flock(int(lock.file.Fd()), syscall.LOCK_UN)
	closeErr := lock.file.Close()
	lock.file = nil
	return errors.Join(err, closeErr)
}

func ReadAttempt(root, id string) (Attempt, error) {
	path, err := AttemptPath(root, id)
	if err != nil {
		return Attempt{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Attempt{}, err
	}
	var attempt Attempt
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&attempt); err != nil {
		return Attempt{}, fmt.Errorf("read proof attempt %s: %w", id, err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return Attempt{}, fmt.Errorf("read proof attempt %s: trailing JSON", id)
	}
	if attempt.AttemptID != id || attempt.ControlRoot != root {
		return Attempt{}, fmt.Errorf("read proof attempt %s: record identity contradicts its path", id)
	}
	if err := validateAttempt(attempt); err != nil {
		return Attempt{}, fmt.Errorf("read proof attempt %s: %w", id, err)
	}
	return attempt, nil
}

func ReadAttempts(root string) ([]Attempt, error) {
	paths, err := filepath.Glob(filepath.Join(attemptsDir(root), "*.json"))
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	attempts := make([]Attempt, 0, len(paths))
	for _, path := range paths {
		id := strings.TrimSuffix(filepath.Base(path), ".json")
		attempt, err := ReadAttempt(root, id)
		if err != nil {
			return nil, err
		}
		attempts = append(attempts, attempt)
	}
	return attempts, nil
}

func writeAttempt(attempt Attempt) error {
	if err := validateAttempt(attempt); err != nil {
		return err
	}
	path, err := AttemptPath(attempt.ControlRoot, attempt.AttemptID)
	if err != nil {
		return err
	}
	encoded, err := json.MarshalIndent(attempt, "", "  ")
	if err != nil {
		return err
	}
	durable, err := atomicfile.WriteText(path, string(encoded)+"\n", attempt.ControlRoot)
	if err != nil {
		return err
	}
	if !durable {
		return fmt.Errorf("proof attempt %s durability is unknown", attempt.AttemptID)
	}
	return nil
}

func validateAttempt(attempt Attempt) error {
	if (attempt.SchemaVersion != LegacyAttemptSchemaVersion && attempt.SchemaVersion != AttemptSchemaVersion) || !safeAttemptID(attempt.AttemptID) ||
		attempt.GoalID == "" || attempt.GoalRevision == 0 || attempt.AccountingRevision == 0 ||
		attempt.AccountingRevision > attempt.GoalRevision || attempt.ReservedMinutes == 0 ||
		attempt.ControlRoot == "" || attempt.ExecutionRoot == "" || attempt.ProofIdentity.IdentityDigest == "" ||
		attempt.Launcher.Pid < 1 || attempt.Launcher.Ref().Mode() == identity.CompareInvalid {
		return fmt.Errorf("proof attempt has incomplete accounting, identity, or launcher facts")
	}
	if attempt.SchemaVersion == LegacyAttemptSchemaVersion && attempt.TestResult != nil {
		return fmt.Errorf("legacy proof attempt cannot carry schema-2 testing evidence")
	}
	if attempt.TestResult != nil {
		if err := ValidateTestResult(*attempt.TestResult); err != nil {
			return fmt.Errorf("proof attempt testing evidence: %w", err)
		}
		if attempt.TestResult.AttemptID != attempt.AttemptID {
			return fmt.Errorf("proof attempt testing evidence names a different attempt")
		}
	}
	if err := validateProofIdentity(attempt.ProofIdentity); err != nil {
		return err
	}
	started, startErr := time.Parse(time.RFC3339Nano, attempt.StartedAt)
	deadline, deadlineErr := time.Parse(time.RFC3339Nano, attempt.Deadline)
	if startErr != nil || deadlineErr != nil || !deadline.After(started) {
		return fmt.Errorf("proof attempt has an invalid time bound")
	}
	ended, endedErr := time.Parse(time.RFC3339Nano, attempt.EndedAt)
	if attempt.Terminal != nil {
		if attempt.EndedAt == "" || attempt.Terminal.At != attempt.EndedAt {
			return fmt.Errorf("terminal proof attempt has contradictory end time")
		}
		if endedErr != nil || ended.Before(started) || attempt.ObservedMinutes == 0 {
			return fmt.Errorf("terminal proof attempt has invalid observed time")
		}
		if attempt.Terminal.Result != TerminalSuccess && attempt.Terminal.Result != TerminalFailed &&
			attempt.Terminal.Result != TerminalCancelled && attempt.Terminal.Result != TerminalUnknown {
			return fmt.Errorf("proof attempt terminal result %q is invalid", attempt.Terminal.Result)
		}
		if attempt.Terminal.Result == TerminalSuccess && attempt.Terminal.ExitStatus != 0 {
			return fmt.Errorf("successful proof attempt has nonzero exit status")
		}
		if attempt.Terminal.Result == TerminalFailed && attempt.Terminal.ExitStatus == 0 {
			return fmt.Errorf("failed proof attempt has zero exit status")
		}
		if attempt.Terminal.Result == TerminalSuccess && attempt.CancellationIntent != "" {
			return fmt.Errorf("successful proof attempt contradicts cancellation intent")
		}
	} else if attempt.EndedAt != "" || attempt.ObservedMinutes != 0 || len(CommittedDeliveryReceipt(attempt)) > 0 {
		return fmt.Errorf("nonterminal proof attempt carries terminal evidence")
	}
	seenProcesses := map[string]bool{}
	for _, key := range attempt.ProcessKeys {
		if !safeProcessKey(key) || seenProcesses[key] {
			return fmt.Errorf("proof attempt has an invalid or duplicate process key")
		}
		seenProcesses[key] = true
	}
	if attempt.PreviousAttempt != "" {
		if !safeAttemptID(attempt.PreviousAttempt) || attempt.Retry == nil || attempt.Retry.PriorAttempt != attempt.PreviousAttempt {
			return fmt.Errorf("proof retry does not bind its previous attempt")
		}
	} else if attempt.Retry != nil {
		return fmt.Errorf("proof retry evidence has no previous attempt")
	}
	if retry := attempt.Retry; retry != nil {
		if retry.SchemaVersion != 1 || retry.Automatic || retry.Cause == "" || retry.Rationale == "" ||
			retry.EvidencePath == "" || !validSHA256(retry.EvidenceSHA256) {
			return fmt.Errorf("proof retry evidence is incomplete")
		}
	}
	if owner := attempt.ReservationOwner; owner != nil {
		ownerDeadline, ownerDeadlineErr := time.Parse(time.RFC3339Nano, owner.Deadline)
		if owner.ControlRoot != attempt.ControlRoot || owner.RunID == "" || owner.RunGeneration < 1 ||
			owner.LaunchNonce == "" || owner.GoalRevision != attempt.GoalRevision || owner.ObligationRevision == 0 ||
			owner.AttemptOrdinal == 0 || !sameUint64Value(owner.BudgetEpoch, attempt.BudgetEpoch) ||
			ownerDeadlineErr != nil || ownerDeadline.UTC().Format(time.RFC3339Nano) != attempt.Deadline {
			return fmt.Errorf("governed proof reservation owner is incomplete or contradictory")
		}
	}
	if pending := attempt.PendingCoverage; pending != nil {
		if pending.Producer.Ref().Mode() == identity.CompareInvalid || pending.ProducerClass != "full" ||
			pending.Expected.IdentityDigest != attempt.ProofIdentity.IdentityDigest {
			return fmt.Errorf("pending coverage producer is incomplete or contradictory")
		}
		if evidence := pending.Evidence; evidence != nil {
			if evidence.SchemaVersion != 1 || evidence.Producer.Ref().Mode() == identity.CompareInvalid ||
				evidence.Producer.Ref() != pending.Producer.Ref() || evidence.ProducerClass != pending.ProducerClass ||
				evidence.AttemptID != attempt.AttemptID || len(evidence.PackageInventory) == 0 || len(evidence.Measurements) == 0 ||
				evidence.EngineDigest == "" || evidence.EngineManifest == "" || evidence.BehaviorPolicy < 1 ||
				evidence.Platform == "" || evidence.Toolchain == "" || !validSHA256(evidence.RatchetDigest) ||
				evidence.CompletedAt == "" {
				return fmt.Errorf("coverage producer evidence is incomplete or contradictory")
			}
		}
	}
	for id, digest := range attempt.PendingTestGroups {
		if id == "" || !validSHA256(digest) {
			return fmt.Errorf("pending test component identity is invalid")
		}
	}
	return nil
}

func validateProofIdentity(value ProofIdentity) error {
	if value.ScopeClass == "" || value.CommandClass == "" || !validSHA256(value.ManifestDigest) ||
		!validSHA256(value.Configuration) || value.Platform == "" || !validSHA256(value.Toolchain) ||
		(value.RatchetDigest != "" && !validSHA256(value.RatchetDigest)) || value.BehaviorPolicy < 1 ||
		!validSHA256(value.IdentityDigest) || value.IdentityDigest != value.digest() {
		return fmt.Errorf("proof identity is incomplete or has a contradictory digest")
	}
	for index, section := range value.Sections {
		if section == "" || index > 0 && value.Sections[index-1] >= section {
			return fmt.Errorf("proof identity sections are not a sorted unique set")
		}
	}
	return nil
}

func validSHA256(value string) bool {
	if len(value) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func safeProcessKey(value string) bool {
	return value != "" && len(value) <= 340 && filepath.Base(value) == value && !strings.ContainsAny(value, `/\\`)
}

func sameUint64Value(left, right *uint64) bool {
	return left == nil && right == nil || left != nil && right != nil && *left == *right
}

// ReserveLocked applies duplicate and retry rules while the caller holds the
// goal-revision lock followed by the proof mutation lock.
func ReserveLocked(request AdmissionRequest) (Attempt, LaunchResult, error) {
	now := request.Now.UTC()
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if request.Identity.IdentityDigest == "" {
		request.Identity.IdentityDigest = request.Identity.digest()
	}
	if request.ReservedMinutes == 0 || request.AccountingRevision == 0 || request.AccountingRevision > request.GoalRevision {
		return Attempt{}, LaunchResult{}, fmt.Errorf("proof reservation requires positive bounded accounting facts")
	}
	previous, decision, decided, err := repeatDecisionLocked(request)
	if err != nil {
		return Attempt{}, LaunchResult{}, err
	}
	if decided {
		return Attempt{}, decision, nil
	}
	var retry *RetryEvidence
	if previous != nil {
		retry, err = readRetryDecision(request.RetryDecisionPath, request.ControlRoot, request.GoalID, previous.ProofIdentity.IdentityDigest, previous.AttemptID)
		if err != nil {
			return Attempt{}, LaunchResult{}, err
		}
	}
	id := request.AttemptID
	if id == "" {
		id, err = newAttemptID(now)
		if err != nil {
			return Attempt{}, LaunchResult{}, err
		}
	}
	if path, pathErr := AttemptPath(request.ControlRoot, id); pathErr != nil {
		return Attempt{}, LaunchResult{}, pathErr
	} else if _, statErr := os.Lstat(path); statErr == nil || !os.IsNotExist(statErr) {
		return Attempt{}, LaunchResult{}, fmt.Errorf("proof attempt id %s is already retained", id)
	}
	attempt := Attempt{
		SchemaVersion: AttemptSchemaVersion, AttemptID: id, GoalID: request.GoalID,
		GoalRevision: request.GoalRevision, AccountingRevision: request.AccountingRevision,
		BudgetEpoch: request.BudgetEpoch, ReservedMinutes: request.ReservedMinutes,
		StartedAt: now.Format(time.RFC3339Nano), Deadline: now.Add(time.Duration(request.ReservedMinutes) * time.Minute).Format(time.RFC3339Nano),
		ProofIdentity: request.Identity, ControlRoot: request.ControlRoot, ExecutionRoot: request.ExecutionRoot,
		Launcher: request.Launcher, ProcessKeys: []string{}, Retry: retry, ReservationOwner: request.ReservationOwner,
		Load: &AttemptLoad{Start: sampleLoad(request.ControlRoot, id, request.Launcher.Pid, now)},
	}
	if previous != nil {
		attempt.PreviousAttempt = previous.AttemptID
	}
	if request.ReservationOwner != nil {
		ownerDeadline, err := time.Parse(time.RFC3339Nano, request.ReservationOwner.Deadline)
		if err != nil || ownerDeadline.Before(now) {
			return Attempt{}, LaunchResult{}, fmt.Errorf("governed reservation owner has no live deadline")
		}
		attempt.Deadline = ownerDeadline.UTC().Format(time.RFC3339Nano)
	}
	if err := writeAttempt(attempt); err != nil {
		return Attempt{}, LaunchResult{}, err
	}
	return attempt, LaunchResult{SchemaVersion: 1, Disposition: DispositionExecuted, AttemptID: id, PriorAttempt: attempt.PreviousAttempt, ExitStatus: 0}, nil
}

// NoChildDecisionLocked resolves retained duplicate, reusable-success, and
// retry-required outcomes while the caller holds the goal-revision and proof
// mutation locks. The false result means a genuinely new execution still
// needs prospective budget admission.
func NoChildDecisionLocked(request AdmissionRequest) (LaunchResult, bool, error) {
	_, decision, decided, err := repeatDecisionLocked(request)
	return decision, decided, err
}

// JoinedComponentDecisionLocked applies only the component breaker for a
// nested worker that is joining an already admitted outer attempt. A typed
// retry decision is still required after the newest matching failure.
func JoinedComponentDecisionLocked(request AdmissionRequest) (LaunchResult, bool, error) {
	previous, decision, decided, err := componentDecisionLocked(request)
	if err != nil || decided || previous == nil {
		return decision, decided, err
	}
	if _, err := readRetryDecision(request.RetryDecisionPath, request.ControlRoot, request.GoalID,
		previous.ProofIdentity.IdentityDigest, previous.AttemptID); err != nil {
		return LaunchResult{}, false, err
	}
	return LaunchResult{}, false, nil
}

func repeatDecisionLocked(request AdmissionRequest) (*Attempt, LaunchResult, bool, error) {
	componentPrevious, componentDecision, componentDecided, err := componentDecisionLocked(request)
	if err != nil || componentDecided || componentPrevious != nil {
		return componentPrevious, componentDecision, componentDecided, err
	}
	return noChildDecisionLocked(request)
}

// componentDecisionLocked applies the same live/failure/reuse protocol when
// the caller changes modes or asks for a superset. The component identities
// are already bound into the enclosing proof identity; this lookup only
// prevents a second reservation or an undiagnosed rerun.
func componentDecisionLocked(request AdmissionRequest) (*Attempt, LaunchResult, bool, error) {
	if len(request.ComponentIdentities) == 0 {
		return nil, LaunchResult{}, false, nil
	}
	attempts, err := ReadAttempts(request.ControlRoot)
	if err != nil {
		return nil, LaunchResult{}, false, err
	}
	type componentObservation struct {
		attempt Attempt
		state   string
	}
	latest := map[string]componentObservation{}
	for index := range attempts {
		candidate := attempts[index]
		if candidate.GoalID != request.GoalID || candidate.AccountingRevision != request.AccountingRevision {
			continue
		}
		planned := componentInputs(candidate.ProofIdentity.IdentityInputs)
		for id, identity := range candidate.PendingTestGroups {
			planned[id] = identity
		}
		for id, identity := range request.ComponentIdentities {
			if planned[id] != identity {
				continue
			}
			if observed, ok := latest[id]; ok {
				newest := newerAttempt(&observed.attempt, candidate)
				if newest.AttemptID == observed.attempt.AttemptID {
					continue
				}
			}
			if candidate.Terminal == nil {
				latest[id] = componentObservation{attempt: candidate, state: "live"}
				continue
			}
			state := "failed"
			if candidate.TestResult != nil {
				for _, group := range candidate.TestResult.Groups {
					if group.ID != id || group.ExecutionIdentity != identity {
						continue
					}
					if candidate.Terminal.Result == TerminalSuccess && (group.Status == "passed" || group.Status == "reused") && group.CollectionComplete {
						state = "success"
					}
					break
				}
			}
			latest[id] = componentObservation{attempt: candidate, state: state}
		}
	}
	var newestLive, newestFailed *Attempt
	failedIDs := map[string]bool{}
	successCount := 0
	for _, observed := range latest {
		switch observed.state {
		case "live":
			newestLive = newerAttempt(newestLive, observed.attempt)
		case "failed":
			failedIDs[observed.attempt.AttemptID] = true
			newestFailed = newerAttempt(newestFailed, observed.attempt)
		case "success":
			successCount++
		}
	}
	if newestLive != nil {
		return newestLive, LaunchResult{SchemaVersion: 1, Disposition: DispositionLiveDuplicate,
			AttemptID: newestLive.AttemptID, ExitStatus: ExitLiveDuplicate}, true, nil
	}
	if newestFailed != nil {
		if request.RetryDecisionPath == "" {
			return newestFailed, LaunchResult{SchemaVersion: 1, Disposition: DispositionRetryRequired,
				AttemptID: newestFailed.AttemptID, PriorAttempt: newestFailed.AttemptID,
				EvidencePath: mustAttemptPath(request.ControlRoot, newestFailed.AttemptID), ExitStatus: ExitRetryRequired}, true, nil
		}
		if len(failedIDs) != 1 {
			return nil, LaunchResult{}, false, fmt.Errorf("component retry decision is ambiguous across %d failed attempts", len(failedIDs))
		}
		return newestFailed, LaunchResult{}, false, nil
	}
	if successCount == len(request.ComponentIdentities) && !request.ExecuteAfresh {
		return nil, LaunchResult{SchemaVersion: 1, Disposition: DispositionReusableSuccess,
			EvidencePath: attemptsDir(request.ControlRoot), ExitStatus: ExitReusableSuccess}, true, nil
	}
	return nil, LaunchResult{}, false, nil
}

func componentInputs(inputs []string) map[string]string {
	result := map[string]string{}
	for _, input := range inputs {
		if !strings.HasPrefix(input, "group:") {
			continue
		}
		parts := strings.SplitN(strings.TrimPrefix(input, "group:"), ":", 2)
		if len(parts) == 2 {
			result[parts[0]] = parts[1]
		}
	}
	return result
}

// BindJoinedTestComponentsLocked publishes the component set owned by a
// nested testing worker while its caller holds the goal-revision and proof
// mutation locks. Publishing under the same lock as the newest-state check
// prevents a second nested caller from launching the same missing group.
func BindJoinedTestComponentsLocked(root, attemptID string, identities map[string]string) (Attempt, error) {
	attempt, err := ReadAttempt(root, attemptID)
	if err != nil {
		return Attempt{}, err
	}
	if attempt.SchemaVersion != AttemptSchemaVersion || attempt.Terminal != nil || attempt.CancellationIntent != "" || attempt.TestResult != nil || len(attempt.PendingTestGroups) != 0 {
		return Attempt{}, fmt.Errorf("live schema-%d proof attempt without an existing testing owner is required", AttemptSchemaVersion)
	}
	attempt.PendingTestGroups = make(map[string]string, len(identities))
	for id, identity := range identities {
		if id == "" || !validSHA256(identity) {
			return Attempt{}, fmt.Errorf("joined testing component %q has invalid identity", id)
		}
		attempt.PendingTestGroups[id] = identity
	}
	if len(attempt.PendingTestGroups) == 0 {
		return Attempt{}, fmt.Errorf("joined testing component inventory is empty")
	}
	if err := writeAttempt(attempt); err != nil {
		return Attempt{}, err
	}
	return attempt, nil
}

func newerAttempt(current *Attempt, candidate Attempt) *Attempt {
	if current == nil {
		copy := candidate
		return &copy
	}
	currentStarted, _ := time.Parse(time.RFC3339Nano, current.StartedAt)
	candidateStarted, _ := time.Parse(time.RFC3339Nano, candidate.StartedAt)
	if candidateStarted.After(currentStarted) {
		copy := candidate
		return &copy
	}
	return current
}

func noChildDecisionLocked(request AdmissionRequest) (*Attempt, LaunchResult, bool, error) {
	if request.Identity.IdentityDigest == "" {
		request.Identity.IdentityDigest = request.Identity.digest()
	}
	attempts, err := ReadAttempts(request.ControlRoot)
	if err != nil {
		return nil, LaunchResult{}, false, err
	}
	var previous *Attempt
	for index := range attempts {
		candidate := &attempts[index]
		if candidate.GoalID != request.GoalID || candidate.ProofIdentity.IdentityDigest != request.Identity.IdentityDigest {
			continue
		}
		candidateStarted, _ := time.Parse(time.RFC3339Nano, candidate.StartedAt)
		previousStarted := time.Time{}
		if previous != nil {
			previousStarted, _ = time.Parse(time.RFC3339Nano, previous.StartedAt)
		}
		if previous == nil || candidateStarted.After(previousStarted) {
			copy := *candidate
			previous = &copy
		}
	}
	if previous == nil {
		return nil, LaunchResult{}, false, nil
	}
	if previous.Terminal == nil {
		return previous, LaunchResult{SchemaVersion: 1, Disposition: DispositionLiveDuplicate,
			AttemptID: previous.AttemptID, ExitStatus: ExitLiveDuplicate}, true, nil
	}
	if previous.Terminal.Result == TerminalSuccess {
		if request.ExecuteAfresh {
			return nil, LaunchResult{}, false, nil
		}
		return previous, LaunchResult{SchemaVersion: 1, Disposition: DispositionReusableSuccess,
			AttemptID: previous.AttemptID, EvidencePath: mustAttemptPath(request.ControlRoot, previous.AttemptID),
			ExitStatus: ExitReusableSuccess}, true, nil
	}
	if request.RetryDecisionPath == "" {
		return previous, LaunchResult{SchemaVersion: 1, Disposition: DispositionRetryRequired,
			AttemptID: previous.AttemptID, PriorAttempt: previous.AttemptID,
			EvidencePath: mustAttemptPath(request.ControlRoot, previous.AttemptID), ExitStatus: ExitRetryRequired}, true, nil
	}
	return previous, LaunchResult{}, false, nil
}

func readRetryDecision(path, root, goalID, identityDigest, prior string) (*RetryEvidence, error) {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		return nil, fmt.Errorf("retry decision must be a readable regular file")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var decision RetryDecision
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&decision); err != nil {
		return nil, fmt.Errorf("retry decision is malformed: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return nil, fmt.Errorf("retry decision has trailing JSON")
	}
	if decision.SchemaVersion != 1 || decision.PriorAttempt != prior || strings.TrimSpace(decision.Cause) == "" ||
		strings.EqualFold(strings.TrimSpace(decision.Cause), "technical issue") || strings.TrimSpace(decision.Rationale) == "" || decision.EvidencePath == "" {
		return nil, fmt.Errorf("retry decision has incomplete accountable evidence")
	}
	priorAttempt, err := ReadAttempt(root, decision.PriorAttempt)
	if err != nil || priorAttempt.GoalID != goalID || priorAttempt.ProofIdentity.IdentityDigest != identityDigest {
		return nil, fmt.Errorf("retry decision prior attempt is outside the same goal and input history")
	}
	evidencePath := decision.EvidencePath
	if !filepath.IsAbs(evidencePath) {
		evidencePath = filepath.Join(filepath.Dir(path), evidencePath)
	}
	evidenceInfo, err := os.Lstat(evidencePath)
	if err != nil || !evidenceInfo.Mode().IsRegular() {
		return nil, fmt.Errorf("retry evidence must be a readable regular file")
	}
	evidence, err := os.ReadFile(evidencePath)
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256(evidence)
	retry := &RetryEvidence{SchemaVersion: 1, PriorAttempt: prior, Cause: decision.Cause,
		EvidencePath: evidencePath, EvidenceSHA256: hex.EncodeToString(digest[:]), Rationale: decision.Rationale}
	// The decision names the load it retries against: the prior's samples
	// and its attribution ride along, so "flaky" cannot hide a crowded box.
	retry.PriorAttribution = "no load block: the prior attempt was recorded before the attribution landed"
	if priorAttempt.Load != nil {
		retry.PriorLoad = priorAttempt.Load
		retry.PriorAttribution = "no load attribution"
		if priorAttempt.Terminal != nil && priorAttempt.Terminal.Attribution == LoadAttribution && priorAttempt.Load.End != nil {
			retry.PriorAttribution = LoadAttribution + ": " + priorAttempt.Load.End.Describe()
		}
	}
	return retry, nil
}

func newAttemptID(now time.Time) (string, error) {
	var suffix [8]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		return "", err
	}
	return "proof-" + strconv.FormatInt(now.UnixMilli(), 36) + "-" + hex.EncodeToString(suffix[:]), nil
}

func newLaunchID() (string, error) {
	var suffix [12]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		return "", err
	}
	return "launch-" + hex.EncodeToString(suffix[:]), nil
}

func mustAttemptPath(root, id string) string {
	path, _ := AttemptPath(root, id)
	return path
}

func (identity ProofIdentity) digest() string {
	copy := identity
	copy.IdentityDigest = ""
	encoded, _ := json.Marshal(copy)
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:])
}

func BuildProofIdentity(executionRoot, configurationPath, scopeClass, commandClass string, sections []string, behaviorPolicy int) (ProofIdentity, error) {
	context, err := CaptureExecutionContext(executionRoot, configurationPath, os.Environ())
	if err != nil {
		return ProofIdentity{}, err
	}
	return BuildProofIdentityForContext(context, scopeClass, commandClass, sections, behaviorPolicy), nil
}

func BuildProofIdentityForContext(context ExecutionContext, scopeClass, commandClass string, sections []string, behaviorPolicy int) ProofIdentity {
	sortedSections := append([]string(nil), sections...)
	sort.Strings(sortedSections)
	result := ProofIdentity{ScopeClass: scopeClass, CommandClass: commandClass, ManifestDigest: context.ManifestDigest,
		Configuration: context.Configuration, Sections: sortedSections,
		Platform: context.Platform, Toolchain: context.Toolchain, RatchetDigest: context.RatchetDigest, BehaviorPolicy: behaviorPolicy}
	result.IdentityDigest = result.digest()
	return result
}

// BindIdentityInputs adds one consumer's already-normalized proof inputs to
// the shared identity without turning execution paths or runtime labels into
// proof inputs.
func BindIdentityInputs(identity ProofIdentity, inputs []string) ProofIdentity {
	identity.IdentityInputs = append([]string(nil), inputs...)
	sort.Strings(identity.IdentityInputs)
	identity.IdentityDigest = identity.digest()
	return identity
}

func effectiveProofConfigurationDigest(configurationPath string, environment []string) (string, error) {
	keys := config.Keys(configurationPath, "", environment)
	lookup := environmentLookup(environment)
	seen := make(map[string]bool, len(keys))
	for _, key := range keys {
		seen[key] = true
	}
	// These proof controls have defaults, so an environment-only override is
	// an effective input even when the committed file does not name the key.
	for _, key := range []string{"suite.progress-silence-min", "suite.section-cap-min", "suite.evidence-copy-timeout-sec", "suite.evidence-copy-max-mb"} {
		if _, present := lookup(config.EnvName(key)); present && !seen[key] {
			keys = append(keys, key)
			seen[key] = true
		}
	}
	sort.Strings(keys)
	hash := sha256.New()
	for _, key := range keys {
		// The shipped template's model slot is metadata, not a resolvable key.
		// Concrete local/runtime bindings below carry the effective model;
		// the source manifest still binds the template's original bytes.
		if key == "role.code-critic.model.<runtime>" {
			continue
		}
		value, _, err := config.Get(config.GetParams{Key: key, ConfPath: configurationPath, LookupEnv: lookup})
		if err != nil {
			return "", fmt.Errorf("resolve effective proof configuration %s: %w", key, err)
		}
		for _, field := range []string{key, value} {
			if err := binary.Write(hash, binary.BigEndian, uint64(len(field))); err != nil {
				return "", err
			}
			_, _ = io.WriteString(hash, field)
		}
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func CompleteToolchainIdentity() (string, error) {
	return CompleteToolchainIdentityAt("")
}

func CompleteToolchainIdentityAt(root string) (string, error) {
	return CompleteToolchainIdentityAtWithEnvironment(root, os.Environ())
}

func coverageRatchetPath(root string) string {
	name := "coverage-ratchet.json"
	if runtime.GOOS == "linux" {
		name = "coverage-ratchet-linux.json"
	}
	return filepath.Join(root, "scripts", "agents", name)
}

func coverageRatchetDigest(root string) (string, error) {
	data, err := os.ReadFile(coverageRatchetPath(root))
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:]), nil
}

func UpdateAttemptProcesses(root, id string, launcher identity.Ref, processKeys []string) error {
	lock, err := AcquireMutation(root)
	if err != nil {
		return err
	}
	defer lock.Release()
	return updateAttemptProcessesLocked(root, id, launcher, processKeys)
}

func updateAttemptProcessesLocked(root, id string, launcher identity.Ref, processKeys []string) error {
	attempt, err := ReadAttempt(root, id)
	if err != nil {
		return err
	}
	if !lineageContains(launcher.Pid, attempt.Launcher.Ref()) || attempt.Terminal != nil || attempt.CancellationIntent != "" {
		return fmt.Errorf("proof attempt no longer authorizes process publication")
	}
	seen := make(map[string]bool, len(attempt.ProcessKeys)+len(processKeys))
	for _, key := range attempt.ProcessKeys {
		seen[key] = true
	}
	for _, key := range processKeys {
		if key != "" && !seen[key] {
			attempt.ProcessKeys = append(attempt.ProcessKeys, key)
			seen[key] = true
		}
	}
	return writeAttempt(attempt)
}

func attemptLaunchAllowedLocked(root, id string, launcher identity.Ref, now time.Time) error {
	attempt, err := ReadAttempt(root, id)
	if err != nil {
		return err
	}
	deadline, deadlineErr := time.Parse(time.RFC3339Nano, attempt.Deadline)
	if attempt.Terminal != nil || attempt.CancellationIntent != "" || deadlineErr != nil || !now.UTC().Before(deadline) ||
		!lineageContains(launcher.Pid, attempt.Launcher.Ref()) {
		return fmt.Errorf("proof attempt no longer authorizes child creation")
	}
	return nil
}

func RequestCancellation(root, id, reason string) error {
	lock, err := AcquireMutation(root)
	if err != nil {
		return err
	}
	defer lock.Release()
	attempt, err := ReadAttempt(root, id)
	if err != nil {
		return err
	}
	if attempt.Terminal != nil {
		return nil
	}
	if strings.TrimSpace(reason) == "" {
		return fmt.Errorf("proof cancellation requires a reason")
	}
	attempt.CancellationIntent = reason
	return writeAttempt(attempt)
}

func FinalizeAttempt(root, id, result string, exitStatus int, reason string, deliveryReceipt json.RawMessage, now time.Time) (Attempt, error) {
	lock, err := AcquireMutation(root)
	if err != nil {
		return Attempt{}, err
	}
	defer lock.Release()
	return FinalizeAttemptLocked(root, id, result, exitStatus, reason, deliveryReceipt, now)
}

// FinalizeAttemptLocked writes the single terminal commit while the caller
// holds the transition, goal-revision, and proof mutation locks in that order.
func FinalizeAttemptLocked(root, id, result string, exitStatus int, reason string, deliveryReceipt json.RawMessage, now time.Time) (Attempt, error) {
	return FinalizeAttemptWithTestResultLocked(root, id, result, exitStatus, reason, deliveryReceipt, nil, now)
}

// FinalizeAttemptWithTestResultLocked atomically commits the terminal state
// and the normalized schema-2 testing result under the existing proof lock.
func FinalizeAttemptWithTestResultLocked(root, id, result string, exitStatus int, reason string, deliveryReceipt json.RawMessage, testResult *TestResult, now time.Time) (Attempt, error) {
	attempt, err := ReadAttempt(root, id)
	if err != nil {
		return Attempt{}, err
	}
	if attempt.Terminal != nil {
		return Attempt{}, fmt.Errorf("proof attempt is already terminal")
	}
	if result == TerminalSuccess && attempt.CancellationIntent != "" {
		return Attempt{}, fmt.Errorf("proof attempt cancellation prevents success")
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	deadline, deadlineErr := time.Parse(time.RFC3339Nano, attempt.Deadline)
	if result == TerminalSuccess && (deadlineErr != nil || !now.Before(deadline)) {
		return Attempt{}, fmt.Errorf("proof attempt deadline expired before success")
	}
	started, _ := time.Parse(time.RFC3339Nano, attempt.StartedAt)
	observed := uint64(math.Ceil(now.Sub(started).Minutes()))
	if observed == 0 {
		observed = 1
	}
	stamp := now.UTC().Format(time.RFC3339Nano)
	attempt.ObservedMinutes = observed
	attempt.EndedAt = stamp
	attempt.Terminal = &AttemptTerminal{Result: result, ExitStatus: exitStatus, Reason: reason, At: stamp}
	if testResult != nil {
		copyResult := *testResult
		copyResult.AttemptID = attempt.AttemptID
		if err := ValidateTestResult(copyResult); err != nil {
			return Attempt{}, fmt.Errorf("retain test result: %w", err)
		}
		attempt.TestResult = &copyResult
	}
	// The end sample: a failure under a crowded host is attributed in the
	// record. A cancellation or an unknown terminal is not a failure the
	// load caused, and carries the sample without the label. An attempt
	// reserved before the attribution landed has no start sample, and the
	// record says so rather than inventing one.
	end := sampleLoad(root, id, attempt.Launcher.Pid, now)
	if attempt.Load == nil {
		attempt.Load = &AttemptLoad{Start: LoadSample{Sample: hostload.Sample{Detail: "not sampled at start"}}}
	}
	attempt.Load.End = &end
	if result == TerminalFailed && end.Loaded() {
		attempt.Terminal.Attribution = LoadAttribution
	}
	if len(deliveryReceipt) > 0 {
		if attempt.TestResult != nil {
			attempt.DeliveryReceiptBytes = append([]byte(nil), deliveryReceipt...)
			attempt.DeliveryReceipt = nil
		} else {
			attempt.DeliveryReceipt = append(json.RawMessage(nil), deliveryReceipt...)
		}
	}
	if err := writeAttempt(attempt); err != nil {
		return Attempt{}, err
	}
	// Every failed consumption-bounded group of a failed attempt under load
	// is filed as its own patience defect, after the commit and never fatal
	// to it: a filing that cannot be written is reported, the terminal
	// stands.
	if _, err := filePatienceDefects(attempt, end); err != nil {
		fmt.Fprintf(os.Stderr, "proof attempt %s: patience defects not filed: %v\n", id, err)
	}
	return attempt, nil
}

// RecordTestResult attaches normalized group evidence to an enclosing live
// proof. The enclosing launcher still owns the one terminal transition.
func RecordTestResult(root, id string, result TestResult) (Attempt, error) {
	lock, err := AcquireMutation(root)
	if err != nil {
		return Attempt{}, err
	}
	defer lock.Release()
	attempt, err := ReadAttempt(root, id)
	if err != nil {
		return Attempt{}, err
	}
	if attempt.SchemaVersion != AttemptSchemaVersion || attempt.Terminal != nil || attempt.TestResult != nil {
		return Attempt{}, fmt.Errorf("live schema-%d proof attempt without a retained test result is required", AttemptSchemaVersion)
	}
	result.AttemptID = attempt.AttemptID
	if err := ValidateTestResult(result); err != nil {
		return Attempt{}, fmt.Errorf("retain test result: %w", err)
	}
	if len(attempt.PendingTestGroups) > 0 {
		observed := make(map[string]string, len(result.Groups))
		for _, group := range result.Groups {
			observed[group.ID] = group.ExecutionIdentity
		}
		if !reflect.DeepEqual(attempt.PendingTestGroups, observed) {
			return Attempt{}, fmt.Errorf("retained testing result does not match the joined component reservation")
		}
	}
	attempt.TestResult = &result
	if err := writeAttempt(attempt); err != nil {
		return Attempt{}, err
	}
	return attempt, nil
}

func AuthenticateContext(root, id string, callerPID int64) (Attempt, error) {
	attempt, err := ReadAttempt(root, id)
	if err != nil {
		return Attempt{}, err
	}
	if attempt.Terminal != nil || attempt.CancellationIntent != "" {
		return Attempt{}, fmt.Errorf("proof parent context is not live")
	}
	deadline, err := time.Parse(time.RFC3339Nano, attempt.Deadline)
	if err != nil || !time.Now().UTC().Before(deadline) {
		return Attempt{}, fmt.Errorf("proof parent context has reached its absolute deadline")
	}
	if lineageContains(callerPID, attempt.Launcher.Ref()) {
		return attempt, nil
	}
	return Attempt{}, fmt.Errorf("proof parent context does not contain the recorded launcher")
}

// AuthenticateAncestor proves that callerPID descends from one exact durable
// launcher identity.
func AuthenticateAncestor(callerPID int64, ancestor ProcessIdentity) error {
	if !lineageContains(callerPID, ancestor.Ref()) {
		return fmt.Errorf("caller process does not descend from the recorded launcher")
	}
	return nil
}

func lineageContains(callerPID int64, ancestor identity.Ref) bool {
	seen := map[int64]bool{}
	for current := callerPID; current > 0 && !seen[current]; {
		seen[current] = true
		exact, state, err := (identity.KernelProber{}).Probe(current)
		if err != nil || state != identity.Alive {
			return false
		}
		if identity.Compare(exact, ancestor).Matches {
			return true
		}
		parent, ok := identity.ParentPid(current)
		if !ok || parent == current {
			break
		}
		current = parent
	}
	return false
}

func EncodeResult(writer io.Writer, path string, result LaunchResult) error {
	encoded, err := json.Marshal(result)
	if err != nil {
		return err
	}
	if writer != nil {
		fmt.Fprintf(writer, "PROOF-RESULT %s\n", encoded)
	}
	if path != "" {
		if err := atomicfile.WriteVolatile(path, string(encoded)+"\n"); err != nil {
			return err
		}
	}
	return nil
}
