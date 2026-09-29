package main

import "testing"

// Idempotency rows of the disk object (engine-owns-disk-lifetimes U6a, R9).
func init() {
	registerIdempotency("disk show", idemRead, "reads the last pass reports and the evidence roots; changes nothing", nil)
	registerIdempotency("disk clean", idemStateful,
		"a pass with nothing left to release, no registration of a removed checkout left to forget and every cache within its cap is success and changes no store, record, marker or cache entry (only the reports' timestamps move); --strays, --release and --discard repeat as success and write nothing",
		func(t *testing.T) {
			witnessDiskSweepRepeat(t, newDiskBed(t))
			witnessDiskTrimRepeat(t)
		})
}
