# flaky-leftovers

- State: claimed
- Priority: 1
- Sequence: 26
- Risk: severity=2 novelty=2 exposure=1 accumulation=1 basis="Production lock hand-off in diskstore and fixture custody: a wrong release can drop a writer's scratch or leave a child unowned (severity 2); a new lock hand-off mechanism (novelty 2); engine-internal, no adopter surface (exposure 1); a one-time cleanup (accumulation 1)"
- Tier: 2
- Intent: Retire the three flaky-test leftovers: WriterDrain's wall-clock re-probe of fork-inherited writer locks, the eight fixture-custody exit bounds, and the single-pid seams still swapped in sequential tests
- Origin: main
- Next step: Design page, Astra critique under the materiality stop rule, then build and code critique
- OpenedAt: 2026-10-02T10:06:58Z
- Revision: 5
- Pinned: m1e
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=5
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-10-02T10:07:05Z revision=2 opid=YC9HTGKNZT616F94APVV8Y3EF1-m1e-9c612d71 authority=proven digest=cd6ade91f3b51a741d452ed529f3acd35321ec3eb712276d03182560d747d379 episode=2
- Claimed: machine=m1e lineage=main-1790454088-93948-21671b at=2026-10-02T10:16:27Z revision=5 accountingRevision=5 episodeAt=2026-10-02T10:16:27Z episodeRevision=5
- StopCapability: generation=5 revision=5 machine=m1e claimEpoch=9 fenceEpoch=0

History:
- 2026-10-02T10:06:58Z 540D7DS4S8P8ASGCXWMA4YF0H5-m1e-9c612d71 open actor=human:Wido targets=flaky-leftovers
- 2026-10-02T10:07:05Z YC9HTGKNZT616F94APVV8Y3EF1-m1e-9c612d71 approve actor=human:Wido targets=flaky-leftovers
- 2026-10-02T10:07:17Z 579SWA1TCRBSKPCFAG9YKCXH72-m1e-9c612d71 set-priority actor=human:Wido targets=flaky-leftovers reason=priority-order subject=flaky-leftovers from=unranked to=1:26 requested-sequence=append
- 2026-10-02T10:07:22Z JKP2211RSXVGR48HBWQADSZ8SJ-m1e-9c612d71 set-pin actor=human:Wido targets=flaky-leftovers
- 2026-10-02T10:16:27Z D2VVM51K6YGVSFX2CXP5K6KMG6-m1e-f456f182 claim actor=m1e+main-1790454088-93948-21671b targets=flaky-leftovers
Integrity: sha256=db748705fb9522466dd2c8336becd2d290b91f9022f7d4cc46f4adfc2172429f
