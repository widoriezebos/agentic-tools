# seats-claim-disjoint-areas

- State: approved
- Risk: severity=2 novelty=2 exposure=2 accumulation=1 basis="A claim-time rule that only narrows which goal a seat takes (severity 2: a wrong rule idles a seat, never corrupts work); the inventory it reads exists (novelty 2); every seat's claim path (exposure 2); nothing accumulates (1)."
- Tier: 2
- Intent: Two seats never build in the same package at the same time: a seat's claim passes over a ready goal whose accepted design touches a package that a claimed goal's design touches, and the lane's conflict returns stop being the cost of parallel seats.
- Origin: main
- Next step: Tier-2 design, at most 1,000 words: the touched-package set of a goal comes from its accepted design's moved-effects inventory (or, absent one, from the units table's named files); goal claim (and the steward's seat start) skips a ready goal whose set intersects a claimed goal's set and says why; a person may override with goal claim G --anyway --reason; measured need: six lane conflict returns in the night of 2026-10-03/04, about an hour of seat time each, in metasystem/internal/ui/web bundle, cmd/metasystem/intent_delivery.go and the typed roots. Generic for any repository: package means the directory of a changed file.
- OpenedAt: 2026-10-04T07:40:18Z
- Revision: 2
- Budget: elapsedLimit=1d attemptLimit=8 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-10-04T07:45:43Z revision=2 opid=JATG4ZRQ1T30PA1EEWYTDJJ3SH-m1e-718ba0eb authority=proven digest=d050cb8544d6b347f44ae5970a7226df93c6ff33e84ff3d925a1ff69291f49d6 episode=2

History:
- 2026-10-04T07:40:18Z AG3GBPNYP7YQK4MFCDSTWMM94D-m1e-718ba0eb open actor=human:Wido targets=seats-claim-disjoint-areas
- 2026-10-04T07:45:43Z JATG4ZRQ1T30PA1EEWYTDJJ3SH-m1e-718ba0eb approve actor=human:Wido targets=seats-claim-disjoint-areas
Integrity: sha256=3ae99d1d2b38c717f907be514e9adaf60d7fa994fd26fcc8d5a72249c4268622
