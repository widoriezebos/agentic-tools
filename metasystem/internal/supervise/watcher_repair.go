package supervise

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
)

// WatcherRestartRequest binds a repair request to the exact watcher identity
// and generation published by the supervision owner.
type WatcherRestartRequest struct {
	Schema       int       `json:"schema"`
	Generation   int64     `json:"generation"`
	Pid          int64     `json:"pid"`
	PidStartedAt int64     `json:"pidStartedAt"`
	InstanceTag  string    `json:"instanceTag"`
	RequestedAt  time.Time `json:"requestedAt"`
	Reason       string    `json:"reason"`
	Completed    bool      `json:"completed"`
	CompletedAt  time.Time `json:"completedAt,omitempty"`
	EndReason    string    `json:"endReason,omitempty"`
	Replacement  *struct {
		Pid          int64  `json:"pid"`
		PidStartedAt int64  `json:"pidStartedAt"`
		InstanceTag  string `json:"instanceTag"`
	} `json:"replacement,omitempty"`
}

// EndWatcherRestart retires any earlier request when the role breaker ends
// healing. A missing request is already safely ended.
func EndWatcherRestart(repoRoot, reason string, now time.Time) error {
	request, err := loadWatcherRestartRequest(repoRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if request.Completed {
		return nil
	}
	request.Completed = true
	request.CompletedAt = now.UTC()
	request.EndReason = reason
	return saveWatcherRestartRequest(repoRoot, request)
}

func watcherRestartRequestPath(repoRoot string) string {
	return filepath.Join(SupervisionDir(repoRoot), "watcher-restart-request.json")
}

func saveWatcherRestartRequest(repoRoot string, request WatcherRestartRequest) error {
	data, err := json.MarshalIndent(request, "", "  ")
	if err != nil {
		return err
	}
	durable, err := atomicfile.WriteText(watcherRestartRequestPath(repoRoot), string(append(data, '\n')), repoRoot)
	if err != nil {
		return err
	}
	if !durable {
		return fmt.Errorf("watcher restart request was published with durability unknown")
	}
	return nil
}

func loadWatcherRestartRequest(repoRoot string) (WatcherRestartRequest, error) {
	data, err := os.ReadFile(watcherRestartRequestPath(repoRoot))
	if err != nil {
		return WatcherRestartRequest{}, err
	}
	var request WatcherRestartRequest
	if err := json.Unmarshal(data, &request); err != nil {
		return WatcherRestartRequest{}, fmt.Errorf("watcher restart request is malformed: %w", err)
	}
	if request.Schema != 1 || request.Generation < 1 || request.Pid < 1 || request.PidStartedAt < 1 || request.InstanceTag == "" || request.RequestedAt.IsZero() || request.Reason == "" {
		return WatcherRestartRequest{}, fmt.Errorf("watcher restart request is incomplete")
	}
	return request, nil
}

// RequestWatcherRestart asks the current owner to replace only the exact
// watcher it published. A changed generation or identity needs a new request.
func RequestWatcherRestart(repoRoot, reason string, now time.Time) error {
	data, err := os.ReadFile(filepath.Join(SupervisionDir(repoRoot), "state.json"))
	if err != nil {
		return fmt.Errorf("read supervision state for watcher restart: %w", err)
	}
	var state stateDocument
	if err := json.Unmarshal(data, &state); err != nil {
		return fmt.Errorf("read supervision state for watcher restart: %w", err)
	}
	watcher, ok := state.Components[string(Watcher)]
	if state.Generation < 1 || !ok || watcher.Pid < 1 || watcher.PidStartedAt < 1 || watcher.InstanceTag == "" {
		return fmt.Errorf("supervision state has no exact watcher generation to restart")
	}
	request := WatcherRestartRequest{
		Schema: 1, Generation: state.Generation, Pid: watcher.Pid, PidStartedAt: watcher.PidStartedAt,
		InstanceTag: watcher.InstanceTag, RequestedAt: now.UTC(), Reason: reason,
	}
	if existing, readErr := loadWatcherRestartRequest(repoRoot); readErr == nil && !existing.Completed &&
		existing.Generation == request.Generation && existing.Pid == request.Pid &&
		existing.PidStartedAt == request.PidStartedAt && existing.InstanceTag == request.InstanceTag {
		return nil
	}
	return saveWatcherRestartRequest(repoRoot, request)
}

// DiskWatcherRepairs is the owner's request adapter for one checkout.
type DiskWatcherRepairs struct {
	Root string
}

func (d *DiskWatcherRepairs) WatcherRestartRequested(held Held) (bool, error) {
	request, err := loadWatcherRestartRequest(d.Root)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	if request.Completed {
		return false, nil
	}
	if request.Generation != held.Generation || request.Pid != held.Identity.Pid ||
		request.PidStartedAt != held.Identity.StartedAtSec || request.InstanceTag != held.Tag {
		return false, fmt.Errorf("watcher restart request does not name the owner's current watcher")
	}
	ended, err := watcherHealingEnded(d.Root)
	if err != nil {
		return false, err
	}
	if ended {
		return false, nil
	}
	return true, nil
}

func watcherHealingEnded(repoRoot string) (bool, error) {
	data, err := os.ReadFile(filepath.Join(repoRoot, "artifacts", "agents", "steward", "health.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("read watcher health breaker: %w", err)
	}
	var record struct {
		State struct {
			FailureCounts map[string]int `json:"failureCounts"`
		} `json:"state"`
		Verdict struct {
			Roles []struct {
				Role              string `json:"role"`
				FailureEscalation string `json:"failureEscalation"`
			} `json:"roles"`
		} `json:"verdict"`
	}
	if err := json.Unmarshal(data, &record); err != nil {
		return false, fmt.Errorf("read watcher health breaker: %w", err)
	}
	if record.State.FailureCounts["repo-watcher"] >= 5 {
		return true, nil
	}
	for _, role := range record.Verdict.Roles {
		if role.Role == "repo-watcher" && role.FailureEscalation == "AUTO_HEAL_ENDED" {
			return true, nil
		}
	}
	return false, nil
}

func (d *DiskWatcherRepairs) CompleteWatcherRestart(previous, replacement Held) error {
	request, err := loadWatcherRestartRequest(d.Root)
	if err != nil {
		return err
	}
	if request.Completed || request.Generation != previous.Generation || request.Pid != previous.Identity.Pid ||
		request.PidStartedAt != previous.Identity.StartedAtSec || request.InstanceTag != previous.Tag {
		return fmt.Errorf("watcher restart completion does not match the pending request")
	}
	request.Completed = true
	request.CompletedAt = time.Now().UTC()
	request.Replacement = &struct {
		Pid          int64  `json:"pid"`
		PidStartedAt int64  `json:"pidStartedAt"`
		InstanceTag  string `json:"instanceTag"`
	}{Pid: replacement.Identity.Pid, PidStartedAt: replacement.Identity.StartedAtSec, InstanceTag: replacement.Tag}
	return saveWatcherRestartRequest(d.Root, request)
}
