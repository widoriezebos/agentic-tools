# testenv-custodian-ends-the-child-under-load

- State: queued
- Risk: severity=2 novelty=1 exposure=2 accumulation=1 basis="A fixture-custody race that hangs every gate and lane proof on a loaded host; the fix touches testenv and the custodian only."
- Tier: 2
- Intent: internal/testenv's TestSetupChildIsEndedWhenTheBinaryIsKilled waits for the custodian to end the killed binary's setup child; under host load 14-25 on 2026-10-13 18:08 the child stayed alive for 36 minutes and the gate hung until its 40-minute deadline (goroutine dump: evidence lane-test-20261009/panes/gate-los5b-testenv-hang-dump.log). The custodian must end the child under load, or the test must observe why it did not, within a bound that does not depend on the wall clock (R-104).
- Origin: human
- Next step: Reproduce under load (run the package with -count=3 while a full cmd gate runs), read the custodian's log for the dead binary's registry, find why the child was not ended, fix the custodian or the fixture, mutation-check, hand in.
- OpenedAt: 2026-10-13T16:45:43Z
- Revision: 1
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0

History:
- 2026-10-13T16:45:43Z Z4YCHEPYMAGPTA0QCD6SHNYNSJ-m1e-c6925449 open actor=human:Wido targets=testenv-custodian-ends-the-child-under-load
Integrity: sha256=8dea2d07c384a1806e811eaa19d4b862e78b1a2f5be5a9e7a117352a9324ce2e
