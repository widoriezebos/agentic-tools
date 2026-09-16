# fixture-children-cannot-outlive-their-test

- State: approved
- Priority: 1
- Sequence: 70
- Risk: severity=3 novelty=1 exposure=3 accumulation=3 basis="severity 3: sixteen orphans took the machine to a load average of 40 and starved every seat for hours; novelty 1: this is the known fixture-leak class in a new place, with the code identified; exposure 3: the fixture runs in internal/goal, which every deep proof selects; accumulation 3: each aborted run adds burners that ignore TERM and never exit"
- Tier: 2
- Intent: On 2026-09-16 sixteen orphaned shells, each running sh -c with trap "" TERM and a bare while :; do :; done, survived the test that spawned them by three and a half hours and drove the machine to a load average of 40, starving every seat. Source: the installed-hanging-git fixture in internal/goal/attention_test.go:156, whose wrapper spawns a TERM-ignoring busy loop to simulate an unresponsive git. It escapes three ways: cleanup lives in the test own defer, so a killed test binary (session limit, cancelled job, timeout, suite abort) never reaps it; the kill targets Kill(-groupID) using the wrapper own pid as a group id although nothing sets Setpgid, so the negative-pid kill can hit no group and its error is discarded; and the child own pid is recorded in the child marker but used for assertions rather than as the kill target. DONE: (1) no fixture simulates an unresponsive process with a bare busy loop, it hangs on a sleeping wait so a leak costs no CPU; (2) every fixture child is reaped by its own recorded identity with SIGKILL, and any process-group kill sets Setpgid so the group truly exists; (3) reaping survives an aborted run, proven by a test that kills the test binary mid-fixture and asserts no child survives; (4) a census names any fixture child that outlives its run, so the next one is seen rather than found by a human noticing a hot machine.
- Origin: human
- Next step: Diagnosis is done and the cause is identified in internal/goal/attention_test.go:140-180 (the wrapper source, the group marker, the deferred group kill) and waitForGroupAbsence at :662. Design is small: decide the reaping identity (recorded child pid plus Setpgid group), where teardown lives so it survives an abort (TestMain or the harness tmp sweep), and the census rule. Then build in one or two units of at most 300 lines, each with a witness that kills the test binary mid-fixture and asserts no survivor. Opened by Wido at the terminal on 2026-09-16 after m1c found and killed sixteen of these orphans by hand; he asked for the highest priority and for it to be taken as soon as a slot opens.
- OpenedAt: 2026-09-16T06:19:56Z
- Revision: 3
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-16T06:20:12Z revision=2 opid=ZCWSQMQBA1KPT17HD66XK8X4TN-m1c-c6925449 authority=proven digest=0807f412291385a225138361bf9908294a165b2a4c8ab3fdf6b806d593ced5b7

History:
- 2026-09-16T06:19:56Z KZ4JS51YKMW6TAMMQSB3EKBFX7-m1c-c6925449 open actor=human:Wido targets=fixture-children-cannot-outlive-their-test reason=TierOverride: derived=3 set=2 why=the four risk answers derive tier 3, but the defect is one identified fixture with a known fix shape and no design unknowns: a bare busy loop replaced by a sleeping wait, reaping by recorded pid, and teardown that survives an abort. Tier 2 fits the work, not the blast radius.
- 2026-09-16T06:20:12Z ZCWSQMQBA1KPT17HD66XK8X4TN-m1c-c6925449 approve actor=human:Wido targets=fixture-children-cannot-outlive-their-test
- 2026-09-16T06:20:16Z YEKCVT7R6K6HVVY4PN52DT6QA4-m1c-c6925449 set-priority actor=human:Wido targets=fixture-children-cannot-outlive-their-test reason=priority-order subject=fixture-children-cannot-outlive-their-test from=unranked to=1:70 requested-sequence=append
Integrity: sha256=beb587948933e5b4456cb3946e61ed1bfb5ccfa61ea05cb2a1719d14f9753835
