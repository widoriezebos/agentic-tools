# a-live-read-proves-its-subject

- State: queued
- Risk: severity=1 novelty=1 exposure=2 accumulation=1 basis="One git flag so two existing digests of the same bytes agree; a mistake is the refusal we have today"
- Tier: 1
- Intent: A unit run's live read proves the committed subject: the worktree diff the read digests is written with the same git flags as the commit's patch digest (--full-index), so a clean run-read is promoted at collection instead of refused and replaced by a committed critic; a tier-1 goal can complete its read (finding 436)
- Origin: main
- Next step: On m1e's build lane after a-settled-reply-names-its-machine and before a-ready-goal-behind-a-claim-is-a-row (one line, minutes): opus-builder adds --full-index to worktreeDiff (internal/launch/unit_run.go:1182) and a test that computes both digests from one committed unit through the production paths (worktreeDiff and read_subject_compute) and asserts they are equal, failing first; Read each round: yes; the steward submits with work review --changes --last
- OpenedAt: 2026-10-27T17:41:56Z
- Revision: 1
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=360 activeJobLimit=1 reviewRoundLimit=0
- BudgetExceptions: 0

History:
- 2026-10-27T17:41:56Z 87D50ZJR623QVE7YY5N17HMQ1X-m1e-55cdfc19 open actor=human:Wido targets=a-live-read-proves-its-subject
Integrity: sha256=60bab77ceb993391a24892e5cdab93020f84d74ff8b2f6579d3881f73f380527
