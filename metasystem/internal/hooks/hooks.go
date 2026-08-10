// Package hooks checks that this repository runs under the metasystem it ships
// (the Go port of check-own-hooks.py). The template never adopts itself, so its
// own lifecycle hooks can silently be inert; a metasystem whose own repository
// does not run under it is testing a claim it never makes true of itself.
package hooks

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
)

// CheckOwnHooks verifies the live Claude settings carry every lifecycle hook
// the metasystem ships, invoke the supervision hook, and enter the vendored
// metasystem directory (since here the metasystem is one level down, not the
// project root). It returns nil when the repository runs under itself.
func CheckOwnHooks(livePath, shippedPath string) error {
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

	flat := string(liveData)
	if !strings.Contains(flat, "supervision-hook.sh") {
		return fmt.Errorf("this repository's hooks do not invoke the supervision hook")
	}
	if !strings.Contains(flat, "$CLAUDE_PROJECT_DIR/metasystem") {
		return fmt.Errorf("this repository's hooks do not enter the vendored metasystem directory")
	}
	return nil
}
