package spend

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/mission"
)

type transcriptRequest struct {
	id, file, session, cwd, model, timestamp string
	line                                     int
	usage                                    map[string]any
	detail                                   string
	dayEligible                              bool
}

var transcriptBytesRead func(int)

type observedTranscriptReader struct {
	reader io.Reader
}

func (r observedTranscriptReader) Read(buffer []byte) (int, error) {
	count, err := r.reader.Read(buffer)
	if count > 0 && transcriptBytesRead != nil {
		transcriptBytesRead(count)
	}
	return count, err
}

func readSeat(repoRoot, machine string, now time.Time, delegates map[string]bool, settings config.SpendSettings) ([]pricedMeasurement, SeatSummary, []UnmeasuredEntry, error) {
	seat := SeatSummary{CodexUnmeasured: true}
	visitedCursorPaths := map[string]bool{}
	var unmeasured []UnmeasuredEntry
	recordUnreadable := func(path string, err error) {
		displayPath := path
		if filepath.IsAbs(path) {
			displayPath = relativePath(repoRoot, path)
		}
		seat.UnreadableFiles++
		unmeasured = append(unmeasured, UnmeasuredEntry{
			ID: displayPath, File: displayPath, Goal: "seat", Machine: machine,
			Provenance: "seat unreadable", Detail: err.Error(),
		})
	}
	toplevel, err := gitToplevel(repoRoot)
	if err != nil {
		recordUnreadable(repoRoot, err)
		return nil, seat, unmeasured, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		recordUnreadable("~", fmt.Errorf("cannot resolve home directory: %w", err))
		return nil, seat, unmeasured, nil
	}
	slug := strings.ReplaceAll(toplevel, string(filepath.Separator), "-")
	projects := filepath.Join(home, ".claude", "projects")
	dirs, err := os.ReadDir(projects)
	if os.IsNotExist(err) {
		seat.CacheWriteFailures += pruneTranscriptCursors(repoRoot, visitedCursorPaths)
		return nil, seat, nil, nil
	}
	if err != nil {
		recordUnreadable(projects, fmt.Errorf("cannot list Claude transcript root %s: %w", projects, err))
		return nil, seat, unmeasured, nil
	}
	var files []string
	for _, dir := range dirs {
		if !dir.IsDir() || (dir.Name() != slug && !strings.HasPrefix(dir.Name(), slug+"-")) {
			continue
		}
		dirPath := filepath.Join(projects, dir.Name())
		entries, listErr := os.ReadDir(dirPath)
		if listErr != nil {
			recordUnreadable(dirPath, fmt.Errorf("cannot list Claude transcript slug %s: %w", dirPath, listErr))
			continue
		}
		for _, entry := range entries {
			if filepath.Ext(entry.Name()) == ".jsonl" {
				files = append(files, filepath.Join(dirPath, entry.Name()))
			}
		}
	}
	sort.Strings(files)
	delegateDigest := delegateSessionDigest(delegates)
	requests := map[string]transcriptRequest{}
	var invalid []transcriptRequest
	for _, path := range files {
		fileSession := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		if delegates[fileSession] {
			continue
		}
		info, statErr := os.Stat(path)
		if statErr != nil {
			recordUnreadable(path, fmt.Errorf("cannot stat Claude transcript %s: %w", path, statErr))
			continue
		}
		dayEligible := !info.ModTime().Before(now.Add(-48 * time.Hour))
		visitedCursorPaths[transcriptCursorPath(repoRoot, path)] = true
		fileRequests, fileInvalid, foreign, cacheWriteFailed, readErr := readTranscriptCursor(repoRoot, path, info, toplevel, dayEligible, delegates, delegateDigest)
		if cacheWriteFailed {
			seat.CacheWriteFailures++
		}
		if readErr != nil {
			recordUnreadable(path, readErr)
			continue
		}
		if foreign {
			seat.SkippedForeignFiles++
			continue
		}
		seat.Files++
		if !dayEligible {
			seat.AgedFiles++
		}
		for key, request := range fileRequests {
			requests[key] = request
		}
		invalid = append(invalid, fileInvalid...)
	}
	seat.CacheWriteFailures += pruneTranscriptCursors(repoRoot, visitedCursorPaths)

	ordered := make([]transcriptRequest, 0, len(requests)+len(invalid))
	for _, request := range requests {
		ordered = append(ordered, request)
	}
	ordered = append(ordered, invalid...)
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].file != ordered[j].file {
			return ordered[i].file < ordered[j].file
		}
		return ordered[i].line < ordered[j].line
	})
	var measured []pricedMeasurement
	for _, request := range ordered {
		id := request.id
		if id == "" {
			id = fmt.Sprintf("%s:%d", filepath.Base(request.file), request.line)
		}
		if request.detail != "" {
			entry := seatUnmeasured(repoRoot, request, id, request.detail)
			entry.Machine = machine
			unmeasured = append(unmeasured, entry)
			seat.UnmeasuredRequests++
			continue
		}
		timestamp, stampErr := parseTime(request.timestamp)
		classes, usageErr := transcriptTokens(request.usage)
		if stampErr != nil || usageErr != nil {
			detail := "unparsable timestamp"
			if usageErr != nil {
				detail = usageErr.Error()
			}
			entry := seatUnmeasured(repoRoot, request, id, detail)
			entry.Machine = machine
			unmeasured = append(unmeasured, entry)
			seat.UnmeasuredRequests++
			continue
		}
		tokens := tokensFromMission(classes)
		model := config.CanonicalModel(request.model)
		money, priced, unpriced, foreign := price("claude", model, classes, (*mission.UsageCost)(nil), false, settings)
		item := pricedMeasurement{
			goal: "seat", machine: machine, day: timestamp.Format("2006-01-02"), runtime: "claude", model: model,
			tokens: tokens, money: money, priced: priced, unpriced: unpriced, foreign: foreign, dayEligible: request.dayEligible,
		}
		measured = append(measured, item)
		seat.LifetimeTokens += tokens.Total()
		seat.UnattributedRequests++
		if item.day == now.Format("2006-01-02") && item.dayEligible {
			seat.DayTokens += tokens.Total()
		}
	}
	return measured, seat, unmeasured, nil
}

