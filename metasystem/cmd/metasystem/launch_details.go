package main

import "github.com/widoriezebos/agentic-tools/metasystem/internal/launch"

// launchDetails is a launch refusal's code-first detail line ("CODE facts:
// reason") for --verbose and --json; nil when err carries no code, since its
// plain text is already the summary.
func launchDetails(err error) []string {
	if launch.ErrorCode(err) == "" {
		return nil
	}
	return []string{launch.ErrorDetail(err)}
}
