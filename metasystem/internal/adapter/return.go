package adapter

import (
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"os"
	"regexp"
	"slices"
	"strings"
)

// Return normalization: a runtime's final output rarely arrives as one clean
// JSON document. The return object may be the whole reply, sit inside a code
// fence, hide in a wrapper's result field, or be embedded as an escaped
// string. This scans the candidate output and the transcript for every JSON
// object in sight, scores each against the return contract's required fields,
// and canonicalizes the best match — with the session and model identity the
// harness OBSERVED overriding whatever the delegate claimed, because identity
// is evidence and a delegate's claim is not.

// returnRequiredFields are the return contract's required members; the more
// of them an object carries, the more likely it is the actual return.
var returnRequiredFields = []string{
	"jobId", "round", "runtime", "sessionId", "model", "evidence", "gaps", "mode",
}

// fencedBlockRe matches code fences, with or without a json language tag.
var fencedBlockRe = regexp.MustCompile("(?is)```(?:json)?\\s*(.*?)```")

// NormalizeReturn finds the return object in the runtime's candidate output
// or transcript, reconciles its session and model identity against what the
// harness observed, and writes the canonical return.json plus the return.md
// pointer beside it. sessionID is the session the adapter recorded at
// handshake; an empty value reads as unobserved.
func NormalizeReturn(candidatePath, transcriptPath, recordPath, outputPath, markdownPath, sessionID string) error {
	record, err := readObject(recordPath)
	if err != nil {
		return fmt.Errorf("cannot read the job record: %w", err)
	}

	var sources []any
	for _, path := range []string{candidatePath, transcriptPath} {
		if path == "" || !isRegularFile(path) {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		sources = append(sources, parseEmbeddedJSON(string(data))...)
	}

	best := bestReturnCandidate(sources)
	if best == nil {
		return fmt.Errorf("no JSON return object found in runtime output")
	}

	result := maps.Clone(best)
	reconcileIdentity(result, record, sessionID)

	if err := atomicWriteJSON(outputPath, result); err != nil {
		return err
	}
	return os.WriteFile(markdownPath, []byte("# Agent return\n\nCanonical JSON: return.json\n"), 0o644)
}

// bestReturnCandidate scores every object reachable from the sources against
// the required fields and returns the first object with the highest score, or
// nil when nothing carries even one required field.
func bestReturnCandidate(sources []any) map[string]any {
	var best map[string]any
	bestScore := 0
	for _, source := range sources {
		var reachable []any
		nestedValues(source, &reachable)
		for _, value := range reachable {
			object, ok := value.(map[string]any)
			if !ok {
				continue
			}
			score := 0
			for _, field := range returnRequiredFields {
				if _, present := object[field]; present {
					score++
				}
			}
			if score > bestScore {
				best, bestScore = object, score
			}
		}
	}
	return best
}

// nestedValues appends value and everything reachable inside it: the
// structured_output object and parsed result string common runtime wrappers
// carry, every object and array member, and any JSON embedded in string
// members. Values parsed out of embedded strings are appended as-is, not
// recursed into. Object members are visited in sorted key order so the same
// input always yields the same candidate order.
func nestedValues(value any, out *[]any) {
	*out = append(*out, value)
	switch v := value.(type) {
	case map[string]any:
		if structured, ok := v["structured_output"].(map[string]any); ok {
			*out = append(*out, structured)
		}
		if result, ok := v["result"].(string); ok {
			*out = append(*out, parseEmbeddedJSON(result)...)
		}
		for _, key := range slices.Sorted(maps.Keys(v)) {
			switch child := v[key].(type) {
			case map[string]any:
				nestedValues(child, out)
			case []any:
				nestedValues(child, out)
			case string:
				if strings.Contains(child, "{") {
					*out = append(*out, parseEmbeddedJSON(child)...)
				}
			}
		}
	case []any:
		for _, child := range v {
			nestedValues(child, out)
		}
	}
}

// parseEmbeddedJSON extracts every JSON value it can find in free text: the
// whole text as one document, each code-fenced block, and a decode attempt
// from every opening brace. Duplicates are fine — scoring picks one winner.
func parseEmbeddedJSON(text string) []any {
	var values []any
	if trimmed := strings.TrimSpace(text); trimmed != "" {
		if value, ok := decodeWholeJSON(trimmed); ok {
			values = append(values, value)
		}
	}
	for _, match := range fencedBlockRe.FindAllStringSubmatch(text, -1) {
		if value, ok := decodeWholeJSON(strings.TrimSpace(match[1])); ok {
			values = append(values, value)
		}
	}
	for index := 0; index < len(text); index++ {
		if text[index] != '{' {
			continue
		}
		if value, ok := decodeLeadingJSON(text[index:]); ok {
			values = append(values, value)
		}
	}
	return values
}

// decodeWholeJSON parses text that must be exactly one JSON document.
func decodeWholeJSON(text string) (any, bool) {
	decoder := json.NewDecoder(strings.NewReader(text))
	decoder.UseNumber()
	var value any
	if decoder.Decode(&value) != nil {
		return nil, false
	}
	if _, err := decoder.Token(); err != io.EOF {
		return nil, false
	}
	return value, true
}

// decodeLeadingJSON parses one JSON value from the start of text, ignoring
// whatever follows it.
func decodeLeadingJSON(text string) (any, bool) {
	decoder := json.NewDecoder(strings.NewReader(text))
	decoder.UseNumber()
	var value any
	if decoder.Decode(&value) != nil {
		return nil, false
	}
	return value, true
}

// reconcileIdentity replaces the return's session and model identity with
// what the harness observed, preserving a differing claim in the claimed
// object. Only the schema family whose model member is an object carries
// reconcilable identity; older shapes pass through untouched. Under schema
// version 2 and version 3 the claimed object always carries both members: null
// is how this family says "claimed nothing", and an absent object is rejected
// by the provider that enforces the schema.
func reconcileIdentity(result, record map[string]any, sessionID string) {
	model, ok := result["model"].(map[string]any)
	if !ok {
		return
	}
	observedSession := sessionID
	if observedSession == "" {
		observedSession = "unobserved"
	}
	observedModel, _ := record["effectiveModel"].(string)
	if observedModel == "" {
		observedModel = "unobserved"
	}

	model = maps.Clone(model)
	claimed := map[string]any{}
	if prior, ok := result["claimed"].(map[string]any); ok {
		claimed = maps.Clone(prior)
	}
	if claimedSession, ok := result["sessionId"].(string); ok && claimedSession != observedSession {
		claimed["sessionId"] = claimedSession
	}
	if claimedModel, ok := model["effective"].(string); ok && claimedModel != observedModel {
		claimed["model"] = claimedModel
	}
	result["sessionId"] = observedSession
	model["effective"] = observedModel
	result["model"] = model
	if numberEquals(result["schemaVersion"], 2) || numberEquals(result["schemaVersion"], 3) {
		result["claimed"] = map[string]any{
			"sessionId": claimed["sessionId"],
			"model":     claimed["model"],
		}
	}
}

// numberEquals reports whether a decoded JSON value is the given number.
func numberEquals(value any, target float64) bool {
	switch number := value.(type) {
	case json.Number:
		parsed, err := number.Float64()
		return err == nil && parsed == target
	case float64:
		return number == target
	}
	return false
}

func isRegularFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}
