package spend

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
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

func readSeat(repoRoot, machine string, now time.Time, delegates map[string]bool, settings config.SpendSettings) ([]pricedMeasurement, SeatSummary, []UnmeasuredEntry, error) {
	seat := SeatSummary{CodexUnmeasured: true}
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
		return nil, seat, nil, nil
	}
	if err != nil {
		recordUnreadable(projects, fmt.Errorf("cannot list Claude transcript root %s: %w", projects, err))
		return nil, seat, unmeasured, nil
	}
	var files []string
	for _, dir := range dirs {
		if !dir.IsDir() || !strings.HasPrefix(dir.Name(), slug) {
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
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			recordUnreadable(path, fmt.Errorf("cannot read Claude transcript %s: %w", path, readErr))
			continue
		}
		scanner := bufio.NewScanner(bytes.NewReader(data))
		scanner.Buffer(make([]byte, 0, 64*1024), 32*1024*1024)
		fileRequests := map[string]transcriptRequest{}
		var fileInvalid []transcriptRequest
		line := 0
		for scanner.Scan() {
			line++
			var raw map[string]any
			decoder := json.NewDecoder(bytes.NewReader(scanner.Bytes()))
			decoder.UseNumber()
			if err := decoder.Decode(&raw); err != nil {
				fileInvalid = append(fileInvalid, transcriptRequest{file: path, line: line, detail: "line is not JSON: " + err.Error(), dayEligible: dayEligible})
				continue
			}
			if textOr(raw["type"], "") != "assistant" {
				continue
			}
			session := textOr(raw["sessionId"], "")
			if delegates[session] {
				continue
			}
			cwd := textOr(raw["cwd"], "")
			if !seatCWD(toplevel, cwd) {
				continue
			}
			message, _ := raw["message"].(map[string]any)
			request := transcriptRequest{
				id: textOr(raw["requestId"], ""), file: path, line: line, session: session, cwd: cwd,
				model: textOr(message["model"], "unknown"), timestamp: textOr(raw["timestamp"], ""), dayEligible: dayEligible,
			}
			request.usage, _ = message["usage"].(map[string]any)
			key := request.id
			if key == "" {
				key = fmt.Sprintf("%s:%d", path, line)
			}
			fileRequests[key] = request
		}
		if err := scanner.Err(); err != nil {
			recordUnreadable(path, fmt.Errorf("cannot scan Claude transcript %s: %w", path, err))
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
