package main

import "testing"

// TestLaunchManagerCompressesLogsAboveTheSetting: the launch manager's one
// constructor reads disk.compress-above-mib, so a finished launch's log at
// or above it is gzipped (Part B 3.2 "Launch"); the compiled default is
// 1 MiB.
func TestLaunchManagerCompressesLogsAboveTheSetting(t *testing.T) {
	t.Parallel()
	if got := newLaunchManager().CompressAbove; got != 1<<20 {
		t.Fatalf("CompressAbove = %d, want the compiled 1 MiB", got)
	}
}
