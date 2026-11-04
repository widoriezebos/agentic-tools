# a-green-unit-is-always-committable

- State: approved
- Risk: severity=2 novelty=1 exposure=3 accumulation=1 basis="finding 497 (2026-11-02): a review circled on an engine-written receipts.log the build left behind and the unit had to be committed by hand; every unit build on every seat passes freezeBuildOutcome; the restore is bounded to two shapes with one generic fail-safe (decision 154) and the kept bytes make it reversible."
- Tier: 2
- Intent: A green unit is always committable: at the end of a build the engine puts back the engine-written excluded paths the builder touched (receipts.log, the reads records and the other workspace exclusions), in two common shapes with one generic stop by name for everything else, keeps the builder bytes beside the round, records what it put back and tells the builder up front not to touch those paths (D10 of plans/designs/a-check-guards-its-own-inputs.md, the split follow-up of a-check-guards-its-own-inputs).
- Origin: human
- Next step: Build U1c2a (the restore) then U1c2b (--last; the record and the words) from the page D10 and the FINAL brief section U1c2 of agentic-tools-evidence/lane-test-20261009/briefs/a-check-guards-its-own-inputs-U1-brief.md (Astra rounds closed, decision 156); work review between parts; one hand-in.
- OpenedAt: 2026-11-04T01:25:27Z
- Revision: 2
- Budget: elapsedLimit=1d attemptLimit=6 reservedJobMinutesLimit=600 activeJobLimit=1 reviewRoundLimit=6
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-11-04T01:25:38Z revision=2 opid=5FCN07DPVQ0BE3F50JVBNJ0JD4-m1e-262937c7 authority=proven digest=42bd31aee07335a959c19548a9663fed6076a42f22641bb75ae4dc1225df0cb1 episode=2

History:
- 2026-11-04T01:25:27Z 54E1GGMFVR5HPB9MG4SGJ543D9-m1e-262937c7 open actor=human:Wido targets=a-green-unit-is-always-committable
- 2026-11-04T01:25:38Z 5FCN07DPVQ0BE3F50JVBNJ0JD4-m1e-262937c7 approve actor=human:Wido targets=a-green-unit-is-always-committable
Integrity: sha256=e69b874f8f2698bf89ccc20dc62872ddb6f9303eb726b8965db9cd3e6744f682
