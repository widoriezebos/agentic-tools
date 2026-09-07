# steward-revives-a-done-goal

- State: queued
- Risk: severity=3 novelty=2 exposure=2 accumulation=2 basis="severity 3: it spent real delegate money continuing a goal that was already done, and the job outlived the runner that spawned it by reparenting, so killing the steward did not stop the spend; novelty 2: the revival intent needs a liveness-and-state test it does not have; exposure 2: every seat whose steward ticks while a goal completes; accumulation 2: each occurrence burns a full job cap and the pattern repeats whenever a landing takes longer than a tick"
- Tier: 3
- Intent: The steward's revival judged a goal stalled and revived it while its holder was mid-landing, then kept running after the goal was done. Evidence from m1d 2026-09-07: the steward logged 'steward revival: intent b2271fe9e3c7f9b9 revives fixture-review-by-date-expired via job steward-b2271fe9e3c7f9b9', and that job was not merely an intent - it had a record with status running and a live codex exec on gpt-5.6-sol started 11:02:20, still writing events nine minutes later on a goal that had already been marked done, and it survived the kill of its own steward because it had been reparented. m1d cancelled it. DONE means a revival refuses when the goal is done, when its holder is alive, or when the goal's state changed after the intent was formed, and the revived job dies with the steward that spawned it rather than being reparented; fixtures prove all three refusals and the custody
- Origin: main
- Next step: read the revival intent path in internal/steward and the custody the spawn uses; the three refusals are cheap, the custody is the real work (a reparented child is exactly the leak the process-custody rules exist to prevent). Adjacent and already recorded elsewhere: the fence in goal metasystem-stop-verb refuses a revival's job launch on a stopped checkout, which bounds this on stopped checkouts but not on live ones, which is where the money went
- OpenedAt: 2026-09-07T09:18:48Z
- Revision: 1
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0

History:
- 2026-09-07T09:18:48Z WZJX6A2QTBA57ZRAPPEVQ25X3R-m1b-c6925449 open actor=m1b+main-1788680071-18713-e76d5d targets=steward-revives-a-done-goal
Integrity: sha256=6b5e56864f49baebe3e4f1cd06cb25f75bc78cae5a9b4c7081f09ae5030a0401
