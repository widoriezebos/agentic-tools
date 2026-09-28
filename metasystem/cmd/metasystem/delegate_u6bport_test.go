package main

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/delegation"
)

// dispatch-fixtures structured-budget-within/-extended (lines 1520-1535,
// 1590-1600): the delegate verb preserves the typed winning outcome of a
// launch, and a refusal the lifecycle recorded (REFUSED-BUDGET,
// REFUSED-OPID-MISMATCH) passes through with its exit code.
func TestDelegateResultPreservesTheTypedOutcome(t *testing.T) {
	t.Parallel()
	decode := func(t *testing.T, out *bytes.Buffer) map[string]any {
		t.Helper()
		var object map[string]any
		if err := json.Unmarshal(bytes.TrimSpace(out.Bytes()), &object); err != nil || bytes.Count(bytes.TrimSpace(out.Bytes()), []byte("\n")) != 0 {
			t.Fatalf("the boundary did not answer one JSON line: %q (%v)", out.String(), err)
		}
		return object
	}

	for _, tc := range []struct {
		name   string
		result delegation.Result
		code   int
		want   map[string]any
	}{
		{"started job", delegation.Result{Stdout: []byte("structured-budget-within\n")}, 0,
			map[string]any{"outcome": "WON", "headline": "started", "jobId": "structured-budget-within"}},
		{"waited job", delegation.Result{}, 0,
			map[string]any{"outcome": "WON", "headline": "started"}},
		{"recorded budget refusal", delegation.Result{ExitCode: 1,
			Outcome: []byte(`{"outcome":"REFUSED-BUDGET","headline":"refused","detail":"BUDGET_REFUSED: goal g","jobId":"j"}` + "\n")}, 1,
			map[string]any{"outcome": "REFUSED-BUDGET", "headline": "refused", "detail": "BUDGET_REFUSED: goal g", "jobId": "j"}},
		{"claim refusal without a headline", delegation.Result{ExitCode: 1,
			Outcome: []byte(`{"outcome":"REFUSED-OPID-MISMATCH","evidence":{"recordPath":"artifacts/agents/jobs/op.json"}}` + "\n")}, 1,
			map[string]any{"outcome": "REFUSED-OPID-MISMATCH", "headline": "refused", "jobId": "op"}},
	} {
		var stdout, stderr bytes.Buffer
		code := writeDelegateResult(tc.result, "dispatch", nil, "", &stdout, &stderr)
		if code != tc.code {
			t.Fatalf("%s: exit %d, want %d", tc.name, code, tc.code)
		}
		got := decode(t, &stdout)
		for key, value := range tc.want {
			if got[key] != value {
				t.Fatalf("%s: %s=%v, want %v (%v)", tc.name, key, got[key], value, got)
			}
		}
		if _, has := got["jobId"]; !has && tc.want["jobId"] != nil {
			t.Fatalf("%s: jobId missing", tc.name)
		}
	}
}
