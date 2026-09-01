# Alert channel design critique — revision 12, round 6 (Sol)

Chain: revision 12 -> critic design-critic-137ec854313e987c87cdfc1f
(codex gpt-5.6-sol, xhigh, fresh context), 2026-09-02. Two material
findings, both refinements of the one remedy-eligibility corner: the
journal-time checks read mutable state without an atomic snapshot
rule, and the current-goal gate is omitted from the mirrored
precondition set. Revision 13 folds both and re-scopes the validity
claim to its honest reach (verified at journal time, best-effort at
read time — the human acts later regardless).

## AC12-ANSWER-JOURNAL-ATOMICITY-001 — high, material=True

CLAIM: AC12-ANSWER-JOURNAL-ATOMICITY-001, the journal-time eligibility snapshot defect: revision twelve checks mutable chain and reviews-target state before saving the episode but specifies no consistency boundary joining that check to the durable save. A concurrent follow-up can add a newer chain member, an explicit close can set chainClosed, or evidence garbage collection can remove the reviews target after the loaded-table check and before the episode is journaled. The row then remains follow-up or fresh-dispatch even though the mirrored dispatcher gate already fails at the exact journal moment. Calling later changes post-journal degradation does not cover this pre-save window. An implementer following the zero-additional-open scan contract would build this race unless they invent a lock or final recheck, and either choice changes the stated scan and locking contract.

EVIDENCE: metasystem/plans/alert-channel-design.md lines 1314-1324 define eligibility from one already-read table; lines 1451-1506 require only one check before the write; and lines 1554-1561 claim acceptance at the moment of journaling. In shipped code, metasystem/scripts/agents/dispatch.sh lines 1703-1725 and 1963-2018 create follow-ups under a chain lock, while lines 2082-2113 close a chain under that lock. The producer does not acquire it. Metasystem/internal/evidence/gc.go lines 375-449 can remove a reviews target without the alert lock. Thus all three checked facts can change between verification and the episode save.

## AC12-ANSWER-GOAL-ELIGIBILITY-001 — high, material=True

CLAIM: AC12-ANSWER-GOAL-ELIGIBILITY-001, the omitted current-goal gate: both advertised commands require the recorded goal to remain a currently claimed accepted goal, but the producer equates a nonempty historical goalId with that current state and never mirrors the dispatcher's goal-binding check. The design's own outage proof explicitly retains and later scans records after their goal becomes unclaimed. Such a record can pass every new chain or reviews check and receive a concrete command that the dispatcher immediately refuses because the goal is no longer claimed. An implementer would therefore still build an Answer that is dead on arrival in an ordinary delayed-scan case.

EVIDENCE: metasystem/plans/alert-channel-design.md lines 1265-1269 select on terminal status and nonempty goalId, while the outage interleaving at lines 1965-1968 expressly includes a goal becoming unclaimed before the first later tick. Lines 1482-1495 say fresh dispatch has no other applicable check, and lines 1554-1561 claim the advertised command passed the dispatcher's preconditions. In contrast, metasystem/scripts/agents/dispatch.sh lines 1327-1334 perform goal binding for fresh dispatch and lines 1770-1775 perform it for follow-up; metasystem/internal/dispatch/stop.go lines 53-58 refuse any goal that is not presently claimed and accepted.

## Critic-declared gaps (verbatim)

- The task calls this semantic critique round six, but the generated runtime notice identifies the launched critic job as round one. No same-session continuation was supplied, so the return preserves the harness-observed round number one.
- The critique brief at metasystem/plans/alert-channel-r12-critique-brief.md does not declare the failsafe round, threat model, risk appetite, or critic-chain round budget required by the design-critique skill. The review covered normal trusted operation and the full current design, but hostile or corrupt-state cases cannot be classified as in or out of scope, and lawful chain exhaustion cannot be confirmed.
- No revision-twelve implementation exists to execute. The dispatcher also cannot be exercised through its real process boundary in this read-only critic runtime because its lease and census gates require live supervision. The findings are grounded in the written design and shipped lock and gate code; live proof was not required.
- The worktree acquired an unrelated tracked modification at metasystem/records/narrator-digest.log during the critic run. I did not inspect or modify that file; metasystem/plans/alert-channel-design.md remains byte-identical to its landed revision-twelve commit.
