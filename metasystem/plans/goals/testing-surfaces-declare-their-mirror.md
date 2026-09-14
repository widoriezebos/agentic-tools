# testing-surfaces-declare-their-mirror

- State: approved
- Risk: severity=2 novelty=2 exposure=2 accumulation=2 basis="severity 2: a narrowed proof selection silently under-tests; novelty 2: a new contract feature; exposure 2: every change to a mirrored surface; accumulation 2: grows with each mirrored surface"
- Tier: 2
- Intent: Some testing surfaces must select exactly the fallback surface's proof breadth, and today that is enforced only by a test that fails after the omission is made. On 2026-09-14 a new proof group was added to the residual surface but not to context-budget, and TestContextTestingContractSelectsProof caught it only in a full package run. DONE: a surface that must mirror another declares it in testing.json and inherits the other's lists, or the contract validator names the missing group and surface at load time, so the omission cannot be written.
- Origin: human
- Next step: Add a mirror declaration to the testing contract with validation at load, migrate context-budget to it, and prove the 2026-09-14 omission is refused at load rather than at test time.
- OpenedAt: 2026-09-14T16:16:33Z
- Revision: 2
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-14T18:54:48Z revision=2 opid=G0Y3VRH6FBYNH4ZCPR1Z389N53-m1e-c6925449 authority=proven digest=0a7e950be00b236cb18c2b805390e3f8534c1028b3d822f2fca6e77b2e032f88

History:
- 2026-09-14T16:16:33Z D4SS1C2JRH2TPCF6FCHX7HQ213-m1e-c6925449 open actor=human:Wido targets=testing-surfaces-declare-their-mirror
- 2026-09-14T18:54:48Z G0Y3VRH6FBYNH4ZCPR1Z389N53-m1e-c6925449 approve actor=human:Wido targets=testing-surfaces-declare-their-mirror
Integrity: sha256=0802b1ef09296a6f80efe20ef8993b3135375637abcc06b87059ef7035086aca
