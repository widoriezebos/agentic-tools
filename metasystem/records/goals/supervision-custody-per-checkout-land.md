# supervision-custody-per-checkout-land

- State: done
- Risk: severity=3 novelty=2 exposure=3 accumulation=3 basis="The custody fix's landing member: the parent's three review rounds are spent with the fix rejected on a lockout; this member carries one corrected implementer round and one critic round; every machine running more than one checkout is exposed until it lands."
- Tier: 3
- Intent: Land the supervision custody fix of goal supervision-custody-per-checkout from preserve/scc-build3-r1 with the third critic's corrections (records/misc/supervision-custody-per-checkout-critique-cc3.md, SCC-31 to SCC-36): the recorded checkout path keeps the form the registry writers already use (the state root) so no checkout armed by the previous binary is locked out; the guard applies only to a live owner, a dead owner without a row is taken over as before; selection keys on rows that survive the registry contract's drops and compaction; the suite self-check rejects any main pid outside the scenario's own bed and the stop-hook scenario creates its own main. Wido's word of 2026-09-05 (R-80-m2): raise to five and land it; the configured maximum is three review rounds per goal, so the rounds are spent here as the engine's arc split. DONE means the invariant test covers an owner armed by the previous binary and passes, the supervise, registry, up and command packages are green, the supervision suite leaves the seat's supervision alive, a critic round finds nothing material, and the change lands; the parent concludes with it.
- Origin: main
- Next step: One implementer round from preserve/scc-build3-r1, one critic round (ids from SCC-41), land.
- Concluded: Landed e516ad5d after four implementer rounds and three critic rounds on this member: selection by the recorded state-root path from the rows production writes, the foreign-tag veto unconditional, the recorded-path guard on live owners only, compaction keeping active publications, the fixture self-check auditing every bring-up. Proof on this checkout: rebuilt, re-armed, metasystem up twice accepted; registry, supervise, up and command packages green; the supervision suite leaves the seat's supervision alive. Residue scheduled: goal:supervision-fixture-self-check-announcement-scan (SCC-61, risk accepted by Wido under R-81-m2), goal:registry-design-names-selection-rows.
- OpenedAt: 2026-09-05T11:21:33Z
- Revision: 6
- Labels: robustness
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-05T11:21:38Z revision=2 opid=SWSWXRA3CTKTN90GS7K33QSFS4-m2-5fcf08ab authority=relayed digest=9cadb757ab8f46e91e5b74da65a8db9812d08c1cd9d55139fcb78aba82a4833e reviewBy=2026-09-06
- Sliced: machine=m2 lineage=main-1788441779-14484-82d6ed revision=3 at=2026-09-05T11:22:47Z
- AcceptedRisk: finding=SCC-61 chain=scp-build1-cc3 by=Wido opid=SN855TFP34HK8NSW8S2QRN0VE8-m2-5fcf08ab

History:
- 2026-09-05T11:21:33Z 014KS18H3W5TBK62AT0Z7MTGV3-m2-5fcf08ab open actor=human:Wido targets=supervision-custody-per-checkout-land
- 2026-09-05T11:21:38Z SWSWXRA3CTKTN90GS7K33QSFS4-m2-5fcf08ab approve actor=human:Wido targets=supervision-custody-per-checkout-land authorityOutcome=TEMPORARY_HUMAN_WORD authorityReviewBy=2026-09-06 authorityRuling=R-32-m1 temporaryHumanWord="Yes, raise to five and land it (Recommended)"
- 2026-09-05T11:21:44Z JR1WC02HCCMNK39WWVC1NP4G4Y-m2-5fcf08ab claim actor=m2+main-1788441779-14484-82d6ed targets=supervision-custody-per-checkout-land
- 2026-09-05T11:22:47Z D2SC3MZWSMG9A19J0XJAH3ZJ2V-m2-5fcf08ab slice-start actor=m2+main-1788441779-14484-82d6ed targets=supervision-custody-per-checkout-land
- 2026-09-05T14:36:33Z SN855TFP34HK8NSW8S2QRN0VE8-m2-5fcf08ab accept-risk actor=human:Wido targets=supervision-custody-per-checkout-land reason=R-81-m2: the finding is in the fixture self-check's announcement scan, not the custody code; scheduled as goal supervision-fixture-self-check-announcement-scan.
- 2026-09-05T15:13:53Z K1068KB8Q1BGH1B175WFR5DMB7-m2-5fcf08ab done actor=m2+main-1788441779-14484-82d6ed targets=supervision-custody-per-checkout-land
Integrity: sha256=129efed0eff3d8da0071d3fa30a5e2a49034ded70eb7720e326f84a73ccd33eb
