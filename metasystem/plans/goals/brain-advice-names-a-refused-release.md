# brain-advice-names-a-refused-release

- State: queued
- Risk: severity=1 novelty=1 exposure=2 accumulation=1 basis="severity 1: wrong advice text, no work lost; novelty 1: one branch on the fence in two messages; exposure 2: brain declaration and boot on every machine; accumulation 1: one wrong sentence per stopped claim"
- Tier: 2
- Intent: The brain declaration obstacle in cmd/metasystem/brain.go (around lines 111-120) and the brain boot line in cmd/metasystem/brain_boot.go (around line 405) tell the human to run goal release on a claim held here. When that claim is breach-stopped, release is refused by clearClaimBinding ('only goal resume may clear its launch fence'), so the advice sends the human at a locked door. Predates breach-stop-wedges-seat; found by its closing critic on 2026-09-10 as BSW-11. DONE means both messages say goal resume (a human act with the standing budget) when the held claim carries a StopFence, and goal release otherwise, with a fixture for each branch.
- Origin: main
- Next step: Appetite: 30 minutes. Branch on GoalFile.IsFencedClaim (landed by breach-stop-wedges-seat) at both sites; two fixtures.
- OpenedAt: 2026-09-10T07:23:47Z
- Revision: 1
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-10T07:23:47Z A5RMYFZJ3TWEQHSJMTMK71VB5J-m1-c6925449 open actor=human:Wido targets=brain-advice-names-a-refused-release
Integrity: sha256=08c752de991107327bdde111606e61272a53403604f822c989fed44de7c8f49a
