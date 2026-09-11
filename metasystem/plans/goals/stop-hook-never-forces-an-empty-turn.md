# stop-hook-never-forces-an-empty-turn

- State: queued
- Risk: severity=2 novelty=2 exposure=3 accumulation=1 basis="severity 2: a wrong refusal rule either forces turns as today or lets an unsafe stop through; novelty 2: refusal causes are reclassified between seat and steward; exposure 3: every seat's every stop; accumulation 1: one hook"
- Tier: 3
- Intent: The Stop hook refused 331 times in five days ('could not prove that stopping is safe'), forcing 292 coordinator turns that cost 157 million prompt tokens, 90 of which only polled again and 73 were text-only; the recorded causes were 'the narrator digest could not be read' (184 across m1, m1b and m1d since 2026-09-10), 'stop deadline expired' (117 on m1) and 'supervision arming failed' (about 95) (claude-sessions.md section 4c and the seats' stop-refusal records, delivery deep dive). DONE means: a refusal names a condition the seat can act on and the one command that clears it; a read failure of a digest, hook-evidence or arming state that the seat cannot change never refuses the stop and is reported to the steward instead; a refusal is not repeated for an unchanged condition within one stop deadline, so no turn is forced when nothing changed; proven by a fixture replaying the three recorded causes and by a week of stop refusals under 10 per seat per day. Goal 4 of plans/delivery-efficiency-plan.md.
- Origin: human
- Next step: Read the Stop hook (scripts/agents and internal supervision), classify the recorded refusal causes into seat-actionable and steward-owned, write the design, critique, build, land.
- OpenedAt: 2026-09-11T15:45:01Z
- Revision: 2
- Pinned: m1e
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0

History:
- 2026-09-11T15:45:01Z T6Z7DYJ3CFGDRPYXNRNTTWXERM-m1e-5083721b open actor=m1e+main-1789141490-27414-b9c0d2 targets=stop-hook-never-forces-an-empty-turn
- 2026-09-11T15:47:04Z PNHSKRR4SQA1Q4SS26QWE5YQCW-m1-c6925449 set-pin actor=human:Wido targets=stop-hook-never-forces-an-empty-turn
Integrity: sha256=391cd2097a96ad339e897a1005d2f87bcf6d496d688622621f74006d260d031d
