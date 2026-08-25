package usage

import "path/filepath"

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
		fields := eventStreamUsageValue(ctx.EventsPath)
		source := ctx.EventsPath
		// A dead STREAMED round has no appended result line in
		// events.jsonl — its partial stream is the only usage
		// evidence (acp-adapter-seam slice three): fall back to the
		// sibling stream artifact when the events walk found no
		// tokens.
		if fields["inputTokens"] == nil && fields["outputTokens"] == nil {
			stream := filepath.Join(filepath.Dir(ctx.EventsPath), "claude-stream.jsonl")
			if streamed := eventStreamUsageValue(stream); streamed["inputTokens"] != nil || streamed["outputTokens"] != nil {
				fields, source = streamed, stream
			}
		}
		return RecoveryOutcome{State: Recovered, Fields: fields, Source: source}
	})
}
