# land-retry-rediscards-valid-receipt

- State: queued
- Risk: severity=1 novelty=1 exposure=2 accumulation=2 basis="severity 1: nothing breaks, time is burned; novelty 1: keep a receipt whose tree still matches; exposure 2: every landing retry on every seat; accumulation 2: it compounds on busy days, when each battery is slowest and retries are likeliest"
- Tier: 2
- Intent: scripts/agents/land.sh removes the stale test receipt before creating a new one, so any retry re-runs the full battery even when the identical tree already passed it. Seen 2026-09-07 on goal fixture-review-by-date-expired: two tier-1 landing attempts failed on plumbing (goal-item-not-held because the goal CLI pushes ledger commits without moving the local branch, then the commit comparison tripping on records/narrator-digest.log which the narrator appended to mid-battery), and each bounce re-proved a tree that had already passed green, about 35 minutes for a one-line change, until Wido stopped it and landed it himself. DONE means a retry reuses a receipt whose recorded tree still equals the candidate tree, and the fixtures prove both the reuse and the refusal when the tree moved
- Origin: main
- Next step: read the receipt handling in scripts/agents/land.sh and internal/landing/receipt.go; the receipt already records the tree it proved, so the reuse test is an equality check; add the land-fixtures.sh cases. Two adjacent defects seen in the same incident are separate goals if they do not fall out of this one: the goal CLI pushing ledger commits without moving the local branch, and the narrator appending to records/narrator-digest.log mid-battery
- OpenedAt: 2026-09-07T09:07:25Z
- Revision: 1
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-07T09:07:25Z Q2RCV8X0YE8RAP9GVS5GA63NZ2-m1b-c6925449 open actor=m1b+main-1788680071-18713-e76d5d targets=land-retry-rediscards-valid-receipt
Integrity: sha256=91e521983f053167fe60265a8fbe1c72b0007e6fc405de3d7b91553764349d2b
