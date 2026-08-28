package steward

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sys/unix"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// ComponentResult says whether one completed attempt produced all of its
// mandatory results. A failed or uncertain completion never refreshes the
// component's last successful pass.
type ComponentResult string

const (
	ComponentOK            ComponentResult = "OK"
	ComponentError         ComponentResult = "ERROR"
	ComponentIndeterminate ComponentResult = "INDETERMINATE"
)

// ComponentEvidence binds a periodic producer's work to its installation
// generation. Process presence is recorded separately from completed work so
// a live loop cannot pass health on the strength of a heartbeat alone.
type ComponentEvidence struct {
	Component            string                    `json:"component"`
	Generation           int                       `json:"generation"`
	Pid                  int64                     `json:"pid"`
	PidStartedAt         int64                     `json:"pidStartedAt"`
	PidStartTicks        int64                     `json:"pidStartTicks,omitempty"`
	BootID               string                    `json:"bootId,omitempty"`
	SuccessPid           int64                     `json:"successPid,omitempty"`
	SuccessPidStartedAt  int64                     `json:"successPidStartedAt,omitempty"`
	SuccessPidStartTicks int64                     `json:"successPidStartTicks,omitempty"`
	SuccessBootID        string                    `json:"successBootId,omitempty"`
	SuccessAttemptSeq    int64                     `json:"successAttemptSeq,omitempty"`
	AttemptSeq           int64                     `json:"attemptSeq"`
	TurnKeyDigest        string                    `json:"turnKeyDigest,omitempty"`
	LastAttempt          time.Time                 `json:"lastAttempt"`
	LastCompletion       time.Time                 `json:"lastCompletion"`
	LastSuccess          time.Time                 `json:"lastSuccess"`
	Result               ComponentResult           `json:"result"`
	Outcome              string                    `json:"outcome"`
	EvidenceDigest       string                    `json:"evidenceDigest"`
	AttemptHistory       []ComponentAttemptHistory `json:"attemptHistory,omitempty"`
}

// ComponentAttemptHistory is the durable terminal fact for an attempt that
// no longer owns the component's current slot. Hook history is retained so a
// later turn cannot erase evidence that its predecessor died before emission.
type ComponentAttemptHistory struct {
	Generation     int             `json:"generation"`
	AttemptSeq     int64           `json:"attemptSeq"`
	TurnKeyDigest  string          `json:"turnKeyDigest,omitempty"`
	AttemptedAt    time.Time       `json:"attemptedAt"`
	CompletedAt    time.Time       `json:"completedAt"`
	Result         ComponentResult `json:"result"`
	Outcome        string          `json:"outcome"`
	EvidenceDigest string          `json:"evidenceDigest"`
}

const componentAttemptHistoryLimit = 100

func appendAttemptHistory(record *ComponentEvidence, completedAt time.Time, result ComponentResult, outcome, evidence string) {
	entry := ComponentAttemptHistory{
		Generation: record.Generation, AttemptSeq: record.AttemptSeq, TurnKeyDigest: record.TurnKeyDigest,
		AttemptedAt: record.LastAttempt, CompletedAt: completedAt.UTC(), Result: result, Outcome: outcome,
		EvidenceDigest: evidenceDigest(evidence),
	}
	if size := len(record.AttemptHistory); size > 0 {
		last := record.AttemptHistory[size-1]
		if last.Generation == entry.Generation && last.AttemptSeq == entry.AttemptSeq {
			record.AttemptHistory[size-1] = entry
			return
		}
	}
	record.AttemptHistory = append(record.AttemptHistory, entry)
	if len(record.AttemptHistory) > componentAttemptHistoryLimit {
		record.AttemptHistory = append([]ComponentAttemptHistory(nil), record.AttemptHistory[len(record.AttemptHistory)-componentAttemptHistoryLimit:]...)
	}
}

// ComponentEvidencePath is the durable record for one periodic producer.
func ComponentEvidencePath(repoRoot, component string) string {
	return filepath.Join(repoRoot, "artifacts", "agents", "steward", "components", component+".json")
}

