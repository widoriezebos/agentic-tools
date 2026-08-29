# timing-tests-synthetic-clock

- State: claimed
- Intent: Timing-logic tests run on an injected synthetic clock and finish in microseconds; only legs that spawn real processes keep the real clock — the race gate's 25min is mostly wall-clock waits on arithmetic (Wido 2026-08-27 evening: why does timing-sensitive testing take so long, should we use a synthetic clock)
- Origin: main
- Next step: SLICE 1 ATTEMPTED (2026-08-29, within budget): the compressed-scale wedge is NOT a single leaker — bisection converged then falsified (the isolated pair passes; the 60-test window flakes even without the victim). Narrowed truth: cumulative cross-test flakiness under scale 50 (order- and load-dependent), consistent with compressed graces abandoning process groups. Compression stays OPT-IN (landed 9e1c291; suite green at default). REMAINING SLICES resized: (2) 3h instrument leaked-group accounting (count abandoned pgids per test under compression; fix winddown to kill-through with a real-fact floor instead of abandoning); (3) 3h sub-second taint identity + recovery windows; (4) 3h t.Parallel decoupling. Claim released for queue rotation per the keep-going order.
- OpenedAt: 2026-08-27T17:12:26Z
- Revision: 6
- Labels: shared
- Budget: elapsedLimit=1d4h attemptLimit=16 reservedJobMinutesLimit=300 activeJobLimit=1
- Claimed: machine=m2 lineage=mac-coordinator at=2026-08-29T14:12:41Z revision=5
- StopCapability: generation=5 revision=5 machine=m2 claimEpoch=1 fenceEpoch=0

History:
- 2026-08-27T17:12:26Z GRZ4RPVHPK0D6H2SKE8P1X46EV-m2-bc1be9cb open actor=human:wido targets=timing-tests-synthetic-clock
- 2026-08-27T17:15:51Z 8TK863Y9F7XH960CTKX092C0AN-m2-bc1be9cb edit actor=m2+mac-coordinator targets=timing-tests-synthetic-clock
- 2026-08-29T14:12:00Z 6RK75MKGKCSA79BE53CY00SBD0-m2-bc1be9cb edit actor=m2+mac-coordinator targets=timing-tests-synthetic-clock
- 2026-08-29T14:12:27Z 20QV9034V6V3WG3Z4STZDRR5EV-m2-bc1be9cb set-budget actor=human:wido targets=timing-tests-synthetic-clock
- 2026-08-29T14:12:41Z 6RDN6FT5A113SMWB9VKR2PGV20-m2-bc1be9cb claim actor=m2+mac-coordinator targets=timing-tests-synthetic-clock
- 2026-08-29T14:40:54Z 20RDB340JW8DFKARY5S1KD1BKA-m2-bc1be9cb edit actor=m2+mac-coordinator targets=timing-tests-synthetic-clock
Integrity: sha256=9e0468daa066946fc975a9e3159836b92d209811a410b3751adaf36a6c589d15
