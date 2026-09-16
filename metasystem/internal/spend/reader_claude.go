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
	classes                                           rawTokenClasses
	cause                                             cause
	detail                                            string
	dayEligible                                       bool
}

type rawTokenClasses struct {
	Input, CacheCreation, CacheRead, Output, Reasoning float64
	HasCacheRead, HasReasoning                         bool
}

func (classes rawTokenClasses) missionClasses() map[string]float64 {
	missionClasses := map[string]float64{
		"inputTokens": classes.Input + classes.CacheCreation, "outputTokens": classes.Output,
	}
	if classes.HasCacheRead {
		missionClasses["cachedInputTokens"] = classes.CacheRead
	}
	if classes.HasReasoning {
		missionClasses["reasoningTokens"] = classes.Reasoning
	}
	return missionClasses
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

func readSeat(repoRoot, machine string, now time.Time, jobs readerJobs, settings config.SpendSettings) ([]pricedMeasurement, SeatSummary, []UnmeasuredEntry, map[string]attributionCall, error) {
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
			return nil, seat, unmeasured, nil, nil
		}
		for _, file := range discovered.files {
			file.registered, file.toplevel = registered, discovered.toplevel
			files = append(files, file)
		}
	}
	requests := map[string]transcriptRequest{}
	attributed := map[string]attributionCall{}
	for _, file := range files {
		registered := file.registered
		path := file.path
		fileSession := file.session
		legacyOwned := jobs.referencedSessions[fileSession] || jobs.referencedSessions[file.parentSession]
		info, statErr := os.Stat(path)
		if statErr != nil {
			if !legacyOwned {
				recordUnreadable(path, fmt.Errorf("cannot stat %s transcript %s: %w", registered.name, path, statErr))
			}
			continue
		}
		visitedCursorPaths[transcriptCursorPath(repoRoot, path)] = true
		scanJobs := jobs
		if legacyOwned {
			scanJobs.referencedSessions = map[string]bool{}
		}
		result := registered.scan(file, readerCursor{repoRoot: repoRoot, toplevel: file.toplevel, now: now, info: info}, scanJobs)
		if result.cacheWriteFailed && !legacyOwned {
			seat.CacheWriteFailures++
		}
		if result.err != nil {
			if !legacyOwned {
				recordUnreadable(path, result.err)
			}
			continue
		}
		if result.foreign && !legacyOwned {
			seat.SkippedForeignFiles++
			continue
		}
		file.transcriptMetadata = result.transcriptMetadata
		attributed["\x00"+file.path] = attributionCall{file: file}
		if !legacyOwned {
			seat.Files++
		}
		if result.aged && !legacyOwned {
			seat.AgedFiles++
		}
		for key, request := range result.calls {
			request.runtime = registered.name
			call := attributionCall{request: request, file: file}
			if retained, exists := attributed[key]; !exists || earlierStamped(request, retained.request) {
				attributed[key] = call
			}
			if legacyOwned {
				continue
			}
			if retained, exists := requests[key]; !exists || earlierStamped(request, retained) {
				requests[key] = request
			}
		}
		for _, gap := range result.unmeasured {
			if legacyOwned {
				continue
			}
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
		if stampErr != nil {
			detail := "unparsable timestamp"
			entry := seatUnmeasured(repoRoot, request, id, detail)
			entry.Machine = machine
			unmeasured = append(unmeasured, entry)
			seat.UnmeasuredRequests++
			continue
		}
		classes := request.classes.missionClasses()
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
	return measured, seat, unmeasured, attributed, nil
}

func earlierStamped(candidate, retained transcriptRequest) bool {
	candidateStamp, candidateErr := parseTime(candidate.timestamp)
	retainedStamp, retainedErr := parseTime(retained.timestamp)
	if candidateErr == nil && retainedErr == nil {
		return candidateStamp.Before(retainedStamp)
	}
	return candidateErr == nil && retainedErr != nil
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
	var metadata transcriptMetadata
	calls, invalid, foreign, cacheWriteFailed, err := readTranscriptCursor(
		cursor.repoRoot, file.path, cursor.info, cursor.toplevel, dayEligible, jobs.referencedSessions, jobDigest(jobs), &metadata,
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
		calls: calls, transcriptMetadata: metadata,
		unmeasured: unmeasured, foreign: foreign, aged: !dayEligible,
		cacheWriteFailed: cacheWriteFailed, err: err,
	}
}

type transcriptMetadata struct {
	starters    []turnStarter
	kind        delegateKind
	kindMissing bool
}

func readTranscriptCursor(repoRoot, path string, info os.FileInfo, toplevel string, dayEligible bool, delegates map[string]bool, jobsDigest string, metadata *transcriptMetadata) (map[string]transcriptRequest, []transcriptRequest, bool, bool, error) {
	cachePath := transcriptCursorPath(repoRoot, path)
	cache, valid := loadTranscriptCursor(cachePath, path)
	valid = valid && cache.JobDigest == jobsDigest
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
		*metadata = transcriptCursorMetadata(cache)
		return requests, invalid, foreign, false, nil
	}
	if !grown {
		cache = transcriptCursorCache{
			SchemaVersion: transcriptCursorSchemaVersion, Path: path, JobDigest: jobsDigest,
			Starters: []turnStarter{}, Requests: map[string]cachedTranscriptRequest{}, Invalid: []cachedTranscriptRequest{},
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
		cache.Starters = []turnStarter{}
	}
	cacheWriteFailed := writeSpendCache(cachePath, cache) != nil
	if foreign {
		return nil, nil, true, cacheWriteFailed, nil
	}
	requests, invalid, foreign := transcriptCursorSnapshot(cache, path, toplevel, dayEligible, delegates)
	*metadata = transcriptCursorMetadata(cache)
	return requests, invalid, foreign, cacheWriteFailed, nil
}

func transcriptCursorMetadata(cache transcriptCursorCache) transcriptMetadata {
	kind := cache.DelegateKind
	if kind == "" {
		kind = kindOther
	}
	return transcriptMetadata{append([]turnStarter(nil), cache.Starters...), kind, cache.KindMissing || cache.DelegateKind == ""}
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
	if textOr(raw["type"], "") == "user" {
		text, toolResult := transcriptUserText(raw)
		if cache.DelegateKind == "" {
			cache.DelegateKind, cache.KindMissing = delegateKindLine(text)
		}
		if !toolResult {
			if starter, ok := classifyTurnStarter(raw, text, lineNumber); ok {
				cache.Starters = append(cache.Starters, starter)
			}
		}
		return false
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
		ID: textOr(message["id"], textOr(raw["requestId"], "")), Session: session, CWD: cwd,
		Model: textOr(message["model"], "unknown"), Timestamp: textOr(raw["timestamp"], ""), Line: lineNumber, Cause: causeUnstarted,
	}
	if len(cache.Starters) > 0 {
		starter := cache.Starters[len(cache.Starters)-1]
		request.Cause, request.CauseDetail = starter.Cause, starter.Detail
	}
	request.Usage, _ = message["usage"].(map[string]any)
	key := request.ID
	if key == "" {
		key = fmt.Sprintf("%s:%d", path, lineNumber)
	}
	request.ID = key
	if retained, exists := cache.Requests[key]; !exists {
		cache.Requests[key] = request
	} else if !measurableTranscriptRequest(retained) && measurableTranscriptRequest(request) {
		cache.Requests[key] = request
	} else if output, ok := transcriptNumber(request.Usage["output_tokens"]); ok && output > retainedOutput(retained) {
		usage := make(map[string]any, len(retained.Usage))
		for name, value := range retained.Usage {
			usage[name] = value
		}
		usage["output_tokens"] = request.Usage["output_tokens"]
		retained.Usage = usage
		cache.Requests[key] = retained
	}
	return false
}

func transcriptUserText(raw map[string]any) (string, bool) {
	message, _ := raw["message"].(map[string]any)
	if text, ok := message["content"].(string); ok {
		return text, false
	}
	var parts []string
	var toolResult bool
	items, _ := message["content"].([]any)
	for _, item := range items {
		block, _ := item.(map[string]any)
		toolResult = toolResult || textOr(block["type"], "") == "tool_result"
		if text := textOr(block["text"], ""); text != "" {
			parts = append(parts, text)
		}
	}
	return strings.Join(parts, "\n"), toolResult
}

func delegateKindLine(text string) (delegateKind, bool) {
	line, _, _ := strings.Cut(strings.TrimSpace(text), "\n")
	line = strings.TrimSpace(line)
	kind := delegateKind(strings.TrimPrefix(line, "Kind: "))
	if strings.HasPrefix(line, "Kind: ") && (kind == kindDesign || kind == kindBuildRead || kind == kindCritique || kind == kindOther) {
		return kind, false
	}
	return kindOther, true
}

type starterRule struct {
	match  bool
	cause  cause
	detail string
}

func classifyTurnStarter(raw map[string]any, text string, line int) (turnStarter, bool) {
	s := strings.TrimSpace(text)
	origin, _ := raw["origin"].(map[string]any)
	o, ps := textOr(origin["kind"], ""), textOr(raw["promptSource"], "")
	rules := []starterRule{
		{strings.HasPrefix(s, "Stop hook feedback"), causeStopHook, stopHookDetail(s)},
		{o == "task-notification" || strings.HasPrefix(s, "<task-notification>") || strings.HasPrefix(s, "[SYSTEM NOTIFICATION"), causeNotification, notificationSource(s)},
		{o == "coordinator" || strings.HasPrefix(s, "The coordinator sent a message"), causePeer, "coordinator"},
		{o == "peer" || strings.HasPrefix(s, "Another Claude session sent a message") || strings.Contains(s[:min(len(s), 400)], "<cross-session-message"), causePeer, ""},
		{strings.HasPrefix(s, "This session is being continued"), causeCompaction, ""},
		{o == "auto-continuation" || strings.HasPrefix(s, "Your claude.ai usage limit"), causeUsageLimitResume, ""},
		{ps == "sdk" || strings.HasPrefix(s, "# Task Direction"), causeHuman, "sdk"},
		{o == "human", causeHuman, "human"}, {ps == "typed", causeHuman, "typed"}, {ps == "queued", causeHuman, "queued"},
		{strings.HasPrefix(s, "<local-command"), causeHuman, "local-command"}, {strings.HasPrefix(s, "<command-name>"), causeHuman, "command-name"},
		{strings.HasPrefix(s, "<bash-input>"), causeHuman, "bash-input"}, {strings.HasPrefix(s, "<bash-stdout>"), causeHuman, "bash-stdout"},
		{strings.HasPrefix(s, "[Request interrupted"), causeHuman, "interrupted"},
	}
	for _, rule := range rules {
		if rule.match {
			return turnStarter{line, textOr(raw["timestamp"], ""), rule.cause, rule.detail}, true
		}
	}
	if raw["isMeta"] == true {
		return turnStarter{}, false
	}
	return turnStarter{line, textOr(raw["timestamp"], ""), causeHuman, "unclassified"}, true
}

func stopHookDetail(text string) string {
	_, rest, ok := strings.Cut(text, "Stop blocked;")
	detail, _, closed := strings.Cut(rest, ";")
	if !ok || !closed || strings.TrimSpace(detail) == "" {
		return "unparsed"
	}
	return strings.TrimSpace(detail)
}

func notificationSource(text string) string {
	if strings.HasPrefix(text, "[SYSTEM NOTIFICATION") {
		return "monitor"
	}
	for _, source := range [][2]string{{"<note>", "agent"}, {"<result>", "agent"}, {"<exit-code>", "bash"}, {"<output>", "bash"}} {
		if strings.Contains(text, source[0]) {
			return source[1]
		}
	}
	return "other"
}

func measurableTranscriptRequest(request cachedTranscriptRequest) bool {
	_, err := rawTranscriptTokens(request.Usage)
	_, stampErr := parseTime(request.Timestamp)
	return request.Model != "<synthetic>" && err == nil && stampErr == nil
}

func retainedOutput(request cachedTranscriptRequest) float64 {
	output, _ := transcriptNumber(request.Usage["output_tokens"])
	return output
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

func cloneTranscriptCursor(cache transcriptCursorCache) transcriptCursorCache {
	clone := cache
	clone.Requests = make(map[string]cachedTranscriptRequest, len(cache.Requests))
	for key, request := range cache.Requests {
		clone.Requests[key] = request
	}
	clone.Invalid = append([]cachedTranscriptRequest(nil), cache.Invalid...)
	clone.Starters = append([]turnStarter(nil), cache.Starters...)
	clone.Tail = append([]byte(nil), cache.Tail...)
	return clone
}

func transcriptRequestFromCache(path string, request cachedTranscriptRequest, dayEligible bool) transcriptRequest {
	classes, usageErr := rawTranscriptTokens(request.Usage)
	detail := request.Detail
	if detail == "" {
		if request.Model == "<synthetic>" {
			detail = "synthetic model"
		} else if usageErr != nil {
			detail = usageErr.Error()
		}
	}
	return transcriptRequest{
		id: request.ID, file: path, session: request.Session, cwd: request.CWD, model: request.Model,
		timestamp: request.Timestamp, line: request.Line, classes: classes, cause: request.Cause,
		detail: detail, dayEligible: dayEligible,
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

func rawTranscriptTokens(usage map[string]any) (rawTokenClasses, error) {
	legacy, err := transcriptTokens(usage)
	if err != nil {
		return rawTokenClasses{}, err
	}
	creation, _ := transcriptNumber(usage["cache_creation_input_tokens"])
	_, hasCacheRead := usage["cache_read_input_tokens"]
	_, hasReasoning := usage["thinking_tokens"]
	return rawTokenClasses{Input: legacy["inputTokens"] - creation, CacheCreation: creation,
		CacheRead: legacy["cachedInputTokens"], Output: legacy["outputTokens"], Reasoning: legacy["reasoningTokens"],
		HasCacheRead: hasCacheRead, HasReasoning: hasReasoning}, nil
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
