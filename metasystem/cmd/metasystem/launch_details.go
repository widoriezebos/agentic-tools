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

// launchAccount is a launch refusal as a person reads it and its details:
// the plain reason, and the code-first detail line (or the whole message
// when err carries no code).
func launchAccount(err error) (string, []string) {
	message := err.Error()
	if details := launchDetails(err); details != nil {
		return unitRunnerAccount(message), details
	}
	return unitRunnerAccount(message), []string{message}
}
