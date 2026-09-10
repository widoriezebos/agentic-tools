# receipt-groups-run-concurrently

- State: queued
- Risk: severity=2 novelty=2 exposure=3 accumulation=1 basis="severity 2: every landing waits 50 minutes for 15 minutes of work; novelty 2: concurrency in a runner that owns processes and caps; exposure 3: every receipt on every seat; accumulation 1: nothing built on it"
- Tier: 3
- Intent: The schema-2 receipt runs its selected groups one after another. Measured 2026-09-10 on goal receipt-beds-run-the-candidate-engine (proof run proof-mtvb7pc9-8ce278d20b305ce2): 41 groups, about 50 minutes wall clock, of which four independent groups took 35 minutes (section/dispatcher-adapter-and-mission-runner-fixtures 613 s, goal-full-coverage 603 s, missionrunner-full-coverage 487 s, section/supervision-and-census-fixtures 393 s) and the other 37 groups six minutes together. The groups are separate processes over separate inputs; nothing in the contract or the runner (internal/proofrun/test_build.go, no concurrency key in testing.json) orders them. Wido, 2026-09-10: 'I'm constantly baffled by the time that it takes to run the test suite before you can integrate.' DONE means: the runner executes a receipt's groups concurrently up to a bound the machine sets (the slots of machine-concurrency-governor where it exists, else a contract key with a documented default), process-owning section beds that cannot share a machine are marked so in the contract and serialized among themselves only, per-group logs, identities and results stay exactly as they are, and a fixture proves a receipt of four independent groups finishes in about the longest group's time. Sibling levers, not this goal: blast-radius-beside-risk (selecting by reach, so unrelated full-coverage suites are not pulled in by surface) and the group-evidence reuse that reproducible engine digests (receipt-beds-run-the-candidate-engine) make possible.
- Origin: main
- Next step: Tier 3, design first on the design lane (the bed-sharing question decides the shape). Opened by m1b 2026-09-10 14:45Z at Wido's question.
- OpenedAt: 2026-09-10T10:36:26Z
- Revision: 1
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0

History:
- 2026-09-10T10:36:26Z 2QCH5496Y5PCGP64QSJQ0M1V6J-m1b-c6925449 open actor=m1b+main-1788680071-18713-e76d5d targets=receipt-groups-run-concurrently
Integrity: sha256=27f5c9c616f609753726c1a50e9c7e6d7e81be3a3e43c1b74647e4aa0f153117
