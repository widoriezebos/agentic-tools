# a-closed-delegates-objects-live-in-the-repository

- State: approved
- Priority: 1
- Sequence: 56
- Risk: severity=3 novelty=1 exposure=2 accumulation=2 basis="Lane test finding 341 (2026-10-15 23:03: three steward job worktrees removed by hand while their quarantine stores were alternates of the seat repository; nothing reachable lost, by luck); memory worktrees-hold-objects-via-alternates; plans/machinery-open-findings-plan-2026-10-06.md finding 19; KI-35."
- Tier: 3
- Intent: A delegate's git objects are absorbed into the repository as soon as its chain is closed and custody is dead, in apply passes under the store's critical section with a complete use census, so a kept worktree holds nothing the repository depends on; the machinery never sends a person to git worktree remove: a records-gone worktree, registered or not, is absorbed and released only through metasystem disk clean --discard, through the existing locked release sequence with tip archival, nested repositories kept.
- Origin: human
- Next step: U1 brief written from the accepted page (designs/a-closed-delegates-objects-live-in-the-repository.md, Astra 3 → 3, stop rule), Astra critique, Codex build by hand, Opus read, gate, hand landing.
- OpenedAt: 2026-10-15T22:09:53Z
- Revision: 3
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-10-15T22:10:01Z revision=2 opid=RVJTGZET500Z0PZ0EG32J7DJME-m1e-670f65d0 authority=proven digest=6a879a21135fb3a44dfc7b4e02ad0fb16cfd72b1f2e29a71547228fa15edca3d episode=2

History:
- 2026-10-15T22:09:53Z G2FCTPYHGDWV5J7H6ZAF9DC54R-m1e-670f65d0 open actor=human:Wido targets=a-closed-delegates-objects-live-in-the-repository
- 2026-10-15T22:10:01Z RVJTGZET500Z0PZ0EG32J7DJME-m1e-670f65d0 approve actor=human:Wido targets=a-closed-delegates-objects-live-in-the-repository
- 2026-10-15T22:10:07Z ZAN9D4AAQ71NBQN09FKFZGE66S-m1e-670f65d0 set-priority actor=human:Wido targets=a-closed-delegates-objects-live-in-the-repository reason=priority-order subject=a-closed-delegates-objects-live-in-the-repository from=unranked to=1:56 requested-sequence=append
Integrity: sha256=7205237f0fbe7d23992c8529a81dee28e5838a807f5054d1138d12f87d9063d8