func componentEvidenceLockPath(repoRoot, component string) string {
	return filepath.Join(repoRoot, "artifacts", "agents", "steward", "components", component+".flock")
}

func componentDurabilityPendingPath(repoRoot, component string) string {
	return filepath.Join(repoRoot, "artifacts", "agents", "steward", "components", component+".durability-pending")
}

func lockComponentEvidence(repoRoot, component string, operation int) (*os.File, error) {
	path := componentEvidenceLockPath(repoRoot, component)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	if err := unix.Flock(int(file.Fd()), operation); err != nil {
		file.Close()
		return nil, err
	}
	return file, nil
}

func unlockComponentEvidence(file *os.File) {
	_ = unix.Flock(int(file.Fd()), unix.LOCK_UN)
	_ = file.Close()
}

// beginComponentAttempt persists the next attempt before the producer does
// any work. A new generation starts with no inherited completion or success.
func beginComponentAttempt(repoRoot, component string, generation int, process identity.Ref, now time.Time) (ComponentEvidence, error) {
	lock, err := lockComponentEvidence(repoRoot, component, unix.LOCK_EX)
	if err != nil {
		return ComponentEvidence{}, err
	}
	defer unlockComponentEvidence(lock)

	path := ComponentEvidencePath(repoRoot, component)
	previous, err := loadComponentEvidence(path)
	if err != nil && !os.IsNotExist(err) {
		return ComponentEvidence{}, err
	}
	if previous.Component != component || previous.Generation != generation {
		previous = ComponentEvidence{Component: component, Generation: generation}
	}
	previous.Pid = process.Pid
	previous.PidStartedAt = process.StartedAtSec
	previous.PidStartTicks = process.StartTicks
	previous.BootID = process.BootID
	previous.AttemptSeq++
	previous.LastAttempt = now.UTC()
	previous.Result = ComponentIndeterminate
	previous.Outcome = "ATTEMPTING"
	previous.EvidenceDigest = evidenceDigest(fmt.Sprintf("%s|%d|%d|%s", component, generation, previous.AttemptSeq, previous.LastAttempt.Format(time.RFC3339Nano)))
	if err := saveComponentEvidence(repoRoot, path, previous); err != nil {
		return ComponentEvidence{}, err
	}
	return previous, nil
}

// BeginComponentAttempt lets an internal component record its own work before
// that work starts. Callers may complete only the exact sequence it returns.
func BeginComponentAttempt(repoRoot, component string, generation int, process identity.Ref, now time.Time) (ComponentEvidence, error) {
	return beginComponentAttempt(repoRoot, component, generation, process, now)
}

