# goals-live-on-branches

- State: queued
- Risk: severity=2 novelty=3 exposure=3 accumulation=2 basis="severity 2: a mistaken push or rewrite loses reviewed work that main does not hold yet, recoverable from the reader's records; novelty 3: seats push branches for the first time and two landing lanes gain a new input; exposure 3: every goal's build, read, park and landing; accumulation 2: stale branches pile up if cleanup fails, visible in ls-remote"
- Tier: 3
- Intent: A goal's unlanded work is a pushed git branch, one commit per unit, so it survives a lost machine or an emptied /tmp, can be parked and resumed on any enrolled machine, and its land-ready units can be taken onto a release lane and, later, approved by a human before they land. Wido 2026-09-16 23:00 CEST: 'create the design and the backlog item for switching to using branches for goals. That makes it much easier to park goals and to have pre or land ready goals be put onto a release lane. Even if they are across several machines.' Today a unit is a diff file and a detached worktree under /tmp on one machine; on 2026-09-16 three read-clean units and two design amendments existed nowhere else. DONE: (1) every build and fix round of a unit is a commit on goal/<goal-id>, pushed to origin before the round is read, with a trailer naming the goal and the unit, and the branch never carries the ledgers or the goal store; (2) a read names the commit id it read, and a landing proves the landed diff is that commit's diff; (3) a parked goal is resumed on another enrolled machine from the branch alone; (4) the hand lane and the batch lane (units-land-in-batches-under-one-proof) take a unit from a fetched branch commit; (5) the branch is deleted when the goal's last unit lands. The human word before a landing is the separate goal goal-landing-needs-a-human-word.
- Origin: human
- Next step: NEXT (m1c, 2026-09-17 00:10 CEST): design revision 2 is on main (15d2659bd, plans/goals-live-on-branches-design.md). A Codex critique of revision 1 found 16 material problems; a fresh Claude Fable author folded all 16, rejected none. The critique and the fold ledger are records/misc/goals-live-on-branches-critique-r1.md and -fold-ledger-r1.md. Twelve units, each at most 300 lines. Next: one Codex critique of revision 2, stopping when its findings no longer change what gets built; then Codex builds unit 1 (digest, range rule, goal branch check) with a fresh Opus read. First mover: fixture-children-cannot-outlive-their-test's unlanded units, moved the day unit 3 lands. Priority, sequence and claim are Wido's.
- OpenedAt: 2026-09-16T21:00:37Z
- Revision: 2
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0

History:
- 2026-09-16T21:00:37Z CNC78J1TDXB8EMQH6V4Z1DTQSP-m1c-fde8080b open actor=human:Wido targets=goals-live-on-branches
- 2026-09-16T22:01:48Z D674VKW5KA503MSAWM3MQYG3PJ-m1c-fde8080b edit actor=m1c+main-1789560465-86072-c7b126 targets=goals-live-on-branches
Integrity: sha256=3f239562ff0a24e4d743b5d4185f8cf191745ebf6a789ab17ffecd82d45f8bac
