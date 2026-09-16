package spend

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/mission"
)

type transcriptRequest struct {
	id, file, session, cwd, model, timestamp, runtime string
	line                                              int
	usage                                             map[string]any
	detail                                            string
	dayEligible                                       bool
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
	var files []transcriptFile
	for index := range readerRegistry {
		registered := &readerRegistry[index]
		if !registered.inScope || registered.capability != readerCapabilityPerCall {
			continue
		}
		discovered := registered.discover(repoRoot)
		seat.UnreadableFiles += discovered.counters.UnreadableFiles
		for _, gap := range discovered.gaps {
			gap.Machine = machine
			unmeasured = append(unmeasured, gap)
		}
		if discovered.fatal {
			return nil, seat, unmeasured, nil
		}
		for _, file := range discovered.files {
			file.registered, file.toplevel = registered, discovered.toplevel
			files = append(files, file)
		}
	}
	requests := map[string]transcriptRequest{}
	for _, file := range files {
		registered := file.registered
		path := file.path
		fileSession := file.session
		if delegates[fileSession] || delegates[file.parentSession] {
			continue
		}
		info, statErr := os.Stat(path)
		if statErr != nil {
			recordUnreadable(path, fmt.Errorf("cannot stat %s transcript %s: %w", registered.name, path, statErr))
			continue
		}
		visitedCursorPaths[transcriptCursorPath(repoRoot, path)] = true
		result := registered.scan(file, readerCursor{repoRoot: repoRoot, toplevel: file.toplevel, now: now, info: info}, readerJobs(delegates))
		if result.cacheWriteFailed {
			seat.CacheWriteFailures++
		}
		if result.err != nil {
			recordUnreadable(path, result.err)
			continue
		}
		if result.foreign {
			seat.SkippedForeignFiles++
			continue
		}
		seat.Files++
		if result.aged {
			seat.AgedFiles++
		}
		for key, request := range result.calls {
			request.runtime = registered.name
			requests[key] = request
		}
		for _, gap := range result.unmeasured {
			gap.Machine = machine
			unmeasured = append(unmeasured, gap)
			seat.UnmeasuredRequests++
		}
	}
	seat.CacheWriteFailures += pruneTranscriptCursors(repoRoot, visitedCursorPaths)

	ordered := make([]transcriptRequest, 0, len(requests))
	for _, request := range requests {
		ordered = append(ordered, request)
	}
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
		money, priced, unpriced, foreign := price(request.runtime, model, classes, (*mission.UsageCost)(nil), false, settings)
		item := pricedMeasurement{
			goal: "seat", machine: machine, day: timestamp.Format("2006-01-02"), runtime: request.runtime, model: model,
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
func claudeReader() reader {
	return reader{
		name: "claude", capability: readerCapabilityPerCall, inScope: true,
		discover: discoverClaudeTranscripts, scan: scanClaudeTranscript,
	}
}
func discoverClaudeTranscripts(repoRoot string) discoveryResult {
	var result discoveryResult
	recordUnreadable := func(path string, err error) {
		displayPath := path
		if filepath.IsAbs(path) {
			displayPath = relativePath(repoRoot, path)
		}
		result.counters.UnreadableFiles++
		result.gaps = append(result.gaps, UnmeasuredEntry{
			ID: displayPath, File: displayPath, Goal: "seat", Provenance: "seat unreadable", Detail: err.Error(),
		})
	}
	var err error
	result.toplevel, err = gitToplevel(repoRoot)
	if err != nil {
		recordUnreadable(repoRoot, err)
		result.fatal = true
		return result
	}
	home, err := os.UserHomeDir()
	if err != nil {
		recordUnreadable("~", fmt.Errorf("cannot resolve home directory: %w", err))
		result.fatal = true
		return result
	}
	slug := strings.ReplaceAll(result.toplevel, string(filepath.Separator), "-")
	projects := filepath.Join(home, ".claude", "projects")
	_ = filepath.WalkDir(projects, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			if path != projects || !os.IsNotExist(walkErr) {
				recordUnreadable(path, fmt.Errorf("cannot list Claude transcript path %s: %w", path, walkErr))
			}
			if path == projects && !os.IsNotExist(walkErr) {
				result.fatal = true
				return fs.SkipAll
			}
			if entry != nil && entry.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		relative, _ := filepath.Rel(projects, path)
		parts := strings.Split(relative, string(filepath.Separator))
		if len(parts) == 1 {
			if path == projects {
				return nil
			}
			if !entry.IsDir() {
				return nil
			}
			if entry.Name() != slug && !strings.HasPrefix(entry.Name(), slug+"-") {
				return fs.SkipDir
			}
			return nil
		}
		if filepath.Ext(entry.Name()) == ".jsonl" {
			result.files = append(result.files, claudeTranscriptFile(filepath.Join(projects, parts[0]), path))
			if entry.IsDir() {
				return fs.SkipDir
			}
		}
		return nil
	})
	sort.Slice(result.files, func(i, j int) bool { return result.files[i].path < result.files[j].path })
	return result
}
func claudeTranscriptFile(slugPath, path string) transcriptFile {
	file := transcriptFile{path: path, session: strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))}
	relative, err := filepath.Rel(slugPath, path)
	if err != nil {
		return file
	}
	parts := strings.Split(filepath.Clean(relative), string(filepath.Separator))
	for index, part := range parts[:len(parts)-1] {
		if part == "subagents" {
			file.delegate = true
			if index > 0 {
				file.parentSession = parts[index-1]
			}
			break
		}
	}
	return file
}
func scanClaudeTranscript(file transcriptFile, cursor readerCursor, jobs readerJobs) scanResult {
	dayEligible := !cursor.info.ModTime().Before(cursor.now.Add(-48 * time.Hour))
	calls, invalid, foreign, cacheWriteFailed, err := readTranscriptCursor(
		cursor.repoRoot, file.path, cursor.info, cursor.toplevel, dayEligible, jobs, delegateSessionDigest(jobs),
	)
	unmeasured := make([]UnmeasuredEntry, 0, len(invalid))
	for _, request := range invalid {
		id := request.id
		if id == "" {
			id = fmt.Sprintf("%s:%d", filepath.Base(request.file), request.line)
		}
		unmeasured = append(unmeasured, seatUnmeasured(cursor.repoRoot, request, id, request.detail))
	}
	return scanResult{
		calls: calls, unmeasured: unmeasured, foreign: foreign, aged: !dayEligible,
		cacheWriteFailed: cacheWriteFailed, err: err,
	}
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
