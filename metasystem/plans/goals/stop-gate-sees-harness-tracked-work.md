# stop-gate-sees-harness-tracked-work

- State: approved
- Risk: severity=2 novelty=2 exposure=3 accumulation=2 basis="severity 2: wasted model turns and context, no wrong landing; novelty 2: extends the registered wait to local work and human answers; exposure 3: every seat, every wait; accumulation 2: grows with parallel work"
- Tier: 2
- Intent: The stop gate reports a seat idle ('no task in flight, 0 jobs') while the seat waits on harness-tracked work: Codex builds, subagent reads, local engine and bed runs, or a human answer. On 2026-09-15 this forced dozens of empty turns on m1e, about 50 on m1b and 30 to 60 minutes on m1c, each with a 15 to 36 second hook and more context. registered-wait-matches-the-runtime-session covers job watches only. DONE: a seat can register in-flight local work (by pid) or a pending human question (with a deadline) as a wait; the stop gate allows the stop while that wait is live and names it; the wait ends when the work ends; fixtures prove a live local wait and a human wait each allow the stop and a dead one does not.
- Origin: human
- Next step: Design, tier 2, beside registered-wait-matches-the-runtime-session: extend the wait registry to pid-backed local work and to a pending human question, and decide each liveness rule.
- OpenedAt: 2026-09-15T05:55:06Z
- Revision: 2
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-15T05:55:14Z revision=2 opid=Q4A1XGD8SS1NP18ZXR6H4J2D7P-m1e-c6925449 authority=proven digest=0e2336de904436c47d7feb8c7c51efa0f94188b03a99d611ddc81305c50a88b7

History:
- 2026-09-15T05:55:06Z 41RARJ72SCV6WTBF0AMDG4AF1E-m1e-c6925449 open actor=human:Wido targets=stop-gate-sees-harness-tracked-work
- 2026-09-15T05:55:14Z Q4A1XGD8SS1NP18ZXR6H4J2D7P-m1e-c6925449 approve actor=human:Wido targets=stop-gate-sees-harness-tracked-work
Integrity: sha256=41214bc66b91cee44d8358a39894971e4e84ab6933a9005361c9c958eec16708
