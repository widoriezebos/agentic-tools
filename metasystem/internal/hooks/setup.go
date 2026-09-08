package hooks

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strings"
)

const gitSteeringUnset = "unset GIT_DIR GIT_WORK_TREE GIT_COMMON_DIR GIT_INDEX_FILE GIT_CEILING_DIRECTORIES GIT_DISCOVERY_ACROSS_FILESYSTEM GIT_OBJECT_DIRECTORY GIT_ALTERNATE_OBJECT_DIRECTORIES GIT_CONFIG GIT_CONFIG_PARAMETERS GIT_CONFIG_COUNT GIT_CONFIG_GLOBAL GIT_CONFIG_SYSTEM GIT_CONFIG_NOSYSTEM GIT_GRAFT_FILE GIT_SHALLOW_FILE GIT_REPLACE_REF_BASE GIT_IMPLICIT_WORK_TREE GIT_NO_REPLACE_OBJECTS GIT_PREFIX"

var lifecycleCommand = regexp.MustCompile(`scripts/agents/supervision-hook\.sh\s+([a-z][a-z0-9-]{0,31})\s+(start|receipt|stop|end)(?:[)[:space:]]|$)`)

// MergeSettings removes only recognized handlers emitted by this installation,
// retains foreign siblings in their original groups, and appends one desired
// group per lifecycle handler declared by the shipped configuration.
func MergeSettings(existing, shipped []byte, runtime, installationRel string, registrationAtInstallation bool) ([]byte, error) {
	desired, known, err := desiredSettings(shipped, runtime, installationRel, registrationAtInstallation)
	if err != nil {
		return nil, err
	}
	current, err := decodeObject(existing)
	if err != nil {
		return nil, fmt.Errorf("hook settings conflict: %w", err)
	}
	shippedObject, err := decodeObject(shipped)
	if err != nil {
		return nil, fmt.Errorf("shipped hook configuration is invalid: %w", err)
	}
	for key, value := range shippedObject {
		if key == "hooks" || key == "_comment" {
			continue
		}
		if _, present := current[key]; !present {
			current[key] = value
		}
	}
	hooksValue, present := current["hooks"]
	var eventMap map[string]any
	if !present {
		eventMap = map[string]any{}
	} else {
		var ok bool
		eventMap, ok = hooksValue.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("hook settings conflict: hooks is not an object")
		}
	}
	for event, value := range eventMap {
		groups, ok := value.([]any)
		if !ok {
			return nil, fmt.Errorf("hook settings conflict: event %s is not an array", event)
		}
		kept := make([]any, 0, len(groups))
		for _, rawGroup := range groups {
			group, ok := rawGroup.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("hook settings conflict: event %s contains a non-object group", event)
			}
			rawHandlers, ok := group["hooks"].([]any)
			if !ok {
				return nil, fmt.Errorf("hook settings conflict: event %s group has no handler array", event)
			}
			handlers := make([]any, 0, len(rawHandlers))
			for _, rawHandler := range rawHandlers {
				handler, ok := rawHandler.(map[string]any)
				if !ok {
					return nil, fmt.Errorf("hook settings conflict: event %s contains a non-object handler", event)
				}
				command, _ := handler["command"].(string)
				if known[command] {
					continue
				}
				if uncertainMetaSystemCommand(command, runtime) {
					return nil, fmt.Errorf("hook settings conflict: event %s contains an unrecognized MetaSystem handler", event)
				}
				handlers = append(handlers, handler)
			}
			if len(handlers) == 0 {
				continue
			}
			group["hooks"] = handlers
			kept = append(kept, group)
		}
		eventMap[event] = kept
	}
	for event, groups := range desired {
		existingGroups, _ := eventMap[event].([]any)
		eventMap[event] = append(existingGroups, groups...)
	}
	current["hooks"] = eventMap
	encoded, err := json.MarshalIndent(current, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(encoded, '\n'), nil
}

