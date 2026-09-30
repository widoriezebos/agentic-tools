# landing-proof-records-are-written

- State: queued
- Risk: severity=2 novelty=1 exposure=2 accumulation=2 basis="Landing evidence for every seat; a wrong record misleads the next landing; the gap is dormant today so nothing relies on it yet."
- Tier: 2
- Intent: What: Every landing writes its proof record (green, red, canary, fix), so the next landing reads a true history. Why: The record format exists, but nothing in production writes it: the landing route passes no recorder, and the "landing was green" call is used only by tests. The history is always empty. Pros: Later landings can trust and reuse past results. Cons: It touches the landing path that is being redesigned now, so it belongs in that work or right after it.
- Origin: human
- Next step: Next: Decide whether this folds into landing-lane-runtime-redesign. If not, after that lands: design the writer on the landing path (landpath) with one Astra round, then build. Done when: after a real landing its green record exists and the next preparation reads it.
- OpenedAt: 2026-09-28T08:23:51Z
- Revision: 4
- Labels: landing
- BudgetExceptions: 0

History:
- 2026-09-28T08:23:51Z WMPQJXTF9C48XF27VZDE5QXMW1-m1e-c6925449 open actor=human:Wido targets=landing-proof-records-are-written
- 2026-09-28T08:24:24Z 16GEZK3EHFYPX730B5TWWE3X4G-m1e-c6925449 approve actor=human:Wido targets=landing-proof-records-are-written,steward-sees-stuck-capacity
- 2026-09-30T19:04:33Z 4RM8KYH1BE9A3P4Q2W4G5HV6BM-m1e-b6a4eb0a unapprove actor=human:wido targets=landing-proof-records-are-written reason=Held: it would change the landing lane while the lane is being redesigned; revisit under landing-lane-runtime-redesign (backlog sync 2026-09-30)
- 2026-09-30T19:04:45Z 936VPSR8WAMD2PHKZ77FMYGJRE-m1e-b6a4eb0a edit actor=human:wido targets=landing-proof-records-are-written
Integrity: sha256=0d665f27f6143e8343cb1ed4e7c127f2b6e15f6fdbcff1ca283213cbd504ef7d
