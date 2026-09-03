# one-approval-gate

- State: queued
- Intent: ONE human approval, not two (Wido's finding 2026-09-03 during the first headless run): today a piece of work passes TWO human gates with TWO budgets - the ledger's goal approve (the approved tuple: elapsed/attempts/minutes/active) and the mission contract's signature (its own fences: wall-clock, cycles, jobs, concurrency, job-cap, plus a priced exposure). One should suffice: the mission contract's limits must DERIVE from the goal's approved ledger tuple, and the contract signature must BE the goal approval (or reference it by revision) so the human signs once and the runner reads the ledger. DONE means: sealing a mission contract for an approved goal takes its fences from the approved tuple (no second hand-typed budget), the Approval line is replaced by or bound to the goal approval record, a mismatch refuses at seal, and the first headless run's contract (mission birth-token) is the specimen.
- Origin: main
- Next step: INTENT: one human word governs both the backlog item and the unattended run of it. CONSTRAINTS: the goal ledger is the source of truth for budget; the contract may only NARROW it, never widen; exposure (money) stays explicit at approval as part of the tuple or its box; keep the runner's per-run fences (cycles, wall-clock) as DERIVED numbers with a recorded derivation rule. FREEDOMS: whether the contract carries the goal revision it was approved under, or the approve verb emits the sealed contract; whether the Approval line survives as a byte attestation of the derived contract. Tier 3 (design-bearing, touches the approval feature and the contract package). Budget Wido's word at approval.
- OpenedAt: 2026-09-03T15:47:08Z
- Revision: 1

History:
- 2026-09-03T15:47:08Z 5ZH6MSW3W3PKXQRH48E1NP56T8-m0-c5dbf036 open actor=human:Wido targets=one-approval-gate
Integrity: sha256=ef699c9cea1628c0a67bcb4d461c432cad878fbb2a5348c5637e576e4c6e2f1b
