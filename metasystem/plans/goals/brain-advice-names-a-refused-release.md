# brain-advice-names-a-refused-release

- State: queued
- Risk: severity=1 novelty=1 exposure=2 accumulation=1 basis="severity 1: wrong advice text, no work lost; novelty 1: one branch on the fence in two messages; exposure 2: brain declaration and boot on every machine; accumulation 1: one wrong sentence per stopped claim"
- Tier: 2
- Intent: What: When a claim has been stopped for running over its budget, the system's advice tells you to run goal resume, not goal release. Why: Two messages tell you to run goal release on a stopped claim, and goal release then refuses. The advice sends you to a locked door. Pros: A thirty-minute fix that removes a confusing dead end. Cons: None worth noting.
- Origin: main
- Next step: Next: At the two places that give this advice (cmd/metasystem/brain.go and brain_boot.go), check whether the claim is stopped (IsFencedClaim): advise goal resume when it is, goal release otherwise, in the two-line message style (line 1 the reason, line 2 the command). Done when: two tests show each case prints the right command.
- OpenedAt: 2026-09-10T07:23:47Z
- Revision: 3
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-10T07:23:47Z A5RMYFZJ3TWEQHSJMTMK71VB5J-m1-c6925449 open actor=human:Wido targets=brain-advice-names-a-refused-release
- 2026-09-13T08:17:20Z 9F34MDHVMV62PWRTEV2XR1ZEGS-m1-c6925449 edit actor=human:Wido targets=brain-advice-names-a-refused-release
- 2026-09-30T18:47:38Z VGJEZPW5TXG165TYFYHVJQ8SKK-m1e-b6a4eb0a edit actor=human:wido targets=brain-advice-names-a-refused-release
Integrity: sha256=8cfb528d46955a392f562cc94c0c91087b5bff7caaf7038e3ff93347ca69d62f
