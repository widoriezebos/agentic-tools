# a-stop-waits-for-the-unit-boundary

- State: queued
- Risk: severity=3 novelty=2 exposure=3 accumulation=2 basis="A wrong stop loses one round and is recoverable (severity 3); the mechanism reuses FindBreachStops and the existing admission checks (novelty 2); every goal's budget passes through it (exposure 3); two units, no new store beyond the terminal reason and the overrun fields (accumulation 2)"
- Tier: 3
- Intent: No automatic stop cuts a unit mid-work: an elapsed-budget breach waits for the unit's durable handoff, the breached unit still gets its own read and allowed corrections, and the overrun is recorded on the goal
- Origin: main
- Next step: U1a (no automatic elapsed stop cuts a unit: BreachStopEligible in FindBreachStops, DEFERRED route, prober seams incl. CompleteRevival) then U1b (grace on the runner's authorization through the three admission checks, elapsedOverrun from the goal clock); design plans/designs/a-stop-waits-for-the-unit-boundary.md accepted at the stop 2026-10-19; brief with inventory, Astra, Codex by hand, Opus read, hand gate, landing
- OpenedAt: 2026-10-19T09:44:21Z
- Revision: 1
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0

History:
- 2026-10-19T09:44:21Z E9KH14VBNRFY6DDSVYRYM5MFRK-m1e-c5a1e456 open actor=human:Wido targets=a-stop-waits-for-the-unit-boundary
Integrity: sha256=5ad64c48c073c734d45e292e56b85551d0249067d622bf60d2a82808570d6d40
