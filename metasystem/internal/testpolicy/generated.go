package testpolicy

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/pathpattern"
)

// Generated declares outputs and the argv that rebuild them, relative to the
// installation. Commands run in order without a shell; an omitted cwd is ".".
type Generated struct {
	Paths   []string `json:"paths"`
	Command []string `json:"command"`
	Then    []string `json:"then,omitempty"`
	Cwd     string   `json:"cwd,omitempty"`
}

func validateGenerated(sets []Generated) error {
	identities := map[string]bool{}
	patterns := map[string]bool{}
	for i, set := range sets {
		if identities[set.Identity()] {
			return fmt.Errorf("generated set %d repeats the same paths", i+1)
		}
		identities[set.Identity()] = true
		if len(set.Paths) == 0 {
			return fmt.Errorf("generated set %d requires paths", i+1)
		}
		for _, pattern := range set.Paths {
			parsed, err := pathpattern.Parse(pattern)
			if err != nil {
				return fmt.Errorf("generated set %d: %w", i+1, err)
			}
			if patterns[parsed.String()] {
				return fmt.Errorf("generated set %d repeats path pattern %q", i+1, pattern)
			}
			patterns[parsed.String()] = true
		}
		if set.Cwd != "" && !validRelative(set.Cwd) {
			return fmt.Errorf("generated set %d cwd must be relative to the installation", i+1)
		}
		for j, argv := range [][]string{set.Command, set.Then} {
			if j == 1 && argv == nil {
				continue
			}
			if len(argv) == 0 || strings.TrimSpace(argv[0]) == "" {
				return fmt.Errorf("generated set %d requires a command argv", i+1)
			}
			for _, arg := range argv {
				if strings.ContainsRune(arg, 0) {
					return fmt.Errorf("generated set %d command contains a NUL", i+1)
				}
			}
		}
	}
	return nil
}

// Identity binds a regeneration recipe to its sorted output patterns.
func (set Generated) Identity() string {
	paths := append([]string(nil), set.Paths...)
	sort.Strings(paths)
	data, _ := json.Marshal(paths)
	return string(data)
}

// Generates reports whether path is an output of this set.
func (set Generated) Generates(path string) bool {
	for _, value := range set.Paths {
		pattern, err := pathpattern.Parse(value)
		if err == nil && pattern.Match(path) {
			return true
		}
	}
	return false
}
