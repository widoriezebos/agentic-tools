# testenv-custodian-ends-the-child-under-load

- State: done
- Risk: severity=2 novelty=1 exposure=2 accumulation=1 basis="A fixture-custody race that hangs every gate and lane proof on a loaded host; the fix touches testenv and the custodian only."
- Tier: 2
- Intent: internal/testenv's TestSetupChildIsEndedWhenTheBinaryIsKilled waits for the custodian to end the killed binary's setup child; under host load 14-25 on 2026-10-13 18:08 the child stayed alive for 36 minutes and the gate hung until its 40-minute deadline (goroutine dump: evidence lane-test-20261009/panes/gate-los5b-testenv-hang-dump.log). The custodian must end the child under load, or the test must observe why it did not, within a bound that does not depend on the wall clock (R-104).
- Origin: human
- Next step: Reproduce under load (run the package with -count=3 while a full cmd gate runs), read the custodian's log for the dead binary's registry, find why the child was not ended, fix the custodian or the fixture, mutation-check, hand in.
- Concluded: U1 landed by hand at 3bb2966bf (09:32 10-15): the setup fixture's custody waits name the pending condition and its evidence, write one still-waiting record after two minutes, and end under a ten-minute ceiling when the test binary has no deadline (the lane's proof form). The historical hang did not reproduce in 86 repetitions; the goal's whole delivery is the diagnostic (Astra's critique of the 24-hour plan, F1); a recurrence is an obligation on the flake register, not a build unit. Read 1 CORRECT (message), correction 1 (Astra 3 rounds), read 2 LAND, 0 material.
- OpenedAt: 2026-10-13T16:45:43Z
- Revision: 3
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-10-13T16:45:51Z revision=2 opid=J8TPAW59SE49HS1TP1X5X2VXP7-m1e-c6925449 authority=proven digest=dccf9a045509467c70af8c693aed9d22c4e5ccb08f7ffac3d6d4f5500585b5e7 episode=2

History:
- 2026-10-13T16:45:43Z Z4YCHEPYMAGPTA0QCD6SHNYNSJ-m1e-c6925449 open actor=human:Wido targets=testenv-custodian-ends-the-child-under-load
- 2026-10-13T16:45:51Z J8TPAW59SE49HS1TP1X5X2VXP7-m1e-c6925449 approve actor=human:Wido targets=testenv-custodian-ends-the-child-under-load
- 2026-10-15T07:34:21Z 9EKKJ35PMS9JGP5CFWFJAEE5K0-m1e-670f65d0 done actor=human:Wido targets=testenv-custodian-ends-the-child-under-load
Integrity: sha256=62492cf85c4d02fa96790513c04b90c09e53082daed6aad0b0beea2f3159f780
