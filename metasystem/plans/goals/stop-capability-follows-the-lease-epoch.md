# stop-capability-follows-the-lease-epoch

- State: approved
- Priority: 1
- Sequence: 49
- Risk: severity=3 novelty=2 exposure=2 accumulation=2 basis="severity 3: a seat locked out of proving its own claimed goal cannot land it through the engine; novelty 2: the divergence path is not understood yet; exposure 2: any seat whose epoch moved; accumulation 2: persists until noticed"
- Tier: 3
- Intent: A goal's recorded stop capability must not silently disagree with the checkout lease it was claimed under. On 2026-09-14 this seat's lease carried claimEpoch 5 while the claimed goal's StopCapability carried ClaimEpoch 1; the proof gate requires equality, so metasystem test run refused with 'active coordinator does not own the claimed goal reservation', and neither metasystem up nor goal edit restamps the capability, so the seat could not run engine proofs for its own goal. DONE: re-arming or reconciling restamps the claimed goal's stop capability from the live lease under the same holder, the divergence is reported by health with its remedy, and a test reproduces the 5-versus-1 case and proves the proof gate admits after reconciliation.
- Origin: human
- Next step: Find where StopCapability.ClaimEpoch is stamped and what diverged it; make up or a reconcile verb restamp it under the same holder; add a health role for the divergence and the reproduction test.
- OpenedAt: 2026-09-14T16:16:51Z
- Revision: 3
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-14T18:55:00Z revision=2 opid=KDRTQXH5QHR6350ECWCWKMKDRW-m1e-c6925449 authority=proven digest=7c559ba9a6440da8d32f3a8cc2543b5fc2471afb39c428b3e60a9e1cb763e626

History:
- 2026-09-14T16:16:51Z 4ZFFPWKQ1BQVB911V8QP1K87BF-m1e-c6925449 open actor=human:Wido targets=stop-capability-follows-the-lease-epoch
- 2026-09-14T18:55:00Z KDRTQXH5QHR6350ECWCWKMKDRW-m1e-c6925449 approve actor=human:Wido targets=stop-capability-follows-the-lease-epoch
- 2026-09-14T18:56:13Z HYZ4K6NXZPAE8Y2ECYHNDX9SSK-m1e-c6925449 set-priority actor=human:Wido targets=stop-capability-follows-the-lease-epoch reason=priority-order subject=stop-capability-follows-the-lease-epoch from=unranked to=1:49 requested-sequence=append
Integrity: sha256=ab2f09b6262322f05e64206f96cc864ef0bffa393c994762316f0fbce8b0aca9
