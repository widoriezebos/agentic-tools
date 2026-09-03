# job-record-birth-token

- State: approved
- Intent: Every job record incarnation carries a mandatory, immutable, machine-minted birth token: the create path mints it under the record lock (timestamp plus nonce generation - a second-precision timestamp alone collides on same-second identifier reuse), ignores any caller-supplied value, and the field joins immutableFields. Proven necessary by executable spike (records/misc/alert-channel-spike-verdicts.md, 2026-09-01): no shipped field qualifies - createdAt is neither mandatory nor immutable through the real writers, startedAt and claimEpoch are optional and caller-supplied, inode identity changes on every atomic rewrite - and the alert design's retention pin (its critical identifier-reuse closure) depends on exactly this contract. Consumer: alert-escalation-channel revision 11; Ruling R applies - whoever builds this runs every reader of the record identity
- Origin: main
- Next step: PAUSED BY WIDO 2026-09-03 15:52Z (m0 and m0b paused; nothing under way). Approved and was claimed by m0 as the first headless run's target; released so the 4h elapsed clock does not burn during the pause. RESUME: re-claim, then follow plans/first-headless-run-plan.md step 6 from 'shut down the seat owner' (contract sealed+signed on main, mission birth-token born and waiting; the post-session handover watcher was disarmed for the pause and must be re-armed).
- OpenedAt: 2026-09-01T21:26:07Z
- Revision: 11
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=240 activeJobLimit=1
- Approved: by=human:Wido at=2026-09-03T15:45:11Z revision=8 opid=85B9MXYAAKYPMG6YR1AQWW8X0V-m0-c5dbf036 authority=relayed digest=19f4b7d2be58a073ad58d696f7996bd2535d26361dac33300b296180ca3beab0 reviewBy=2026-09-06
- Sliced: machine=m1 lineage=main-1788333680-2840-7f79f4 revision=3 at=2026-09-02T17:23:10Z

History:
- 2026-09-01T21:26:07Z YGRS58CS27XPHHVC7FAVCK8B9R-m0b-6638932d open actor=m0b+main-1788250419-3170380-8a1fb3 targets=job-record-birth-token
- 2026-09-01T21:26:10Z 3ENA11H1YYQ6XEAJTD6C9KB5VG-m0b-6638932d set-budget actor=m0b+main-1788250419-3170380-8a1fb3 targets=job-record-birth-token
- 2026-09-02T17:20:05Z RWS1F0GP1EMFEWTXN2S7GNZ19D-m1-7bb1546e claim actor=m1+main-1788333680-2840-7f79f4 targets=job-record-birth-token
- 2026-09-02T17:23:10Z GQDP4CME4V36GAXS851NKJBC3N-m1-7bb1546e slice-start actor=m1+main-1788333680-2840-7f79f4 targets=job-record-birth-token
- 2026-09-02T17:38:28Z XH0TMFV0J0FH590V6BS3PXQEE8-m1-7bb1546e release actor=m1+main-1788333680-2840-7f79f4 targets=job-record-birth-token
- 2026-09-02T19:31:10Z KQ02QQX3R5Z76DMG2VXPHH2YHE-m1-7bb1546e claim actor=m1+main-1788333680-2840-7f79f4 targets=job-record-birth-token
- 2026-09-02T19:47:09Z GA8SENTAZDGXY570DXWKZ4DRWZ-m1-7bb1546e release actor=m1+main-1788333680-2840-7f79f4 targets=job-record-birth-token
- 2026-09-03T15:45:11Z 85B9MXYAAKYPMG6YR1AQWW8X0V-m0-c5dbf036 approve actor=human:Wido targets=job-record-birth-token authorityOutcome=TEMPORARY_HUMAN_WORD authorityReviewBy=2026-09-06 authorityRuling=R-32-m1 temporaryHumanWord="approval for both. Go (job-record-birth-token as the first headless run target; relayed from Wido through m0, 2026-09-03)"
- 2026-09-03T15:46:40Z DVYARMXDAAWP52GZ0XWHEXZ5KQ-m0-c5dbf036 claim actor=m0+main-1788178136-1684505-4ffe42 targets=job-record-birth-token
- 2026-09-03T15:59:16Z QRKT7DP158FV0SKGB0Y0QTXT5Y-m0-c5dbf036 edit actor=m0+main-1788178136-1684505-4ffe42 targets=job-record-birth-token
- 2026-09-03T15:59:20Z C08MS6EJY3570ZYVA0MJYG3A5V-m0-c5dbf036 release actor=m0+main-1788178136-1684505-4ffe42 targets=job-record-birth-token
Integrity: sha256=ba610a02bdfc1343afa6b0236ecfa1df45e9770e698021b064b9ba1962e83c56
