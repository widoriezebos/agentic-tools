package main

import (
	"path/filepath"
	"testing"
)

// TestIntentCloseRerunsAClosedCriticWithoutClosure: a critic chain an earlier
// engine closed without a closure (a person's accepted risk once closed with
// no read) runs the whole close owner again, which records the closure the
// register now earns; a chain closed with its closure is unchanged and
// starts nothing.
func TestIntentCloseRerunsAClosedCriticWithoutClosure(t *testing.T) {
	t.Parallel()
	b := newDeliveryBed(t)
	record := map[string]any{"jobId": "critclosed", "role": "code-critic", "status": "completed", "round": 1, "chainClosed": true,
		"findingRegister": []any{}, "findingRegisterRound": 1, "reviewRoundLimit": 3, "criticRoundsConsumed": 1}
	b.writeJob(record)
	b.writeJSON(filepath.Join(b.install, "artifacts", "agents", "critclosed", "rounds", "1", "return.json"),
		map[string]any{"jobId": "critclosed", "round": 1, "findings": []any{}, "verdict": "clean"})
	decisions := filepath.Join(b.root(), "decisions.md")
	b.writeFile(decisions, deliveryDispositionsHeader)
	b.handler = func(intentProcess) intentProcessResult {
		closed := b.job("critclosed")
		closed["closure"] = map[string]any{"criticRoot": "critclosed"}
		b.writeJob(closed)
		return intentProcessResult{}
	}
	code, result := b.do("work", "finish", "j2:critclosed", "--dispositions", decisions)
	if len(b.calls) != 1 || result.Outcome != intentConfirmed {
		t.Fatalf("a closed critic without its closure did not run the close again: code=%d %+v calls=%v", code, result, b.calls)
	}
	code, result = b.do("work", "finish", "j2:critclosed", "--dispositions", decisions)
	if len(b.calls) != 1 || result.Outcome != intentUnchanged {
		t.Fatalf("a closed critic with its closure ran the close again: code=%d %+v calls=%v", code, result, b.calls)
	}
}