// BeginHookAttempt starts a new turn generation after a successful emission
// and reuses the same generation when the same failed turn is retried.
func BeginHookAttempt(repoRoot string, process identity.Ref, turnKey string, now time.Time) (ComponentEvidence, error) {
	if strings.TrimSpace(turnKey) == "" {
		return ComponentEvidence{}, fmt.Errorf("hook attempt needs a turn key")
	}
	if process.Pid < 1 || process.StartedAtSec < 1 {
		return ComponentEvidence{}, fmt.Errorf("hook attempt needs an exact process identity")
	}
	lock, err := lockComponentEvidence(repoRoot, "supervision-hook", unix.LOCK_EX)
	if err != nil {
		return ComponentEvidence{}, err
	}
	defer unlockComponentEvidence(lock)

	path := ComponentEvidencePath(repoRoot, "supervision-hook")
	previous, err := loadComponentEvidence(path)
	if err != nil && !os.IsNotExist(err) {
		return ComponentEvidence{}, err
	}
	turnKeyDigest := evidenceDigest(turnKey)
	if previous.Component == "supervision-hook" && previous.Outcome == "ATTEMPTING" {
		completion := now.UTC()
		if completion.Before(previous.LastAttempt) {
			return ComponentEvidence{}, fmt.Errorf("hook attempt clock is earlier than the unresolved prior attempt")
		}
		previous.LastCompletion = completion
		previous.Result = ComponentError
		previous.Outcome = "INTERRUPTED_BY_NEXT_TURN"
		appendAttemptHistory(&previous, completion, previous.Result, previous.Outcome, "the next hook turn found this attempt incomplete")
		previous.EvidenceDigest = evidenceDigest("the next hook turn found this attempt incomplete")
		// The failed closure lands before the next attempt. If publishing the
		// successor fails, the interrupted turn still remains durable evidence.
		if err := saveComponentEvidence(repoRoot, path, previous); err != nil {
			return ComponentEvidence{}, err
		}
	}
	if previous.Component != "supervision-hook" || previous.TurnKeyDigest != turnKeyDigest || previous.Result == ComponentOK {
		generation := previous.Generation + 1
		if generation < 1 {
			generation = 1
		}
		history := append([]ComponentAttemptHistory(nil), previous.AttemptHistory...)
		previous = ComponentEvidence{
			Component: "supervision-hook", Generation: generation,
			TurnKeyDigest: turnKeyDigest, AttemptHistory: history,
		}
	}
	previous.Pid = process.Pid
	previous.PidStartedAt = process.StartedAtSec
	previous.PidStartTicks = process.StartTicks
	previous.BootID = process.BootID
	previous.AttemptSeq++
	previous.LastAttempt = now.UTC()
	previous.Result = ComponentIndeterminate
	previous.Outcome = "ATTEMPTING"
	previous.EvidenceDigest = evidenceDigest(fmt.Sprintf("supervision-hook|%d|%d|%s", previous.Generation, previous.AttemptSeq, previous.LastAttempt.Format(time.RFC3339Nano)))
	if err := saveComponentEvidence(repoRoot, path, previous); err != nil {
		return ComponentEvidence{}, err
	}
	return previous, nil
}

// completeComponentAttempt records a completion only for the exact attempt
// still on disk. Every completion advances lastCompletion; only OK advances
// lastSuccess.
func completeComponentAttempt(repoRoot, component string, generation int, attemptSeq int64, result ComponentResult, outcome, evidence string, now time.Time) (ComponentEvidence, error) {
	if result != ComponentOK && result != ComponentError && result != ComponentIndeterminate {
		return ComponentEvidence{}, fmt.Errorf("component %s completion has invalid result %q", component, result)
	}
	if outcome == "" {
		return ComponentEvidence{}, fmt.Errorf("component %s completion needs an outcome", component)
	}
	lock, err := lockComponentEvidence(repoRoot, component, unix.LOCK_EX)
	if err != nil {
		return ComponentEvidence{}, err
	}
	defer unlockComponentEvidence(lock)

	path := ComponentEvidencePath(repoRoot, component)
	record, err := loadComponentEvidence(path)
	if err != nil {
		return ComponentEvidence{}, err
	}
	if record.Component != component || record.Generation != generation || record.AttemptSeq != attemptSeq {
		return ComponentEvidence{}, fmt.Errorf("component %s attempt changed before completion", component)
	}
	completion := now.UTC()
	if completion.Before(record.LastAttempt) {
		return ComponentEvidence{}, fmt.Errorf("component %s completion clock is earlier than its attempt", component)
	}
	if result == ComponentOK {
		// Publish a durable pending completion before exposing OK. If the
		// promotion's directory sync is uncertain, restore this state so a
		// reader can never accept an unproven lastSuccess.
		pending := record
		pending.LastCompletion = completion
		pending.Result = ComponentIndeterminate
		pending.Outcome = "DURABILITY_PENDING"
		pending.EvidenceDigest = evidenceDigest(evidence)
		if err := saveComponentEvidence(repoRoot, path, pending); err != nil {
			return ComponentEvidence{}, err
		}
		markerDurable, markerErr := atomicfile.WriteText(componentDurabilityPendingPath(repoRoot, component), "pending\n", repoRoot)
		if markerErr != nil {
			return ComponentEvidence{}, markerErr
		}
		if !markerDurable {
			return ComponentEvidence{}, fmt.Errorf("component evidence %s durability marker was published with durability unknown", filepath.Base(path))
		}

		promoted := pending
		promoted.Result = result
		promoted.Outcome = outcome
		promoted.LastSuccess = completion
		promoted.SuccessPid = record.Pid
		promoted.SuccessPidStartedAt = record.PidStartedAt
		promoted.SuccessPidStartTicks = record.PidStartTicks
		promoted.SuccessBootID = record.BootID
		promoted.SuccessAttemptSeq = record.AttemptSeq
		if component == "supervision-hook" {
			appendAttemptHistory(&promoted, completion, result, outcome, evidence)
		}
		durable, err := writeComponentEvidence(repoRoot, path, promoted)
		if err != nil {
			return ComponentEvidence{}, err
		}
		if !durable {
			restoredDurably, restoreErr := writeComponentEvidence(repoRoot, path, pending)
			if restoreErr != nil {
				return ComponentEvidence{}, fmt.Errorf("component evidence %s OK promotion had unknown durability and its pending state could not be restored: %w", filepath.Base(path), restoreErr)
			}
			if restoredDurably {
				_ = os.Remove(componentDurabilityPendingPath(repoRoot, component))
			}
			return ComponentEvidence{}, fmt.Errorf("component evidence %s OK promotion had unknown durability; completion remains pending", filepath.Base(path))
		}
		if err := os.Remove(componentDurabilityPendingPath(repoRoot, component)); err != nil && !os.IsNotExist(err) {
			return ComponentEvidence{}, fmt.Errorf("component evidence %s is durable but its pending marker could not be cleared: %w", filepath.Base(path), err)
		}
		return promoted, nil
	}
	record.LastCompletion = completion
	record.Result = result
	record.Outcome = outcome
	record.EvidenceDigest = evidenceDigest(evidence)
	if component == "supervision-hook" {
		appendAttemptHistory(&record, completion, result, outcome, evidence)
	}
	if err := saveComponentEvidence(repoRoot, path, record); err != nil {
		return ComponentEvidence{}, err
	}
	return record, nil
}

