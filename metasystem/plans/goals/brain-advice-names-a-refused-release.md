# brain-advice-names-a-refused-release

- State: queued
- Risk: severity=1 novelty=1 exposure=2 accumulation=1 basis="severity 1: wrong advice text, no work lost; novelty 1: one branch on the fence in two messages; exposure 2: brain declaration and boot on every machine; accumulation 1: one wrong sentence per stopped claim"
- Tier: 2
- Intent: The brain declaration obstacle in cmd/metasystem/brain.go (around lines 111-120) and the brain boot line in cmd/metasystem/brain_boot.go (around line 405) tell the human to run goal release on a claim held here. When that claim is breach-stopped, release is refused by clearClaimBinding ('only goal resume may clear its launch fence'), so the advice sends the human at a locked door. Predates breach-stop-wedges-seat; found by its closing critic on 2026-09-10 as BSW-11. DONE means both messages say goal resume (a human act with the standing budget) when the held claim carries a StopFence, and goal release otherwise, with a fixture for each branch.
- Origin: main
- Next step: Appetite: 30 minutes. Branch on GoalFile.IsFencedClaim (landed by breach-stop-wedges-seat) at both sites; two fixtures. || Umbrella brain-checkout-refusal-guidance (2026-09-13 backlog consolidation): this goal leads the fix for the members below. || Absorbed 2026-09-13 from goal brain-checkout-seat-is-told-why-claim-refuses: Seat guidance says stop and report an authority refusal instead of refetching. The sentence (plans/backlog-ordered-by-priority-design.md line 360) is absent from AGENTS.md and scripts/agents/roles/steward-continuation.md (grep 2026-09-13); the 1400-word audit cap sits at 1398 so a trim comes first (Wido 2026-09-09 all
- OpenedAt: 2026-09-10T07:23:47Z
- Revision: 2
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-10T07:23:47Z A5RMYFZJ3TWEQHSJMTMK71VB5J-m1-c6925449 open actor=human:Wido targets=brain-advice-names-a-refused-release
- 2026-09-13T08:17:20Z 9F34MDHVMV62PWRTEV2XR1ZEGS-m1-c6925449 edit actor=human:Wido targets=brain-advice-names-a-refused-release
Integrity: sha256=223ecfe30dde410daec602808d8d20b765cf0cfffa08793f2895dd1a8cf5ed2b
