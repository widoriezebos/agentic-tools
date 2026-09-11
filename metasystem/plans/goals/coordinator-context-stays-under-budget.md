# coordinator-context-stays-under-budget

- State: queued
- Risk: severity=2 novelty=2 exposure=3 accumulation=1 basis="severity 2: an over-bounded output hides evidence the seat needs; novelty 2: bounded verb output is a new discipline; exposure 3: every seat's every turn; accumulation 1: verbs and docs"
- Tier: 3
- Intent: Coordinator calls carried 386 to 507 thousand prompt tokens each, 5.33 billion cache-read tokens in five days, 19 compactions dropping 14.85 million tokens, while delegates ran efficiently at 76 to 126 thousand (claude-sessions.md sections 1a and 4b of the delivery deep dive); goal list prints 9 MB of JSON. DONE means: a coordinator turn's context stays under 150 thousand tokens by construction: tool output over a size bound lands in a file with a summary line, ledger and status verbs print bounded summaries with --json for detail, briefs and evidence are referenced by path, and the seat's instruction set says so; proven by a week of coordinator per-call prompt tokens with a median under 150 thousand and no compaction. Goal 14 of plans/delivery-efficiency-plan.md.
- Origin: human
- Next step: Measure which tool outputs dominate the context (goal list, watch, test logs, land output), bound them at the verb, write the instruction change, critique, build, land.
- OpenedAt: 2026-09-11T15:45:19Z
- Revision: 1
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0

History:
- 2026-09-11T15:45:19Z 5F6TSCSSSW5B4H5JVZDBKEWHPE-m1e-5083721b open actor=m1e+main-1789141490-27414-b9c0d2 targets=coordinator-context-stays-under-budget
Integrity: sha256=5fc58d2480f4285cdd4612c3b91d8cad2649a64ce60468658c5be20e60d9fbe1