// CheckSettings validates the event, matcher, command type, runtime action,
// timeout, and synchronous behavior of every desired lifecycle group,
// including duplicate ownership.
func CheckSettings(live, shipped []byte, runtime, installationRel string, registrationAtInstallation bool) error {
	desired, known, err := desiredSettings(shipped, runtime, installationRel, registrationAtInstallation)
	if err != nil {
		return err
	}
	current, err := decodeObject(live)
	if err != nil {
		return fmt.Errorf("cannot read hook configuration: %w", err)
	}
	events, ok := current["hooks"].(map[string]any)
	if !ok {
		return fmt.Errorf("hook configuration has no hooks object")
	}
	owned, err := countRecognizedHandlers(events, known, runtime)
	if err != nil {
		return err
	}
	for event, wantedGroups := range desired {
		groups, ok := events[event].([]any)
		if !ok {
			return fmt.Errorf("hook configuration is missing lifecycle event %s", event)
		}
		for _, wanted := range wantedGroups {
			wantedGroup := wanted.(map[string]any)
			wantedHandlers := wantedGroup["hooks"].([]any)
			for _, rawWantedHandler := range wantedHandlers {
				wantedHandler := rawWantedHandler.(map[string]any)
				matches := 0
				for _, rawCandidate := range groups {
					candidate, ok := rawCandidate.(map[string]any)
					if !ok {
						return fmt.Errorf("hook configuration event %s contains a non-object group", event)
					}
					if !sameMatcher(candidate, wantedGroup) {
						continue
					}
					candidateHandlers, ok := candidate["hooks"].([]any)
					if !ok {
						return fmt.Errorf("hook configuration event %s group has no handler array", event)
					}
					for _, rawCandidateHandler := range candidateHandlers {
						candidateHandler, ok := rawCandidateHandler.(map[string]any)
						if !ok {
							return fmt.Errorf("hook configuration event %s contains a non-object handler", event)
						}
						if sameHandlerContract(candidateHandler, wantedHandler) {
							matches++
						}
					}
				}
				if matches != 1 {
					return fmt.Errorf("hook configuration event %s has %d handlers with the expected matcher, type, command, runtime action, timeout, and synchronous behavior; want 1", event, matches)
				}
			}
		}
	}
	wantOwned := 0
	for _, groups := range desired {
		for _, group := range groups {
			wantOwned += len(group.(map[string]any)["hooks"].([]any))
		}
	}
	if owned != wantOwned {
		return fmt.Errorf("hook configuration has %d owned handlers; want %d", owned, wantOwned)
	}
	return nil
}

func countRecognizedHandlers(events map[string]any, known map[string]bool, runtime string) (int, error) {
	owned := 0
	for event, raw := range events {
		groups, ok := raw.([]any)
		if !ok {
			return 0, fmt.Errorf("hook configuration event %s is not an array", event)
		}
		for _, rawGroup := range groups {
			group, ok := rawGroup.(map[string]any)
			if !ok {
				return 0, fmt.Errorf("hook configuration event %s contains a non-object group", event)
			}
			handlers, ok := group["hooks"].([]any)
			if !ok {
				return 0, fmt.Errorf("hook configuration event %s group has no handler array", event)
			}
			for _, rawHandler := range handlers {
				handler, ok := rawHandler.(map[string]any)
				if !ok {
					return 0, fmt.Errorf("hook configuration event %s contains a non-object handler", event)
				}
				command, _ := handler["command"].(string)
				if known[command] {
					owned++
				} else if uncertainMetaSystemCommand(command, runtime) {
					return 0, fmt.Errorf("hook configuration event %s contains an unrecognized MetaSystem handler", event)
				}
			}
		}
	}
	return owned, nil
}

