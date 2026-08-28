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
	Component            string          `json:"component"`
	Generation           int             `json:"generation"`
	Pid                  int64           `json:"pid"`
	PidStartedAt         int64           `json:"pidStartedAt"`
	PidStartTicks        int64           `json:"pidStartTicks,omitempty"`
	BootID               string          `json:"bootId,omitempty"`
	SuccessPid           int64           `json:"successPid,omitempty"`
	SuccessPidStartedAt  int64           `json:"successPidStartedAt,omitempty"`
	SuccessPidStartTicks int64           `json:"successPidStartTicks,omitempty"`
	SuccessBootID        string          `json:"successBootId,omitempty"`
	AttemptSeq           int64           `json:"attemptSeq"`
	LastAttempt          time.Time       `json:"lastAttempt"`
	LastCompletion       time.Time       `json:"lastCompletion"`
	LastSuccess          time.Time       `json:"lastSuccess"`
	Result               ComponentResult `json:"result"`
	Outcome              string          `json:"outcome"`
	EvidenceDigest       string          `json:"evidenceDigest"`
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
	if err := saveComponentEvidence(repoRoot, path, record); err != nil {
		return ComponentEvidence{}, err
	}
	return record, nil
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
