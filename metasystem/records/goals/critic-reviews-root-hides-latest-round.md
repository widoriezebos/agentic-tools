# critic-reviews-root-hides-latest-round

- State: done
- Risk: severity=1 novelty=1 exposure=2 accumulation=1 basis="severity 1: a misleading refusal with a working workaround, no wrong record is written; novelty 1: one door check or one lookup change in code that exists; exposure 2: every chain whose critic is dispatched after a follow-up round, on every machine; accumulation 1: each occurrence costs one seat detour, nothing compounds"
- Tier: 2
- Intent: A code-critic dispatched with --reviews naming a chain root whose latest implementer round is a follow-up reviews the latest round's tree, but the critique register reads diff.patch from the root's own round (internal/dispatch/finding_register.go, the reviewed-round lookup) and refuses with 'reviewed implementer round has no diff.patch; run conformance --stage review first' although conformance ran for the latest round. Either the delegate door refuses --reviews naming a root whose latest round is a follow-up and says to name the round job, or the register resolves the reviewed job to the chain's latest implementer round; either way the refusal names the round and the file it looked for.
- Origin: main
- Next step: Seen 2026-09-06 on hook-root-installation-fix: review hrif-review1b-20260906 over build round 2; worked around by running conformance for the root job, which writes an identical diff.patch under round 1. Small chain, MECHANICAL: prefer the door refusal (fails early, before a critic is spent), one fixture per shape (root named after a follow-up refused; round job named passes), refusal text names the round. Also make the follow-up door's 'advance the canonical register before reading exhaustion' name the verb: job critique-register-advance --root-job <critic> --round-job <critic>.
- Concluded: Landed at 01cd6013 through chain critic-latest-build1 (one implementer round on Codex Sol, mechanical reach, the orchestrator's gate as the examination). A fresh code-critic or warden dispatch that names a chain root whose latest implementer round is a follow-up is refused at claim, naming the named job, the latest round job and both rounds; a critic's follow-up keeps its inherited binding because it resolves the register of the round it continues; the register's missing-diff refusal names the job, round, path and the exact conformance command; the follow-up door names the register-advance verb with its arguments. Proof: package tests for every shape; the dispatch fixture bed passed on the reviewed tree from this Mac.
- OpenedAt: 2026-09-06T07:18:14Z
- Revision: 5
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-06T07:43:32Z revision=2 opid=CQRW2VXG5W7PPT2TS87P9P3VKJ-m1-a4f8999f authority=proven digest=def22c4d0036cfa3684b6a821ef9e5e3964f8f551e3ebe35bb0905937d474574
- Sliced: machine=m1b lineage=main-1788680071-18713-e76d5d revision=3 at=2026-09-06T10:39:14Z

History:
- 2026-09-06T07:18:14Z 145EFXP3QPNQAYBJKZA61DFSXE-m1-a4f8999f open actor=m1+main-1788594343-3833-fb64b9 targets=critic-reviews-root-hides-latest-round
- 2026-09-06T07:43:32Z CQRW2VXG5W7PPT2TS87P9P3VKJ-m1-a4f8999f approve actor=human:Wido targets=critic-reviews-root-hides-latest-round
- 2026-09-06T10:37:25Z TF2WXJS070HPYZDNX5KAR14F7X-m1b-c6925449 claim actor=m1b+main-1788680071-18713-e76d5d targets=critic-reviews-root-hides-latest-round
- 2026-09-06T10:39:14Z 9ZV2ENFVZYSSYHS5N6JWSSXQ5J-m1b-c6925449 slice-start actor=m1b+main-1788680071-18713-e76d5d targets=critic-reviews-root-hides-latest-round
- 2026-09-06T11:14:55Z 1PBNP57P07FV4A2YYC0M4JZM5R-m1b-c6925449 done actor=m1b+main-1788680071-18713-e76d5d targets=critic-reviews-root-hides-latest-round
Integrity: sha256=a816907b957a338e7a3fa6bbf25e882649b914977f83aaba9f932614ab3521d3
