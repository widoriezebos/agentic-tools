package census

import (
	"testing"
	"time"
)

func TestParseProcessTable(t *testing.T) {
	// Real ps -axo output shape, with padded and unpadded day fields.
	output := "" +
		"  501     1   501 Mon Aug 10 09:12:03 2026 /usr/local/bin/claude serve\n" +
		"  777   501   777 Sun Aug  3 23:59:59 2026 metasystem-fake-agent job\n" +
		"  888     1   888 not a valid lstart line at all\n"
	utc := time.UTC
	procs := parseProcessTable(output, utc)
	if len(procs) != 2 {
		t.Fatalf("want 2 parsed rows, got %d: %+v", len(procs), procs)
	}
	if procs[0].Pid != 501 || procs[0].Argv != "/usr/local/bin/claude serve" {
		t.Fatalf("row 0 wrong: %+v", procs[0])
	}
	// Aug 10 2026 09:12:03 UTC = a real epoch (not -1).
	if procs[0].Started <= 0 {
		t.Fatalf("row 0 start time not parsed: %d", procs[0].Started)
	}
	// The padded "Aug  3" row parses too.
	if procs[1].Pid != 777 || procs[1].Started <= 0 {
		t.Fatalf("padded-day row failed: %+v", procs[1])
	}
}

func TestParseProcessTableMalformedLstart(t *testing.T) {
	// A row matching the SHAPE but with an impossible date is KEPT with
	// started=-1 (faithful to python enumerate_ps: an agent-shaped row with a
	// bad time must be failed by the classifier, not silently dropped).
	output := "  99     1    99 Xxx Yyy 99 99:99:99 9999 weird\n"
	procs := parseProcessTable(output, time.UTC)
	if len(procs) != 1 || procs[0].Pid != 99 || procs[0].Started != -1 {
		t.Fatalf("a shape-matching bad-date row must be kept with started=-1: %+v", procs)
	}
}

func TestJoinPids(t *testing.T) {
	if got := joinPids([]int64{1, 22, 333}); got != "1,22,333" {
		t.Fatalf("joinPids = %q", got)
	}
}

func TestNormalizeLstart(t *testing.T) {
	if got := normalizeLstart("Mon Aug  3 23:59:59 2026"); got != "Mon Aug 3 23:59:59 2026" {
		t.Fatalf("normalizeLstart = %q", got)
	}
}

// RunProductionCensus runs against the live machine: assert it yields a valid
// schemaVersion-2 verdict (classification of real processes is
// non-deterministic, so only the envelope is asserted here; the fixture-path
// conformance proves the classification).
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
