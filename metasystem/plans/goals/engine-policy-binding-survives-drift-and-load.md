# engine-policy-binding-survives-drift-and-load

- State: queued
- Risk: severity=2 novelty=2 exposure=3 accumulation=2 basis="severity 2: refused runs and manual re-arms, no wrong landing; novelty 2: changes the policy-binding judgment and its timing; exposure 3: every engine run on every seat; accumulation 2: worse with load and landing rate"
- Tier: 2
- Intent: Seats lose engine runs to TEST_POLICY_ENGINE_REQUIRED refusals unrelated to the candidate: 'witness resolution exceeded the configured 20-second bound' under load and after ledger-only drift; 'retained trusted-base engine returned a mismatched policy decision' when the enrolled checkout, engine and tree sit on different tips; every test plan refused after a seat lands engine code until a manual re-arm; and a dirty memory/receipts.log blocking the fast-forward. On 2026-09-15 m1e, m1b and m1c each lost runs and re-arm cycles to these. DONE: ledger-only drift never requires a re-arm or fails policy binding; witness resolution does not time out at a load average of 30 on the test machine; test plan and test run re-arm themselves after a landed engine change; each refusal names which cause it is; a fixture proves each.
- Origin: human
- Next step: Design, tier 2: inventory the TEST_POLICY_ENGINE_REQUIRED paths (witness resolution bound, trusted-base decision, re-arm on a landed engine, operational logs), decide the drift rule and the bound under load, then build in units.
- OpenedAt: 2026-09-15T05:55:22Z
- Revision: 1
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-15T05:55:22Z 8EAG3N9FPAMVM02CM6W74JD46H-m1e-c6925449 open actor=human:Wido targets=engine-policy-binding-survives-drift-and-load
Integrity: sha256=a85ebefa7da970774e548e9e3ca039b3e2492292a6096a800182ab40d095529a
