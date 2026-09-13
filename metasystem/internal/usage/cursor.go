package usage

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

const maxCallLineBytes = 32 * 1024 * 1024

// CursorBusyError reports that a caller requiring an immediate answer could
// not acquire the session cursor lock.
type CursorBusyError struct {
	Path string
}

func (e *CursorBusyError) Error() string {
	return fmt.Sprintf("call cursor is busy: %s", e.Path)
}

// SessionRegistryBusyError reports that an immediate registration could not
// acquire the shared session-registry lock.
type SessionRegistryBusyError struct {
	Path string
}

func (e *SessionRegistryBusyError) Error() string {
	return fmt.Sprintf("call session registry is busy: %s", e.Path)
}

type lineParser func(line []byte, ordinal int64) (*CallSample, *Marker, bool)

type sampleRow struct {
	Kind string `json:"kind"`
	CallSample
}

type markerRow struct {
	Kind string `json:"kind"`
	Marker
}

var writeCallCursor = atomicWriteJSON

func readUnderCursor(stateRoot, runtime, session, path string, parse lineParser, opts ReadOptions) (Reading, error) {
	lockPath := CursorPath(stateRoot, runtime, session) + ".lock"
	var lock *os.File
	var err error
	if opts.NonBlocking {
		lock, err = tryLockCallFile(lockPath)
	} else {
		lock, err = lockCallFile(lockPath)
	}
	if err != nil {
		return Reading{}, err
	}
	defer unlockCallFile(lock)

	cursorPath := CursorPath(stateRoot, runtime, session)
	samplesPath := SamplesPath(stateRoot, runtime, session)
	cursor, loaded, err := loadCallCursor(cursorPath, runtime, session)
	if err != nil {
		return Reading{}, err
	}
	if err := reconcileCallRows(samplesPath, cursor, loaded); err != nil {
		return Reading{}, fmt.Errorf("cannot reconcile call cursor %s with samples %s: %w", cursorPath, samplesPath, err)
	}
	previousReadAt := time.Time{}
	if loaded {
		previousReadAt = cursor.LastReadAt
	}
	if !loaded {
		cursor = freshCallCursor(runtime, session, path)
	}

	info, err := os.Lstat(path)
	if err != nil {
		return Reading{}, fmt.Errorf("cannot inspect call stream %s: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		return Reading{}, fmt.Errorf("call stream is not a regular file: %s", path)
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return Reading{}, fmt.Errorf("cannot read file identity for call stream %s", path)
	}
	dev, inode := uint64(stat.Dev), uint64(stat.Ino)
	if loaded && cursor.Path == path && cursor.Dev == dev && cursor.Inode == inode && info.Size() == cursor.Offset {
		reading := readingFromCursor(opts.Capability, cursor, 0, 0)
		reading.PreviousReadAt = previousReadAt
		return reading, nil
	}

	restarted := ""
	committedBytes := cursor.SamplesBytes
	if loaded && cursor.Path != path {
		restarted = "path changed"
		cursor = freshCallCursor(runtime, session, path)
		cursor.SamplesBytes = committedBytes
	} else if loaded && (cursor.Dev != dev || cursor.Inode != inode) {
		restarted = "inode changed"
		cursor = freshCallCursor(runtime, session, path)
		cursor.SamplesBytes = committedBytes
	} else if loaded && info.Size() < cursor.Offset {
		restarted = "truncated"
		cursor = freshCallCursor(runtime, session, path)
		cursor.SamplesBytes = committedBytes
	}
	cursor.Path = path
	cursor.Dev = dev
	cursor.Inode = inode

	stream, err := os.Open(path)
	observeCallOpen(path)
	if err != nil {
		return Reading{}, fmt.Errorf("cannot open call stream %s: %w", path, err)
	}
	defer stream.Close()
	if _, err := stream.Seek(cursor.Offset, io.SeekStart); err != nil {
		return Reading{}, fmt.Errorf("cannot seek call stream %s to %d: %w", path, cursor.Offset, err)
	}
	remaining := info.Size() - cursor.Offset
	reader := io.MultiReader(bytes.NewReader(cursor.Tail), io.LimitReader(observedCallReader{reader: stream}, remaining))
	cursor.Tail = nil

	newSamples := 0
	newMarkers := 0
	newSidechains := int64(0)
	var pendingRows []any
	buffered := bufio.NewReader(reader)
	for {
		line, readErr := buffered.ReadBytes('\n')
		if len(line) > maxCallLineBytes {
			return Reading{}, fmt.Errorf("cannot scan call stream %s: token too long", path)
		}
		if readErr == nil {
			cursor.Line++
			sample, marker, sidechain := parse(bytes.TrimSuffix(line, []byte{'\n'}), cursor.Line)
			if sidechain {
				cursor.SidechainCount++
				newSidechains++
			}
			if sample != nil && !cursor.Seen[sample.InvocationID] {
				if restarted != "" {
					sample.Source += "; restarted: " + restarted
				}
				pendingRows = append(pendingRows, sampleRow{Kind: "sample", CallSample: *sample})
				cursor.Seen[sample.InvocationID] = true
				cursor.SampleCount++
				cursor.Latest = sample
				newSamples++
			}
			if marker != nil {
				pendingRows = append(pendingRows, markerRow{Kind: "marker", Marker: *marker})
				cursor.CompactionCount++
				cursor.LastMarker = marker
				newMarkers++
			}
			continue
		}
		if readErr != io.EOF {
			return Reading{}, fmt.Errorf("cannot read call stream %s: %w", path, readErr)
		}
		if len(line) > 0 {
			cursor.Tail = append([]byte(nil), line...)
		}
		break
	}
	if cursor.SidechainCount > 0 && newSamples > 0 {
		suffix := fmt.Sprintf("; sidechain records skipped: %d", cursor.SidechainCount)
		cursor.Latest.Source = withoutSidechainCount(cursor.Latest.Source) + suffix
		for index, pending := range pendingRows {
			if row, ok := pending.(sampleRow); ok {
				row.Source = withoutSidechainCount(row.Source) + suffix
				pendingRows[index] = row
			}
		}
	} else if newSidechains > 0 && cursor.Latest != nil {
		cursor.Latest.Source = withoutSidechainCount(cursor.Latest.Source) + fmt.Sprintf("; sidechain records skipped: %d", cursor.SidechainCount)
	}

	cursor.Size = info.Size()
	cursor.Offset = info.Size()
	cursor.LastReadAt = opts.Now
	if cursor.LastReadAt.IsZero() {
		cursor.LastReadAt = time.Now().UTC()
	}
	if !loaded && len(pendingRows) > 0 {
		checkpoint := freshCallCursor(runtime, session, path)
		checkpoint.Dev = dev
		checkpoint.Inode = inode
		if err := writeCallCursor(cursorPath, checkpoint); err != nil {
			return Reading{}, fmt.Errorf("cannot write initial call cursor %s: %w", cursorPath, err)
		}
	}
	newSamplesBytes, err := appendCallRows(samplesPath, pendingRows, cursor.SamplesBytes)
	if err != nil {
		return Reading{}, err
	}
	cursor.SamplesBytes = newSamplesBytes
	if err := writeCallCursor(cursorPath, cursor); err != nil {
		return Reading{}, fmt.Errorf("cannot write call cursor %s: %w", cursorPath, err)
	}
	reading := readingFromCursor(opts.Capability, cursor, newSamples, newMarkers)
	reading.PreviousReadAt = previousReadAt
	return reading, nil
}

func Calls(stateRoot, runtime, session string, since time.Time) ([]CallSample, []Marker, error) {
	if err := validateCallLocation(stateRoot, runtime); err != nil {
		return nil, nil, err
	}
	lock, err := lockCallFile(CursorPath(stateRoot, runtime, session) + ".lock")
	if err != nil {
		return nil, nil, err
	}
	defer unlockCallFile(lock)

	cursorPath := CursorPath(stateRoot, runtime, session)
	path := SamplesPath(stateRoot, runtime, session)
	cursor, loaded, err := loadCallCursor(cursorPath, runtime, session)
	if err != nil {
		return nil, nil, err
	}
	if err := reconcileCallRows(path, cursor, loaded); err != nil {
		return nil, nil, fmt.Errorf("cannot reconcile call cursor %s with samples %s: %w", cursorPath, path, err)
	}
	if !loaded || cursor.SamplesBytes == 0 {
		return nil, nil, nil
	}

	observeCallOpen(path)
	file, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, fmt.Errorf("cannot open call samples %s: %w", path, err)
	}
	defer file.Close()

	var samples []CallSample
	var markers []Marker
	scanner := bufio.NewScanner(io.LimitReader(file, cursor.SamplesBytes))
	scanner.Buffer(make([]byte, 0, 64*1024), maxCallLineBytes)
	for scanner.Scan() {
		var header struct {
			Kind string `json:"kind"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &header); err != nil {
			continue
		}
		switch header.Kind {
		case "sample":
			var row sampleRow
			if err := json.Unmarshal(scanner.Bytes(), &row); err == nil && !row.At.Before(since) {
				samples = append(samples, row.CallSample)
			}
		case "marker":
			var row markerRow
			if err := json.Unmarshal(scanner.Bytes(), &row); err == nil && !row.At.Before(since) {
				row.Marker.Kind = "compaction"
				markers = append(markers, row.Marker)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, nil, fmt.Errorf("cannot read call samples %s: %w", path, err)
	}
	return samples, markers, nil
}

func RegisterSession(stateRoot, runtime, session string, pid, pidStartedAt int64) error {
	return registerSession(stateRoot, runtime, session, pid, pidStartedAt, false)
}

// RegisterSessionNonBlocking records a session only when the registry lock is
// immediately available.
func RegisterSessionNonBlocking(stateRoot, runtime, session string, pid, pidStartedAt int64) error {
	return registerSession(stateRoot, runtime, session, pid, pidStartedAt, true)
}

func registerSession(stateRoot, runtime, session string, pid, pidStartedAt int64, nonBlocking bool) error {
	if err := validateCallLocation(stateRoot, runtime); err != nil {
		return err
	}
	path := filepath.Join(stateRoot, "artifacts", "agents", "context", "sessions.jsonl")
	lockPath := path + ".lock"
	var lock *os.File
	var err error
	if nonBlocking {
		lock, err = tryLockCallFile(lockPath)
		var busy *CursorBusyError
		if errors.As(err, &busy) {
			return &SessionRegistryBusyError{Path: busy.Path}
		}
	} else {
		lock, err = lockCallFile(lockPath)
	}
	if err != nil {
		return err
	}
	defer unlockCallFile(lock)

	needsSeparator := false
	observeCallOpen(path)
	file, err := os.Open(path)
	if err == nil {
		info, statErr := file.Stat()
		if statErr != nil {
			file.Close()
			return fmt.Errorf("cannot inspect session registry %s: %w", path, statErr)
		}
		if info.Size() > 0 {
			var last [1]byte
			if _, readErr := file.ReadAt(last[:], info.Size()-1); readErr != nil {
				file.Close()
				return fmt.Errorf("cannot inspect session registry tail %s: %w", path, readErr)
			}
			needsSeparator = last[0] != '\n'
		}
		scanner := bufio.NewScanner(file)
		scanner.Buffer(make([]byte, 0, 64*1024), maxCallLineBytes)
		for scanner.Scan() {
			var row CallRegistration
			if json.Unmarshal(scanner.Bytes(), &row) == nil && row.Runtime == runtime && row.Session == session && row.PID == pid && row.PIDStartedAt == pidStartedAt {
				file.Close()
				return nil
			}
		}
		if err := scanner.Err(); err != nil {
			file.Close()
			return fmt.Errorf("cannot read session registry %s: %w", path, err)
		}
		if err := file.Close(); err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("cannot open session registry %s: %w", path, err)
	}

	row := CallRegistration{Runtime: runtime, Session: session, PID: pid, PIDStartedAt: pidStartedAt, FirstSeen: time.Now().UTC()}
	encoded, err := json.Marshal(row)
	if err != nil {
		return err
	}
	if needsSeparator {
		encoded = append([]byte{'\n'}, encoded...)
	}
	observeCallOpen(path)
	registry, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("cannot append session registry %s: %w", path, err)
	}
	defer registry.Close()
	encoded = append(encoded, '\n')
	written, err := registry.Write(encoded)
	if err == nil && written != len(encoded) {
		err = io.ErrShortWrite
	}
	return err
}

func freshCallCursor(runtime, session, path string) CursorState {
	return CursorState{
		SchemaVersion: 2,
		Runtime:       runtime,
		Session:       session,
		Path:          path,
		Seen:          map[string]bool{},
	}
}

func loadCallCursor(path, runtime, session string) (CursorState, bool, error) {
	observeCallOpen(path)
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return CursorState{}, false, nil
	}
	if err != nil {
		return CursorState{}, false, fmt.Errorf("cannot read call cursor %s: %w", path, err)
	}
	var members map[string]json.RawMessage
	if json.Unmarshal(data, &members) != nil {
		return CursorState{}, false, nil
	}
	for _, member := range []string{"line", "sidechainCount", "samplesBytes"} {
		if !validRequiredCursorNumber(members[member]) {
			return CursorState{}, false, nil
		}
	}
	var cursor CursorState
	if json.Unmarshal(data, &cursor) != nil || !validCallCursor(cursor, runtime, session) {
		return CursorState{}, false, nil
	}
	if cursor.Seen == nil {
		cursor.Seen = map[string]bool{}
	}
	return cursor, true, nil
}

func validRequiredCursorNumber(raw json.RawMessage) bool {
	if len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return false
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if decoder.Decode(&value) != nil {
		return false
	}
	number, ok := value.(json.Number)
	if !ok {
		return false
	}
	_, err := number.Int64()
	return err == nil
}

func validCallCursor(cursor CursorState, runtime, session string) bool {
	if cursor.SchemaVersion != 2 || cursor.Runtime != runtime || cursor.Session != session || cursor.Path == "" {
		return false
	}
	if cursor.Dev == 0 || cursor.Inode == 0 || cursor.Size < 0 || cursor.Offset < 0 || cursor.Size != cursor.Offset {
		return false
	}
	if cursor.SampleCount < 0 || cursor.CompactionCount < 0 || cursor.Line < 0 || cursor.SidechainCount < 0 || cursor.SamplesBytes < 0 || cursor.SidechainCount > cursor.Line || int64(len(cursor.Seen)) != cursor.SampleCount {
		return false
	}
	if (cursor.Latest == nil) != (cursor.SampleCount == 0) || (cursor.LastMarker == nil) != (cursor.CompactionCount == 0) {
		return false
	}
	if cursor.Latest != nil && (cursor.Latest.Runtime != runtime || cursor.Latest.Session != session || cursor.Latest.InvocationID == "" || cursor.Latest.PromptTokens < 0 || cursor.Latest.InputTokens < 0 || cursor.Latest.CacheCreation < 0 || cursor.Latest.CacheRead < 0 || cursor.Latest.Ordinal < 0 || cursor.Latest.Source == "") {
		return false
	}
	if cursor.Latest != nil && !cursor.Seen[cursor.Latest.InvocationID] {
		return false
	}
	for _, seen := range cursor.Seen {
		if !seen {
			return false
		}
	}
	if cursor.LastMarker != nil && (cursor.LastMarker.Runtime != runtime || cursor.LastMarker.Session != session || cursor.LastMarker.Kind != "compaction" || cursor.LastMarker.Ordinal < 0) {
		return false
	}
	if len(cursor.Tail) > maxCallLineBytes || int64(len(cursor.Tail)) > cursor.Offset || cursor.Line > cursor.Offset-int64(len(cursor.Tail)) {
		return false
	}
	if runtime == "claude" {
		if cursor.Latest != nil && (cursor.Latest.Ordinal < 1 || cursor.Latest.Ordinal > cursor.Line) {
			return false
		}
		if cursor.LastMarker != nil && (cursor.LastMarker.Ordinal < 1 || cursor.LastMarker.Ordinal > cursor.Line) {
			return false
		}
	}
	return true
}

func reconcileCallRows(path string, cursor CursorState, loaded bool) error {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		if loaded && cursor.SamplesBytes > 0 {
			return fmt.Errorf("call samples are missing below committed boundary %d", cursor.SamplesBytes)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("cannot inspect call samples: %w", err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("call samples path is not a regular file")
	}
	if !loaded {
		if info.Size() == 0 {
			return nil
		}
		return fmt.Errorf("committed boundary is unavailable for a nonempty samples log")
	}
	if info.Size() < cursor.SamplesBytes {
		return fmt.Errorf("call samples length %d is below committed boundary %d", info.Size(), cursor.SamplesBytes)
	}
	if info.Size() == cursor.SamplesBytes {
		return nil
	}
	observeCallOpen(path)
	file, err := os.OpenFile(path, os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("cannot open call samples for recovery: %w", err)
	}
	if err := file.Truncate(cursor.SamplesBytes); err != nil {
		_ = file.Close()
		return fmt.Errorf("cannot truncate uncommitted call samples: %w", err)
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return fmt.Errorf("cannot sync recovered call samples: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("cannot close recovered call samples: %w", err)
	}
	return nil
}

func appendCallRows(path string, rows []any, committedBytes int64) (int64, error) {
	if len(rows) == 0 {
		return committedBytes, nil
	}
	var encodedRows bytes.Buffer
	for _, row := range rows {
		encoded, err := json.Marshal(row)
		if err != nil {
			return committedBytes, fmt.Errorf("cannot encode call sample row: %w", err)
		}
		encodedRows.Write(encoded)
		encodedRows.WriteByte('\n')
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return committedBytes, fmt.Errorf("cannot create call samples directory: %w", err)
	}
	observeCallOpen(path)
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return committedBytes, fmt.Errorf("cannot append call samples: %w", err)
	}
	info, statErr := file.Stat()
	if statErr != nil {
		_ = file.Close()
		return committedBytes, fmt.Errorf("cannot inspect call samples before append: %w", statErr)
	}
	if !info.Mode().IsRegular() || info.Size() != committedBytes {
		_ = file.Close()
		return committedBytes, fmt.Errorf("call samples length %d does not equal committed boundary %d", info.Size(), committedBytes)
	}
	written, writeErr := file.Write(encodedRows.Bytes())
	if writeErr == nil && written != encodedRows.Len() {
		writeErr = io.ErrShortWrite
	}
	if writeErr == nil {
		writeErr = file.Sync()
	}
	closeErr := file.Close()
	if writeErr != nil {
		return committedBytes, fmt.Errorf("cannot append call samples: %w", writeErr)
	}
	if closeErr != nil {
		return committedBytes, fmt.Errorf("cannot close call samples: %w", closeErr)
	}
	return committedBytes + int64(encodedRows.Len()), nil
}

func readingFromCursor(capability Capability, cursor CursorState, newSamples, newMarkers int) Reading {
	reading := Reading{
		Capability: capability,
		Latest:     cursor.Latest,
		NewSamples: newSamples,
		NewMarkers: newMarkers,
		Cursor:     cursor,
	}
	if reading.Latest == nil {
		if cursor.SampleCount == 0 && cursor.SidechainCount > 0 {
			reading.Reason = "unknown (only sidechain records so far)"
		} else if cursor.SampleCount == 0 {
			reading.Reason = "unknown (no call recorded yet)"
		}
	}
	return reading
}

const sidechainSourceLabel = "; sidechain records skipped: "

func withoutSidechainCount(source string) string {
	position := strings.LastIndex(source, sidechainSourceLabel)
	if position < 0 {
		return source
	}
	return source[:position]
}

func lockCallFile(path string) (*os.File, error) {
	return lockCallFileWithFlags(path, unix.LOCK_EX)
}

func tryLockCallFile(path string) (*os.File, error) {
	return lockCallFileWithFlags(path, unix.LOCK_EX|unix.LOCK_NB)
}

func lockCallFileWithFlags(path string, flags int) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	observeCallOpen(path)
	lock, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	if err := unix.Flock(int(lock.Fd()), flags); err != nil {
		lock.Close()
		if flags&unix.LOCK_NB != 0 && (errors.Is(err, unix.EWOULDBLOCK) || errors.Is(err, unix.EAGAIN)) {
			return nil, &CursorBusyError{Path: path}
		}
		return nil, err
	}
	return lock, nil
}

func unlockCallFile(lock *os.File) {
	_ = unix.Flock(int(lock.Fd()), unix.LOCK_UN)
	_ = lock.Close()
}

type observedCallReader struct {
	reader io.Reader
}

func (reader observedCallReader) Read(buffer []byte) (int, error) {
	count, err := reader.reader.Read(buffer)
	if callBytesRead != nil {
		callBytesRead(count)
	}
	return count, err
}