// CompleteComponentAttempt lets an internal component finish only the exact
// attempt it began. It cannot advance another component's evidence.
func CompleteComponentAttempt(repoRoot, component string, generation int, attemptSeq int64, result ComponentResult, outcome, evidence string, now time.Time) (ComponentEvidence, error) {
	return completeComponentAttempt(repoRoot, component, generation, attemptSeq, result, outcome, evidence, now)
}

// CompleteHookAttempt binds hook success to the exact payload already emitted.
// Client rendering is outside the hook's evidence boundary.
func CompleteHookAttempt(repoRoot string, generation int, attemptSeq int64, result ComponentResult, outcome, healthLine, payload string, now time.Time) (ComponentEvidence, error) {
	if result == ComponentOK && outcome != "EMITTED" {
		return ComponentEvidence{}, fmt.Errorf("a successful hook completion must use outcome EMITTED")
	}
	if result != ComponentOK && outcome == "EMITTED" {
		return ComponentEvidence{}, fmt.Errorf("a failed hook completion cannot claim emission")
	}
	if result == ComponentOK && (strings.TrimSpace(healthLine) == "" || !hookPayloadContainsHealthLine(payload, healthLine)) {
		return ComponentEvidence{}, fmt.Errorf("hook completion payload does not contain its health line")
	}
	if strings.Contains(payload, "DISPLAYED") || outcome == "DISPLAYED" {
		return ComponentEvidence{}, fmt.Errorf("the hook cannot claim client display")
	}
	return completeComponentAttempt(repoRoot, "supervision-hook", generation, attemptSeq, result, outcome, payload, now)
}

func hookPayloadContainsHealthLine(payload, healthLine string) bool {
	if strings.Contains(payload, healthLine) {
		return true
	}
	decoder := json.NewDecoder(strings.NewReader(payload))
	for {
		var object map[string]any
		if err := decoder.Decode(&object); err != nil {
			return false
		}
		if message, ok := object["systemMessage"].(string); ok && strings.Contains(message, healthLine) {
			return true
		}
	}
}

