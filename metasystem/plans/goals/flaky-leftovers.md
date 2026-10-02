# flaky-leftovers

- State: queued
- Risk: severity=2 novelty=2 exposure=1 accumulation=1 basis="Production lock hand-off in diskstore and fixture custody: a wrong release can drop a writer's scratch or leave a child unowned (severity 2); a new lock hand-off mechanism (novelty 2); engine-internal, no adopter surface (exposure 1); a one-time cleanup (accumulation 1)"
- Tier: 2
- Intent: Retire the three flaky-test leftovers: WriterDrain's wall-clock re-probe of fork-inherited writer locks, the eight fixture-custody exit bounds, and the single-pid seams still swapped in sequential tests
- Origin: main
- Next step: Design page, Astra critique under the materiality stop rule, then build and code critique
- OpenedAt: 2026-10-02T10:06:58Z
- Revision: 1
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=5
- BudgetExceptions: 0

History:
- 2026-10-02T10:06:58Z 540D7DS4S8P8ASGCXWMA4YF0H5-m1e-9c612d71 open actor=human:Wido targets=flaky-leftovers
Integrity: sha256=57900a9a841e526761f5a99d3a2eca4e1ffdaf573672120332e8e81b0b68fda0
