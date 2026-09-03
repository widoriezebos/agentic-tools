# Dispositions, the closing review: token spend fence design revision 2

Closing review: job fence-review-1-r2 (gpt-5.6-sol; the launcher marked
it advisory because context isolation could not be proven in its
sandbox; its findings were verified by reading the design and the
cited code). Under R-60-m1 the ladder has no further design round: the
agreed parts build and each disputed point becomes the named test
obligation the reviewer supplied. One table per file for the join.

## Closing round — 2 material findings, verdictMaterialCount=2, both bound as test obligations

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| TSF-R2-crossing-observation-carriage | accepted | Verified: revision 2 names a typed crossing slice only as the parameter of the proposed UpdateSpendEpisodes; HealthVerdict and RoleVerdict (internal/steward/health.go) carry no crossings and no measurement-valid flag, so an implementer would have to re-measure or parse the health line, and an unknown measurement read as an empty slice would clear live episodes without proof. Changes what gets built (health.go, tick.go). | Bound as TEST OBLIGATION, not a fold: the build carries one typed spend observation (crossings plus a measurement-valid flag) from checkSpendFence to UpdateSpendEpisodes through the tick, and an invalid measurement clears nothing; test TestTickCarriesSpendObservationAndUnknownDoesNotClearEpisodes. |
| TSF-R2-multiple-rearm-state | accepted | Verified: the proposed episode gains only an Owner field and a one-way digest, so a stored spend episode has no structured (scope, ceiling, multiple) identity, and the clearance rule (no crossing of the scope-and-ceiling pair at any multiple) keeps a 2x episode open while the ceiling is still crossed at 1x, so a return to 2x never re-alerts. Contradicts the design's own raised-ceiling statement. Changes what gets built (alert_episode.go). | Bound as TEST OBLIGATION: a spend-owned episode persists its structured identity (scope-id, ceiling, multiple) and is cleared per multiple when that multiple is no longer crossed, whatever lower multiples do; test TestSpendFenceHigherMultipleRearmsWhileLowerMultipleRemainsCrossed. |

The design builds as revision 2 plus these two obligations; the Fable
code review checks both tests exist and discriminate.

## Gap answers during the build (job fence-build, rounds 1-2)

| Gap | Answer |
| --- | --- |
| Round 1: the build brief's section-6 line carried revision 1's "dead on a crossing" while revision 2 keeps the role alive. | The brief was corrected (landed 2333e5ab); revision 2 section 6 binds. |
| Round 2: `AggregateUsage` persists a per-round derived `source` (fence.go:715) that revision 2's `JobMeasurement` (Record, Tokens, Cost, ProviderUnit, Provenance, Detail) cannot carry, so the mission aggregate's bytes could not be preserved. | AUTHORIZED, the recommended correction: `JobMeasurement` gains `Source` (the derived round's source value, empty otherwise) and `AggregateUsage` copies it unchanged into its output; its observable bytes and `TestAggregateUsageSumsTerminalJobs` stay as today. No second derivation path, no other change to the aggregate. An additive field on a new type is a gap answer, not a design change. |