func desiredSettings(shipped []byte, runtime, installationRel string, registrationAtInstallation bool) (map[string][]any, map[string]bool, error) {
	source, err := decodeObject(shipped)
	if err != nil {
		return nil, nil, fmt.Errorf("shipped hook configuration is invalid: %w", err)
	}
	sourceEvents, ok := source["hooks"].(map[string]any)
	if !ok {
		return nil, nil, fmt.Errorf("shipped hook configuration has no hooks object")
	}
	desired := map[string][]any{}
	known := knownLegacyCommands(runtime, installationRel, registrationAtInstallation)
	for event, raw := range sourceEvents {
		groups, ok := raw.([]any)
		if !ok {
			return nil, nil, fmt.Errorf("shipped event %s is not an array", event)
		}
		for _, rawGroup := range groups {
			group, ok := rawGroup.(map[string]any)
			if !ok {
				return nil, nil, fmt.Errorf("shipped event %s contains a non-object group", event)
			}
			handlers, ok := group["hooks"].([]any)
			if !ok {
				return nil, nil, fmt.Errorf("shipped event %s group has no handlers", event)
			}
			newHandlers := make([]any, 0, len(handlers))
			for _, rawHandler := range handlers {
				handler, ok := rawHandler.(map[string]any)
				if !ok {
					return nil, nil, fmt.Errorf("shipped event %s contains a non-object handler", event)
				}
				command, _ := handler["command"].(string)
				match := lifecycleCommand.FindStringSubmatch(command)
				if len(match) != 3 || match[1] != runtime {
					return nil, nil, fmt.Errorf("shipped event %s has a handler without the expected runtime action", event)
				}
				if registrationAtInstallation {
					known[command] = true
				}
				desiredCommand := renderCommand(runtime, match[2], installationRel, strings.Contains(command, `"decision":"block"`))
				known[desiredCommand] = true
				newHandler := cloneMap(handler)
				newHandler["command"] = desiredCommand
				newHandlers = append(newHandlers, newHandler)
			}
			newGroup := cloneMap(group)
			newGroup["hooks"] = newHandlers
			desired[event] = append(desired[event], newGroup)
		}
	}
	return desired, known, nil
}

func renderCommand(runtime, action, installationRel string, failClosed bool) string {
	if action == "stop" || failClosed {
		return renderGitRequiredCommand(runtime, action, installationRel, failClosed)
	}
	destination := renderDestination(installationRel)
	// A fresh adopted installation can exist before `git init`. Ordinary
	// lifecycle discovery is then a quiet no-op, but a malformed repository,
	// missing selected installation, or invoked handler failure remains an
	// error. Looking for an actual .git entry distinguishes absence from a Git
	// command that failed against a repository it should have understood.
	discovery := `command -v git >/dev/null 2>&1 || exit $?; repo=$(git rev-parse --show-toplevel 2>/dev/null); git_status=$?; if [ "$git_status" -ne 0 ]; then if git rev-parse --git-dir >/dev/null 2>&1; then git rev-parse --show-toplevel >/dev/null; exit $?; fi; probe=$(pwd -P) || exit $?; while [ "$probe" != / ] && [ ! -e "$probe/.git" ] && [ ! -L "$probe/.git" ]; do probe=${probe%/*}; [ -n "$probe" ] || probe=/; done; if [ ! -e "$probe/.git" ] && [ ! -L "$probe/.git" ]; then exit 0; fi; git rev-parse --show-toplevel >/dev/null; exit $?; fi`
	return gitSteeringUnset + `; ` + discovery + `; cd ` + destination + ` && bash scripts/agents/supervision-hook.sh ` + runtime + ` ` + action
}

func renderGitRequiredCommand(runtime, action, installationRel string, failClosed bool) string {
	destination := renderDestination(installationRel)
	core := gitSteeringUnset + `; repo=$(git rev-parse --show-toplevel) && cd ` + destination + ` && bash scripts/agents/supervision-hook.sh ` + runtime + ` ` + action
	if failClosed {
		return `(` + core + `) || printf '%s\n' '{"decision":"block","reason":"Metasystem Stop hook launcher failed before a safe verdict; stopping is refused."}'`
	}
	return core
}

func renderDestination(installationRel string) string {
	destination := `"$repo"`
	if installationRel != "" && installationRel != "." {
		destination = `"$repo/` + shellDoubleQuoted(installationRel) + `"`
	}
	return destination
}

func shellDoubleQuoted(value string) string {
	return strings.NewReplacer(
		"\\", "\\\\",
		"\"", "\\\"",
		"$", "\\$",
		"`", "\\`",
	).Replace(value)
}

