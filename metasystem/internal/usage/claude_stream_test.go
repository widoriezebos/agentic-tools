package usage

import (
	"os"
	"path/filepath"
	"testing"
)

// Proof 8 (acp-adapter-seam slice three): a dead streamed round —
// no appended result line in events.jsonl, partial stream with a
// usage block — recovers through the stream fallback; a blocking
// round recovers exactly as before.
func TestClaudeRecovererStreamFallback(t *testing.T) {
	recoverer, ok := recoverers["claude"]
	if !ok {
		t.Fatal("claude recoverer unregistered")
	}
	dir := t.TempDir()
	events := filepath.Join(dir, "events.jsonl")
	stream := filepath.Join(dir, "claude-stream.jsonl")

	// Blocking round: usage in events.jsonl wins, source unchanged.
	if err := os.WriteFile(events, []byte(`{"type":"result","usage":{"input_tokens":7,"output_tokens":2}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	outcome := recoverer(RecoveryContext{EventsPath: events})
	if outcome.Fields["inputTokens"] == nil || outcome.Source != events {
		t.Fatalf("blocking recovery changed: %+v", outcome)
	}

	// Dead streamed round: events.jsonl holds only session-init (no
	// usage); the stream's partial usage is the evidence.
	if err := os.WriteFile(events, []byte(`{"type":"system","subtype":"init"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stream, []byte(`{"type":"assistant","usage":{"input_tokens":4,"output_tokens":1}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	outcome = recoverer(RecoveryContext{EventsPath: events})
	if outcome.Fields["inputTokens"] == nil || outcome.Source != stream {
		t.Fatalf("stream fallback missed: %+v", outcome)
	}
}
