package census

import (
	"testing"
	"time"
)

// RunProductionCensus runs against the live machine: assert it yields a valid
// schemaVersion-2 verdict (classification of real processes is
// non-deterministic, so only the envelope is asserted here; the fixture-path
// tests cover the classification).
func TestRunProductionCensusEnvelope(t *testing.T) {
	root := t.TempDir()
	// No supervision state, no runtimes -> errors, but a well-formed verdict.
	v, err := RunProductionCensus(root, root, "fp", 60, time.Unix(1786000000, 0))
	if err != nil {
		t.Fatal(err)
	}
	if v.SchemaVersion != 2 || v.Writer != "watch-background-jobs.sh" {
		t.Fatalf("bad verdict envelope: %+v", v)
	}
	if v.Counts == nil || v.Inventory == nil || v.Errors == nil {
		t.Fatal("verdict collections must be non-nil")
	}
}
