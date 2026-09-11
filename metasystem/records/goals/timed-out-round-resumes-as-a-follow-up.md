# timed-out-round-resumes-as-a-follow-up

- State: done
- Risk: severity=2 novelty=1 exposure=2 accumulation=1 basis="severity 2: a wrong resume carries an unreviewed partial tree into a chain's record; novelty 1: the follow-up rebase and the terminal-patch records exist, this is one more admitted terminal state; exposure 2: dispatch.sh and internal/dispatch on every machine; accumulation 1: one rule, one fixture"
- Tier: 2
- Intent: A follow-up round is refused after a round that hit its cap (scripts/agents/dispatch.sh: 'follow-up requires the newest record to be completed or failed with protocol_error; use a fresh dispatch after pending, running, timeout, or process-lost'), although the timed-out round's worktree and staged index are intact. On 2026-09-11 hcl-build1-20260911 hit its 120-minute cap with 52 files staged and every named test green; the only lawful continuation was a fresh chain (hcl-build2-20260911) whose first step applied the coordinator's saved patch verbatim: one dispatch record, a manual patch hand-off and a coordinator's quarter hour for nothing. Wido 2026-09-11: 'yes, that sounds like a backlog item we need'.
- Origin: main
- Next step: Design the resume rule: a timed-out or process-lost newest round whose worktree still holds its staged candidate admits a follow-up that starts from that index (the timed-out round records its terminal tree; the follow-up's diff is taken against it and the rebase planning runs as today); a pending or running round still refuses; a lost or dirty-beyond-the-index worktree still needs a fresh chain. Fixture in scripts/agents/dispatch-fixtures.sh: a fake round that times out with a staged file, then a follow-up whose worktree holds that file; a second case where the worktree is gone and the follow-up refuses naming a fresh dispatch.
- Concluded: Absorbed by goal:capped-round-continues-instead-of-restarting in the 2026-09-11 backlog consolidation on Wido's word; Program 21 capped-round-continues-instead-of-restarting's NEXT names the same refusal (dispatch.sh, now :2242, 'use a fresh dispatch after pending, running, timeout, or process-lost') as its research-first question and its DONE makes the successor a continuation of the capped round; this goal was op. Its specific requirement is appended to that goal's next step.
- OpenedAt: 2026-09-11T13:10:46Z
- Revision: 2
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-11T13:10:46Z 0FZDY3C0RKWQ1QSJPQ8P0EAQHV-m1d-a38bcdde open actor=m1d+main-1788941004-20871-6e7a43 targets=timed-out-round-resumes-as-a-follow-up
- 2026-09-11T22:07:04Z 8K0ANZY5C899THCTMCJFX9Y4WP-m1-c6925449 done actor=human:Wido targets=timed-out-round-resumes-as-a-follow-up
Integrity: sha256=e403cf6c6cc98ee1d4a08c5466cfac61e5948ddb0c68a36c4e29260e8ae659b1
