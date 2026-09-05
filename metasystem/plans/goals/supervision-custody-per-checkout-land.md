# supervision-custody-per-checkout-land

- State: queued
- Risk: severity=3 novelty=2 exposure=3 accumulation=3 basis="The custody fix's landing member: the parent's three review rounds are spent with the fix rejected on a lockout; this member carries one corrected implementer round and one critic round; every machine running more than one checkout is exposed until it lands."
- Tier: 3
- Intent: Land the supervision custody fix of goal supervision-custody-per-checkout from preserve/scc-build3-r1 with the third critic's corrections (records/misc/supervision-custody-per-checkout-critique-cc3.md, SCC-31 to SCC-36): the recorded checkout path keeps the form the registry writers already use (the state root) so no checkout armed by the previous binary is locked out; the guard applies only to a live owner, a dead owner without a row is taken over as before; selection keys on rows that survive the registry contract's drops and compaction; the suite self-check rejects any main pid outside the scenario's own bed and the stop-hook scenario creates its own main. Wido's word of 2026-09-05 (R-80-m2): raise to five and land it; the configured maximum is three review rounds per goal, so the rounds are spent here as the engine's arc split. DONE means the invariant test covers an owner armed by the previous binary and passes, the supervise, registry, up and command packages are green, the supervision suite leaves the seat's supervision alive, a critic round finds nothing material, and the change lands; the parent concludes with it.
- Origin: main
- Next step: One implementer round from preserve/scc-build3-r1, one critic round (ids from SCC-41), land.
- OpenedAt: 2026-09-05T11:21:33Z
- Revision: 1
- Labels: robustness
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0

History:
- 2026-09-05T11:21:33Z 014KS18H3W5TBK62AT0Z7MTGV3-m2-5fcf08ab open actor=human:Wido targets=supervision-custody-per-checkout-land
Integrity: sha256=7303bcc6c7fb938b558b523c497a4f5b070b08ffe449e71d32ff683c0a8b1f67