func readTranscriptCursor(repoRoot, path string, info os.FileInfo, toplevel string, dayEligible bool, delegates map[string]bool, delegateDigest string) (map[string]transcriptRequest, []transcriptRequest, bool, bool, error) {
	cachePath := transcriptCursorPath(repoRoot, path)
	cache, valid := loadTranscriptCursor(cachePath, path)
	valid = valid && cache.DelegateDigest == delegateDigest
	unchanged := valid && cache.Size == info.Size() && cache.ModTimeNanos == info.ModTime().UnixNano()
	grown := valid && cache.Size < info.Size()
	if valid && cache.FirstCWD != "" && !seatCWD(toplevel, cache.FirstCWD) {
		if grown {
			return nil, nil, true, false, nil
		}
		if unchanged {
			return nil, nil, true, false, nil
		}
	}
	if unchanged {
		requests, invalid, foreign := transcriptCursorSnapshot(cache, path, toplevel, dayEligible, delegates)
		return requests, invalid, foreign, false, nil
	}
	if !grown {
		cache = transcriptCursorCache{
			SchemaVersion: spendCacheSchemaVersion, Path: path, DelegateDigest: delegateDigest,
			Requests: map[string]cachedTranscriptRequest{}, Invalid: []cachedTranscriptRequest{},
		}
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, nil, false, false, fmt.Errorf("cannot read Claude transcript %s: %w", path, err)
	}
	defer file.Close()
	if _, err := file.Seek(cache.Offset, io.SeekStart); err != nil {
		return nil, nil, false, false, fmt.Errorf("cannot read Claude transcript %s from offset %d: %w", path, cache.Offset, err)
	}
	remaining := info.Size() - cache.Offset
	reader := io.MultiReader(bytes.NewReader(cache.Tail), io.LimitReader(observedTranscriptReader{reader: file}, remaining))
	cache.Tail = nil
	foreign, err := scanTranscriptCursor(&cache, reader, path, toplevel, delegates)
	if err != nil {
		return nil, nil, false, false, fmt.Errorf("cannot scan Claude transcript %s: %w", path, err)
	}
	cache.Size = info.Size()
	cache.Offset = info.Size()
	cache.ModTimeNanos = info.ModTime().UnixNano()
	if foreign {
		cache.Requests = map[string]cachedTranscriptRequest{}
		cache.Invalid = []cachedTranscriptRequest{}
		cache.Tail = nil
	}
	cacheWriteFailed := writeSpendCache(cachePath, cache) != nil
	if foreign {
		return nil, nil, true, cacheWriteFailed, nil
	}
	requests, invalid, foreign := transcriptCursorSnapshot(cache, path, toplevel, dayEligible, delegates)
	return requests, invalid, foreign, cacheWriteFailed, nil
}

