# proof-admission-fits-a-seat-proving-several-units

- State: approved
- Priority: 1
- Sequence: 66
- Risk: severity=2 novelty=2 exposure=3 accumulation=2 basis="severity 2: blocked or misattributed proofs and manual workarounds, no wrong landing; novelty 2: changes attempt accounting and retry scoping; exposure 3: every seat that proves more than one unit; accumulation 2: grows with parallel units"
- Tier: 2
- Intent: A seat proving several units hits admission refusals unrelated to its candidates. Proof attempts are charged to the one goal the machine may claim, so on 2026-09-15 other units' proofs and controls spent coordinator-context-stays-under-budget's attempt limit and blocked unit C1b; a retry decision is refused as 'ambiguous across 2 failed attempts' when components' latest failures come from different candidates, even for a single section; and a breach-stopped goal refuses set-budget until resumed, then refuses the run until a release and re-claim (m1c, about 40 minutes). DONE: a proof attempt is charged to the goal its candidate belongs to without switching the machine's claim; a retry decision is scoped to the candidate tree so another candidate's failures never make it ambiguous; set-budget on a breach-stopped goal works in one step and the claim's reservation follows; fixtures prove each.
- Origin: human
- Next step: Design, tier 2: decide how a proof attempt names its goal independently of the machine claim, scope retry decisions to the candidate tree, and fix the breach-stop set-budget ordering; coordinate the reservation part with stop-capability-follows-the-lease-epoch.
- OpenedAt: 2026-09-15T05:55:38Z
- Revision: 3
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-15T05:55:46Z revision=2 opid=3H0Z7SEBVR9RPFF7P5ADZZ89EK-m1e-c6925449 authority=proven digest=df213f5b455651166a6abb2d08a57b84719620d87525a412953c07bbb2721bcd

History:
- 2026-09-15T05:55:38Z CA91K6SM3RVGSVM19ABM3BR9EV-m1e-c6925449 open actor=human:Wido targets=proof-admission-fits-a-seat-proving-several-units
- 2026-09-15T05:55:46Z 3H0Z7SEBVR9RPFF7P5ADZZ89EK-m1e-c6925449 approve actor=human:Wido targets=proof-admission-fits-a-seat-proving-several-units
- 2026-09-15T05:58:54Z H2NQ83JQQ6WZ0WDFJFFGWQB6QR-m1e-c6925449 set-priority actor=human:Wido targets=proof-admission-fits-a-seat-proving-several-units reason=priority-order subject=proof-admission-fits-a-seat-proving-several-units from=unranked to=1:66 requested-sequence=66
Integrity: sha256=36eb07ef6c9508be4e75bcdeb73db551eac2edb4aaee72e25cd2c795a453ed07
