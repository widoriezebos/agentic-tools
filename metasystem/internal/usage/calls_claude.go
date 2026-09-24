package usage

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func claudeTranscript(opts ReadOptions, session string) (path string, reason string) {
	if opts.Transcript != "" {
		return opts.Transcript, ""
	}
	home, reason := callHome(opts)
	if reason != "" {
		return "", reason
	}
	var candidates []string
	for _, cwd := range []string{opts.Toplevel, opts.Installation} {
		if cwd == "" {
			continue
		}
		directory := filepath.Join(home, ".claude", "projects", claudeSlug(cwd))
		candidate := filepath.Join(directory, session+".jsonl")
		candidates = append(candidates, candidate)
		if !pathWithin(directory, candidate) {
			continue
		}
		info, err := os.Lstat(candidate)
		if err == nil && info.Mode().IsRegular() {
			return candidate, ""
		}
	}
	return "", fmt.Sprintf("unknown (no transcript at %s)", strings.Join(candidates, " or "))
}

// ClaudeTranscript resolves the transcript that belongs to one Claude
// session without reading or changing usage evidence.
func ClaudeTranscript(session string, opts ReadOptions) (path string, reason string) {
	return claudeTranscript(opts, session)
}

// MemoryDirectory resolves the Claude project memory directory independently
// of any caller-supplied transcript path.
func MemoryDirectory(opts ReadOptions) (path string, reason string) {
	home, reason := callHome(opts)
	if reason != "" {
		return "", reason
	}
	projects := filepath.Join(home, ".claude", "projects")
	var candidates []string
	for _, cwd := range []string{opts.Toplevel, opts.Installation} {
		if cwd == "" {
			continue
		}
		candidate := filepath.Join(projects, claudeSlug(cwd), "memory")
		candidates = append(candidates, candidate)
		if !pathWithin(projects, candidate) {
			continue
		}
		info, err := os.Lstat(candidate)
		if err == nil && info.IsDir() && info.Mode()&os.ModeSymlink == 0 {
			return candidate, ""
		}
	}
	return "", fmt.Sprintf("unknown (no memory directory at %s)", strings.Join(candidates, " or "))
}

func claudeSlug(cwd string) string {
	bytes := []byte(cwd)
	for index, value := range bytes {
		if (value >= 'A' && value <= 'Z') || (value >= 'a' && value <= 'z') || (value >= '0' && value <= '9') {
			continue
		}
		bytes[index] = '-'
	}
	return string(bytes)
}

func parseClaudeLine(line []byte, ordinal int64, runtime, session string) (sample *CallSample, marker *Marker, sidechain bool) {
	if callJSONDecodes != nil {
		callJSONDecodes()
	}
	var raw map[string]any
	decoder := json.NewDecoder(bytes.NewReader(line))
	decoder.UseNumber()
	if err := decoder.Decode(&raw); err != nil {
		return nil, nil, false
	}
	timestamp := callTimestamp(raw["timestamp"])
	if textValue(raw["type"]) == "system" && textValue(raw["subtype"]) == "compact_boundary" {
		metadata, _ := raw["compactMetadata"].(map[string]any)
		return nil, &Marker{
			Runtime: runtime,
			Session: session,
			Kind:    "compaction",
			At:      timestamp,
			Ordinal: ordinal,
			Detail: fmt.Sprintf("trigger=%s preTokens=%s",
				textValue(metadata["trigger"]), numberText(metadata["preTokens"])),
		}, false
	}
	if textValue(raw["type"]) != "assistant" {
		return nil, nil, false
	}
	if value, ok := raw["isSidechain"].(bool); ok && value {
		return nil, nil, true
	}
	message, _ := raw["message"].(map[string]any)
	usage, ok := message["usage"].(map[string]any)
	if !ok {
		return nil, nil, false
	}
	input, ok := callTokenField(usage, "input_tokens", false)
	if !ok {
		return nil, nil, false
	}
	creation, ok := callTokenField(usage, "cache_creation_input_tokens", true)
	if !ok {
		return nil, nil, false
	}
	read, ok := callTokenField(usage, "cache_read_input_tokens", true)
	if !ok {
		return nil, nil, false
	}
	invocation := textValue(raw["requestId"])
	if invocation == "" {
		invocation = fmt.Sprintf("line:%d", ordinal)
	}
	if input > math.MaxInt64-creation || input+creation > math.MaxInt64-read {
		return nil, nil, false
	}
	return &CallSample{
		Runtime:       runtime,
		Session:       session,
		InvocationID:  invocation,
		PromptTokens:  input + creation + read,
		InputTokens:   input,
		CacheCreation: creation,
		CacheRead:     read,
		At:            timestamp,
		Ordinal:       ordinal,
		Source:        "claude-transcript",
	}, nil, false
}

func pathWithin(directory, candidate string) bool {
	relative, err := filepath.Rel(directory, candidate)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func callTimestamp(raw any) time.Time {
	value, ok := raw.(string)
	if !ok || value == "" {
		return time.Time{}
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}
	}
	return parsed
}

func textValue(raw any) string {
	value, _ := raw.(string)
	return value
}

func numberText(raw any) string {
	switch value := raw.(type) {
	case json.Number:
		return value.String()
	case float64:
		return fmt.Sprint(value)
	case int:
		return fmt.Sprint(value)
	case int64:
		return fmt.Sprint(value)
	default:
		return ""
	}
}

func callToken(raw any, optional bool) (int64, bool) {
	if raw == nil && optional {
		return 0, true
	}
	if _, boolean := raw.(bool); boolean {
		return 0, false
	}
	var number float64
	switch value := raw.(type) {
	case json.Number:
		if integer, err := value.Int64(); err == nil {
			return integer, integer >= 0
		}
		parsed, err := value.Float64()
		if err != nil {
			return 0, false
		}
		number = parsed
	case float64:
		number = value
	case int:
		number = float64(value)
	case int64:
		number = float64(value)
	default:
		return 0, false
	}
	if number < 0 || math.IsNaN(number) || math.IsInf(number, 0) || number >= float64(math.MaxInt64) {
		return 0, false
	}
	return int64(number), true
}

func callTokenField(object map[string]any, key string, optional bool) (int64, bool) {
	raw, present := object[key]
	if !present {
		if optional {
			return 0, true
		}
		return 0, false
	}
	return callToken(raw, false)
}
