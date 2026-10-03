package main

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestIntentCloseRerunsAClosedCriticWithoutClosure: a critic chain an earlier
// engine closed without a closure although a person's accepted risk made its
// register landable runs the whole close owner again, which records the
// closure; once the closure is there it is unchanged and starts nothing. A
// chain closed without a closure whose register earns none (here a clean
// register with no read subject) is unchanged and starts nothing.
func TestIntentCloseRerunsAClosedCriticWithoutClosure(t *testing.T) {
	t.Parallel()
	b := newDeliveryBed(t)
	risk := map[string]any{"findingId": "F-2", "critic": "critrisk", "rigorClass": "unproven", "factsDigest": strings.Repeat("a", 64), "facts": nil,
		"artifact": "metasystem/x.go", "title": "title", "status": "accepted-risk", "resolution": "accepted-risk", "decisionOpid": "person-op",
		"evidence": "evidence", "evidenceDigest": strings.Repeat("b", 64), "multiplicity": 1}
	for _, job := range []struct {
		id       string
		register []any
	}{{"critrisk", []any{risk}}, {"critclean", []any{}}} {
		b.writeJob(map[string]any{"jobId": job.id, "role": "code-critic", "status": "completed", "round": 1, "chainClosed": true,
			"findingRegister": job.register, "findingRegisterRound": 1, "reviewRoundLimit": 3, "criticRoundsConsumed": 1})
		b.writeJSON(filepath.Join(b.install, "artifacts", "agents", job.id, "rounds", "1", "return.json"),
			map[string]any{"jobId": job.id, "round": 1, "findings": []any{}, "verdict": "clean"})
	}
	decisions := filepath.Join(b.root(), "decisions.md")
	b.writeFile(decisions, deliveryDispositionsHeader)
	b.handler = func(intentProcess) intentProcessResult {
		closed := b.job("critrisk")
		closed["closure"] = map[string]any{"criticRoot": "critrisk"}
		b.writeJob(closed)
		return intentProcessResult{}
	}
	code, result := b.do("work", "finish", "j2:critrisk", "--dispositions", decisions)
	if len(b.calls) != 1 || result.Outcome != intentConfirmed {
		t.Fatalf("a closed critic without its earned closure did not run the close again: code=%d %+v calls=%v", code, result, b.calls)
	}
	code, result = b.do("work", "finish", "j2:critrisk", "--dispositions", decisions)
	if len(b.calls) != 1 || result.Outcome != intentUnchanged {
		t.Fatalf("a closed critic with its closure ran the close again: code=%d %+v calls=%v", code, result, b.calls)
	}
	code, result = b.do("work", "finish", "j2:critclean", "--dispositions", decisions)
	if len(b.calls) != 1 || result.Outcome != intentUnchanged {
		t.Fatalf("a closed critic whose register earns no closure ran the close again: code=%d %+v calls=%v", code, result, b.calls)
	}
}
