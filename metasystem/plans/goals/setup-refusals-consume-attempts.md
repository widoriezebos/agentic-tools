# setup-refusals-consume-attempts

- State: queued
- Tier: 1
- Intent: A dispatch refused before any agent ran (refusalClass setup: a census fingerprint that has not caught up with a freshly armed engine, a brief header defect, a script-engine skew) still consumes one of the goal's attempts and its reserved job minutes. On m2 on 2026-09-04 the dispatcher-skew item lost two of its three tier-1 attempts to setup refusals (a census race right after steward arm, and a probe of the same refusal) and its box closed while the real work sat finished on a preserve branch. DONE means a job record that ends in a setup refusal releases its reservation and does not count as an attempt; the goalbudget test pins it; a refusal that happens after the agent started still counts.
- Origin: main
- Next step: TIER 1 per R-54-m1 (one branch in the budget admission and its test): build, go test ./internal/goalbudget/... ./internal/dispatch/..., land through a chain; box 1h/3/60m/1. Waits for human approval for execution.
- OpenedAt: 2026-09-04T10:10:08Z
- Revision: 1
- Labels: robustness
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=360 activeJobLimit=1 reviewRoundLimit=0

History:
- 2026-09-04T10:10:08Z YXF7VM88PN6VYT2PKPAZJ8VYX8-m2-5fcf08ab open actor=m2+main-1788441779-14484-82d6ed targets=setup-refusals-consume-attempts
Integrity: sha256=b9776055dc8fa187fbb4a08fbda31f078d758d41173a9112e46e333176367e63
