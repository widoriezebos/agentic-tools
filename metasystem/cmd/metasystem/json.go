package main

import (
	"fmt"
	"os"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/jsonedit"
)

// The json family relays: the shell-facing rendering and edit decisions
// live in internal/jsonedit; these verbs parse
// flags, read files, and print.

// runJSONObject builds a compact JSON object from key=value arguments (string
// values, split on the first '='), printed without HTML escaping. For shell
// callers that need to construct a small JSON object from strings.
func runJSONObject(args []string) int {
	line, err := jsonedit.Object(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Println(line)
	return 0
}
