# registry-design-names-selection-rows

- State: approved
- Risk: severity=1 novelty=1 exposure=2 accumulation=2 basis="The registry design's REG-3 text no longer describes what the reduction keeps for owner selection; every reader of the design is exposed to a stale contract, and each further reduction change widens the gap."
- Tier: 2
- Intent: The custody landing (e516ad5d) made the registry reduction keep a published-owner projection from relaunched and launched rows, drop a row that names a second checkout for a known identity, and made compaction retain active production publications; docs/design/supervision-registry.md REG-3 still says everything not listed may be dropped, and the dropped conflicts are listed but no production caller reads the list (critic findings SCC-53 and SCC-63 of chains scp-build1-cc2 and scp-build1-cc3). DONE means REG-3 names the rows selection depends on and the drop rule, and a dropped conflict is logged by the reduction's caller so drop-and-log is real.
- Origin: main
- Next step: One doc round and one logging line; tier from the risk basis. Residue of goal supervision-custody-per-checkout.
- OpenedAt: 2026-09-05T15:09:55Z
- Revision: 2
- Labels: robustness
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-05T15:11:17Z revision=2 opid=46WB6BQ63V4QB797EJRQQNN34C-m2-5fcf08ab authority=relayed digest=82c2803f394766177b8d600a9b58804eb1096eec3c4d0460e766a3b882809287 reviewBy=2026-09-06

History:
- 2026-09-05T15:09:55Z 7231WAM1A3DZTBYH58G0GKFYBS-m2-5fcf08ab open actor=human:Wido targets=registry-design-names-selection-rows
- 2026-09-05T15:11:17Z 46WB6BQ63V4QB797EJRQQNN34C-m2-5fcf08ab approve actor=human:Wido targets=registry-design-names-selection-rows authorityOutcome=TEMPORARY_HUMAN_WORD authorityReviewBy=2026-09-06 authorityRuling=R-32-m1 temporaryHumanWord="Yes, accept and land now (Recommended)"
Integrity: sha256=033a6ff9b139d66c4a56d29336731823801d73451aeef4b9cbf3bdc1eb80ca70
