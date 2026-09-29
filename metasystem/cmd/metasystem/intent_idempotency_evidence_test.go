package main

import "testing"

// Idempotency rows of the evidence object (engine-owns-disk-lifetimes U6c,
// R-129).
func init() {
	registerIdempotency("evidence show", idemStateful,
		"reads the segment, the tombstones and the accepted ledger as it stands and fetches nothing; its one write settles a removal cut short (rolled back, or its set-aside copy removed once committed), so a repeat changes nothing",
		func(t *testing.T) { witnessEvidenceShowSettlesOnce(t, newEvidenceVerbBed(t)) })
	registerIdempotency("evidence export", idemStateful,
		"an item whose content already has a verified archive under DIR is reported already exported, verified, and nothing is written; changed content is a new archive",
		func(t *testing.T) { witnessEvidenceExportRepeat(t, newEvidenceVerbBed(t)) })
	registerIdempotency("evidence dispose", idemStateful,
		"a preview writes only its plan; executing a plan whose items are already removed succeeds, says so, and writes no second receipt",
		func(t *testing.T) { witnessEvidenceDisposeRepeat(t, newEvidenceVerbBed(t)) })
}
