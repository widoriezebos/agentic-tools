# adopted-validation-expects-a-template-only-section

- State: approved
- Risk: severity=1 novelty=1 exposure=1 accumulation=1 basis="severity 1: one missing name in a section list; novelty 1: the template-only list already exists and the fix adds the section to it; exposure 1: one selector list; accumulation 1: none"
- Tier: 1
- Intent: An adopted copy's validation does not expect a template-only section. Since 612b1e57 (2026-09-11 04:06Z) the selector lists the new section fixture-bed-scenarios-fixtures, the validator runs it only in template mode (validate-metasystem.sh line 2912), but the selector's template-only case (validate-section-selector.sh line 80: witness-gate-fixtures, suite-progress-fixtures, land-fixtures, adoption-fixtures) does not name it, so a filled or copied target's nested validation ends 'VALIDATION RUN INVALID: selector sections with NO recorded result: fixture-bed-scenarios-fixtures; suite progress structure is incomplete: 0 starts and 0 ends; expected 1 of each'. TestAdoptionComparisonSelectedScenarios fails deterministically on untouched main d449741e (161 s, reproduced by m1e), so section/adoption-fixtures is red for every deep landing on the fleet. DONE means the section is declared template-only wherever the older four are (the selector case and suite-progress-fixtures.sh's list if it has the same role), the adoption comparison test passes on main, and the adopt bed is green.
- Origin: main
- Next step: INTENT: adopted targets stop expecting the template-only harness section. CONSTRAINTS: follow the existing template-only pattern exactly (the case at validate-section-selector.sh:80 and, if it serves the same purpose, the list at suite-progress-fixtures.sh:137); no other section moves; prove with METASYSTEM_ADOPTION_COMPARISON=1 go test -run TestAdoptionComparisonSelectedScenarios ./cmd/metasystem green and the adopt bed green. FREEDOMS: none needed. Tier 1: implement (Sol, MECHANICAL), no critique (R-54-m1), tier-1 direct-fix landing with a standard-mode receipt. Blocks m1e's engine landing of testing-contract-owns-record-paths and every deep landing on the fleet.
- OpenedAt: 2026-09-11T06:36:53Z
- Revision: 2
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=360 activeJobLimit=1 reviewRoundLimit=0
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-11T06:37:23Z revision=2 opid=02S8RJF1G1PWDHNAPKSZD1CSD5-m1-c6925449 authority=proven digest=10c960462d3739beaaf15aaf322ad3b5db192f1219260306400fbd5d7fa3dbbb

History:
- 2026-09-11T06:36:53Z GHM2C1A2WW90MT2KSV9EA2RK6B-m1e-892cdaec open actor=m1e+main-1789030447-51011-5722fc targets=adopted-validation-expects-a-template-only-section
- 2026-09-11T06:37:23Z 02S8RJF1G1PWDHNAPKSZD1CSD5-m1-c6925449 approve actor=human:Wido targets=adopted-validation-expects-a-template-only-section
Integrity: sha256=25f9c6a0e3698a99c117c6916e8b6b10fbef838f414ac90f08c678e9d054d881
