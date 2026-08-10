package report

import (
	"strings"
	"testing"
)

func TestStopBlock(t *testing.T) {
	b := StopBlock("PLAN says do X")
	if b["decision"] != "block" {
		t.Fatalf("stop-block must be a block decision: %v", b)
	}
	reason, _ := b["reason"].(string)
	if !strings.Contains(reason, "unblocked and nothing is in flight") {
		t.Fatalf("reason missing the standing guidance: %q", reason)
	}
	if !strings.HasSuffix(reason, "PLAN says do X") {
		t.Fatalf("reason must append the caller detail: %q", reason)
	}
}

func TestStopBlockEmptyDetail(t *testing.T) {
	b := StopBlock("")
	if !strings.HasSuffix(b["reason"].(string), "\n\n") {
		t.Fatalf("with no detail the reason still ends with the separator: %q", b["reason"])
	}
}
