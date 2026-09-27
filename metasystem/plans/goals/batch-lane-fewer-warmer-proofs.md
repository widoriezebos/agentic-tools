# batch-lane-fewer-warmer-proofs

- State: queued
- Risk: severity=3 novelty=2 exposure=2 accumulation=2 basis="Changes how every landing is proven and who is ejected; a wrong rule could land a red on main or eject a good unit; shared by all seats."
- Tier: 3
- Intent: The batch landing lane proves a batch once, warm, and blames members only on evidence: per-member prefix proofs are dropped or reuse the batch proof; batch and member proofs share the -trimpath machine caches from engine-owns-disk-lifetimes; the join admission set includes the static audits that let reds reach main on 2026-09-27 (testenv wall-clock audit, proofrun inventory, shell parse check); a red that does not reproduce is recorded as a flake defect with its log instead of ejecting a member; builder and delegate launches get the same host-load awareness as the batch start.
- Origin: human
- Next step: Write the design from the 2026-09-27 lane read (prefix receipts at receipts.go:121-215 contradict design r3:204; per-run GOCACHE at test.go:1545-1549; admission groups at landing_batch_admission.go:26-146; red diagnosis at red.go:49-179) and have Codex Astra critique it.
- OpenedAt: 2026-09-27T18:19:20Z
- Revision: 1
- Labels: efficiency, landing
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0

History:
- 2026-09-27T18:19:20Z 0M2QF8S8FM3FTNN8CAMPD493XR-m1e-c6925449 open actor=human:Wido targets=batch-lane-fewer-warmer-proofs
Integrity: sha256=b5b9032a09e1e9b5e7225ee3335f21c89df405fd0e8742169d35882ecb804024
