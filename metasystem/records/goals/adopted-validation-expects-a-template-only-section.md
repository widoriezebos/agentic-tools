# adopted-validation-expects-a-template-only-section

- State: done
- Risk: severity=1 novelty=1 exposure=1 accumulation=1 basis="severity 1: one missing name in a section list; novelty 1: the template-only list already exists and the fix adds the section to it; exposure 1: one selector list; accumulation 1: none"
- Tier: 1
- Intent: An adopted copy's validation does not expect a template-only section. Since 612b1e57 (2026-09-11 04:06Z) the selector lists the new section fixture-bed-scenarios-fixtures, the validator runs it only in template mode (validate-metasystem.sh line 2912), but the selector's template-only case (validate-section-selector.sh line 80: witness-gate-fixtures, suite-progress-fixtures, land-fixtures, adoption-fixtures) does not name it, so a filled or copied target's nested validation ends 'VALIDATION RUN INVALID: selector sections with NO recorded result: fixture-bed-scenarios-fixtures; suite progress structure is incomplete: 0 starts and 0 ends; expected 1 of each'. TestAdoptionComparisonSelectedScenarios fails deterministically on untouched main d449741e (161 s, reproduced by m1e), so section/adoption-fixtures is red for every deep landing on the fleet. DONE means the section is declared template-only wherever the older four are (the selector case and suite-progress-fixtures.sh's list if it has the same role), the adoption comparison test passes on main, and the adopt bed is green.
- Origin: main
- Next step: INTENT: adopted targets stop expecting the template-only harness section. CONSTRAINTS: follow the existing template-only pattern exactly (the case at validate-section-selector.sh:80 and, if it serves the same purpose, the list at suite-progress-fixtures.sh:137); no other section moves; prove with METASYSTEM_ADOPTION_COMPARISON=1 go test -run TestAdoptionComparisonSelectedScenarios ./cmd/metasystem green and the adopt bed green. FREEDOMS: none needed. Tier 1: implement (Sol, MECHANICAL), no critique (R-54-m1), tier-1 direct-fix landing with a standard-mode receipt. Blocks m1e's engine landing of testing-contract-owns-record-paths and every deep landing on the fleet.
- Concluded: Landed at 986d5862 (2026-09-11 08:20Z): the selector's template-only case and the suite-progress fixture's inactive list name fixture-bed-scenarios-fixtures, the enumeration fixture's adopted count is 39, and enumerate-suite-fixtures.sh is owned by the gate-plumbing surface. Proven by the adoption comparison test green on the candidate (333 s) after failing on untouched main d449741e (161 s), and by the landing's own deep receipt (42 of 42 groups, attempt on tree ceb6fe01). Two chains: implementer-df2547e5fd0cba72b50c09ec (closed before its receipt, so it could take no round) and implementer-30366b2d88a5d46ccf829363 (the carried chain that landed). Tier 1, no critique. Note for the fleet: the tier-1 direct-fix lane refused the landing for unclassified paths (path-classes.txt names none of the four files); the chain lane accepted the closed implementer chain.
- OpenedAt: 2026-09-11T06:36:53Z
- Revision: 7
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=360 activeJobLimit=1 reviewRoundLimit=0
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-11T06:37:23Z revision=2 opid=02S8RJF1G1PWDHNAPKSZD1CSD5-m1-c6925449 authority=proven digest=10c960462d3739beaaf15aaf322ad3b5db192f1219260306400fbd5d7fa3dbbb
- Sliced: machine=m1e lineage=main-1789030447-51011-5722fc revision=3 at=2026-09-11T06:37:53Z

History:
- 2026-09-11T06:36:53Z GHM2C1A2WW90MT2KSV9EA2RK6B-m1e-892cdaec open actor=m1e+main-1789030447-51011-5722fc targets=adopted-validation-expects-a-template-only-section
- 2026-09-11T06:37:23Z 02S8RJF1G1PWDHNAPKSZD1CSD5-m1-c6925449 approve actor=human:Wido targets=adopted-validation-expects-a-template-only-section
- 2026-09-11T06:37:27Z FQTAVD7RMY1TY77YREXFXKY341-m1e-892cdaec claim actor=m1e+main-1789030447-51011-5722fc targets=adopted-validation-expects-a-template-only-section
- 2026-09-11T06:37:53Z ER05CZ2FP0X3BD8HVGYDG09EHC-m1e-892cdaec slice-start actor=m1e+main-1789030447-51011-5722fc targets=adopted-validation-expects-a-template-only-section
- 2026-09-11T07:11:04Z E9E22760VF91VHTVA1CEE9BAJY-m1e-892cdaec release actor=m1e+main-1789030447-51011-5722fc targets=adopted-validation-expects-a-template-only-section
- 2026-09-11T07:11:07Z QS2FAA60NYB7P5NC16W5PGEDKY-m1e-892cdaec claim actor=m1e+main-1789030447-51011-5722fc targets=adopted-validation-expects-a-template-only-section
- 2026-09-11T08:20:49Z NFQ2K5GGWQCW5EV3JY4WFVSXW1-m1e-892cdaec done actor=m1e+main-1789030447-51011-5722fc targets=adopted-validation-expects-a-template-only-section
Integrity: sha256=cb6394d2a99475160a1adb7e3cdd412823caa10461b45712a2e3256534d3b8eb
