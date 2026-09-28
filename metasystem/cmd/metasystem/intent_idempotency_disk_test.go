package main

import "testing"

// Idempotency rows of the disk object (engine-owns-disk-lifetimes U6a, R9).
func init() {
	registerIdempotency("disk show", idemRead, "reads the last pass reports and the evidence roots; changes nothing", nil)
	registerIdempotency("disk clean", idemStateful,
		"a pass with nothing left to release is success and changes no store, record or marker; --strays, --release and --discard repeat as success and write nothing",
		func(t *testing.T) { witnessDiskCleanRepeat(t, newDiskBed(t)) })
}
