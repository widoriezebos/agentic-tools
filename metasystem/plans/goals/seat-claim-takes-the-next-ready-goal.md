# seat-claim-takes-the-next-ready-goal

- State: queued
- Risk: severity=2 novelty=1 exposure=2 accumulation=2 basis="A stalled seat wastes a start and idles (severity 2); a small change in an existing verb (novelty 1); every seat (exposure 2); every seat start (accumulation 2)"
- Tier: 2
- Intent: A seat never stalls on a goal another seat took first (Wido 2026-10-02: 'This should be machinery behaviour'): when a steward-started seat's goal claim names a goal that another machine has claimed in the meantime, the claim itself takes this machine's next ready goal and says which, and only answers 'nothing claimable' when the frontier is empty. The seat brief does not need to handle it.
- Origin: main
- Next step: Build: in the claim verb for the steward-seat lineage, fall back from a taken named goal to the machine's next ready goal; test with two machines and one goal; land
- OpenedAt: 2026-10-02T21:06:51Z
- Revision: 1
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0

History:
- 2026-10-02T21:06:51Z HKKY7NCV27P0HC74EDX26V571C-m1e-9c612d71 open actor=human:Wido targets=seat-claim-takes-the-next-ready-goal
Integrity: sha256=5f25a67bde840d525fb19591474bd8034fbc47303dd1005332eda48125cf608d
