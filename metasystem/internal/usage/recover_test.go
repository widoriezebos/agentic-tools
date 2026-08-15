package usage

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// The recovery table: registered runtimes recover from their event
// streams; an undeclared provider is honestly unsupported; the claude
// and codex recoverers preserve today's field-for-field parse,
// including the malformed-tail case (a truncated final line never
// becomes an error).
func TestRecoveryTable(t *testing.T) {
	dir := t.TempDir()
	events := filepath.Join(dir, "events.jsonl")
	os.WriteFile(events, []byte(
		`{"usage":{"input_tokens":10,"output_tokens":4,"reasoning_tokens":2}}`+"\n"+
			`{"torn`), 0o644)
	ctx := RecoveryContext{Repo: dir, RoundDir: dir, EventsPath: events}

	for _, runtime := range []string{"claude", "codex"} {
		outcome := Recover(runtime, ctx)
		if outcome.State != Recovered || outcome.Source != events {
			t.Fatalf("%s recovery wrong: %+v", runtime, outcome)
		}
		for field, want := range map[string]string{
			"availability": "native", "inputTokens": "10",
			"cachedInputTokens": "<nil>", "outputTokens": "4",
			"reasoningTokens": "2", "cost": "<nil>", "providerUnits": "<nil>",
		} {
			if got := fmt.Sprint(outcome.Fields[field]); got != want {
				t.Fatalf("%s field %s = %s, want %s", runtime, field, got, want)
			}
		}
	}

	// Devin declares no dead-round recovery: unsupported, stated.
	outcome := Recover("devin", ctx)
	if outcome.State != Unsupported || !strings.Contains(outcome.Detail, "devin") {
		t.Fatalf("devin recovery not declared unsupported: %+v", outcome)
	}
	if outcome = Recover("ghostrt", ctx); outcome.State != Unsupported {
		t.Fatalf("unknown provider not unsupported: %+v", outcome)
	}

	if got := RecovererList(); !reflect.DeepEqual(got, []string{"claude", "codex"}) {
		t.Fatalf("recoverer list wrong: %v", got)
	}
}

// Registration rejects nil and duplicate keys loudly.
func TestRecovererRegistrationGuards(t *testing.T) {
	expectPanic := func(name string, f func()) {
		defer func() {
			if recover() == nil {
				t.Fatalf("%s did not panic", name)
			}
		}()
		f()
	}
	expectPanic("nil recoverer", func() { RegisterRecoverer("x-nil", nil) })
	expectPanic("duplicate recoverer", func() { RegisterRecoverer("codex", func(RecoveryContext) RecoveryOutcome { return RecoveryOutcome{} }) })
}
