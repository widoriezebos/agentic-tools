package main

import "testing"

// Idempotency rows of the evidence object (engine-owns-disk-lifetimes U6c,
// R-129).
func init() {
	registerIdempotency("evidence show", idemRead, "reads the segment, the tombstones and the accepted ledger as it stands; changes nothing and fetches nothing, and reports an open disposal for evidence dispose to settle", nil)
	registerIdempotency("evidence export", idemStateful,
		"an item whose content already has a verified archive under DIR is reported already exported, verified, and nothing is written; changed content is a new archive",
		func(t *testing.T) { witnessEvidenceExportRepeat(t, newEvidenceVerbBed(t)) })
	registerIdempotency("evidence dispose", idemStateful,
		"a preview writes only its plan; executing a plan whose items are already removed succeeds, says so, and writes no second receipt",
		func(t *testing.T) { witnessEvidenceDisposeRepeat(t, newEvidenceVerbBed(t)) })
}
