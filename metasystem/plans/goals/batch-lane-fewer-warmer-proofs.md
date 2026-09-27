# batch-lane-fewer-warmer-proofs

- State: approved
- Risk: severity=3 novelty=2 exposure=2 accumulation=2 basis="Changes how every landing is proven and who is ejected; a wrong rule could land a red on main or eject a good unit; shared by all seats."
- Tier: 3
- Intent: The batch landing lane proves a batch once, warm, and blames members only on evidence: per-member prefix proofs are dropped or reuse the batch proof; batch and member proofs share the -trimpath machine caches from engine-owns-disk-lifetimes; the join admission set includes the static audits that let reds reach main on 2026-09-27 (testenv wall-clock audit, proofrun inventory, shell parse check); a red that does not reproduce is recorded as a flake defect with its log instead of ejecting a member; builder and delegate launches get the same host-load awareness as the batch start.
- Origin: human
- Next step: Write the design from the 2026-09-27 lane read (prefix receipts at receipts.go:121-215 contradict design r3:204; per-run GOCACHE at test.go:1545-1549; admission groups at landing_batch_admission.go:26-146; red diagnosis at red.go:49-179) and have Codex Astra critique it.
- OpenedAt: 2026-09-27T18:19:20Z
- Revision: 2
- Labels: efficiency, landing
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-27T18:20:26Z revision=2 opid=K72PV9EQVEB11663MWTTDJ9KQ2-m1e-c6925449 authority=proven digest=1b8aa225363f1341250145b189000f2fb62a11ad5cc86d036cbc7bc43220a287 episode=2

History:
- 2026-09-27T18:19:20Z 0M2QF8S8FM3FTNN8CAMPD493XR-m1e-c6925449 open actor=human:Wido targets=batch-lane-fewer-warmer-proofs
- 2026-09-27T18:20:26Z K72PV9EQVEB11663MWTTDJ9KQ2-m1e-c6925449 approve actor=human:Wido targets=batch-lane-fewer-warmer-proofs,pluggable-proof-runner
Integrity: sha256=e3acc1df4c4bcbe88c00d9de0cffb1a6c1e43e5d25bf02b61591640462f0d441
