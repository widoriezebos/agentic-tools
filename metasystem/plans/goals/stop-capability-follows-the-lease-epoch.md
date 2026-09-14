# stop-capability-follows-the-lease-epoch

- State: queued
- Risk: severity=3 novelty=2 exposure=2 accumulation=2 basis="severity 3: a seat locked out of proving its own claimed goal cannot land it through the engine; novelty 2: the divergence path is not understood yet; exposure 2: any seat whose epoch moved; accumulation 2: persists until noticed"
- Tier: 3
- Intent: A goal's recorded stop capability must not silently disagree with the checkout lease it was claimed under. On 2026-09-14 this seat's lease carried claimEpoch 5 while the claimed goal's StopCapability carried ClaimEpoch 1; the proof gate requires equality, so metasystem test run refused with 'active coordinator does not own the claimed goal reservation', and neither metasystem up nor goal edit restamps the capability, so the seat could not run engine proofs for its own goal. DONE: re-arming or reconciling restamps the claimed goal's stop capability from the live lease under the same holder, the divergence is reported by health with its remedy, and a test reproduces the 5-versus-1 case and proves the proof gate admits after reconciliation.
- Origin: human
- Next step: Find where StopCapability.ClaimEpoch is stamped and what diverged it; make up or a reconcile verb restamp it under the same holder; add a health role for the divergence and the reproduction test.
- OpenedAt: 2026-09-14T16:16:51Z
- Revision: 1
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0

History:
- 2026-09-14T16:16:51Z 4ZFFPWKQ1BQVB911V8QP1K87BF-m1e-c6925449 open actor=human:Wido targets=stop-capability-follows-the-lease-epoch
Integrity: sha256=84a2bec87171bc8a67470e3afb2ea8e7d25aa6a28fe1b18d95983cfdfd6addd8
