package launch

// The adapter-declared local configuration a new machine or a second session
// gets.
//
// Each adapter declares its own local configuration (`adapter.sh
// local-config-paths`); `validate session-isolation` takes the paths in a
// file. Neither a seat launch nor a second session runs the adapters for it:
// the list is written here, once, and manifest_test.go runs every shipped
// adapter and compares their declarations with it, so a new adapter that
// declares a local file fails there, by name, rather than launching machines
// or sessions that quietly lack it.

import "strings"

// LocalConfigPaths is the adapters' declared local configuration, in the
// sorted order the manifest is written in.
var LocalConfigPaths = []string{
	".claude/settings.json",
	".claude/settings.local.json",
	".codex/config.toml",
	".devin/config.json",
	".devin/config.local.json",
	".devin/hooks.v1.json",
}

// Manifest is the file's contents: one relative path per line.
func Manifest() string {
	return strings.Join(LocalConfigPaths, "\n") + "\n"
}