func loadComponentEvidence(path string) (ComponentEvidence, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return ComponentEvidence{}, err
	}
	var record ComponentEvidence
	if err := json.Unmarshal(data, &record); err != nil {
		return ComponentEvidence{}, fmt.Errorf("component evidence %s is malformed: %w", filepath.Base(path), err)
	}
	if record.Component == "" || record.Generation < 0 || record.Pid < 1 || record.PidStartedAt < 1 ||
		record.AttemptSeq < 1 || record.LastAttempt.IsZero() || record.Outcome == "" || !validEvidenceDigest(record.EvidenceDigest) {
		return ComponentEvidence{}, fmt.Errorf("component evidence %s is incomplete", filepath.Base(path))
	}
	if record.Result != ComponentOK && record.Result != ComponentError && record.Result != ComponentIndeterminate {
		return ComponentEvidence{}, fmt.Errorf("component evidence %s has an invalid result", filepath.Base(path))
	}
	for _, attempt := range record.AttemptHistory {
		if attempt.Generation < 0 || attempt.AttemptSeq < 1 || attempt.AttemptedAt.IsZero() || attempt.CompletedAt.IsZero() ||
			attempt.CompletedAt.Before(attempt.AttemptedAt) || attempt.Outcome == "" || !validEvidenceDigest(attempt.EvidenceDigest) ||
			(attempt.Result != ComponentOK && attempt.Result != ComponentError && attempt.Result != ComponentIndeterminate) {
			return ComponentEvidence{}, fmt.Errorf("component evidence %s has invalid attempt history", filepath.Base(path))
		}
	}
	if record.Result == ComponentOK && record.SuccessAttemptSeq == 0 {
		// Records written before exact-attempt binding carried one successful
		// sequence implicitly: the current sequence whose completion succeeded.
		record.SuccessAttemptSeq = record.AttemptSeq
	}
	if record.LastSuccess.After(record.LastCompletion) ||
		(record.Outcome == "ATTEMPTING" && record.Result != ComponentIndeterminate) ||
		(record.Outcome != "ATTEMPTING" && (record.LastCompletion.IsZero() || record.LastCompletion.Before(record.LastAttempt))) ||
		(record.Result == ComponentOK && !record.LastSuccess.Equal(record.LastCompletion)) {
		return ComponentEvidence{}, fmt.Errorf("component evidence %s has inconsistent boundaries", filepath.Base(path))
	}
	return record, nil
}

func loadComponentEvidenceForHealth(repoRoot, component string) (ComponentEvidence, bool, error) {
	lock, err := lockComponentEvidence(repoRoot, component, unix.LOCK_SH)
	if err != nil {
		return ComponentEvidence{}, false, err
	}
	defer unlockComponentEvidence(lock)
	record, err := loadComponentEvidence(ComponentEvidencePath(repoRoot, component))
	if err != nil {
		return ComponentEvidence{}, false, err
	}
	_, markerErr := os.Stat(componentDurabilityPendingPath(repoRoot, component))
	if markerErr == nil {
		return record, true, nil
	}
	if !os.IsNotExist(markerErr) {
		return ComponentEvidence{}, false, markerErr
	}
	return record, false, nil
}

var componentEvidenceWriter = atomicfile.WriteText

func writeComponentEvidence(repoRoot, path string, record ComponentEvidence) (bool, error) {
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return false, err
	}
	return componentEvidenceWriter(path, string(append(data, '\n')), repoRoot)
}

func saveComponentEvidence(repoRoot, path string, record ComponentEvidence) error {
	durable, err := writeComponentEvidence(repoRoot, path, record)
	if err != nil {
		return err
	}
	if !durable {
		return fmt.Errorf("component evidence %s was published with durability unknown", filepath.Base(path))
	}
	return nil
}

func evidenceDigest(evidence string) string {
	sum := sha256.Sum256([]byte(evidence))
	return hex.EncodeToString(sum[:])
}

func validEvidenceDigest(value string) bool {
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == sha256.Size && value == strings.ToLower(value)
}
