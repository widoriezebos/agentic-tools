# idle-backlog-line-names-a-few-not-all

- State: approved
- Risk: severity=1 novelty=1 exposure=3 accumulation=1 basis="It costs every reader of every Stop message on every machine a screen of ids; nothing is at risk; one function and its tests."
- Tier: 3
- Intent: The turn verdict's idle-with-backlog block (enforceIdleBacklog in internal/goal/turnverdict.go) lists every claimable goal by id in one line: on 2026-09-06 with the grandfather sweep's 108 approved goals the Stop message on m1c ran to more than a hundred ids, which no reader can act on and which drowns the OPEN WORK line above it. The line exists to say that claimable work waits and to point at it, not to enumerate the queue. DONE means the line names the count and at most five goals (the seat's pinned ones first, then queue order), points the reader at the goal listing verb for the rest, keeps the same block-once behavior and block source, and the tests that pin the display (internal/goal) and any fixture greps update to the new shape.
- Origin: main
- Next step: Read enforceIdleBacklog and the tests that grep 'claimable goals await'; decide the five-name order (pinned to this machine first, then queue order) and the pointer wording; brief one round with the tests.
- OpenedAt: 2026-09-06T11:12:19Z
- Revision: 2
- Labels: robustness
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-06T11:13:04Z revision=2 opid=SF93QKH1A383QFP6X19MWXPCV1-m1-7cd0bd60 authority=proven digest=5d4927118ebb1bbe5841924bbdd6e2b76afdac7581fa88c263d6c293fa26dfce

History:
- 2026-09-06T11:12:19Z DZTBZNAMN85YVTHMK69C62D0RW-m1c-7cd0bd60 open actor=m1c+main-1788680061-17829-64951c targets=idle-backlog-line-names-a-few-not-all
- 2026-09-06T11:13:04Z SF93QKH1A383QFP6X19MWXPCV1-m1-7cd0bd60 approve actor=human:Wido targets=idle-backlog-line-names-a-few-not-all
Integrity: sha256=463f08e36afbe2ff64797d19128edc2ccd6c0dcfcd8588d85766ea0723c6f9dc
