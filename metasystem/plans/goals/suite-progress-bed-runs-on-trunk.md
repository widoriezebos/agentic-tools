# suite-progress-bed-runs-on-trunk

- State: approved
- Priority: 1
- Sequence: 59
- Risk: severity=2 novelty=2 exposure=2 accumulation=1 basis="severity 2: a deep section stays red and hides suite-progress regressions; novelty 2: the cause is not yet known; exposure 2: every run of the suite-progress deep section; accumulation 1: one bed"
- Tier: 2
- Intent: scripts/agents/suite-progress-fixtures.sh exits 64 within about five seconds when run directly on unmodified trunk (checked by m1b on 2026-09-15 at ded41d16, code-identical to 68682486). It prints only its suite-cost banner and a PROOF-RESULT line before the exit. In proof runs the section is red or invalid, and it was already invalid at a20afc40 on 2026-09-14. Exit 64 is the code fixture-budget.sh returns for an invalid private child invocation, so the bed may be calling the child detection with arguments it does not accept. DONE: the cause is named with its evidence; the bed passes on trunk under the harness; a leg pins the cause.
- Origin: human
- Next step: Diagnose first: run the bed directly with bash -x from the metasystem directory, find the command that returns 64, and name it. Then fix and prove. Free for the next seat by sequence.
- OpenedAt: 2026-09-14T22:36:16Z
- Revision: 3
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-14T22:36:22Z revision=2 opid=3RF2B4M0QHEVCKYK067QR8XMWK-m1e-c6925449 authority=proven digest=19510596338f81617da931c28e62e137d302eeed08061541c02b9b91964571e6

History:
- 2026-09-14T22:36:16Z TBNWKG59397D989ZF3NN8D64ZM-m1e-c6925449 open actor=human:Wido targets=suite-progress-bed-runs-on-trunk
- 2026-09-14T22:36:22Z 3RF2B4M0QHEVCKYK067QR8XMWK-m1e-c6925449 approve actor=human:Wido targets=suite-progress-bed-runs-on-trunk
- 2026-09-14T22:36:28Z M59P8F6P05HDB6866B151S0PXC-m1e-c6925449 set-priority actor=human:Wido targets=suite-progress-bed-runs-on-trunk reason=priority-order subject=suite-progress-bed-runs-on-trunk from=unranked to=1:59 requested-sequence=append
Integrity: sha256=07ddf01e5fc1a017e360a56444aba1fb18c4518caba54755c78dfda48d861bc2
