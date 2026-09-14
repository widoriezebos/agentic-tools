# delegate-sandbox-runs-the-beds

- State: queued
- Priority: 1
- Sequence: 1
- Risk: severity=2 novelty=1 exposure=3 accumulation=1 basis="severity 2: a wrong temporary directory breaks every delegate build, but the change is one adapter setting with a fixture; novelty 1: moving a temporary directory; exposure 3: every dispatch on every runtime; accumulation 1: nothing downstream reads the directory"
- Tier: 2
- Intent: Delivery-efficiency phase D, delegate loop. The adapter places the delegate's shell and Go temporary directories outside the linked worktree's git administration directory, so the independent-repository fixtures and the three process-owning beds (supervision-fixtures.sh, supervision-hook-fixtures.sh, land-fixtures.sh) run to completion inside the delegate sandbox. The implementer packet then requires, in the return's evidence, every bed the round's diff touches and full-package Go runs for every package it changes, and the return validator refuses a package reported green from a selection that names only the round's own new tests. Why: the stop-decisions build took nineteen rounds because the delegate could not run the beds it changed and the seat relayed one failure per round by hand; three of those rounds repaired the previous round's own blind change. DONE: a fixture proves a delegate under a chain runs all three beds to completion in its sandbox; a return whose evidence lacks a touched bed or a full package run is refused with the missing item named. every mechanism in this goal lives in the Go engine, the ledger verbs and the adapter contract, never in one runtime; it is proven on at least two runtimes.
- Origin: human
- Next step: Design page: the adapter setting, the packet clause and the validator check, each with its fixture; then build.
- OpenedAt: 2026-09-14T15:31:22Z
- Revision: 2
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-14T15:31:22Z NRM60XBC3CTEMVXAVZ14G5514K-m1e-c6925449 open actor=human:Wido targets=delegate-sandbox-runs-the-beds
- 2026-09-14T15:32:22Z 08EJFDPT7HYH4K6YHYBWYXWJ9J-m1e-c6925449 set-priority actor=human:Wido targets=actionable-metrics,brief-declares-the-round-boundary,builder-proves-each-rule-by-mutation,chain-landing-carries-the-reviewed-diff,coordinator-context-stays-under-budget,coordinator-wakes-on-events-not-polls,critique-closes-on-folded-proof,cross-cutting-change-inventories-its-readers,delegate-sandbox-runs-the-beds,dispatch-bases-chains-on-the-remote-tip,failed-job-attention,fixture-repo-copies-exclude-the-artifacts-store,fixture-stewards-outlive-their-suite,fixture-waits-name-their-producer,hook-root-resolver-design,human-authority-surface-runs-its-cmd-tests,human-goal-verbs-forgiving,human-wait-does-not-consume-goal-budget,job-record-birth-token,lease-sweep-death-evidence,live-job-on-a-done-goal-unnoticed,machine-concurrency-governor,metasystem-stop-escalation-proofs,metasystem-stop-fleet-form,new-elapsed-budgets-use-explicit-hours,receipt-admission-caps-concurrent-batteries,repo-root-paths-ride-agent-commits-unjudged,run-scoped-build-caches-have-a-janitor,skipped-test-fails-its-group-on-the-other-os,small-change-lane,stop-batch-strands-a-resumable-goal,stop-hook-never-forces-an-empty-turn,token-spend-fence,watch-verb,watcher-repair-request-never-names-current,winddown-census-handoff-leak reason=priority-order subject=delegate-sandbox-runs-the-beds from=unranked to=1:1 requested-sequence=1
Integrity: sha256=edbb7bfc80045cd53b4f66bb1fd55fa1d623359c2be1395604be7dbb39939f72