func knownLegacyCommands(runtime, installationRel string, registrationAtInstallation bool) map[string]bool {
	known := map[string]bool{}
	for _, action := range []string{"start", "receipt", "stop", "end"} {
		failClosed := action == "stop" && runtime == "claude"
		known[renderCommand(runtime, action, installationRel, failClosed)] = true
		known[renderGitRequiredCommand(runtime, action, installationRel, failClosed)] = true
	}
	legacyDirectory := `$CLAUDE_PROJECT_DIR`
	if !registrationAtInstallation {
		legacyDirectory += "/" + installationRel
	}
	legacyPrefix := `cd "` + legacyDirectory + `" && `
	for _, action := range []string{"start", "receipt", "stop", "end"} {
		known[legacyPrefix+"bash scripts/agents/supervision-hook.sh "+runtime+" "+action] = true
	}
	if registrationAtInstallation {
		for _, action := range []string{"start", "receipt", "stop", "end"} {
			known["bash scripts/agents/supervision-hook.sh "+runtime+" "+action] = true
		}
	}
	if runtime == "claude" {
		known[`(cd "`+legacyDirectory+`" && bash scripts/agents/supervision-hook.sh claude stop) || printf '%s\n' '{"decision":"block","reason":"Metasystem Stop hook launcher failed before a safe verdict; stopping is refused."}'`] = true
		for _, noun := range []string{"Metasystem", "Harness"} {
			known[`if ! cd "`+legacyDirectory+`" 2>/dev/null; then echo '{"systemMessage":"`+noun+` hook could not resolve the project directory (CLAUDE_PROJECT_DIR)."}'; else bash scripts/receipt.sh check >/dev/null 2>&1; rc=$?; if [ "$rc" -eq 1 ]; then echo '{"systemMessage":"`+noun+` retro due: run scripts/receipt.sh check for details, then skills/retro."}'; elif [ "$rc" -ne 0 ]; then echo '{"systemMessage":"`+noun+` receipt check errored; run scripts/receipt.sh check to see why."}'; fi; fi`] = true
		}
	}
	return known
}

func sameMatcher(candidate, wanted map[string]any) bool {
	wantedMatcher, wantedHasMatcher := wanted["matcher"]
	candidateMatcher, candidateHasMatcher := candidate["matcher"]
	if !wantedHasMatcher || wantedMatcher == "" {
		return !candidateHasMatcher || candidateMatcher == ""
	}
	return candidateHasMatcher && equalJSON(candidateMatcher, wantedMatcher)
}

func sameHandlerContract(candidate, wanted map[string]any) bool {
	for _, field := range []string{"type", "command", "timeout"} {
		candidateValue, candidateHas := candidate[field]
		wantedValue, wantedHas := wanted[field]
		if candidateHas != wantedHas || (wantedHas && !equalJSON(candidateValue, wantedValue)) {
			return false
		}
	}
	// Lifecycle handlers must remain synchronous. An absent async field and an
	// explicit false are equivalent provider defaults; true cannot enforce a
	// blocking lifecycle decision.
	wantedValue, wantedHas := wanted["async"]
	wantedBool, wantedIsBool := wantedValue.(bool)
	if !wantedHas || (wantedIsBool && !wantedBool) {
		candidateValue, candidateHas := candidate["async"]
		if !candidateHas {
			return true
		}
		candidateBool, ok := candidateValue.(bool)
		return ok && !candidateBool
	}
	candidateValue, candidateHas := candidate["async"]
	return candidateHas && equalJSON(candidateValue, wantedValue)
}

func uncertainMetaSystemCommand(command, runtime string) bool {
	return command != "" && ((strings.Contains(command, "scripts/agents/supervision-hook.sh") && strings.Contains(command, " "+runtime+" ")) || strings.Contains(command, "scripts/receipt.sh check"))
}

func decodeObject(data []byte) (map[string]any, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var object map[string]any
	if err := decoder.Decode(&object); err != nil {
		return nil, err
	}
	if object == nil {
		return nil, fmt.Errorf("top level is not an object")
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return nil, fmt.Errorf("trailing JSON values")
	}
	return object, nil
}

func cloneMap(source map[string]any) map[string]any {
	copy := make(map[string]any, len(source))
	for key, value := range source {
		copy[key] = value
	}
	return copy
}

func equalJSON(a, b any) bool {
	left, _ := json.Marshal(a)
	right, _ := json.Marshal(b)
	return bytes.Equal(left, right)
}
