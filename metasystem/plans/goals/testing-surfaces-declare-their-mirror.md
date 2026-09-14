# testing-surfaces-declare-their-mirror

- State: queued
- Risk: severity=2 novelty=2 exposure=2 accumulation=2 basis="severity 2: a narrowed proof selection silently under-tests; novelty 2: a new contract feature; exposure 2: every change to a mirrored surface; accumulation 2: grows with each mirrored surface"
- Tier: 2
- Intent: Some testing surfaces must select exactly the fallback surface's proof breadth, and today that is enforced only by a test that fails after the omission is made. On 2026-09-14 a new proof group was added to the residual surface but not to context-budget, and TestContextTestingContractSelectsProof caught it only in a full package run. DONE: a surface that must mirror another declares it in testing.json and inherits the other's lists, or the contract validator names the missing group and surface at load time, so the omission cannot be written.
- Origin: human
- Next step: Add a mirror declaration to the testing contract with validation at load, migrate context-budget to it, and prove the 2026-09-14 omission is refused at load rather than at test time.
- OpenedAt: 2026-09-14T16:16:33Z
- Revision: 1
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-14T16:16:33Z D4SS1C2JRH2TPCF6FCHX7HQ213-m1e-c6925449 open actor=human:Wido targets=testing-surfaces-declare-their-mirror
Integrity: sha256=f6a1ea0c5ed7f268573eeb434dee6edf87e21a5e9d128f05410c3d0276a40b00
