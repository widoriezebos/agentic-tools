package host

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/adapter"
)

// mission-runner scenario, runner-codex: a completed Codex host turn's
// result envelope carries exactly sessionId, outcome, usage, rawPath and
// returnPath; its usage is the typed rendering of the CLI's last usage
// block; the paths name the turn's raw.out and return.json; and the atomic
// write leaves no staging residue beside it.
func TestU6bPortCodexTurnResultEnvelope(t *testing.T) {
	t.Parallel()
	turn := t.TempDir()
	events := filepath.Join(turn, "events.jsonl")
	write(t, events, strings.Join([]string{
		`{"type":"thread.started","thread_id":"codex-fixture-session"}`,
		`{"type":"turn.started","turn_id":"codex-fixture-turn-1"}`,
		`{"type":"turn.completed","usage":{"input_tokens":10,"cached_input_tokens":2,"output_tokens":4,"reasoning_output_tokens":1}}`,
	}, "\n")+"\n")
	usage := filepath.Join(turn, "usage.json")
	if err := adapter.CodexUsage(events, usage); err != nil {
		t.Fatal(err)
	}
	raw := filepath.Join(turn, "raw.out")
	write(t, raw, `{"turnId":"t1"}`+"\n")
	returnPath := filepath.Join(turn, "return.json")
	write(t, returnPath, `{"turnId":"t1"}`+"\n")
	result := filepath.Join(turn, "result.json")
	code, err := FinishTurn(result, "codex-fixture-session", usage, raw, returnPath, "", 0, false)
	if err != nil || code != 0 {
		t.Fatalf("finish = (%d, %v)", code, err)
	}
	got := readObject(t, result)
	want := map[string]any{
		"sessionId": "codex-fixture-session",
		"outcome":   "completed",
		"usage": map[string]any{
			"availability": "native", "inputTokens": float64(10), "cachedInputTokens": float64(2),
			"outputTokens": float64(4), "reasoningTokens": float64(1), "cost": nil, "providerUnits": nil,
		},
		"rawPath":    raw,
		"returnPath": returnPath,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("result envelope:\n got %v\nwant %v", got, want)
	}
	entries, err := os.ReadDir(turn)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "result.json.") || strings.HasSuffix(entry.Name(), ".tmp") {
			t.Fatalf("result staging residue survived: %s", entry.Name())
		}
	}
}
