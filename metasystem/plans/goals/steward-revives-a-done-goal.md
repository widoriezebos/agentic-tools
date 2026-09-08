# steward-revives-a-done-goal

- State: queued
- Priority: 1
- Sequence: 14
- Risk: severity=3 novelty=2 exposure=2 accumulation=2 basis="severity 3: it spent real delegate money continuing a goal that was already done, and the job outlived the runner that spawned it by reparenting, so killing the steward did not stop the spend; novelty 2: the revival intent needs a liveness-and-state test it does not have; exposure 2: every seat whose steward ticks while a goal completes; accumulation 2: each occurrence burns a full job cap and the pattern repeats whenever a landing takes longer than a tick"
- Tier: 3
- Intent: The steward's revival judged a goal stalled and revived it while its holder was mid-landing, then kept running after the goal was done. Evidence from m1d 2026-09-07: the steward logged 'steward revival: intent b2271fe9e3c7f9b9 revives fixture-review-by-date-expired via job steward-b2271fe9e3c7f9b9', and that job was not merely an intent - it had a record with status running and a live codex exec on gpt-5.6-sol started 11:02:20, still writing events nine minutes later on a goal that had already been marked done, and it survived the kill of its own steward because it had been reparented. m1d cancelled it. DONE means a revival refuses when the goal is done, when its holder is alive, or when the goal's state changed after the intent was formed, and the revived job dies with the steward that spawned it rather than being reparented; fixtures prove all three refusals and the custody
- Origin: main
- Next step: SECOND INCIDENT, on m1b's own claim, 2026-09-07 10:25Z, which widens this goal beyond the done-goal case: the steward revived goal metasystem-stop-verb WHILE ITS HOLDER WAS ACTIVELY WORKING IT. The seat was 20 minutes into an out-of-sandbox verification run (a package matrix and four fixture beds, no dispatch job in flight because the orchestrator runs those itself), and the steward read that as stalled. Receipt line landed with commit 6dbaefb0: 'steward revival: intent e0fd62ab4c2a2e32 revives metasystem-stop-verb via job steward-e0fd62ab4c2a2e32'. The job was real: role steward-continuation, runtime codex, pid 53323, cap 120 minutes, running 22 minutes before m1b noticed it in a stray receipts-log modification and cancelled it through delegate --cancel. Nothing surfaced it; it was found only because it dirtied the tree during a landing. So the live-holder refusal named in this goal's DONE is not a nicety: a seat doing the slow, honest part of its work (verification the sandbox cannot do) looks exactly like a stalled seat to the steward, and the duplicate worker it spawns can dispatch, edit or land against the same chain. Suggested test to add: a claimed goal whose holder has published no job for longer than the stall window but whose lease and session are alive is NOT revived. The detection half is goal live-job-on-a-done-goal-unnoticed.
- OpenedAt: 2026-09-07T09:18:48Z
- Revision: 3
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0

History:
- 2026-09-07T09:18:48Z WZJX6A2QTBA57ZRAPPEVQ25X3R-m1b-c6925449 open actor=m1b+main-1788680071-18713-e76d5d targets=steward-revives-a-done-goal
- 2026-09-07T10:48:32Z FNF1Z5ZE8678S42RNPYT4BWYA8-m1b-c6925449 edit actor=m1b+main-1788680071-18713-e76d5d targets=steward-revives-a-done-goal
- 2026-09-08T15:56:07Z ZDGH0E9FSBCF26D3D3X6A1DM6S-m1-7cd0bd60 set-priority actor=human:Wido targets=steward-revives-a-done-goal reason=priority-order subject=steward-revives-a-done-goal from=unranked to=1:14 requested-sequence=14
Integrity: sha256=49b55dc6168fa9533182919f7e67fadec1b9ed4a58ef84f2012c094cee987c09
