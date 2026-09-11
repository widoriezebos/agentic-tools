# proof-attempts-settle-to-minutes-run

- State: queued
- Risk: severity=2 novelty=1 exposure=2 accumulation=2 basis="severity 2: over-charging starves goals of budget they did not spend but grants nothing; novelty 1: the delegated-job settlement is the pattern; exposure 2: every goal that runs proof attempts; accumulation 2: each ended proof attempt keeps its full reservation charged until the goal's revision changes"
- Tier: 2
- Intent: An ended testing-risk proof attempt (1b12f534) still charges its full reservation to the goal's reserved job-minutes, the same over-charge that dispatch-cap-necessity removed for delegated jobs; the settlement box named it as its own component (ProofReservationMinutes) rather than settle it. DONE means an ended proof attempt charges the minutes it ran, rounded up and clamped to its reservation, a live one its reservation, with the refusal line's proof=<n> following, and 1b12f534's own tests amended for exactly that.
- Origin: main
- Next step: One Sol round in internal/dispatch/budget.go's proof-attempt loop mirroring settledJobMinutes for proof attempts, with tests beside the sum-invariant test; one Opus review; land with --chain. Found by dispatch-cap-crit3-20260910 (DCN-09) on 2026-09-10. Wido approves at the terminal.
- OpenedAt: 2026-09-09T22:51:49Z
- Revision: 4
- Pinned: m1e
- BudgetExceptions: 0

History:
- 2026-09-09T22:51:49Z 7MWERD2ZYAKHY1N3KVQVR8TY8K-m1-1701c13c open actor=m1+main-1788940932-18533-7fa6c2 targets=proof-attempts-settle-to-minutes-run
- 2026-09-10T05:48:27Z WEXFFS3GDSTNTEYCMDYSEM1GJA-m1-c6925449 approve actor=human:Wido targets=proof-attempts-settle-to-minutes-run
- 2026-09-11T08:01:57Z YVD6080SP15T0KF02T7S3MDJ8E-m1-c6925449 unapprove actor=human:Wido targets=proof-attempts-settle-to-minutes-run reason=Wido 2026-09-11: standing approval withdrawn to regain control of what is built next; re-approve deliberately
- 2026-09-11T16:17:29Z 8M31V7VHZ6TFG60GMGZQZ9S09F-m1-c6925449 set-pin actor=human:Wido targets=proof-attempts-settle-to-minutes-run
Integrity: sha256=b1078906599a9694e47372dabe6368aa449103ad02e7099d43c2d0bfe477089f
