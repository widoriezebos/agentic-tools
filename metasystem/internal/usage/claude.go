package usage

// Claude's usage recovery — a per-runtime seam file (agnosticism audit
// class 6, critique r1-5: claude appends its full result, including
// top-level usage, to the round's event stream, and the shared parser
// already recognizes claude's field spellings — a claude round IS
// partially recoverable and always was; this file states it).

// The claude dead-round recoverer shares codex's event-stream walk:
// both runtimes land a usage block in rounds/N/events.jsonl.
func init() {
	RegisterRecoverer("claude", func(ctx RecoveryContext) RecoveryOutcome {
		return RecoveryOutcome{State: Recovered, Fields: CodexUsageValue(ctx.EventsPath), Source: ctx.EventsPath}
	})
}
