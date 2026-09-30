package main

import (
	"strings"
	"testing"
)

// machine start reads its launch owner's --json envelope (structured-output
// U2, T8): driven against this tree's real engine, the owner's refusal
// arrives as the envelope's summary, in plain words and without the
// owner's own prefix, never as its first stderr line.
func TestMachineStartReadsTheSeatLaunchEnvelope(t *testing.T) {
	t.Parallel()
	b := newProcessBed(t)
	owners := b.owners()
	engine := intentTestEngine(t)
	owners.delivery = &intentDeliveryOwners{executable: func() (string, error) { return engine, nil }}
	code, result := b.runJSON(owners, "machine", "start", "m9", "--temporary-human-word", "yes", "--review-by", "someday")
	if code == 0 || result.Outcome != intentRefused || result.Summary != "--review-by must be a real date in YYYY-MM-DD form" {
		t.Fatalf("machine start over a refusing owner = %d %+v", code, result)
	}
	if strings.HasPrefix(result.Summary, "seat launch:") {
		t.Fatalf("the summary is the owner's stderr line: %q", result.Summary)
	}
}
