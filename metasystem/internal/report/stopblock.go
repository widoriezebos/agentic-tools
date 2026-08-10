// Package report holds the turn-end report decisions: the stop-hook block that
// refuses to end a turn while planned work is unblocked and idle (the Go port
// of stop-block.py), and later the open-work check.
package report

// stopBlockReason is the fixed guidance a block carries. The refusal is
// bounded — the caller blocks only the first time a given set of open work is
// seen — so this text tells the agent to act or record why it cannot.
const stopBlockReason = "Work named in a plan is unblocked and nothing is in flight. Do it now, " +
	"or record in the plan why it is blocked or waiting on the human. " +
	"This refusal does not repeat for the same work.\n\n"

// StopBlock builds the stop-hook block decision, appending any caller detail.
func StopBlock(detail string) map[string]any {
	return map[string]any{
		"decision": "block",
		"reason":   stopBlockReason + detail,
	}
}
