# stop-records-readable-from-every-checkout

- State: abandoned
- Risk: severity=2 novelty=1 exposure=2 accumulation=1 basis="a stuck goal needs a person to find the right checkout; storage location change; every seat"
- Tier: 2
- Intent: What: When a goal is stopped for running over its budget, any checkout of the shared ledger can resume or release it, not only the checkout whose steward stopped it. Why: the stop record is written into the folder of whichever seat's steward stopped the goal, and goal resume looks only in the folder of the checkout it is run from. On 2026-09-30 a goal stopped by the landing seat's steward could not be cleared from the ui seat at all: release said only resume may clear it, resume said the record was missing, and it took enrolling a terminal in the landing checkout to clear it. Pros: a stopped goal can always be cleared from where the person is working. Cons: the record has to live somewhere every checkout can read (the shared ledger or a machine-wide place), which changes where stop records are kept.
- Origin: human
- Next step: Next: decide where stop records live so every checkout of the same ledger can read them, and add a test that stops a goal from one checkout and resumes it from another. Done when: that test passes, and resume from a checkout that did not stop the goal no longer fails with a missing file.
- OpenedAt: 2026-09-30T19:07:02Z
- Revision: 2
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0
- Abandoned: by=human:Wido at=2026-11-07T09:57:51Z revision=2 opid=D32TYXNYEQXGRSRHHTZ9DPTXKM-m1e-1dae276b carried=a-stop-is-read-where-the-goal-is because=Same intent as a-stop-is-read-where-the-goal-is D12 (fixes analysis 2026-11-07)

History:
- 2026-09-30T19:07:02Z 1YB3PY93CFSEKKVGKZWWQX7Q33-ui-bc2fda53 open actor=human:Wido targets=stop-records-readable-from-every-checkout
- 2026-11-07T09:57:51Z D32TYXNYEQXGRSRHHTZ9DPTXKM-m1e-1dae276b abandon actor=human:Wido targets=stop-records-readable-from-every-checkout carried=a-stop-is-read-where-the-goal-is reason=Same intent as a-stop-is-read-where-the-goal-is D12 (fixes analysis 2026-11-07)
Integrity: sha256=1fe9344dda2a2a64b10e78592b57d790cdda39349c5dd347ce5da09ea5e7790c
