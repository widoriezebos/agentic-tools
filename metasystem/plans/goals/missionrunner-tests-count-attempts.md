# missionrunner-tests-count-attempts

- State: queued
- Risk: severity=1 novelty=2 exposure=2 accumulation=2 basis="The mission-runner tests pass on a quiet Mac in 24 minutes but failed under load; converting their sleeps and deadlines to attempt-counted patience touches the unfinished process-custody design, which must not be pre-empted; every gate that runs them is exposed to load."
- Tier: 2
- Intent: internal/missionrunner's tests use explicit sleeps, deadlines, elapsed-time assertions and process waits; on m2 2026-09-04 the package failed in the adopt fixture's Go gate after 1443 seconds while three other suites ran, and passed on a quiet Mac in 1454 seconds. Converting them to the patience rule (attempts counted in observed progress, a silence-only failsafe) needs three decisions the implementer of chain agm-build1 rightly refused to make alone: the attempt marker and budget per asynchronous actor, whether to touch the process-group conversion whose recorded design says it cannot be implemented safely yet (goal proof-harness-process-custody), and which tests count as inherently long for a -short gate. DONE means those decisions are recorded in a short design note, the tests count attempts, the package passes under the load of two concurrent fixture suites, and -short skips only the tests the note names.
- Origin: main
- Next step: A design note first (three decisions), then the test conversion; tier from the risk basis; waits for Wido's word if above tier 1.
- OpenedAt: 2026-09-04T20:40:58Z
- Revision: 1
- Labels: robustness
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-04T20:40:58Z CK21RB56WY46CB2A7CHW67ASMY-m2-5fcf08ab open actor=human:Wido targets=missionrunner-tests-count-attempts
Integrity: sha256=81f60385285f71c7ee05e6ce64096842260c8c262a53030d693961d026004304
