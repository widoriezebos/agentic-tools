# flaky-leftovers

- State: approved
- Priority: 1
- Sequence: 26
- Risk: severity=2 novelty=2 exposure=1 accumulation=1 basis="Production lock hand-off in diskstore and fixture custody: a wrong release can drop a writer's scratch or leave a child unowned (severity 2); a new lock hand-off mechanism (novelty 2); engine-internal, no adopter surface (exposure 1); a one-time cleanup (accumulation 1)"
- Tier: 2
- Intent: Retire the three flaky-test leftovers: WriterDrain's wall-clock re-probe of fork-inherited writer locks, the eight fixture-custody exit bounds, and the single-pid seams still swapped in sequential tests
- Origin: main
- Next step: Design page, Astra critique under the materiality stop rule, then build and code critique
- OpenedAt: 2026-10-02T10:06:58Z
- Revision: 3
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=5
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-10-02T10:07:05Z revision=2 opid=YC9HTGKNZT616F94APVV8Y3EF1-m1e-9c612d71 authority=proven digest=cd6ade91f3b51a741d452ed529f3acd35321ec3eb712276d03182560d747d379 episode=2

History:
- 2026-10-02T10:06:58Z 540D7DS4S8P8ASGCXWMA4YF0H5-m1e-9c612d71 open actor=human:Wido targets=flaky-leftovers
- 2026-10-02T10:07:05Z YC9HTGKNZT616F94APVV8Y3EF1-m1e-9c612d71 approve actor=human:Wido targets=flaky-leftovers
- 2026-10-02T10:07:17Z 579SWA1TCRBSKPCFAG9YKCXH72-m1e-9c612d71 set-priority actor=human:Wido targets=flaky-leftovers reason=priority-order subject=flaky-leftovers from=unranked to=1:26 requested-sequence=append
Integrity: sha256=2a51be5a9767ac837688ff563591539891eb88b81490a9cb52052f9443b42b71
