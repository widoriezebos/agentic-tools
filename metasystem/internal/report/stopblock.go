// Package report holds the turn-end report decisions — the stop-hook
// block that refuses to end a turn while planned work is unblocked and
// idle, and the open-work check — plus the improvement-mode frontier
// ledger (frontier.go), which shares the CLI's report family: it reads
// git, enforces the noise floor, and rewrites plans/frontier.
package report

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/sys/unix"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
)

// stopBlockReason is the fixed guidance a block carries. The refusal is
// bounded — the caller blocks only the first time a given set of open work is
// seen — so this text tells the agent to act or record why it cannot.
const stopBlockReason = "Work named in a plan is unblocked and nothing is in flight. Do it now, " +
	"or record in the plan why it is blocked or waiting on the human. " +
	"This refusal does not repeat for the same work.\n\n"

// StopBlock builds the stop-hook block decision, appending any caller detail.
func StopBlock(detail string) map[string]any {
	return map[string]any{
		"decision": "block",
		"reason":   stopBlockReason + detail,
	}
}

// StopRefusal records one external stop failure and returns the provider
// response for that occurrence. The first occurrence blocks; later
// occurrences remain visible without keeping the turn open.
func StopRefusal(path, session, cause, remedy, detail, systemMessage string, now time.Time) (map[string]any, error) {
	if path == "" || session == "" || cause == "" || remedy == "" {
		return nil, fmt.Errorf("stop refusal requires a record path, session, cause, and remedy")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("prepare stop-refusal directory: %w", err)
	}
	lockFile, err := os.OpenFile(path+".lock", os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open stop-refusal lock: %w", err)
	}
	defer lockFile.Close()
	if err := unix.Flock(int(lockFile.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		deadline := time.Now().Add(100 * time.Millisecond)
		for err != nil && time.Now().Before(deadline) {
			time.Sleep(10 * time.Millisecond)
			err = unix.Flock(int(lockFile.Fd()), unix.LOCK_EX|unix.LOCK_NB)
		}
		if err != nil {
			return nil, fmt.Errorf("lock stop-refusal record: busy after 100 milliseconds: %w", err)
		}
	}
	defer func() { _ = unix.Flock(int(lockFile.Fd()), unix.LOCK_UN) }()

	record := stopRefusalRecord{SchemaVersion: 1, SessionID: session, Causes: map[string]stopRefusalCause{}}
	if data, readErr := os.ReadFile(path); readErr == nil {
		if err := json.Unmarshal(data, &record); err != nil {
			return nil, fmt.Errorf("read stop-refusal record: %w", err)
		}
		if record.SchemaVersion != 1 || record.SessionID != session || record.Causes == nil {
			return nil, fmt.Errorf("read stop-refusal record: unexpected schema or session")
		}
	} else if !os.IsNotExist(readErr) {
		return nil, fmt.Errorf("read stop-refusal record: %w", readErr)
	}

	digest := fmt.Sprintf("%x", sha256.Sum256([]byte(cause)))
	stamp := now.UTC().Format(time.RFC3339)
	entry, repeated := record.Causes[digest]
	if repeated && entry.Cause != cause {
		return nil, fmt.Errorf("read stop-refusal record: cause digest does not match its cause")
	}
	if !repeated {
		entry = stopRefusalCause{Cause: cause, FirstAt: stamp}
	}
	entry.Count++
	entry.LastAt = stamp
	record.Causes[digest] = entry
	encoded, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("render stop-refusal record: %w", err)
	}
	if _, err := atomicfile.WriteText(path, string(encoded)+"\n", ""); err != nil {
		return nil, fmt.Errorf("write stop-refusal record: %w", err)
	}

	if !repeated {
		response := StopBlock(detail)
		if systemMessage != "" {
			response["systemMessage"] = systemMessage
		}
		return response, nil
	}
	message := fmt.Sprintf("Metasystem allowed this repeated external stop failure to surface without blocking (occurrence %d).\nCause: %s\nRemedy: %s", entry.Count, cause, remedy)
	if systemMessage != "" {
		message += "\n" + systemMessage
	}
	return map[string]any{"systemMessage": message}, nil
}

type stopRefusalRecord struct {
	SchemaVersion int                         `json:"schemaVersion"`
	SessionID     string                      `json:"sessionId"`
	Causes        map[string]stopRefusalCause `json:"causes"`
}

type stopRefusalCause struct {
	Cause   string `json:"cause"`
	Count   int    `json:"count"`
	FirstAt string `json:"firstAt"`
	LastAt  string `json:"lastAt"`
}
