package main

import "testing"

// A writer that names only its pid is recorded at its probed start time; a
// given start time, or a missing pid, is never probed.
func TestEventWriterStartedAtProbesOnlyAPidOnlyWriter(t *testing.T) {
	t.Parallel()
	probes := 0
	probe := func(pid int64) (int64, bool) { probes++; return 1700000000, pid == 42 }
	if got := eventWriterStartedAt("", false, 42, probe); got != 1700000000 {
		t.Fatalf("pid-only writer started at %d", got)
	}
	if got := eventWriterStartedAt("", false, 7, probe); got != 0 {
		t.Fatalf("unprobeable writer started at %d", got)
	}
	if got := eventWriterStartedAt("12", true, 42, probe); got != 12 || probes != 2 {
		t.Fatalf("given start %d probes %d", got, probes)
	}
	if got := eventWriterStartedAt("", false, 0, probe); got != 0 || probes != 2 {
		t.Fatalf("pidless writer %d probes %d", got, probes)
	}
}
