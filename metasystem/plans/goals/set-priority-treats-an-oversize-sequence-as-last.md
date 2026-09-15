# set-priority-treats-an-oversize-sequence-as-last

- State: approved
- Risk: severity=1 novelty=1 exposure=2 accumulation=1 basis="severity 1: a refused reorder, no wrong ledger state; novelty 1: clamp a bound that already has an append form; exposure 2: every human or seat reorder that names a position; accumulation 1: each refusal is visible and retried"
- Tier: 1
- Intent: set-priority refuses a sequence beyond the end of the destination queue ('sequence 90 is outside the current destination range 1..62 for priority 1'). Sequences are dense ranks that every move renumbers (internal/goal/order.go, the position > maximum check where maximum is the queue length plus one), so the refusal only prevents a gap that renumbering would close anyway; omitting --sequence already appends. On 2026-09-15 it failed a batch of priority moves in Wido's reorder of the efficiency list. DONE: set-priority treats a sequence past the end of the destination queue as 'last', lands the goal at the end, and reports the position it used; a sequence below 1 is still refused; a test proves an oversize sequence lands last and the queue stays dense 1..N.
- Origin: human
- Next step: Build, tier 1: in internal/goal/order.go clamp a position above the destination maximum to the end instead of refusing, report the effective position in the result, keep the below-1 refusal, and extend the order tests.
- OpenedAt: 2026-09-15T06:06:03Z
- Revision: 2
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=360 activeJobLimit=1 reviewRoundLimit=0
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-15T06:06:09Z revision=2 opid=4R1Q8B5DY6JN28KCAP8WQ3HW6V-m1e-c6925449 authority=proven digest=255226290d276d64ab74be365ff6bd0a7c5cec57627e31a4fe27a30d8f38b0ed

History:
- 2026-09-15T06:06:03Z 56ZG34WXN2DVPR59QM322CRG8B-m1e-c6925449 open actor=human:Wido targets=set-priority-treats-an-oversize-sequence-as-last
- 2026-09-15T06:06:09Z 4R1Q8B5DY6JN28KCAP8WQ3HW6V-m1e-c6925449 approve actor=human:Wido targets=set-priority-treats-an-oversize-sequence-as-last
Integrity: sha256=2e42986970187411b4b46f54bdfa01886d4c8532738cbfb5fefd94eae167a505
