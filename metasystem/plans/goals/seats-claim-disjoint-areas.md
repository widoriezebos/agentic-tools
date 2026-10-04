# seats-claim-disjoint-areas

- State: queued
- Risk: severity=2 novelty=2 exposure=2 accumulation=1 basis="A claim-time rule that only narrows which goal a seat takes (severity 2: a wrong rule idles a seat, never corrupts work); the inventory it reads exists (novelty 2); every seat's claim path (exposure 2); nothing accumulates (1)."
- Tier: 2
- Intent: Two seats never build in the same package at the same time: a seat's claim passes over a ready goal whose accepted design touches a package that a claimed goal's design touches, and the lane's conflict returns stop being the cost of parallel seats.
- Origin: main
- Next step: Tier-2 design, at most 1,000 words: the touched-package set of a goal comes from its accepted design's moved-effects inventory (or, absent one, from the units table's named files); goal claim (and the steward's seat start) skips a ready goal whose set intersects a claimed goal's set and says why; a person may override with goal claim G --anyway --reason; measured need: six lane conflict returns in the night of 2026-10-03/04, about an hour of seat time each, in metasystem/internal/ui/web bundle, cmd/metasystem/intent_delivery.go and the typed roots. Generic for any repository: package means the directory of a changed file.
- OpenedAt: 2026-10-04T07:40:18Z
- Revision: 1
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0

History:
- 2026-10-04T07:40:18Z AG3GBPNYP7YQK4MFCDSTWMM94D-m1e-718ba0eb open actor=human:Wido targets=seats-claim-disjoint-areas
Integrity: sha256=37a1f502caea468a9d4c042ad5d4ef0ebd45a14cf35926b375b67651dd49655c
