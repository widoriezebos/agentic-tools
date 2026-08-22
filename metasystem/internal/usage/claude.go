package usage

// Claude's usage recovery — a per-runtime seam file:
// claude appends its full result, including
// top-level usage, to the round's event stream, and the shared parser
// already recognizes claude's field spellings — a claude round IS
// partially recoverable; this file states it.

// The claude dead-round recoverer rides the NEUTRAL event-stream
// walk (usage.go): both runtimes land a usage block in
// rounds/N/events.jsonl.
func init() {
	RegisterRecoverer("claude", func(ctx RecoveryContext) RecoveryOutcome {
		return RecoveryOutcome{State: Recovered, Fields: eventStreamUsageValue(ctx.EventsPath), Source: ctx.EventsPath}
	})
}
