# one-goal-act-one-ledger-commit

- State: queued
- Priority: 2
- Sequence: 57
- Risk: severity=2 novelty=2 exposure=3 accumulation=3 basis="Goal publication affects every program and preserves human-act history; defects create ledger churn or lose provenance without changing product code directly."
- Tier: 2
- Intent: What: When several goal actions are taken together (for example open, approve and pin), they go to main as one commit that lists every action, instead of one commit per action. Why: About three of every four commits on main are goal-ledger actions, roughly twelve per code commit. That buries the code history and makes main churn. Pros: A readable history, less push traffic and fewer collisions between seats. Cons: Publishing several actions at once must be all-or-nothing, and the history view must still show each action inside the combined commit.
- Origin: human
- Next step: Next: Write a short design: how several goal actions are published as one commit, what the commit message lists, and how goal history still shows each action separately; one critique round; build. Done when: a batch of goal actions lands as one commit whose message names each action, and a goal's history still lists them one by one.
- OpenedAt: 2026-09-16T21:03:14Z
- Revision: 3
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-16T21:03:14Z 4SP32ZYJ2GE6ZYTD0PYS7555CJ-m1e-c6925449 open actor=human:Wido targets=one-goal-act-one-ledger-commit
- 2026-09-16T21:03:18Z 4TQYVD3VDEN3BEE4TK32A2HEAK-m1e-c6925449 set-priority actor=human:Wido targets=one-goal-act-one-ledger-commit reason=priority-order subject=one-goal-act-one-ledger-commit from=unranked to=2:57 requested-sequence=57
- 2026-09-30T18:45:02Z 7RXT15WZMNZF3B1WDMS9T8G62K-m1e-b6a4eb0a edit actor=human:wido targets=one-goal-act-one-ledger-commit
Integrity: sha256=2e6c0620d4b701d74bb112f82a7ed1a5e07ffd5ee27d95b106d49037cf13dcef
