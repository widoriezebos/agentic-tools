# tests-observe-only-what-they-inject

- State: queued
- Risk: severity=3 novelty=2 exposure=3 accumulation=3 basis="Lane test findings 368, 373, 375, 377, 382 (twice), 384 (2026-10-17): six load or leftover reds in one night; two of two hand gates failed on the legacy-worker test killing testenv's own witness child found in the host process table; every gate and proof is a coin toss until tests observe only injected facts. Step-back analysis of 08:05, Astra-critiqued."
- Tier: 3
- Intent: Tests observe only what they inject: a test or fixture never reads the host's process table, stores or leftovers, so host load and other packages' survivors cannot turn a green suite red; the known offenders are repaired first, then an audit with a shrink-only baseline holds the line.
- Origin: human
- Next step: U1 (D1): the legacy-worker launch test and two siblings take the injected process table (0 production lines); brief FINAL; Codex build in -inject, Opus read, gate, hand landing. U2 (D2): the supervise arming helpers register with the fixture custodian. U3 (D3): the audit with its baseline.
- OpenedAt: 2026-10-17T08:10:36Z
- Revision: 1
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0

History:
- 2026-10-17T08:10:36Z HVE3V00E5T0CE8GAC8JQGHC6TR-m1e-c5a1e456 open actor=human:Wido targets=tests-observe-only-what-they-inject
Integrity: sha256=d3bf5fcc934e1566795ed8e0860ffe5f9b3381cd97a2429beb2ddaab9247bfe9
