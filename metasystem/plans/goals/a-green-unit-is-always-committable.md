# a-green-unit-is-always-committable

- State: queued
- Risk: severity=2 novelty=1 exposure=3 accumulation=1 basis="finding 497 (2026-11-02): a review circled on an engine-written receipts.log the build left behind and the unit had to be committed by hand; every unit build on every seat passes freezeBuildOutcome; the restore is bounded to two shapes with one generic fail-safe (decision 154) and the kept bytes make it reversible."
- Tier: 2
- Intent: A green unit is always committable: at the end of a build the engine puts back the engine-written excluded paths the builder touched (receipts.log, the reads records and the other workspace exclusions), in two common shapes with one generic stop by name for everything else, keeps the builder bytes beside the round, records what it put back and tells the builder up front not to touch those paths (D10 of plans/designs/a-check-guards-its-own-inputs.md, the split follow-up of a-check-guards-its-own-inputs).
- Origin: human
- Next step: Build U1c2a (the restore) then U1c2b (--last; the record and the words) from the page D10 and the FINAL brief section U1c2 of agentic-tools-evidence/lane-test-20261009/briefs/a-check-guards-its-own-inputs-U1-brief.md (Astra rounds closed, decision 156); work review between parts; one hand-in.
- OpenedAt: 2026-11-04T01:25:27Z
- Revision: 1
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0

History:
- 2026-11-04T01:25:27Z 54E1GGMFVR5HPB9MG4SGJ543D9-m1e-262937c7 open actor=human:Wido targets=a-green-unit-is-always-committable
Integrity: sha256=f8392e7296899ef939623460224a9f4c8101f573801d2d35cc01ab4fd74198c5
