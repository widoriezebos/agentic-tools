package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/runtimes"
)

// AutoRuntime is the runtime-selection value that names no runtime itself:
// it resolves to the first runtime of metasystem.runtimes, in the order that
// list gives, whose program is on the PATH of the environment asking. The
// list's order is therefore the installation's preference order, and a key
// naming a runtime explicitly always wins over it.
const AutoRuntime = "auto"

// SeatRuntimeOff is launch.seat.runtime's value, and its default, for an
// installation whose steward starts no seat (Amendment 1 of the seat design,
// Wido 2026-09-30: a seat is opt-in per seat).
const SeatRuntimeOff = "off"

var runtimeSelectionKey = regexp.MustCompile(`^(?:role\.[a-z0-9-]+|mode\.[a-z0-9-]+\.role\.[a-z0-9-]+|launch\.[a-z0-9-]+)\.runtime$`)

// RuntimeSelectionKey reports whether key picks the runtime of a role, a
// mode-scoped role or a launch lane, the keys that accept AutoRuntime.
func RuntimeSelectionKey(key string) bool { return runtimeSelectionKey.MatchString(key) }

// RuntimeChoice is what AutoRuntime resolved to and why.
type RuntimeChoice struct {
	Runtime string
	// Order is metasystem.runtimes as resolved, the preference order.
	Order []string
	// Detected says the runtime's program was found on PATH; false means no
	// listed runtime was, and the first listed one was taken.
	Detected bool
}

// Describe says how the choice was made, for a person reading settings.
func (c RuntimeChoice) Describe() string {
	listed := strings.Join(c.Order, ",")
	if c.Detected {
		return "auto: first of " + listed + " on PATH"
	}
	return "auto: none of " + listed + " on PATH, first listed"
}

var autoChoices sync.Map

// ResolveAutoRuntime resolves AutoRuntime for one configuration and one
// environment. The answer is computed once per PATH and preference order in
// a process, so every reader of one run sees the same agent. With no listed
// runtime on PATH it is the first listed runtime: a host without agents then
// resolves as the shipped order says, and the launch that needs the missing
// program refuses when it starts it.
func ResolveAutoRuntime(confPath string, lookupEnv func(string) (string, bool)) (RuntimeChoice, error) {
	if lookupEnv == nil {
		lookupEnv = os.LookupEnv
	}
	listed, _, err := Get(GetParams{Key: "metasystem.runtimes", ConfPath: confPath, LookupEnv: lookupEnv})
	if err != nil {
		return RuntimeChoice{}, err
	}
	var order []string
	for _, name := range strings.Split(listed, ",") {
		if name = strings.TrimSpace(name); name != "" {
			order = append(order, name)
		}
	}
	if len(order) == 0 {
		return RuntimeChoice{}, fmt.Errorf("metasystem.runtimes names no runtime, so %q selects none", AutoRuntime)
	}
	path, _ := lookupEnv("PATH")
	key := path + "\x00" + strings.Join(order, ",")
	if cached, ok := autoChoices.Load(key); ok {
		return cached.(RuntimeChoice), nil
	}
	choice := RuntimeChoice{Runtime: order[0], Order: order}
	for _, name := range order {
		if declaration, known := runtimes.Lookup(name); known && declaration.Executable != "" && onPath(declaration.Executable, path) {
			choice.Runtime, choice.Detected = name, true
			break
		}
	}
	autoChoices.Store(key, choice)
	return choice, nil
}

// onPath reports whether program is an executable file in one of path's
// directories.
func onPath(program, path string) bool {
	for _, directory := range filepath.SplitList(path) {
		if directory == "" {
			continue
		}
		info, err := os.Stat(filepath.Join(directory, program))
		if err == nil && info.Mode().IsRegular() && info.Mode().Perm()&0o111 != 0 {
			return true
		}
	}
	return false
}
