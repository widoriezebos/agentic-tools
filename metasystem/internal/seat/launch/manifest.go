package launch

// The adapter-declared local configuration a new machine gets.
//
// `validate session-isolation` takes the paths in a file, and second-session.sh
// builds that file by asking every adapter for its own `local-config-paths`.
// This verb cannot: it runs one shell script and one only, the kit's designated
// build owner, so the list is written here instead of collected by running four
// adapters.
//
// That leaves one copy of a contract in two places, which is exactly what drifts
// unnoticed — so it does not go unnoticed: scripts/agents/second-session-fixtures.sh
// declares the same list as a literal it diffs the adapters against, and
// manifest_test.go reads that literal out of the script and compares it with
// this one. A new adapter that declares a local file fails here, by name,
// rather than launching machines that quietly lack it.

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
