package adapter

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// BuildDevinConfig writes a Devin delegate's job config and its provenance. The
// CLI's --config replaces the user configuration rather than layering onto it,
// so the job config is the user's own file with only the permissions block
// swapped for the job's workspace-scoped grant: the organisation id and
// onboarding marker survive (a config missing them makes the CLI print a
// welcome banner into the turn's stdout), any sandbox declaration is dropped,
// and replacing the permissions member is the only safe direction because
// merging could only widen what the job may attempt. The provenance records
// which members were replaced and which were inherited unchanged.
func BuildDevinConfig(recordPath, outputPath, provenancePath string) error {
	requested, err := requestedPermissions(recordPath)
	if err != nil {
		return err
	}
	readRoots := stringList(requested["readRoots"])
	writeRoots := stringList(requested["writeRoots"])

	allow := []any{"read", "grep", "glob", "exec"}
	for _, root := range readRoots {
		allow = append(allow, fmt.Sprintf("Read(%s/**)", root))
	}
	deny := []any{"Fetch(*)", "mcp__*"}
	if len(writeRoots) > 0 {
		allow = append(allow, "edit")
		for _, root := range writeRoots {
			allow = append(allow, fmt.Sprintf("Write(%s/**)", root))
		}
	} else {
		deny = append(deny, "edit", "Write(**)")
	}

	userPath := userDevinConfigPath()
	value := loadObjectOrEmpty(userPath)

	var replaced []string
	for _, key := range []string{"permissions", "sandbox"} {
		if _, ok := value[key]; ok {
			replaced = append(replaced, key)
		}
	}
	sort.Strings(replaced)
	delete(value, "sandbox")
	value["permissions"] = map[string]any{"allow": allow, "ask": []any{}, "deny": deny}

	var inherited []string
	for key := range value {
		if key != "permissions" {
			inherited = append(inherited, key)
		}
	}
	sort.Strings(inherited)

	if err := atomicWriteJSON(outputPath, value); err != nil {
		return fmt.Errorf("write devin config: %w", err)
	}
	provenance := map[string]any{
		"userConfig":       userPath,
		"replacedMembers":  stringSlice(replaced),
		"inheritedMembers": stringSlice(inherited),
	}
	if err := atomicWriteJSON(provenancePath, provenance); err != nil {
		return fmt.Errorf("write devin config provenance: %w", err)
	}
	return nil
}

// userDevinConfigPath is the caller's Devin config location, honoring
// XDG_CONFIG_HOME and defaulting to ~/.config.
func userDevinConfigPath() string {
	home := os.Getenv("XDG_CONFIG_HOME")
	if home == "" {
		if userHome, err := os.UserHomeDir(); err == nil {
			home = filepath.Join(userHome, ".config")
		}
	}
	return filepath.Join(home, "devin", "config.json")
}

// DevinSessionCorrelate identifies this turn's Devin session. A hook signal, if
// one carries a session id, is authoritative. Otherwise the session is the one
// that appears in the current listing, is absent from the pre-launch baseline,
// and runs in this workspace. Exactly one such candidate is this turn's
// session; none means keep waiting; more than one is ambiguous and returned for
// the caller to refuse rather than guess a peer's session. The candidate slice
// is set only on the ambiguous case.
func DevinSessionCorrelate(beforePath, currentPath, signalPath, workspace string) (string, []string) {
	if object, ok := loadAny(signalPath).(map[string]any); ok {
		if sid := firstString(object, "session_id", "sessionId", "id"); sid != "" {
			return sid, nil
		}
	}

	before := map[string]bool{}
	for _, record := range sessionRecords(loadAny(beforePath)) {
		if sid := recordSessionID(record); sid != "" {
			before[sid] = true
		}
	}

	var workspaceResolved string
	if workspace != "" {
		workspaceResolved = resolve(workspace)
	}

	seen := map[string]bool{}
	var candidates []string
	for _, record := range sessionRecords(loadAny(currentPath)) {
		sid := recordSessionID(record)
		if sid == "" || before[sid] || seen[sid] {
			continue
		}
		if !recordInWorkspace(record, workspaceResolved) {
			continue
		}
		seen[sid] = true
		candidates = append(candidates, sid)
	}
	sort.Strings(candidates)
	if len(candidates) == 1 {
		return candidates[0], nil
	}
	return "", candidates
}

// sessionRecords walks a session listing, however it is nested, and returns
// every object that carries a session-identifying key.
func sessionRecords(value any) []map[string]any {
	var out []map[string]any
	var walk func(any)
	walk = func(node any) {
		switch typed := node.(type) {
		case map[string]any:
			if hasSessionKey(typed) {
				out = append(out, typed)
			}
			for _, child := range typed {
				walk(child)
			}
		case []any:
			for _, child := range typed {
				walk(child)
			}
		}
	}
	walk(value)
	return out
}

// hasSessionKey reports whether a record names a session at all, even when that
// name is null or non-string.
func hasSessionKey(record map[string]any) bool {
	for _, key := range []string{"session_id", "sessionId", "id"} {
		if _, ok := record[key]; ok {
			return true
		}
	}
	return false
}

// recordSessionID returns a record's session identifier, the first of the
// spellings that holds a non-empty string.
func recordSessionID(record map[string]any) string {
	return firstString(record, "session_id", "sessionId", "id")
}

