# Dispositions: idle-escalate-crit1, round 1 (corrected copy; supersedes the file landed at 1a380e62, whose IGE-05 value the validator rejects)

Chain under review: idle-escalate-build1 (reviewed tree
21a1a0a6493d3966b83181178f69690214380604). Critic: idle-escalate-crit1,
three material findings. Orchestrator: m1b.

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| IGE-01 | accepted | Read: the escalation is gated on an empty session-stop marker detail, so any stale or invalid marker restores the unbounded loop with an untrue text; the brief's D2 has no such exception and D4 forbids the text. | Round 2 (metasystem/plans/idle-alarm-steward-escalation-fold2-brief.md, D7): the escalation never depends on the marker; the marker stays display-only. |
| IGE-02 | accepted | Read: the escalation clears ShouldBlock unconditionally after the open-work, unwatched and queue branches spent their once-only slots on the same stop. | Round 2, D7: the escalation clears only the idle block; a block set by another branch on the same stop stands, its side effects already done. |
| IGE-03 | accepted | Read: the claim publish (fetch and push, sixty-second deadline) runs inside the four-second Stop child; the parent's deadline refusal orphans a grandchild that completes the side effects. A block with side effects. | Round 2, D8: no network work in the verdict; the third stop only writes local records (intent, incident, state) and the steward's tick performs the claim and the launch. |
| IGE-04 | noted | True as read; the crash window shrinks to local writes under D8 (intent before state, both local and idempotent), and the steward reconciles a minted intent on its next tick. | Covered by D8; no separate change. |
| IGE-05 | accepted | True: one claim per machine refuses a second claim. The right behaviour is the continuation role's own: continue the claim this machine already holds. The builder's silent test edit was a gap-rule breach and is reversed. | Round 2, D9: if the machine holds a claim, the intent names it and no claim is attempted; otherwise the intent names the first claimable goal and the steward claims it as the seat's actor. The removed test scenario returns as the positive held-claim case. |
| IGE-06 | noted | True as read within D3's scope; folded because it is one sentinel digest and a comment. | Round 2, D10: the unreadable-ledger branch counts under a sentinel digest and escalates to incident plus human alarm at three; the hook comment states the bound truthfully. |
| IGE-07 | noted | True as read: seat-idle episodes never clear and repeats keep stale details. | Round 2, D11: a repeat updates the episode's incident details; the continuation's reap clears the episode whose intent it closed. |
