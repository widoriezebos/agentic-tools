# units-land-in-batches-under-one-proof

- State: queued
- Risk: severity=3 novelty=3 exposure=3 accumulation=2 basis="Wido 2026-09-16 15:35 and 15:50 CEST: create batches of release candidates, make sure they are all green already with cheap tests, then batch them into a single release tested with the expensive test, with chances of green as high as they can be, so the price is paid once for many landings; design it like that and get it implemented with highest priority because if this is the biggest bottleneck we better solve it first. Precedents: U2c plus U2d plus U2e proved as one stack (proof-mu41vx96, 53 of 53); the U3b-1a plus 1b stack. Seat m1e landing lanes do this by hand today. Every landing pays a 20 to 45 minute deep proof under the one testrun lock; on 2026-09-16 a trunk flake at a 2 s margin turned a 53-group proof red and the lane lost its slot."
- Tier: 3
- Intent: Units land in BATCHES under ONE proof. DONE: (1) a unit joins a batch only through a cheap gate whose five results (its package tests, its witnesses proved by mutation, an independent read, staticcheck and vet, the changed fixture-bed scenario run alone outside the lock under R-111) are recorded on its queue entry, and a missing result refuses the join; (2) the batch is one candidate tree assembled in join order, and a conflicting join is refused at join time naming the files; (3) batch size is bounded by the union of the selected groups, all groups being the deep-proof ceiling, and a batch proof starts when the lock is free and at least two gated units wait or the maximum wait has elapsed; (4) a red batch is never retried: the failing group's files name the owning unit, that unit is ejected with the failure attached and the rest are proved as a new tree, and a red that names no unit stops the batch as a trunk flake until the flake is fixed at the cause; (5) a batch proof starts only in a quiet window; on green the units land in join order with their receipt rows and the delivery check reuses the green groups by execution identity; fixtures prove each.
- Origin: human
- Next step: Design r1 by a headless Claude Fable delegate (seat m1e brief S5/blb-design-prompt.md, launched by S5/fable-design-launch.sh on a worktree at origin/main). It builds on proof-admission-fits-a-seat-proving-several-units design r3 (plans/proof-admission-fits-a-seat-proving-several-units-design.md: candidate tree digest, candidate-scoped retry, two-lens verdict) and names which of its units are prerequisites. Then a Codex critique, a Codex sol build in units of at most 300 lines, Opus reads, landing through the lane. Design ceilings: 150 tool calls, 5000 words.
- OpenedAt: 2026-09-16T13:45:00Z
- Revision: 1
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0

History:
- 2026-09-16T13:45:00Z 2V898K1J58M679D106Y0DXP2TV-m1e-c6925449 open actor=human:Wido targets=units-land-in-batches-under-one-proof
Integrity: sha256=9a14e00e04f1b5c16ed8ae95b1568fd8bca8f2a13c31ea9eecc8f8264fbc2672
