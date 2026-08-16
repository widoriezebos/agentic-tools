// Package hooks checks that this repository runs under the metasystem it ships.
// The template never adopts itself, so its
// own lifecycle hooks can silently be inert; a metasystem whose own repository
// does not run under it is testing a claim it never makes true of itself.
package hooks

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
)

// CheckOwnHooks verifies the live settings carry every lifecycle hook
// the metasystem ships, invoke the supervision hook, and enter the
// vendored metasystem directory via the RUNTIME-DECLARED marker (the
// registry's liveSelfCheck; core names no runtime here — agnosticism
// audit class 4). It returns nil when the repository runs under itself.
func CheckOwnHooks(livePath, shippedPath, vendoredMarker string) error {
	if strings.TrimSpace(vendoredMarker) == "" {
		return fmt.Errorf("hooks check requires a nonblank vendored-entry marker")
	}
	liveData, err := os.ReadFile(livePath)
	if err != nil {
		return fmt.Errorf("cannot read hook configuration: %w", err)
	}
	shippedData, err := os.ReadFile(shippedPath)
	if err != nil {
		return fmt.Errorf("cannot read hook configuration: %w", err)
	}
	var live, shipped struct {
		Hooks map[string]json.RawMessage `json:"hooks"`
	}
	if json.Unmarshal(liveData, &live) != nil || json.Unmarshal(shippedData, &shipped) != nil {
		return fmt.Errorf("cannot read hook configuration: not JSON")
	}

	var missing []string
	for name := range shipped.Hooks {
		if _, ok := live.Hooks[name]; !ok {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return fmt.Errorf("this repository is missing its own lifecycle hooks: %s", strings.Join(missing, ", "))
	}

	// STRUCTURAL check, not substring (review foundations-11): the
	// supervision command must live inside the SAME event arrays the
	// shipped configuration puts it in, and those commands must enter the
	// vendored metasystem directory. A substring scan over the whole file
	// passed exactly the inert states this package exists to catch — a
	// supervision hook moved to an event that never fires, or the path
	// mentioned in an unrelated hook's arguments.
	for name, shippedRaw := range shipped.Hooks {
		if !anyInvokesSupervision(commandStrings(shippedRaw)) {
			continue
		}
		liveCommands := commandStrings(live.Hooks[name])
		matched := false
		for _, command := range liveCommands {
			if supervisionInvokeRe.MatchString(command) {
				if !strings.Contains(command, vendoredMarker) {
					return fmt.Errorf("this repository's %s supervision hook does not enter the vendored metasystem directory", name)
				}
				matched = true
			}
		}
		if !matched {
			return fmt.Errorf("this repository's %s hooks do not invoke the supervision hook", name)
		}
	}
	return nil
}

// supervisionInvokeRe matches the supervision script in COMMAND position —
// the start of the command or right after a shell connector, optionally
// through an interpreter — never a mere mention in another command's
// arguments (the false-pass foundations-11 names).
var supervisionInvokeRe = regexp.MustCompile(`(?:^|&&|\|\||;)\s*(?:bash\s+|sh\s+)?\S*scripts/agents/supervision-hook\.sh"?(?:\s|$)`)

func anyInvokesSupervision(commands []string) bool {
	for _, command := range commands {
		if supervisionInvokeRe.MatchString(command) {
			return true
		}
	}
	return false
}

// commandStrings collects every "command" string reachable inside one
// event's hook configuration, whatever nesting the settings dialect uses.
func commandStrings(raw json.RawMessage) []string {
	if raw == nil {
		return nil
	}
	var value any
	if json.Unmarshal(raw, &value) != nil {
		return nil
	}
	var commands []string
	var walk func(node any)
	walk = func(node any) {
		switch typed := node.(type) {
		case map[string]any:
			for key, child := range typed {
				if key == "command" {
					if command, ok := child.(string); ok {
						commands = append(commands, command)
					}
					continue
				}
				walk(child)
			}
		case []any:
			for _, child := range typed {
				walk(child)
			}
		}
	}
	walk(value)
	return commands
}
