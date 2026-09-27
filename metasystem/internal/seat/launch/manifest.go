package launch

// The runtime-declared local configuration a new machine gets.
//
// `validate session-isolation` takes the paths in a file. The paths are each
// runtime's declared local configuration in the runtime registry
// (internal/runtimes, LocalConfigPaths), the one definition second-session.sh
// and the goal worktree isolation read too.

import (
	"sort"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/runtimes"
)

// LocalConfigPaths is every adapter-bearing runtime's declared local
// configuration, deduplicated, in the sorted order the manifest is written in.
var LocalConfigPaths = declaredLocalConfigPaths()

func declaredLocalConfigPaths() []string {
	seen := map[string]bool{}
	var paths []string
	for _, runtime := range runtimes.WithAdapter() {
		declared, _ := runtimes.LocalConfigPaths(runtime)
		for _, path := range declared {
			if !seen[path] {
				seen[path] = true
				paths = append(paths, path)
			}
		}
	}
	sort.Strings(paths)
	return paths
}

// Manifest is the file's contents: one relative path per line.
func Manifest() string {
	return strings.Join(LocalConfigPaths, "\n") + "\n"
}
