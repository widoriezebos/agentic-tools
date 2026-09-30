# law-keys-read-committed-bytes

- State: queued
- Priority: 2
- Sequence: 24
- Risk: severity=2 novelty=1 exposure=3 accumulation=1 basis="severity 2: a law silently raised by an uncommitted edit; novelty 1: member-size-gate r2 already designs the committed-bytes reader for its own keys; exposure 3: every seat, every budget check; accumulation 1: one reader"
- Tier: 2
- Intent: What: Budget and limit settings are read from the committed configuration, not from uncommitted edits. Why: Today an uncommitted edit to metasystem.conf silently raises a limit for the seat that made it, and no record shows it. Pros: Limits change only through a visible commit. Cons: Changing a limit takes a commit. Personal roster and launch settings in metasystem.conf.local stay out of scope.
- Origin: human
- Next step: Next: Build one reader in internal/config that reads metasystem.conf as committed; list the limit keys in config/budget.go and move them to it one key family at a time. Done when: a test per key family shows an uncommitted edit changes nothing until it is committed.
- OpenedAt: 2026-09-16T06:43:11Z
- Revision: 4
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-16T06:43:11Z BJ12FA41CRC9F8V4VE8EK64REX-m1e-c6925449 open actor=human:Wido targets=law-keys-read-committed-bytes
- 2026-09-16T06:45:11Z WGN9PK0RZNJVB1HGNQTQNMWT1E-m1e-c6925449 set-priority actor=human:Wido targets=law-keys-read-committed-bytes reason=priority-order subject=law-keys-read-committed-bytes from=unranked to=2:25 requested-sequence=append
- 2026-09-30T18:48:44Z DRV2G3ER650PC3GDMSYHQN3111-ui-bc2fda53 done actor=human:Wido targets=actionable-metrics,arming-binds-the-installed-engine-identity,census-lifecycle-scenario-holds-under-load,chain-landing-carries-the-reviewed-diff,critique-launch-verb-enforces-the-round-cap,dependency-ratchet-refuses-new-wall-clock-and-layout-asserts,dispatch-bases-chains-on-the-remote-tip,failed-job-attention,first-headless-run,fixture-repo-copies-exclude-the-artifacts-store,fixture-waits-name-their-producer,fleet-join-bootstrap,headless-continuous-delivery-proof,headless-fleet-coordination-proof,human-authority-surface-runs-its-cmd-tests,human-break-glass-commit-keeps-the-ledger-valid,human-wait-does-not-consume-goal-budget,job-record-birth-token,land-verb-writes-the-receipt,law-keys-read-committed-bytes,lease-sweep-death-evidence,live-job-on-a-done-goal-unnoticed,machine-concurrency-governor,member-size-gate,metasystem-stop-escalation-proofs,metasystem-stop-fleet-form,never-idle-ironclad,new-elapsed-budgets-use-explicit-hours,one-goal-act-one-ledger-commit,pipelines-never-lose-a-truncated-producer,repo-root-paths-ride-agent-commits-unjudged,run-scoped-build-caches-have-a-janitor,runtime-limit-is-not-a-protocol-error,seat-landings-run-the-selected-sections,skipped-test-fails-its-group-on-the-other-os,small-change-lane,steward-revivals-leave-the-receipt-stream,stop-batch-strands-a-resumable-goal,token-spend-fence,trunk-versus-candidate-is-a-verification-step,watch-verb,watcher-repair-request-never-names-current,winddown-census-handoff-leak reason=priority-order from=2:25 to=2:24
- 2026-09-30T18:50:57Z JYS69B73DSC4AAB0Q7HWFVV3VM-m1e-b6a4eb0a edit actor=human:wido targets=law-keys-read-committed-bytes
Integrity: sha256=208b8449391bc2d921366297111b0b93b9e6127aa3e350a753b07940867aba85
