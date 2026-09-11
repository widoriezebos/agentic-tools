# retained-proof-reuse-crosses-claims-and-attempts

- State: queued
- Risk: severity=2 novelty=2 exposure=3 accumulation=1 basis="severity 2: a wrongly reused group lets a regression land unproven until the cadence battery catches it; novelty 2: reuse and identities exist and only their scope and composition change, but they are the judge; exposure 3: every landing and every retry on every seat; accumulation 1: the cadence run still sweeps the whole battery"
- Tier: 3
- Intent: Retained proof is reused wherever its execution identity still holds, so unchanged groups are skipped instead of re-run. The delivery deep dive (proof-attempts.md section 5) shows reuse fired for 3.6 percent of groups and that identity churn, not the same-goal rule, is the binding constraint: a Go group's input manifest spans the whole module (985 files), so any change anywhere changes its digest; a section group's identity carries the candidate engine digest, so 47 adoption-fixtures runs had 46 distinct identities; and reuse is scoped to attempts of the same goal, the same accounting revision and a successful terminal, so every re-claim and every set-budget resets it and a group that passed inside a failed attempt never counts. Lifting the scope alone would have saved about 75 minutes fleet-wide. DONE means: (1) a Go group's execution identity is the real dependency closure of its packages (from go list -deps), its tools, discovery, environment, contract and behavior-policy digests, with no engine digest; section and steward-consuming groups keep the engine digest; (2) a selected group is reused from any retained attempt on the seat holding a passed, collection-complete result with the same identity, regardless of goal, accounting revision or that attempt's terminal result, with validateRetainedGroupReuse still refusing forged or stale metadata; (3) proven by a re-stage that moved only files outside a group's closure re-running only the groups whose identity changed, a retry after one failed group re-running that group only, and test verify still refusing when a required group has no matching retained result. Cross-machine reuse is out of scope. This changes the judge: own goal, own battery, three green cadence runs before trust. Goal 9 of plans/delivery-efficiency-plan.md. Wido's word 2026-09-11: I want that added.
- Origin: human
- Next step: Design first, because this changes the judge: read digestGroupInputsWithImplicit and groupExecutionIdentity in internal/proofrun/test_build.go and the retained scan in attempt.go; write the design with the identity composition per adapter and the soundness argument; critique; build; land after delivery-receipt-stops-at-first-failure.
- OpenedAt: 2026-09-11T12:00:24Z
- Revision: 2
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0

History:
- 2026-09-11T12:00:24Z NSAM5W0MV6P1BDGHYZ212KW6FA-m1e-7776b921 open actor=m1e+main-1789128024-87765-b9c0d2 targets=retained-proof-reuse-crosses-claims-and-attempts
- 2026-09-11T15:45:31Z 8FB28Q80B2MXRBXT7HRXCWZT58-m1e-5083721b edit actor=m1e+main-1789141490-27414-b9c0d2 targets=retained-proof-reuse-crosses-claims-and-attempts
Integrity: sha256=5c9dffbce4007e2bbd20a0db988db95f612bcd0eb85517f16cef1e120ac1cc71
