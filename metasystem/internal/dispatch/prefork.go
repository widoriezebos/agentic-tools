package dispatch

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

var launchSuffixRE = regexp.MustCompile(`^[a-z0-9]+$`)

func reservationInstanceTag(job, launchOpIDSuffix string) (string, error) {
	if !validJobID.MatchString(job) || !launchSuffixRE.MatchString(launchOpIDSuffix) {
		return "", fmt.Errorf("dispatch: launch operation suffix must be lowercase letters and digits")
	}
	return "metasystem-job-" + job + "-" + launchOpIDSuffix, nil
}

func preforkMarkerPath(root, tag string) (string, error) {
	if tag == "" || filepath.Base(tag) != tag || filepath.Clean(tag) != tag {
		return "", fmt.Errorf("dispatch: unsafe pre-fork marker tag")
	}
	return filepath.Join(root, "artifacts", "agents", "prefork", tag), nil
}

// WritePreforkMarker records the supervisor's exact identity before a child
// can be forked. The custody count distinguishes sequential children of one
// supervisor: a marker is satisfied only by a later custody append.
func WritePreforkMarker(root, job, tag string, supervisorPID, intendedPGID int64, reader identity.Prober) error {
	if supervisorPID < 1 || intendedPGID < 2 || reader == nil {
		return fmt.Errorf("dispatch: pre-fork marker requires a supervisor, intended group, and identity reader")
	}
	exact, state, err := reader.Probe(supervisorPID)
	if err != nil || state != identity.Alive || exact.Pid != supervisorPID || !exact.Ref().NativeExact() {
		return fmt.Errorf("dispatch: supervisor %d exact identity is unavailable", supervisorPID)
	}
	markerPath, err := preforkMarkerPath(root, tag)
	if err != nil {
		return err
	}
	return withRecordSessionLock(root, job, func(recordPath string, _ *SessionIndexTransaction) error {
		record, readErr := readObject(recordPath)
		if readErr != nil {
			return fmt.Errorf("dispatch: cannot read job %s before fork: %w", job, readErr)
		}
		if asString(record["instanceTag"]) != tag {
			return fmt.Errorf("dispatch: pre-fork tag does not match job %s", job)
		}
		if removed, sweepErr := SweepSatisfiedPreforkMarker(root, record); sweepErr != nil {
			return sweepErr
		} else if !removed {
			if _, statErr := os.Stat(markerPath); statErr == nil {
				return fmt.Errorf("dispatch: job %s already has a standing pre-fork marker", job)
			} else if !os.IsNotExist(statErr) {
				return statErr
			}
		}
		marker := map[string]any{
			"schemaVersion":      1,
			"instanceTag":        tag,
			"supervisor":         exactIdentityFields(exact.Ref()),
			"intendedPgid":       intendedPGID,
			"custodyCountBefore": custodyProcessCount(record),
		}
		return writeRecord(markerPath, marker)
	})
}

// SweepSatisfiedPreforkMarker removes only a marker whose corresponding
// custody write is already durable, or whose record has ended. A marker and
// custody entry left together by a crash therefore reconcile in this order.
func SweepSatisfiedPreforkMarker(root string, record map[string]any) (bool, error) {
	tag := asString(record["instanceTag"])
	path, err := preforkMarkerPath(root, tag)
	if err != nil {
		return false, err
	}
	marker, err := readObject(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("dispatch: read pre-fork marker: %w", err)
	}
	if asString(marker["instanceTag"]) != tag {
		return false, fmt.Errorf("dispatch: pre-fork marker tag does not match its path")
	}
	before, ok := numInt(marker["custodyCountBefore"])
	if !ok || before < 0 {
		return false, fmt.Errorf("dispatch: pre-fork marker has no valid custody baseline")
	}
	status := asString(record["status"])
	satisfied := int64(custodyProcessCount(record)) > before
	ended := TerminalStatus(status) || status == "reconciled-proven-absent" || status == "seam-archived"
	if !satisfied && !ended {
		return false, nil
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return false, fmt.Errorf("dispatch: remove satisfied pre-fork marker: %w", err)
	}
	return true, nil
}

func custodyProcessCount(record map[string]any) int {
	items, _ := record["custodyProcesses"].([]any)
	return len(items)
}

type preforkMarker struct {
	path          string
	supervisor    identity.Ref
	intendedPGID  int64
	custodyBefore int64
}

func readPreforkMarker(root string, record map[string]any) (*preforkMarker, error) {
	if _, err := SweepSatisfiedPreforkMarker(root, record); err != nil {
		return nil, err
	}
	tag := asString(record["instanceTag"])
	path, err := preforkMarkerPath(root, tag)
	if err != nil {
		return nil, err
	}
	object, err := readObject(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	supervisorObject, ok := object["supervisor"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("dispatch: pre-fork marker has no supervisor identity")
	}
	supervisor, ok := identityRefFromObject(supervisorObject)
	if !ok || !supervisor.NativeExact() {
		return nil, fmt.Errorf("dispatch: pre-fork marker supervisor identity is not platform-exact")
	}
	intended, ok := numInt(object["intendedPgid"])
	if !ok || intended < 2 {
		return nil, fmt.Errorf("dispatch: pre-fork marker has no intended process group")
	}
	before, _ := numInt(object["custodyCountBefore"])
	return &preforkMarker{path: path, supervisor: supervisor, intendedPGID: intended, custodyBefore: before}, nil
}
