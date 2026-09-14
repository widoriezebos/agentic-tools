# suite-progress-bed-runs-on-trunk

- State: queued
- Risk: severity=2 novelty=2 exposure=2 accumulation=1 basis="severity 2: a deep section stays red and hides suite-progress regressions; novelty 2: the cause is not yet known; exposure 2: every run of the suite-progress deep section; accumulation 1: one bed"
- Tier: 2
- Intent: scripts/agents/suite-progress-fixtures.sh exits 64 within about five seconds when run directly on unmodified trunk (checked by m1b on 2026-09-15 at ded41d16, code-identical to 68682486). It prints only its suite-cost banner and a PROOF-RESULT line before the exit. In proof runs the section is red or invalid, and it was already invalid at a20afc40 on 2026-09-14. Exit 64 is the code fixture-budget.sh returns for an invalid private child invocation, so the bed may be calling the child detection with arguments it does not accept. DONE: the cause is named with its evidence; the bed passes on trunk under the harness; a leg pins the cause.
- Origin: human
- Next step: Diagnose first: run the bed directly with bash -x from the metasystem directory, find the command that returns 64, and name it. Then fix and prove. Free for the next seat by sequence.
- OpenedAt: 2026-09-14T22:36:16Z
- Revision: 1
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-14T22:36:16Z TBNWKG59397D989ZF3NN8D64ZM-m1e-c6925449 open actor=human:Wido targets=suite-progress-bed-runs-on-trunk
Integrity: sha256=f563f193b1b1b0ac323503d8320d51a8da2d4b22d5a4491b65ec6b1186d8cd65