// recordInWorkspace reports whether a session record's working directory
// resolves to this turn's workspace. With no workspace to scope to, every
// record qualifies; a record without a usable directory never does.
func recordInWorkspace(record map[string]any, workspaceResolved string) bool {
	if workspaceResolved == "" {
		return true
	}
	directory := firstString(record, "working_directory", "workingDirectory")
	if directory == "" {
		return false
	}
	return resolve(directory) == workspaceResolved
}

// devinUsageFields are the cumulative counters Devin reports for a session.
var devinUsageFields = []string{
	"total_prompt_tokens",
	"total_completion_tokens",
	"total_cached_tokens",
	"total_steps",
}

// DevinUsage derives a Devin turn's typed usage from the transcript's
// cumulative session metrics. Those metrics are the SESSION total on every
// turn, so each turn publishes the delta against its predecessor's stored
// totals and records its own cumulative for the next turn to subtract. A
// resumed turn that cannot find its predecessor's totals publishes unavailable
// rather than a figure that would double-count every earlier turn. An
// enterprise account reports ACU instead of tokens; ACU rides in providerUnits,
// never as a token count or a cost.
func DevinUsage(usagePath, transcriptPath, cumulativePath, previousPath string, expectPrevious bool) error {
	var metrics map[string]any
	if transcript, err := readObject(transcriptPath); err == nil {
		metrics, _ = transcript["final_metrics"].(map[string]any)
	}
	totals := map[string]int64{}
	for _, field := range devinUsageFields {
		if value, ok := asInt(metrics[field]); ok {
			totals[field] = value
		}
	}
	acuKey, acuValue, hasACU := devinProviderUnit(metrics)

	var previous map[string]any
	if previousPath != "" {
		if object, err := readObject(previousPath); err == nil {
			previous = object
		}
	}
	predecessorMissing := expectPrevious && previous == nil

	unavailable := map[string]any{
		"availability":      "unavailable",
		"inputTokens":       nil,
		"cachedInputTokens": nil,
		"outputTokens":      nil,
		"reasoningTokens":   nil,
		"cost":              nil,
		"providerUnits":     nil,
	}
	if hasACU {
		unavailable["providerUnits"] = map[string]any{"name": "acu", "value": acuValue}
	}

	if len(totals) != len(devinUsageFields) {
		if hasACU {
			if err := atomicWriteJSON(cumulativePath, map[string]any{acuKey: acuValue}); err != nil {
				return fmt.Errorf("write devin cumulative usage: %w", err)
			}
			if predecessorMissing {
				unavailable["providerUnits"] = nil
			} else if earlier, ok := asFloat(previous[acuKey]); ok {
				current, _ := asFloat(acuValue)
				unavailable["providerUnits"] = map[string]any{"name": "acu", "value": current - earlier}
			}
		}
		if err := atomicWriteJSON(usagePath, unavailable); err != nil {
			return fmt.Errorf("write devin usage: %w", err)
		}
		return nil
	}

	cumulative := map[string]any{}
	for field, value := range totals {
		cumulative[field] = value
	}
	if err := atomicWriteJSON(cumulativePath, cumulative); err != nil {
		return fmt.Errorf("write devin cumulative usage: %w", err)
	}
	if predecessorMissing {
		if err := atomicWriteJSON(usagePath, unavailable); err != nil {
			return fmt.Errorf("write devin usage: %w", err)
		}
		return nil
	}

	delta := func(field string) int64 {
		if earlier, ok := asInt(previous[field]); ok {
			return totals[field] - earlier
		}
		return totals[field]
	}
	usage := map[string]any{
		"availability":      "native",
		"inputTokens":       delta("total_prompt_tokens"),
		"cachedInputTokens": delta("total_cached_tokens"),
		"outputTokens":      delta("total_completion_tokens"),
		"reasoningTokens":   nil,
		"cost":              nil,
		"providerUnits":     map[string]any{"name": "devin-steps", "value": delta("total_steps")},
	}
	if err := atomicWriteJSON(usagePath, usage); err != nil {
		return fmt.Errorf("write devin usage: %w", err)
	}
	return nil
}

// devinProviderUnit finds the first metric, in sorted key order, whose name
// mentions ACU and whose value is a number, reporting it as a metered unit. The
// exact key is not fixed across accounts, so anything named for ACU counts and
// nothing is invented when none does.
func devinProviderUnit(metrics map[string]any) (string, any, bool) {
	for _, name := range sortedStringKeys(metrics) {
		if !strings.Contains(strings.ToLower(name), "acu") {
			continue
		}
		if isNumber(metrics[name]) {
			return name, metrics[name], true
		}
	}
	return "", nil, false
}

// WriteUnavailableUsage writes the typed-usage record for a turn whose spend
// cannot be trusted as complete, so an aggregate reads it as unavailable rather
// than as an undercount.
func WriteUnavailableUsage(outputPath string) error {
	value := map[string]any{
		"availability":      "unavailable",
		"inputTokens":       nil,
		"cachedInputTokens": nil,
		"outputTokens":      nil,
		"reasoningTokens":   nil,
		"cost":              nil,
		"providerUnits":     nil,
	}
	if err := atomicWriteJSON(outputPath, value); err != nil {
		return fmt.Errorf("write unavailable usage: %w", err)
	}
	return nil
}