func pruneTranscriptCursors(repoRoot string, visited map[string]bool) int {
	directory := spendCacheDir(repoRoot)
	entries, err := os.ReadDir(directory)
	if os.IsNotExist(err) {
		return 0
	}
	if err != nil {
		return 0
	}
	failures := 0
	for _, entry := range entries {
		name := strings.TrimSuffix(entry.Name(), ".json")
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" || len(name) != sha256.Size*2 {
			continue
		}
		if _, err := hex.DecodeString(name); err != nil {
			continue
		}
		path := filepath.Join(directory, entry.Name())
		if !visited[path] {
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				failures++
			}
		}
	}
	return failures
}

const maxTranscriptLineBytes = 32 * 1024 * 1024

func scanTranscriptCursor(cache *transcriptCursorCache, reader io.Reader, path, toplevel string, delegates map[string]bool) (bool, error) {
	buffered := bufio.NewReader(reader)
	for {
		line, err := buffered.ReadBytes('\n')
		if len(line) > maxTranscriptLineBytes {
			return false, fmt.Errorf("token too long")
		}
		if err == nil {
			cache.Line++
			if applyTranscriptLine(cache, bytes.TrimSuffix(line, []byte{'\n'}), path, cache.Line, toplevel, delegates) {
				return true, nil
			}
			continue
		}
		if err != io.EOF {
			return false, err
		}
		if len(line) > 0 {
			cache.Tail = append([]byte(nil), line...)
			snapshot := cloneTranscriptCursor(*cache)
			foreign := applyTranscriptLine(&snapshot, line, path, cache.Line+1, toplevel, delegates)
			cache.FirstCWD = snapshot.FirstCWD
			if foreign {
				return true, nil
			}
		}
		return false, nil
	}
}

func applyTranscriptLine(cache *transcriptCursorCache, line []byte, path string, lineNumber int, toplevel string, delegates map[string]bool) bool {
	var raw map[string]any
	decoder := json.NewDecoder(bytes.NewReader(line))
	decoder.UseNumber()
	if err := decoder.Decode(&raw); err != nil {
		cache.Invalid = append(cache.Invalid, cachedTranscriptRequest{Line: lineNumber, Detail: "line is not JSON: " + err.Error()})
		return false
	}
	if cache.FirstCWD == "" {
		cache.FirstCWD = textOr(raw["cwd"], "")
		if cache.FirstCWD != "" && !seatCWD(toplevel, cache.FirstCWD) {
			return true
		}
	}
	if textOr(raw["type"], "") != "assistant" {
		return false
	}
	session := textOr(raw["sessionId"], "")
	if delegates[session] {
		return false
	}
	cwd := textOr(raw["cwd"], "")
	if !seatCWD(toplevel, cwd) {
		return false
	}
	message, _ := raw["message"].(map[string]any)
	request := cachedTranscriptRequest{
		ID: textOr(raw["requestId"], ""), Session: session, CWD: cwd,
		Model: textOr(message["model"], "unknown"), Timestamp: textOr(raw["timestamp"], ""), Line: lineNumber,
	}
	request.Usage, _ = message["usage"].(map[string]any)
	key := request.ID
	if key == "" {
		key = fmt.Sprintf("%s:%d", path, lineNumber)
	}
	cache.Requests[key] = request
	return false
}

func transcriptCursorSnapshot(cache transcriptCursorCache, path, toplevel string, dayEligible bool, delegates map[string]bool) (map[string]transcriptRequest, []transcriptRequest, bool) {
	snapshot := cloneTranscriptCursor(cache)
	if len(snapshot.Tail) > 0 {
		if applyTranscriptLine(&snapshot, snapshot.Tail, path, snapshot.Line+1, toplevel, delegates) {
			return nil, nil, true
		}
	}
	requests := make(map[string]transcriptRequest, len(snapshot.Requests))
	for key, request := range snapshot.Requests {
		requests[key] = transcriptRequestFromCache(path, request, dayEligible)
	}
	invalid := make([]transcriptRequest, 0, len(snapshot.Invalid))
	for _, request := range snapshot.Invalid {
		invalid = append(invalid, transcriptRequestFromCache(path, request, dayEligible))
	}
	return requests, invalid, false
}

func delegateSessionDigest(delegates map[string]bool) string {
	sessions := make([]string, 0, len(delegates))
	for session, delegated := range delegates {
		if delegated {
			sessions = append(sessions, session)
		}
	}
	sort.Strings(sessions)
	hash := sha256.New()
	for _, session := range sessions {
		hash.Write([]byte(session))
		hash.Write([]byte{0})
	}
	return hex.EncodeToString(hash.Sum(nil))
}

