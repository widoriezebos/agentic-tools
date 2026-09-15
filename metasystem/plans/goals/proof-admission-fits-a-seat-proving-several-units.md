# proof-admission-fits-a-seat-proving-several-units

- State: queued
- Risk: severity=2 novelty=2 exposure=3 accumulation=2 basis="severity 2: blocked or misattributed proofs and manual workarounds, no wrong landing; novelty 2: changes attempt accounting and retry scoping; exposure 3: every seat that proves more than one unit; accumulation 2: grows with parallel units"
- Tier: 2
- Intent: A seat proving several units hits admission refusals unrelated to its candidates. Proof attempts are charged to the one goal the machine may claim, so on 2026-09-15 other units' proofs and controls spent coordinator-context-stays-under-budget's attempt limit and blocked unit C1b; a retry decision is refused as 'ambiguous across 2 failed attempts' when components' latest failures come from different candidates, even for a single section; and a breach-stopped goal refuses set-budget until resumed, then refuses the run until a release and re-claim (m1c, about 40 minutes). DONE: a proof attempt is charged to the goal its candidate belongs to without switching the machine's claim; a retry decision is scoped to the candidate tree so another candidate's failures never make it ambiguous; set-budget on a breach-stopped goal works in one step and the claim's reservation follows; fixtures prove each.
- Origin: human
- Next step: Design, tier 2: decide how a proof attempt names its goal independently of the machine claim, scope retry decisions to the candidate tree, and fix the breach-stop set-budget ordering; coordinate the reservation part with stop-capability-follows-the-lease-epoch.
- OpenedAt: 2026-09-15T05:55:38Z
- Revision: 1
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-15T05:55:38Z CA91K6SM3RVGSVM19ABM3BR9EV-m1e-c6925449 open actor=human:Wido targets=proof-admission-fits-a-seat-proving-several-units
Integrity: sha256=6b18025e3e158560b0c527628cab8b58a99badc68b831a579b3effade2814f95
