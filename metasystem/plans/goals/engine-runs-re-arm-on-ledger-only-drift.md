# engine-runs-re-arm-on-ledger-only-drift

- State: queued
- Risk: severity=2 novelty=1 exposure=3 accumulation=1 basis="severity 2: a run admitted against a stale engine proves the wrong binary; novelty 1: the projection comparison and the re-arm verb exist; exposure 3: every engine-driven run on every seat; accumulation 1: one admission path"
- Tier: 2
- Intent: An engine-driven run (metasystem test run: a landing receipt, a cadence run, a diagnostic) refuses with TEST_POLICY_ENGINE_REQUIRED when the enrolled source is behind the fetched tip, even when the tip moved by ledger commits only (goal verbs from any seat, which touch no engine input). On 2026-09-12 every engine-driven run on m1e first cost a reset to origin/main, a rebuild and a re-arm, because three seats write the ledger between runs; the same tax lands on every seat that shares the ledger, and it is the first prerequisite for landing through receipts again instead of human commits (Wido, 2026-09-12). DONE means: (1) a run whose enrolled source and captured policy base differ only in commits that touch no engine projection input is admitted, re-arming the enrollment on the tip itself (the same path `metasystem up --repo` takes) or binding the enrolled projection to the tip, and says which in its output; (2) a run whose difference touches the engine projection still refuses with the same typed reason naming both commits and the one command that re-arms; (3) a fixture on the fake ledger proves both: a ledger-only move is admitted, an engine-touching move is refused; (4) the tax is gone from this seat: the next day of receipts and diagnostics shows no reset, rebuild or re-arm between runs on ledger-only moves. RUNTIME INDEPENDENCE (plan principle 0): the mechanism lives in the Go engine and the ledger, never in one runtime; it holds for every seat whatever runtime it hosts.
- Origin: human
- Next step: Design on plans/engine-runs-re-arm-on-ledger-only-drift-design.md (one page): where trustedPolicyEngine compares the enrolled source and the captured policy base (cmd/metasystem/test.go), the ledger-only test (the diff between the two commits touches no engine projection input), the self re-arm or the projection binding, and the fixture on the fake ledger. Built directly by the seat under R-98-m1e, one independent critic, landed by human commit, verified by an engine-driven run on the next ledger move.
- OpenedAt: 2026-09-12T15:37:50Z
- Revision: 1
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-12T15:37:50Z ZBQ1PGQKCBFXQNJZD4YNG4NNHR-m1-c6925449 open actor=human:Wido targets=engine-runs-re-arm-on-ledger-only-drift
Integrity: sha256=f7a6808e2f9159ed94f01b3e14c691a60208a6e77954c5735ad28ac8aa333a57
