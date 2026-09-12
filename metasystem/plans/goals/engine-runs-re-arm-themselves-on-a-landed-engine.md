# engine-runs-re-arm-themselves-on-a-landed-engine

- State: queued
- Risk: severity=2 novelty=1 exposure=3 accumulation=1 basis="severity 2: a run admitted against a stale or unlanded engine proves the wrong binary; novelty 1: the projection comparison, the landed-build resolver and the re-arm verb exist; exposure 3: every engine-driven run on every seat after any seat lands engine code; accumulation 1: one admission path"
- Tier: 2
- Intent: An engine-driven run (metasystem test run: a landing receipt, a cadence run, a diagnostic) refuses with TEST_POLICY_ENGINE_REQUIRED whenever the enrolled engine of the checkout is behind the fetched tip in its ENGINE projection, which with three seats landing engine code happens between most runs: on 2026-09-12 every engine-driven run on m1e first cost a person a reset to origin/main, a rebuild and a re-arm (memory/fixture-beds-run-through-the-engine). A landed engine is trusted by its landing; the seat re-doing by hand what the landing already proved is the tax, and it is the first prerequisite for landing through receipts again instead of human commits (Wido, 2026-09-12). DONE means: (1) a run whose enrolled engine is behind origin/main by landed commits only rebuilds the engine from the fetched tip and re-arms the enrollment itself (the same path metasystem up --repo takes, under the same authority and the same witnesses), records that it did so in the attempt, and runs; (2) a run whose checkout is dirty in an engine input, or whose tip is not an ancestor of origin/main, still refuses with the same typed reason naming the two commits and the one command; (3) a fixture proves both on the fake ledger with a fake landing; (4) the next day of receipts and diagnostics on this seat shows no manual reset, rebuild or re-arm between runs. RUNTIME INDEPENDENCE (plan principle 0): the mechanism lives in the Go engine and the ledger, never in one runtime; it holds for every seat whatever runtime it hosts.
- Origin: human
- Next step: Design on plans/engine-runs-re-arm-on-a-landed-engine-design.md (one page): trustedPolicyEngine in cmd/metasystem/test.go and VerifySourceAtDestination in internal/steward/rearm_resolver.go; the landed test (the enrolled source is an ancestor of the fetched tip and the working tree is clean in every engine input); the self re-arm through the up path; the attempt record naming it; the fixture on the fake ledger. Built directly by the seat under R-98-m1e, one independent critic, landed by human commit, verified by the next engine-driven run after another seat lands engine code.
- OpenedAt: 2026-09-12T15:43:23Z
- Revision: 1
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-12T15:43:23Z 4TSFK59B6Y85KDYSANGF7A36MW-m1-c6925449 open actor=human:Wido targets=engine-runs-re-arm-themselves-on-a-landed-engine
Integrity: sha256=472b572b9e0282ec455b52e4a220e9332b04e62b8fe9cc73f79c73dfc0670a89
