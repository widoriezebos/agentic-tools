Working Mode: implement
Orchestrator Identity: m3+mac-m3 (dispatch delegate under goal severity-tiered-rigor-p2)
Date: 2026-09-04

# Build slice 2a, the risk basis: round six, three sweep tests on the old row grammar

The seat's full run of the goal package on the round-three tree
(`go test -count=1 -cover ./internal/goal`, 26 minutes, coverage 80.1%)
fails exactly three tests that no round ran by name, all in
metasystem/internal/goal/approval_test.go, all for one reason: their
classification drafts are written in the row grammar the design
retired in revision 4.3 (`<goal-id> <tier> <text>`), so the sweep now
refuses them with SWEEP_MALFORMED_ROW:

- TestSTR3MigrationBootstrap01ApprovedAndClaimedLegacyGoals (line 8):
  draft `legacy-claimed 2 claimed migration` and the expected listing
  line `legacy-approved 1 approved migration`;
- TestClassifySweepInstallsTierLawForAnAlreadyTieredLedger (line 90):
  draft `tiered-two 3 stale row` and an expected empty listing;
- TestClassifySweepRecoverySkipsRowsAlreadyApplied (line 129): same shape.

Bring the three tests to the revision-4.3 grammar and expectations
(metasystem/plans/severity-tiered-rigor-p2-build-2a-gap-brief.md, Answer
1): a row is `<goal-id> <s>,<n>,<e>,<a> <basis>`; the listing line the
tool renders is `<goal-id> <s>,<n>,<e>,<a> tier=<derived> <basis>` for a
goal without a tier or whose derivation is at or above its tier, and
`<goal-id> <s>,<n>,<e>,<a> tier=<current> HUMAN-DECISION derived=<d>
<basis>` for a tiered goal whose derivation is below its current tier.
Choose scores whose derivation reproduces the tier each test meant (the
design's table, STR4-TIER-DERIVATION-16, in
metasystem/plans/severity-tiered-rigor-p2-design.md). Where a test
asserted that an already-tiered goal is left out of the sweep, that
expectation is now the opposite by 006 as amended: a tiered goal without
a Risk record is selected and listed in the HUMAN-DECISION form, and the
confirm writes its Risk record and keeps the tier; assert that written
state instead. A production change is not expected; if a test cannot
be satisfied without one, that is the fixture forcing a fix (record it
under `decisions`), not a gap.

# Gate and constraints

gofmt, go vet, go build, staticcheck silent (go-gate.sh --fast); the
three tests by name plus TestSTR4R1SweepBackfill and the classification
sweep tests of approval_test.go by name (`-run 'ClassifySweep|STR3Migration|STR4R1Sweep'`).
Stage nothing, no commit wrapper, nothing under plans, records or
memory. diffBoundary: the tree you inherit plus approval_test.go.

Wall-clock budget: 30 minutes; return by minute 25 whatever the state.
Version-2 implementer JSON. Where an example here contradicts the tree's
existing law, the law wins, the choice is recorded under `decisions`,
and the test is built.

# Gap Rule

stop and report a gap; never fill it silently. The grain is the build
brief's: only a law-changing contract stops you; a mechanical choice is
chosen as the tree does it nearest the seam, recorded under `decisions`,
and built.
