package usage

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// CallSession is one runtime/session pair with a trustworthy persisted cursor.
type CallSession struct {
	Runtime string
	Session string
}

// CallRegistration is one process-to-session observation from the complete
// session registry. FirstSeen belongs to the registry event, not a call.
type CallRegistration struct {
	Runtime      string    `json:"runtime"`
	Session      string    `json:"session"`
	PID          int64     `json:"pid"`
	PIDStartedAt int64     `json:"pidStartedAt"`
	FirstSeen    time.Time `json:"firstSeen"`
}

// CallSessions discovers persisted call stores without reading a sample body.
// Cursor identity is authoritative because a filename may contain a hashed
// session or a hyphenated runtime and therefore cannot be reversed safely.
func CallSessions(stateRoot string) ([]CallSession, error) {
	if !filepath.IsAbs(stateRoot) {
		return nil, fmt.Errorf("state root must be absolute: %s", stateRoot)
	}
	maintenance, err := lockCallMaintenance(stateRoot, false, false)
	if err != nil {
		return nil, err
	}
	defer unlockCallFile(maintenance)
	return callSessionsUnderMaintenance(stateRoot)
}

func callSessionsUnderMaintenance(stateRoot string) ([]CallSession, error) {
	cursorDir := filepath.Join(stateRoot, "artifacts", "agents", "context", "cursors")
	samplesDir := filepath.Join(stateRoot, "artifacts", "agents", "context", "samples")
	stems := map[string]bool{}
	if err := collectCallStoreStems(cursorDir, ".json", stems); err != nil {
		return nil, err
	}
	if err := collectCallStoreStems(samplesDir, ".jsonl", stems); err != nil {
		return nil, err
	}

	names := make([]string, 0, len(stems))
	for stem := range stems {
		names = append(names, stem)
	}
	sort.Strings(names)

	var sessions []CallSession
	for _, stem := range names {
		cursorPath := filepath.Join(cursorDir, stem+".json")
		samplesPath := filepath.Join(samplesDir, stem+".jsonl")
		lock, err := lockCallFile(cursorPath + ".lock")
		if err != nil {
			return nil, fmt.Errorf("cannot lock call session pair cursor=%s samples=%s: %w", cursorPath, samplesPath, err)
		}
		recoverErr := recoverCallRetirement(stateRoot, cursorPath)
		if recoverErr != nil {
			unlockCallFile(lock)
			return nil, recoverErr
		}
		session, present, inspectErr := inspectCallSessionPair(stateRoot, cursorPath, samplesPath)
		unlockCallFile(lock)
		if inspectErr != nil {
			return nil, inspectErr
		}
		if present {
			sessions = append(sessions, session)
		}
	}
	sort.Slice(sessions, func(i, j int) bool {
		if sessions[i].Runtime != sessions[j].Runtime {
			return sessions[i].Runtime < sessions[j].Runtime
		}
		return sessions[i].Session < sessions[j].Session
	})
	return sessions, nil
}

func collectCallStoreStems(directory, extension string, stems map[string]bool) error {
	entries, err := os.ReadDir(directory)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("cannot list call store %s: %w", directory, err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if extension == ".json" && strings.HasSuffix(name, ".json.retiring.json") {
			stem := strings.TrimSuffix(name, ".json.retiring.json")
			if callStorePathHasValidCursor(filepath.Join(directory, name)) {
				stem = strings.TrimSuffix(name, ".json")
			}
			if stem != "" {
				stems[stem] = true
			}
			continue
		}
		if !strings.HasSuffix(name, extension) {
			continue
		}
		stem := strings.TrimSuffix(name, extension)
		if stem == "" {
			continue
		}
		stems[stem] = true
	}
	return nil
}

func inspectCallSessionPair(stateRoot, cursorPath, samplesPath string) (CallSession, bool, error) {
	cursorInfo, cursorExists, err := callStoreMember(cursorPath)
	if err != nil {
		return CallSession{}, false, pairError(cursorPath, samplesPath, err)
	}
	samplesInfo, samplesExists, err := callStoreMember(samplesPath)
	if err != nil {
		return CallSession{}, false, pairError(cursorPath, samplesPath, err)
	}
	if samplesExists && samplesInfo.Mode().Perm()&0o444 == 0 {
		return CallSession{}, false, pairError(cursorPath, samplesPath, fmt.Errorf("call samples are unreadable: %s", samplesPath))
	}
	if !cursorExists {
		if samplesExists && samplesInfo.Size() > 0 {
			return CallSession{}, false, pairError(cursorPath, samplesPath, fmt.Errorf("nonempty call samples have no trustworthy cursor identity"))
		}
		return CallSession{}, false, nil
	}
	if cursorInfo.Mode().Perm()&0o444 == 0 {
		return CallSession{}, false, pairError(cursorPath, samplesPath, fmt.Errorf("call cursor is unreadable: %s", cursorPath))
	}
	cursor, err := readDiscoveredCallCursor(cursorPath)
	if err != nil {
		return CallSession{}, false, pairError(cursorPath, samplesPath, err)
	}
	if err := validateCallLocation(stateRoot, cursor.Runtime); err != nil {
		return CallSession{}, false, pairError(cursorPath, samplesPath, err)
	}
	if CursorPath(stateRoot, cursor.Runtime, cursor.Session) != cursorPath ||
		SamplesPath(stateRoot, cursor.Runtime, cursor.Session) != samplesPath {
		return CallSession{}, false, pairError(cursorPath, samplesPath,
			fmt.Errorf("cursor identity %s/%s does not map to both store basenames", cursor.Runtime, cursor.Session))
	}
	return CallSession{Runtime: cursor.Runtime, Session: cursor.Session}, true, nil
}

