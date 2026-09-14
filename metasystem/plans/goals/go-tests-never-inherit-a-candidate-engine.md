# go-tests-never-inherit-a-candidate-engine

- State: approved
- Priority: 1
- Sequence: 14
- Risk: severity=2 novelty=1 exposure=2 accumulation=2 basis="severity 2: false failures that read as real guard defects; novelty 1: environment hygiene; exposure 2: any seat running beds and packages in one shell; accumulation 2: each occurrence costs a diagnosis"
- Tier: 2
- Intent: A Go test must not be steered by an engine path left in the caller's environment. On 2026-09-14 METASYSTEM_BIN, exported for fixture beds, leaked into a Go test run; runReceiptGit preserved it and the pre-commit guard preferred it over the fixture's own engine, so TestFreshInitializationUsesHumanGitCommitThenRealMigration failed exactly like a real guard failure and cost an hour. DONE: the test harness clears inherited engine and installation variables before any test that invokes the guard or the engine, and a test proves a hostile inherited METASYSTEM_BIN cannot change a result.
- Origin: human
- Next step: Sanitize the engine-selection environment in the shared test helpers and the guard's test path; add the hostile-inheritance proof.
- OpenedAt: 2026-09-14T16:16:39Z
- Revision: 4
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-14T18:54:42Z revision=2 opid=1FVG0RPDDCS253V6D1FVHN0PXJ-m1e-c6925449 authority=proven digest=01c88816aa05e9379310fc5e7cf396643079963790219c3b366e2fd7c7cb0cc6

History:
- 2026-09-14T16:16:39Z B3R2EK7S895J6G3ZMQCF8FYTZE-m1e-c6925449 open actor=human:Wido targets=go-tests-never-inherit-a-candidate-engine
- 2026-09-14T18:54:42Z 1FVG0RPDDCS253V6D1FVHN0PXJ-m1e-c6925449 approve actor=human:Wido targets=go-tests-never-inherit-a-candidate-engine
- 2026-09-14T18:55:55Z YFJ0MPVRDFQ94KDGQC1NCZCPWS-m1e-c6925449 set-priority actor=human:Wido targets=go-tests-never-inherit-a-candidate-engine reason=priority-order subject=go-tests-never-inherit-a-candidate-engine from=unranked to=1:46 requested-sequence=append
- 2026-09-14T20:22:09Z TQTMZ5742QPZ3TZX9KHQ3FQJ10-m1e-c6925449 set-priority actor=human:Wido targets=actionable-metrics,beds-run-under-the-oldest-supported-bash,chain-landing-carries-the-reviewed-diff,degraded-stop-forms-have-one-source,dispatch-bases-chains-on-the-remote-tip,failed-job-attention,fixture-repo-copies-exclude-the-artifacts-store,fixture-stewards-outlive-their-suite,fixture-waits-name-their-producer,go-tests-never-inherit-a-candidate-engine,human-authority-surface-runs-its-cmd-tests,human-goal-verbs-forgiving,human-wait-does-not-consume-goal-budget,job-record-birth-token,lease-sweep-death-evidence,live-job-on-a-done-goal-unnoticed,machine-concurrency-governor,member-size-gate,metasystem-stop-escalation-proofs,metasystem-stop-fleet-form,moved-effects-are-inventoried-in-the-design,new-elapsed-budgets-use-explicit-hours,repo-root-paths-ride-agent-commits-unjudged,run-scoped-build-caches-have-a-janitor,skipped-test-fails-its-group-on-the-other-os,stop-batch-strands-a-resumable-goal,stop-decision-surface-is-a-gate,stop-response-carries-a-structured-report-reference,tests-never-wait-on-wall-time,token-spend-fence,watch-verb,watcher-repair-request-never-names-current,winddown-census-handoff-leak reason=priority-order subject=go-tests-never-inherit-a-candidate-engine from=1:46 to=1:14 requested-sequence=14
Integrity: sha256=80ec92bccda3bf106437b8698d6e43ccc0a71240ab8fd0c6fe4ccde06029a554
