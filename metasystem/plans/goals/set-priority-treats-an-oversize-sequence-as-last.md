# set-priority-treats-an-oversize-sequence-as-last

- State: queued
- Priority: 3
- Sequence: 42
- Risk: severity=1 novelty=1 exposure=2 accumulation=1 basis="severity 1: a refused reorder, no wrong ledger state; novelty 1: clamp a bound that already has an append form; exposure 2: every human or seat reorder that names a position; accumulation 1: each refusal is visible and retried"
- Tier: 1
- Intent: set-priority refuses a sequence beyond the end of the destination queue ('sequence 90 is outside the current destination range 1..62 for priority 1'). Sequences are dense ranks that every move renumbers (internal/goal/order.go, the position > maximum check where maximum is the queue length plus one), so the refusal only prevents a gap that renumbering would close anyway; omitting --sequence already appends. On 2026-09-15 it failed a batch of priority moves in Wido's reorder of the efficiency list. DONE: set-priority treats a sequence past the end of the destination queue as 'last', lands the goal at the end, and reports the position it used; a sequence below 1 is still refused; a test proves an oversize sequence lands last and the queue stays dense 1..N.
- Origin: human
- Next step: Build, tier 1: in internal/goal/order.go clamp a position above the destination maximum to the end instead of refusing, report the effective position in the result, keep the below-1 refusal, and extend the order tests.
- OpenedAt: 2026-09-15T06:06:03Z
- Revision: 4
- BudgetExceptions: 0

History:
- 2026-09-15T06:06:03Z 56ZG34WXN2DVPR59QM322CRG8B-m1e-c6925449 open actor=human:Wido targets=set-priority-treats-an-oversize-sequence-as-last
- 2026-09-15T06:06:09Z 4R1Q8B5DY6JN28KCAP8WQ3HW6V-m1e-c6925449 approve actor=human:Wido targets=set-priority-treats-an-oversize-sequence-as-last
- 2026-09-15T06:06:15Z STNM6WK6WE657ZSDC7B9N36KE8-m1e-c6925449 set-priority actor=human:Wido targets=set-priority-treats-an-oversize-sequence-as-last reason=priority-order subject=set-priority-treats-an-oversize-sequence-as-last from=unranked to=3:42 requested-sequence=append
- 2026-09-16T20:35:19Z 05KYTN17SVSY9CC3DCR8JXE1PV-m1e-c6925449 unapprove actor=human:Wido targets=set-priority-treats-an-oversize-sequence-as-last reason=Wido 2026-09-16 22:40 CEST: clean house on priority 1 first, then a few human convenience features; every priority 2+ goal is unapproved until then
Integrity: sha256=f72210894d82749c21acd1098fb2ac384265def022e1c9c4c40f314d6ffa24e0
