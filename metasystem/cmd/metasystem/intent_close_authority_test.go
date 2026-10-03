package main

import (
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/authority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
)

// humanRecordWriter is the record-writer authority owner's answer for a
// person's classification, the one the fixture close wrappers present.
func humanRecordWriter(_, job string) (string, error) {
	if err := authority.Authorize("record-writer", map[string]any{"class": "HUMAN"}, job); err != nil {
		return "record-writer-refused", err
	}
	return "", nil
}

// TestIntentCloseRecordWriterPreflight: the preflight asks the real
// record-writer owner. This process, classified for real once announced as
// the lease holder, is admitted; an agent session that does not hold the
// work is refused, and a public close then starts nothing and offers no
// repair. The refused caller is named rather than classified, so the test
// means the same from a person's terminal, which the owner admits.
func TestIntentCloseRecordWriterPreflight(t *testing.T) {
	t.Parallel()
	refused := newDeliveryBed(t)
	notHolding := func(root, job string) (string, error) {
		return recordWriterPreflightAs(root, job, func(string, int64) (lease.ClassifyResult, error) {
			return lease.ClassifyResult{Class: lease.ClassMain}, nil
		})
	}
	if cause, err := notHolding(refused.install, "crit1"); err == nil || cause != "record-writer-refused" {
		t.Fatalf("a caller without authority: %q %v", cause, err)
	}
	refused.owners.recordWriter = notHolding
	refused.writeJob(map[string]any{"jobId": "crit1", "role": "code-critic", "status": "completed", "round": 1, "findingRegister": []any{}})
	refused.writeReturn("crit1", 1, "crit1")
	decisions := refused.root() + "/decisions.md"
	refused.writeFile(decisions, deliveryDispositionsHeader)
	code, result := refused.do("work", "finish", "j2:crit1", "--dispositions", decisions)
	data, _ := result.Data.(map[string]any)
	if code == 0 || result.Outcome != intentRefused || len(refused.calls) != 0 || data["cause"] == nil || result.Next != nil {
		t.Fatalf("a refused preflight: %d %+v calls=%v", code, result, refused.calls)
	}

	admitted := newDeliveryBed(t)
	announceProofFixtureHolder(t, admitted.install)
	if cause, err := recordWriterPreflight(admitted.install, "crit1"); err != nil {
		t.Fatalf("the authenticated lease holder: %q %v", cause, err)
	}
}