func cloneTranscriptCursor(cache transcriptCursorCache) transcriptCursorCache {
	clone := cache
	clone.Requests = make(map[string]cachedTranscriptRequest, len(cache.Requests))
	for key, request := range cache.Requests {
		clone.Requests[key] = request
	}
	clone.Invalid = append([]cachedTranscriptRequest(nil), cache.Invalid...)
	clone.Tail = append([]byte(nil), cache.Tail...)
	return clone
}

func transcriptRequestFromCache(path string, request cachedTranscriptRequest, dayEligible bool) transcriptRequest {
	return transcriptRequest{
		id: request.ID, file: path, session: request.Session, cwd: request.CWD, model: request.Model,
		timestamp: request.Timestamp, line: request.Line, usage: request.Usage, detail: request.Detail, dayEligible: dayEligible,
	}
}

func gitToplevel(repoRoot string) (string, error) {
	current, err := filepath.Abs(repoRoot)
	if err != nil {
		return "", fmt.Errorf("cannot resolve Git toplevel from %s: %w", repoRoot, err)
	}
	current = filepath.Clean(current)
	for {
		gitEntry := filepath.Join(current, ".git")
		info, statErr := os.Stat(gitEntry)
		if statErr == nil {
			if info.IsDir() || info.Mode().IsRegular() {
				return current, nil
			}
			return "", fmt.Errorf("cannot resolve Git toplevel from %s: %s is not a file or directory", repoRoot, gitEntry)
		}
		if !os.IsNotExist(statErr) {
			return "", fmt.Errorf("cannot resolve Git toplevel from %s: cannot stat %s: %w", repoRoot, gitEntry, statErr)
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", fmt.Errorf("cannot resolve Git toplevel from %s: no .git file or directory found", repoRoot)
		}
		current = parent
	}
}

func seatCWD(toplevel, cwd string) bool {
	if cwd == "" {
		return false
	}
	abs, err := filepath.Abs(cwd)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(toplevel, abs)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return false
	}
	parts := strings.Split(filepath.Clean(rel), string(filepath.Separator))
	for index := 0; index+2 < len(parts); index++ {
		if parts[index] == "artifacts" && parts[index+1] == "agents" && parts[index+2] == "worktrees" {
			return false
		}
	}
	return true
}

func transcriptTokens(usage map[string]any) (map[string]float64, error) {
	if usage == nil {
		return nil, fmt.Errorf("message.usage is not an object")
	}
	input, ok := transcriptNumber(usage["input_tokens"])
	if !ok {
		return nil, fmt.Errorf("message.usage.input_tokens is not a non-negative number")
	}
	output, ok := transcriptNumber(usage["output_tokens"])
	if !ok {
		return nil, fmt.Errorf("message.usage.output_tokens is not a non-negative number")
	}
	creation := 0.0
	if raw, present := usage["cache_creation_input_tokens"]; present {
		var valid bool
		creation, valid = transcriptNumber(raw)
		if !valid {
			return nil, fmt.Errorf("message.usage.cache_creation_input_tokens is not a non-negative number")
		}
	}
	classes := map[string]float64{
		"inputTokens": input + creation, "outputTokens": output,
	}
	if raw, present := usage["cache_read_input_tokens"]; present {
		cached, valid := transcriptNumber(raw)
		if !valid {
			return nil, fmt.Errorf("message.usage.cache_read_input_tokens is not a non-negative number")
		}
		classes["cachedInputTokens"] = cached
	}
	if raw, present := usage["thinking_tokens"]; present {
		thinking, valid := transcriptNumber(raw)
		if !valid {
			return nil, fmt.Errorf("message.usage.thinking_tokens is not a non-negative number")
		}
		classes["reasoningTokens"] = thinking
	}
	return classes, nil
}

func transcriptNumber(raw any) (float64, bool) {
	if _, boolean := raw.(bool); boolean {
		return 0, false
	}
	var value float64
	switch number := raw.(type) {
	case json.Number:
		parsed, err := number.Float64()
		if err != nil {
			return 0, false
		}
		value = parsed
	case float64:
		value = number
	case int:
		value = float64(number)
	default:
		return 0, false
	}
	return value, value >= 0
}

func seatUnmeasured(repoRoot string, request transcriptRequest, id, detail string) UnmeasuredEntry {
	day := ""
	if timestamp, err := parseTime(request.timestamp); err == nil {
		day = timestamp.Format("2006-01-02")
	}
	return UnmeasuredEntry{
		ID: id, File: relativePath(repoRoot, request.file), Goal: "seat", Day: day,
		Provenance: "seat-unmeasured", Detail: detail,
	}
}