func callStoreMember(path string) (os.FileInfo, bool, error) {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("cannot inspect call store member %s: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		return nil, false, fmt.Errorf("call store member is not a regular file: %s", path)
	}
	return info, true, nil
}

func readDiscoveredCallCursor(path string) (CursorState, error) {
	observeCallOpen(path)
	data, err := os.ReadFile(path)
	if err != nil {
		return CursorState{}, fmt.Errorf("cannot read call cursor %s: %w", path, err)
	}
	var members map[string]json.RawMessage
	if err := json.Unmarshal(data, &members); err != nil {
		return CursorState{}, fmt.Errorf("call cursor is malformed: %w", err)
	}
	for _, member := range []string{"line", "sidechainCount", "samplesBytes"} {
		if !validRequiredCursorNumber(members[member]) {
			return CursorState{}, fmt.Errorf("call cursor has invalid %s", member)
		}
	}
	var cursor CursorState
	if err := json.Unmarshal(data, &cursor); err != nil || !validCallCursor(cursor, cursor.Runtime, cursor.Session) {
		if err != nil {
			return CursorState{}, fmt.Errorf("call cursor is malformed: %w", err)
		}
		return CursorState{}, fmt.Errorf("call cursor identity or committed boundary is invalid")
	}
	return cursor, nil
}

func pairError(cursorPath, samplesPath string, err error) error {
	return fmt.Errorf("call session pair cursor=%s samples=%s: %w", cursorPath, samplesPath, err)
}

// CallRegistrations reads a strict, complete registry snapshot while holding
// its sibling lock. The lock is released before this function returns, so a
// caller can acquire per-session cursor locks without reversing lock order.
func CallRegistrations(stateRoot string) (rows []CallRegistration, present bool, err error) {
	if !filepath.IsAbs(stateRoot) {
		return nil, false, fmt.Errorf("state root must be absolute: %s", stateRoot)
	}
	maintenance, err := lockCallMaintenance(stateRoot, false, false)
	if err != nil {
		return nil, false, err
	}
	defer unlockCallFile(maintenance)
	return callRegistrationsUnderMaintenance(stateRoot)
}

func callRegistrationsUnderMaintenance(stateRoot string) (rows []CallRegistration, present bool, err error) {
	path := filepath.Join(stateRoot, "artifacts", "agents", "context", "sessions.jsonl")
	lock, err := lockCallFile(path + ".lock")
	if err != nil {
		return nil, false, err
	}
	defer unlockCallFile(lock)

	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("cannot inspect session registry %s: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		return nil, true, fmt.Errorf("session registry is not a regular file: %s", path)
	}
	observeCallOpen(path)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, true, fmt.Errorf("cannot read session registry %s: %w", path, err)
	}
	if len(data) == 0 {
		return nil, true, nil
	}
	if data[len(data)-1] != '\n' {
		return nil, true, fmt.Errorf("session registry %s ends with an incomplete row", path)
	}
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 0, 64*1024), maxCallLineBytes)
	line := 0
	for scanner.Scan() {
		line++
		var row CallRegistration
		decoder := json.NewDecoder(bytes.NewReader(scanner.Bytes()))
		if decodeErr := decoder.Decode(&row); decodeErr != nil {
			return nil, true, fmt.Errorf("session registry %s row %d is malformed: %w", path, line, decodeErr)
		}
		if decodeErr := decoder.Decode(&struct{}{}); decodeErr != io.EOF {
			if decodeErr == nil {
				decodeErr = fmt.Errorf("multiple JSON values")
			}
			return nil, true, fmt.Errorf("session registry %s row %d is malformed: %w", path, line, decodeErr)
		}
		if !runtimeNamePattern.MatchString(row.Runtime) || row.Session == "" || row.PID < 1 || row.PIDStartedAt < 1 || row.FirstSeen.IsZero() {
			return nil, true, fmt.Errorf("session registry %s row %d is incomplete", path, line)
		}
		rows = append(rows, row)
	}
	if err := scanner.Err(); err != nil {
		return nil, true, fmt.Errorf("cannot read session registry %s: %w", path, err)
	}
	return rows, true, nil
}
