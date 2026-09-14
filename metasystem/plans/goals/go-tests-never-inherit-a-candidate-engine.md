# go-tests-never-inherit-a-candidate-engine

- State: queued
- Risk: severity=2 novelty=1 exposure=2 accumulation=2 basis="severity 2: false failures that read as real guard defects; novelty 1: environment hygiene; exposure 2: any seat running beds and packages in one shell; accumulation 2: each occurrence costs a diagnosis"
- Tier: 2
- Intent: A Go test must not be steered by an engine path left in the caller's environment. On 2026-09-14 METASYSTEM_BIN, exported for fixture beds, leaked into a Go test run; runReceiptGit preserved it and the pre-commit guard preferred it over the fixture's own engine, so TestFreshInitializationUsesHumanGitCommitThenRealMigration failed exactly like a real guard failure and cost an hour. DONE: the test harness clears inherited engine and installation variables before any test that invokes the guard or the engine, and a test proves a hostile inherited METASYSTEM_BIN cannot change a result.
- Origin: human
- Next step: Sanitize the engine-selection environment in the shared test helpers and the guard's test path; add the hostile-inheritance proof.
- OpenedAt: 2026-09-14T16:16:39Z
- Revision: 1
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-14T16:16:39Z B3R2EK7S895J6G3ZMQCF8FYTZE-m1e-c6925449 open actor=human:Wido targets=go-tests-never-inherit-a-candidate-engine
Integrity: sha256=a1795ef85273d1cfb817985fa1ecba6e2c767ff1a76f0d20366b6b6fc477bdf3
