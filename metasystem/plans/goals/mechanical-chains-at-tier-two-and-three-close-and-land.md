# mechanical-chains-at-tier-two-and-three-close-and-land

- State: parked
- Risk: severity=1 novelty=1 exposure=2 accumulation=1 basis="severity 1: a critic dispatched at the wrong class is refused at close by name and re-dispatched, nothing lands wrongly; novelty 1: the effective-obligations resolver and the landing lanes exist; exposure 2: every critic dispatch and every landing of a MECHANICAL chain on a tier-2 or tier-3 goal; accumulation 1: one resolver and one landing precondition"
- Tier: 1
- Intent: What: A small, low-risk change on a higher-risk goal can be reviewed and landed the normal way. The right reviewer is chosen by rule, and the landing path accepts the change once its review has closed. Why: Today such a change lands only through a temporary exception in the landing check (marked "until U5b"), and choosing the right reviewer is easy to get wrong. It works, but through a side door. Pros: One clear rule for reviewers and landing; the temporary exception can be removed. Cons: It touches review and landing code that the landing-lane redesign is changing right now. Nobody has been blocked recently, so the gain today is small.
- Origin: human
- Next step: Next: When it comes back, check whether the redesigned landing lane still carries the "not design-bearing" exception (internal/landing/observe.go). If it does, give each risk tier a fixed reviewer rule and remove the exception, with a test that a small change on a tier-3 goal is reviewed and lands. Done when: a small change on a tier-2 or tier-3 goal is reviewed and lands without any exception in the landing check.
- OpenedAt: 2026-09-12T22:50:52Z
- Revision: 4
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=360 activeJobLimit=1 reviewRoundLimit=0
- BudgetExceptions: 0
- Parked: by=human:wido at=2026-09-30T18:43:32Z because=Parked: such changes land today through a temporary exception and nobody is blocked. Bring it back when a reviewer on such a change is refused for effort, or when the exception is due to be removed.

History:
- 2026-09-12T22:50:52Z QCV0G24BJ1EC54FV7FJ134N15R-m1e-c6925449 open actor=human:Wido targets=mechanical-chains-at-tier-two-and-three-close-and-land
- 2026-09-13T08:18:04Z 9S5ZKQTWR5PTT0FM1EF71NB9Q2-m1-c6925449 edit actor=human:Wido targets=mechanical-chains-at-tier-two-and-three-close-and-land
- 2026-09-30T18:43:32Z 9QAJR0CX7V4XRYYAE5Q679HP3K-m1e-b6a4eb0a park actor=human:wido targets=mechanical-chains-at-tier-two-and-three-close-and-land reason=Parked: such changes land today through a temporary exception and nobody is blocked. Bring it back when a reviewer on such a change is refused for effort, or when the exception is due to be removed.
- 2026-09-30T18:44:23Z T51MK5XC4VC40ZVH0M4KMA6CT5-m1e-b6a4eb0a edit actor=human:wido targets=mechanical-chains-at-tier-two-and-three-close-and-land
Integrity: sha256=9a4208755f02e76f2a709978e3011acaba5f038e482fa1e2a5b5227db242af80
