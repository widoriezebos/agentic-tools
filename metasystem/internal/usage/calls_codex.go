package usage

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func codexRollout(opts ReadOptions, session string) (path string, reason string) {
	if opts.Transcript != "" {
		return opts.Transcript, ""
	}
	home, reason := callHome(opts)
	if reason != "" {
		return "", reason
	}
	root := filepath.Join(home, ".codex", "sessions")
	matches := make([]string, 0, 1)
	suffix := "-" + session + ".jsonl"
	firstLevel, readReason := codexRolloutDirectories(root)
	if readReason != "" {
		return "", readReason
	}
	for _, first := range firstLevel {
		secondLevel, readReason := codexRolloutDirectories(first)
		if readReason != "" {
			return "", readReason
		}
		for _, second := range secondLevel {
			thirdLevel, readReason := codexRolloutDirectories(second)
			if readReason != "" {
				return "", readReason
			}
			for _, third := range thirdLevel {
				entries, err := os.ReadDir(third)
				if os.IsNotExist(err) {
					continue
				}
				if err != nil {
					return "", codexRolloutReadReason(third, err)
				}
				for _, entry := range entries {
					name := entry.Name()
					if !strings.HasPrefix(name, "rollout-") || !strings.HasSuffix(name, suffix) {
						continue
					}
					candidate := filepath.Join(third, name)
					info, err := os.Lstat(candidate)
					if os.IsNotExist(err) {
						continue
					}
					if err != nil {
						return "", codexRolloutReadReason(third, err)
					}
					if info.Mode().IsRegular() {
						matches = append(matches, candidate)
					}
				}
			}
		}
	}
	if len(matches) == 0 {
		return "", fmt.Sprintf("unknown (no rollout for session %s under %s)", session, root)
	}
	if len(matches) > 1 {
		return "", fmt.Sprintf("unknown (%d rollouts match session %s)", len(matches), session)
	}
	return matches[0], ""
}

func codexRolloutDirectories(path string) ([]string, string) {
	entries, err := os.ReadDir(path)
	if os.IsNotExist(err) {
		return nil, ""
	}
	if err != nil {
		return nil, codexRolloutReadReason(path, err)
	}
	directories := make([]string, 0, len(entries))
	for _, entry := range entries {
		candidate := filepath.Join(path, entry.Name())
		info, err := os.Stat(candidate)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, codexRolloutReadReason(path, err)
		}
		if info.IsDir() {
			directories = append(directories, candidate)
		}
	}
	return directories, ""
}

func codexRolloutReadReason(path string, err error) string {
	return fmt.Sprintf("unknown (cannot read rollout directory %s: %v)", path, err)
}

func parseCodexLine(line []byte, runtime, session string) (sample *CallSample, marker *Marker) {
	return parseCodexRecord(decodeCallLine(line), runtime, session)
}

func parseCodexRecord(raw map[string]any, runtime, session string) (sample *CallSample, marker *Marker) {
	if raw == nil {
		return nil, nil
	}
	timestamp := callTimestamp(raw["timestamp"])
	switch textValue(raw["type"]) {
	case "compacted":
		ordinal, _ := callTokenField(raw, "ordinal", true)
		return nil, &Marker{
			Runtime: runtime,
			Session: session,
			Kind:    "compaction",
			At:      timestamp,
			Ordinal: ordinal,
			Detail:  "compacted",
		}
	case "token_usage_record":
		payload, _ := raw["payload"].(map[string]any)
		invocation := textValue(payload["response_id"])
		if invocation == "" {
			return nil, nil
		}
		usage, ok := payload["usage"].(map[string]any)
		if !ok {
			return nil, nil
		}
		input, ok := callTokenField(usage, "input_tokens", false)
		if !ok {
			return nil, nil
		}
		read, ok := callTokenField(usage, "cached_input_tokens", true)
		if !ok {
			return nil, nil
		}
		creation, ok := callTokenField(usage, "cache_write_input_tokens", true)
		if !ok {
			return nil, nil
		}
		ordinal, _ := callTokenField(raw, "ordinal", true)
		return &CallSample{
			Runtime:       runtime,
			Session:       session,
			InvocationID:  invocation,
			PromptTokens:  input,
			InputTokens:   input,
			CacheCreation: creation,
			CacheRead:     read,
			At:            timestamp,
			Ordinal:       ordinal,
			Source:        "codex-rollout",
		}, nil
	default:
		return nil, nil
	}
}

func codexRecordKind(raw map[string]any) (kind string, tokenCount bool) {
	if raw == nil {
		return "", false
	}
	kind = textValue(raw["type"])
	if kind != "event_msg" {
		return kind, false
	}
	payload, _ := raw["payload"].(map[string]any)
	return kind, textValue(payload["type"]) == "token_count"
}

func decodeCallLine(line []byte) map[string]any {
	if callJSONDecodes != nil {
		callJSONDecodes()
	}
	var raw map[string]any
	decoder := json.NewDecoder(bytes.NewReader(line))
	decoder.UseNumber()
	if err := decoder.Decode(&raw); err != nil {
		return nil
	}
	return raw
}
