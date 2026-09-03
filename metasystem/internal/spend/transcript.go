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
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, seat, nil, nil
	}
	slug := strings.ReplaceAll(filepath.Clean(repoRoot), string(filepath.Separator), "-")
	projects := filepath.Join(home, ".claude", "projects")
	dirs, err := os.ReadDir(projects)
	if os.IsNotExist(err) {
		return nil, seat, nil, nil
	}
	if err != nil {
		return nil, seat, nil, fmt.Errorf("cannot read Claude transcript directory %s: %w", projects, err)
	}
	var files []string
	for _, dir := range dirs {
		if !dir.IsDir() || !strings.HasPrefix(dir.Name(), slug) {
			continue
		}
		matches, globErr := filepath.Glob(filepath.Join(projects, dir.Name(), "*.jsonl"))
		if globErr != nil {
			return nil, seat, nil, globErr
		}
		files = append(files, matches...)
	}
	sort.Strings(files)
	requests := map[string]transcriptRequest{}
	var invalid []transcriptRequest
	for _, path := range files {
		fileSession := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		if delegates[fileSession] {
			continue
		}
		seat.Files++
		info, statErr := os.Stat(path)
		if statErr != nil {
			return nil, seat, nil, fmt.Errorf("cannot stat Claude transcript %s: %w", path, statErr)
		}
		dayEligible := !info.ModTime().Before(now.Add(-48 * time.Hour))
		if !dayEligible {
			seat.AgedFiles++
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil, seat, nil, fmt.Errorf("cannot read Claude transcript %s: %w", path, readErr)
		}
		scanner := bufio.NewScanner(bytes.NewReader(data))
		scanner.Buffer(make([]byte, 0, 64*1024), 32*1024*1024)
		line := 0
		for scanner.Scan() {
			line++
			var raw map[string]any
			decoder := json.NewDecoder(bytes.NewReader(scanner.Bytes()))
			decoder.UseNumber()
			if err := decoder.Decode(&raw); err != nil {
				invalid = append(invalid, transcriptRequest{file: path, line: line, detail: "line is not JSON: " + err.Error(), dayEligible: dayEligible})
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
			if !seatCWD(repoRoot, cwd) {
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
			requests[key] = request
		}
		if err := scanner.Err(); err != nil {
			return nil, seat, nil, fmt.Errorf("cannot scan Claude transcript %s: %w", path, err)
		}
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
	var unmeasured []UnmeasuredEntry
	for _, request := range ordered {
		id := request.id
		if id == "" {
			id = fmt.Sprintf("%s:%d", filepath.Base(request.file), request.line)
		}
		if request.detail != "" {
			entry := seatUnmeasured(repoRoot, request, id, request.detail)
			entry.Machine = machine
			unmeasured = append(unmeasured, entry)
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
	seat.UnmeasuredRequests = len(unmeasured)
	return measured, seat, unmeasured, nil
}

func seatCWD(repoRoot, cwd string) bool {
	if cwd == "" {
		return false
	}
	abs, err := filepath.Abs(cwd)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(repoRoot, abs)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return false
	}
	worktrees := filepath.Join("artifacts", "agents", "worktrees")
	return rel != worktrees && !strings.HasPrefix(rel, worktrees+string(filepath.Separator))
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
