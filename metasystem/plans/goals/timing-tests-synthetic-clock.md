# timing-tests-synthetic-clock

- State: claimed
- Intent: Timing-logic tests run on an injected synthetic clock and finish in microseconds; only legs that spawn real processes keep the real clock — the race gate's 25min is mostly wall-clock waits on arithmetic (Wido 2026-08-27 evening: why does timing-sensitive testing take so long, should we use a synthetic clock)
- Origin: main
- Next step: SLICE 3 LANDED d033841 (m2, 2026-08-30): resolved by measurement - with slice 2's kill-through in, NINE of the ten compression pins no longer reproduce and are gone for good (8 wallrecovery + the same-second taint pin; sub-second taint identity proved unnecessary - the collapse was the leaked-group wedge in a timestamp costume). Full package green at scale 50 pin-free and at default. Janitor gains mission-run-loop + four host start-turn shapes so runner/host groups prove ownership by their own argv at wind-down. ONE wedge remains, pinned with a sharp reason: TestNestedCheckoutMissionBirth at scale 50 - the nested child runner never writes its verification signal inside the floored window; the compounding is in the child's own start path. REMAINING: (4a) diagnose+fix the nested-start compounding (~2h); (4b) t.Parallel decoupling if still wanted after 4a; then the arc's endgame - compression by default - is one decision away. Released for rotation.
- OpenedAt: 2026-08-27T17:12:26Z
- Revision: 15
- Labels: shared
- Budget: elapsedLimit=3h attemptLimit=5 reservedJobMinutesLimit=45 activeJobLimit=1
- Claimed: machine=m2 lineage=mac-coordinator at=2026-08-30T02:51:04Z revision=15
- StopCapability: generation=15 revision=15 machine=m2 claimEpoch=1 fenceEpoch=0

History:
- 2026-08-27T17:12:26Z GRZ4RPVHPK0D6H2SKE8P1X46EV-m2-bc1be9cb open actor=human:wido targets=timing-tests-synthetic-clock
- 2026-08-27T17:15:51Z 8TK863Y9F7XH960CTKX092C0AN-m2-bc1be9cb edit actor=m2+mac-coordinator targets=timing-tests-synthetic-clock
- 2026-08-29T14:12:00Z 6RK75MKGKCSA79BE53CY00SBD0-m2-bc1be9cb edit actor=m2+mac-coordinator targets=timing-tests-synthetic-clock
- 2026-08-29T14:12:27Z 20QV9034V6V3WG3Z4STZDRR5EV-m2-bc1be9cb set-budget actor=human:wido targets=timing-tests-synthetic-clock
- 2026-08-29T14:12:41Z 6RDN6FT5A113SMWB9VKR2PGV20-m2-bc1be9cb claim actor=m2+mac-coordinator targets=timing-tests-synthetic-clock
- 2026-08-29T14:40:54Z 20RDB340JW8DFKARY5S1KD1BKA-m2-bc1be9cb edit actor=m2+mac-coordinator targets=timing-tests-synthetic-clock
- 2026-08-29T14:41:08Z 05QC5NP22JDT5PZFTQVNY4002B-m2-bc1be9cb release actor=m2+mac-coordinator targets=timing-tests-synthetic-clock
- 2026-08-30T00:15:28Z FMMXTJ3Q89WPVBCN7PV9XZM4GK-m2-bc1be9cb set-budget actor=human:wido targets=timing-tests-synthetic-clock
- 2026-08-30T00:15:43Z 7Q7BARE3503WDARD7AMKC2T4X2-m2-bc1be9cb claim actor=m2+mac-coordinator targets=timing-tests-synthetic-clock
- 2026-08-30T01:57:52Z W2V6VQY8950XBRK5JYQYTANRCY-m2-bc1be9cb edit actor=m2+mac-coordinator targets=timing-tests-synthetic-clock
- 2026-08-30T01:58:07Z J8AD9QKW9SAT01A4FEMFVYMD8Q-m2-bc1be9cb release actor=m2+mac-coordinator targets=timing-tests-synthetic-clock
- 2026-08-30T01:58:44Z GHMBHXQPE965BG5Q19WS6Q4ATC-m2-bc1be9cb claim actor=m2+mac-coordinator targets=timing-tests-synthetic-clock
- 2026-08-30T02:50:22Z YNDFSAMRND1655V4197VRGV08C-m2-bc1be9cb edit actor=m2+mac-coordinator targets=timing-tests-synthetic-clock
- 2026-08-30T02:50:37Z F952T9X55BF1ABCRXBNDVXCVZN-m2-bc1be9cb release actor=m2+mac-coordinator targets=timing-tests-synthetic-clock
- 2026-08-30T02:51:04Z DHEV5PSSYTKFEV01TBXHA9BTRK-m2-bc1be9cb claim actor=m2+mac-coordinator targets=timing-tests-synthetic-clock
Integrity: sha256=1cb2d412ebf61d910332bb14f1a7778e8657a58905a6563751dfd328c982b192
