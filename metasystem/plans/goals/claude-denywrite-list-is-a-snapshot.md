# claude-denywrite-list-is-a-snapshot

- State: queued
- Priority: 3
- Sequence: 46
- Risk: severity=2 novelty=2 exposure=2 accumulation=1 basis="severity 2: a delegate could write another job's worktree or a git lock during its round; novelty 2: needs either a worktree relocation or a sandbox rule not yet probed; exposure 2: every claude delegate round on a busy seat; accumulation 1: nothing compounds"
- Tier: 2
- Intent: The claude delegate settings from goal claude-implementer-read-roots-writable (39123c99) deny the existing entries beside a job worktree's ancestor chain, computed once when the settings file is built. Anything created later under those ancestors during the delegate's session (a sibling job's worktree, a new ref file under .git/refs, .git/index.lock in the live git dir) is not in the list and stays writable for the rest of the round (critique cir-critic2, F-1). DONE means either the denial no longer depends on a snapshot (for example the dispatcher moves job worktrees outside the repository root so the whole root can be denied, or the sandbox is given a rule that denies new entries too), or a live probe shows an entry created after settings time is refused, with a test pinning it.
- Origin: main
- Next step: Probe first: does a directory created after settings time under a denied-entries ancestor accept writes (yes, expected)? Then decide: relocate worktrees (dispatch.sh, cleanup, fixtures) versus a sandbox rule; brief, build, critique, land.
- OpenedAt: 2026-09-06T22:18:37Z
- Revision: 2
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-06T22:18:37Z 5DR399F9479DXKSK9GQKWBN16F-m1c-7cd0bd60 open actor=m1c+main-1788680061-17829-64951c targets=claude-denywrite-list-is-a-snapshot
- 2026-09-08T16:01:48Z JB6W0W4236EN30XEV781CM7K6S-m1-7cd0bd60 set-priority actor=human:Wido targets=claude-denywrite-list-is-a-snapshot reason=priority-order subject=claude-denywrite-list-is-a-snapshot from=unranked to=3:46 requested-sequence=46
Integrity: sha256=b11c2cc7168539b3deaa8c7f70192daeca7b3dc80a4db2a1d5fb81c246fd96c3
