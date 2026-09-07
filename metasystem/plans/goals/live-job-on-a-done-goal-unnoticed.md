# live-job-on-a-done-goal-unnoticed

- State: queued
- Risk: severity=2 novelty=1 exposure=2 accumulation=2 basis="severity 2: a delegate keeps spending on work that is finished and nothing says so; novelty 1: compare live job records against their goal's state on a path that already runs each tick; exposure 2: every seat whose steward ticks while a goal completes; accumulation 2: each occurrence burns a full job cap and recurs whenever a landing outlasts a tick"
- Tier: 2
- Intent: Nothing surfaces a job that is still running against a goal that is already done. Evidence from m1d 2026-09-07: job steward-b2271fe9e3c7f9b9 had a record with status running and a live codex exec on gpt-5.6-sol nine minutes after goal fixture-review-by-date-expired was marked done, and the seat only found it by going looking for why its open-work report had gone quiet; no health role, tick or narrator line named it. This is the DETECTION half of the incident: the cause is goal steward-revives-a-done-goal, which stops that particular revival, but any cause producing the same state would be equally invisible. DONE means a live job record whose goal is done (or whose goal revision no longer matches its claim) is named loudly by machinery that already runs, with a fixture that seeds exactly that state and proves the line appears
- Origin: main
- Next step: read the steward tick's job survey and the health roles that already read artifacts/agents/jobs; the comparison is cheap because both facts are on disk. Decide whether the right voice is a health role, an escalation episode as failed-job-attention uses, or a narrator line, and say why in the design note. Do not conflate with failed-job-attention, which surfaces jobs that DIED; this is work continuing after its goal finished
- OpenedAt: 2026-09-07T09:20:41Z
- Revision: 1
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-07T09:20:41Z XZC6YNW7TDWJWHC8Y0BQRPHHER-m1b-c6925449 open actor=m1b+main-1788680071-18713-e76d5d targets=live-job-on-a-done-goal-unnoticed
Integrity: sha256=e20d99e440e093629211a7fb8cae9e7b7a2bd7da7ddf046913cb4ee07e3d6bd9
