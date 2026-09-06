# critic-reviews-root-hides-latest-round

- State: queued
- Risk: severity=1 novelty=1 exposure=2 accumulation=1 basis="severity 1: a misleading refusal with a working workaround, no wrong record is written; novelty 1: one door check or one lookup change in code that exists; exposure 2: every chain whose critic is dispatched after a follow-up round, on every machine; accumulation 1: each occurrence costs one seat detour, nothing compounds"
- Tier: 2
- Intent: A code-critic dispatched with --reviews naming a chain root whose latest implementer round is a follow-up reviews the latest round's tree, but the critique register reads diff.patch from the root's own round (internal/dispatch/finding_register.go, the reviewed-round lookup) and refuses with 'reviewed implementer round has no diff.patch; run conformance --stage review first' although conformance ran for the latest round. Either the delegate door refuses --reviews naming a root whose latest round is a follow-up and says to name the round job, or the register resolves the reviewed job to the chain's latest implementer round; either way the refusal names the round and the file it looked for.
- Origin: main
- Next step: Seen 2026-09-06 on hook-root-installation-fix: review hrif-review1b-20260906 over build round 2; worked around by running conformance for the root job, which writes an identical diff.patch under round 1. Small chain, MECHANICAL: prefer the door refusal (fails early, before a critic is spent), one fixture per shape (root named after a follow-up refused; round job named passes), refusal text names the round. Also make the follow-up door's 'advance the canonical register before reading exhaustion' name the verb: job critique-register-advance --root-job <critic> --round-job <critic>.
- OpenedAt: 2026-09-06T07:18:14Z
- Revision: 1
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-06T07:18:14Z 145EFXP3QPNQAYBJKZA61DFSXE-m1-a4f8999f open actor=m1+main-1788594343-3833-fb64b9 targets=critic-reviews-root-hides-latest-round
Integrity: sha256=b4ca72c0cb1bdae0c601a27298eeadf2fca205e86e6cd60db82d105e926505ee
