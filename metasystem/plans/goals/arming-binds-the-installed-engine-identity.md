# arming-binds-the-installed-engine-identity

- State: queued
- Priority: 2
- Sequence: 25
- Risk: severity=2 novelty=2 exposure=3 accumulation=1 basis="severity 2: a run can execute a binary the seat did not build without any record saying so; novelty 2: identity binding on top of projection comparison is new; exposure 3: every proof run; accumulation 1: the arming record and one check"
- Tier: 2
- Intent: What: When a seat is armed, the record stores exactly which engine binary was installed (its build stamp, a digest of its bytes, and who installed it and when), and a later run against a different binary refuses or says so loudly, naming both. Why: Today the check only compares which source version the engine was built from, so two different builds of the same source look identical; on 2026-09-16 two seats could not tell whose binary they were running and had to compare file sizes by hand. Pros: A swapped or foreign binary is caught at once, and the answer to which binary is running comes from the record. Cons: One more field to keep in sync, and a too-strict check could refuse a harmless reinstall, which is why a seat reinstalling its own build must pass silently.
- Origin: human
- Next step: Next: Read how the arming record is written and how the drift check compares it, and check whether the steward's existing digest of its enrolled binary can serve as the identity source; then add the identity fields and the comparison as one change. Done when: A test that installs a byte-different build of the same source is refused with both identities named, and a test where a seat reinstalls its own build passes.
- OpenedAt: 2026-09-16T06:43:19Z
- Revision: 4
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-16T06:43:19Z XC5NW53TNHSRSHVJ081AZ5S6YE-m1e-c6925449 open actor=human:Wido targets=arming-binds-the-installed-engine-identity
- 2026-09-16T06:45:17Z NYMW6TQA3RFF8ACHKPZ91VXR87-m1e-c6925449 set-priority actor=human:Wido targets=arming-binds-the-installed-engine-identity reason=priority-order subject=arming-binds-the-installed-engine-identity from=unranked to=2:26 requested-sequence=append
- 2026-09-30T18:46:21Z BQ8BH92A7RTV6VN7HF7KV7S4KQ-ui-bc2fda53 edit actor=human:Wido targets=arming-binds-the-installed-engine-identity
- 2026-09-30T18:48:44Z DRV2G3ER650PC3GDMSYHQN3111-ui-bc2fda53 done actor=human:Wido targets=actionable-metrics,arming-binds-the-installed-engine-identity,census-lifecycle-scenario-holds-under-load,chain-landing-carries-the-reviewed-diff,critique-launch-verb-enforces-the-round-cap,dependency-ratchet-refuses-new-wall-clock-and-layout-asserts,dispatch-bases-chains-on-the-remote-tip,failed-job-attention,first-headless-run,fixture-repo-copies-exclude-the-artifacts-store,fixture-waits-name-their-producer,fleet-join-bootstrap,headless-continuous-delivery-proof,headless-fleet-coordination-proof,human-authority-surface-runs-its-cmd-tests,human-break-glass-commit-keeps-the-ledger-valid,human-wait-does-not-consume-goal-budget,job-record-birth-token,land-verb-writes-the-receipt,law-keys-read-committed-bytes,lease-sweep-death-evidence,live-job-on-a-done-goal-unnoticed,machine-concurrency-governor,member-size-gate,metasystem-stop-escalation-proofs,metasystem-stop-fleet-form,never-idle-ironclad,new-elapsed-budgets-use-explicit-hours,one-goal-act-one-ledger-commit,pipelines-never-lose-a-truncated-producer,repo-root-paths-ride-agent-commits-unjudged,run-scoped-build-caches-have-a-janitor,runtime-limit-is-not-a-protocol-error,seat-landings-run-the-selected-sections,skipped-test-fails-its-group-on-the-other-os,small-change-lane,steward-revivals-leave-the-receipt-stream,stop-batch-strands-a-resumable-goal,token-spend-fence,trunk-versus-candidate-is-a-verification-step,watch-verb,watcher-repair-request-never-names-current,winddown-census-handoff-leak reason=priority-order from=2:26 to=2:25
Integrity: sha256=ca94e29f8add5beb2f102dc0a8424ddc269030bbee361c0f13a1a2eddee18b8a
