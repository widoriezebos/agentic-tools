# job-record-birth-token

- State: claimed
- Intent: Every job record incarnation carries a mandatory, immutable, machine-minted birth token: the create path mints it under the record lock (timestamp plus nonce generation - a second-precision timestamp alone collides on same-second identifier reuse), ignores any caller-supplied value, and the field joins immutableFields. Proven necessary by executable spike (records/misc/alert-channel-spike-verdicts.md, 2026-09-01): no shipped field qualifies - createdAt is neither mandatory nor immutable through the real writers, startedAt and claimEpoch are optional and caller-supplied, inode identity changes on every atomic rewrite - and the alert design's retention pin (its critical identifier-reuse closure) depends on exactly this contract. Consumer: alert-escalation-channel revision 11; Ruling R applies - whoever builds this runs every reader of the record identity
- Origin: main
- Next step: Small mechanical item, 4h box (R-44-m0b): design note from the spike's implied rule, Sol critique, build in internal/dispatch record writers, every incarnation-comparison caller run
- OpenedAt: 2026-09-01T21:26:07Z
- Revision: 9
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=240 activeJobLimit=1
- Approved: by=human:Wido at=2026-09-03T15:45:11Z revision=8 opid=85B9MXYAAKYPMG6YR1AQWW8X0V-m0-c5dbf036 authority=relayed digest=19f4b7d2be58a073ad58d696f7996bd2535d26361dac33300b296180ca3beab0 reviewBy=2026-09-06
- Sliced: machine=m1 lineage=main-1788333680-2840-7f79f4 revision=3 at=2026-09-02T17:23:10Z
- Claimed: machine=m0 lineage=main-1788178136-1684505-4ffe42 at=2026-09-03T15:46:40Z revision=9
- StopCapability: generation=9 revision=9 machine=m0 claimEpoch=2 fenceEpoch=0

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
Integrity: sha256=e9d2bc54a66bf7db3a0fd8064b7f85768fd8a673a78ad25e9d7bb950d7f518f5
