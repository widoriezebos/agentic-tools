# new-elapsed-budgets-use-explicit-hours

- State: approved
- Priority: 1
- Sequence: 41
- Risk: severity=3 novelty=2 exposure=3 accumulation=1 basis="severity 3: budget enforcement can stop lawful work or permit unbounded time; novelty 2: a bounded change to existing ledger and elapsed projection; exposure 3: shared goal execution across seats; accumulation 1: each episode is evaluated independently, with no new aggregate accounting"
- Tier: 3
- Intent: New elapsed budget inputs in hours and minutes are stored verbatim, so 24h stays 24h. Reject newly supplied d tokens with an explanation of the eight-hour legacy versus calendar-day ambiguity and an explicit-hour alternative. Preserve historical d records at their original eight-hour interpretation, including journal recovery; do not silently reinterpret or migrate a live budget.
- Origin: main
- Next step: Second independent successor of goal:breach-clock-and-budget-honesty. Reuse accepted Fix 2 and its review decisions in plans/breach-clock-and-budget-honesty-design.md. Retarget all actual input/replay writers and their tests to current main. Wait for goal:budget-raises-preserve-elapsed-origin before operational budget corrections, which otherwise reset the clock. Keep rollout deliberate with old and new binaries; do not absorb human-wait or quota work.
- OpenedAt: 2026-09-11T05:37:45Z
- Revision: 5
- Labels: breach-clock-successor
- Pinned: m1c
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-13T07:45:22Z revision=3 opid=H97CBRR70MC30DBCJDKFQ00ES0-m1e-c6925449 authority=proven digest=5673bd54ca329d6540236ca8d584488b0533ae0581aedf2ef2e467a3b895169d

History:
- 2026-09-11T05:37:45Z 67J50K768RCDA25GHBZ4J69J7M-m1c-66a02980 open actor=m1c+main-1789023008-75159-b69143 targets=new-elapsed-budgets-use-explicit-hours
- 2026-09-13T07:45:18Z 5YS1QE8EXXRCBFNKF91CW9AP7X-m1e-c6925449 set-pin actor=human:Wido targets=new-elapsed-budgets-use-explicit-hours
- 2026-09-13T07:45:22Z H97CBRR70MC30DBCJDKFQ00ES0-m1e-c6925449 approve actor=human:Wido targets=new-elapsed-budgets-use-explicit-hours
- 2026-09-13T07:45:27Z 9SFHNC9T1KRWNJCFEGHEFBJKDM-m1e-c6925449 set-priority actor=human:Wido targets=new-elapsed-budgets-use-explicit-hours reason=priority-order subject=new-elapsed-budgets-use-explicit-hours from=unranked to=1:42 requested-sequence=42
- 2026-09-13T08:18:55Z B5G9KBS61Q28BAQ0ETN1G2V4AY-m1-c6925449 done actor=human:Wido targets=actionable-metrics,breach-clock-and-budget-honesty,chain-landing-carries-the-reviewed-diff,closing-read-and-receipt-in-parallel,dispatch-bases-chains-on-the-remote-tip,failed-job-attention,fixture-repo-copies-exclude-the-artifacts-store,fixture-stewards-outlive-their-suite,fixture-waits-name-their-producer,human-authority-surface-runs-its-cmd-tests,human-goal-verbs-forgiving,human-wait-does-not-consume-goal-budget,job-record-birth-token,lease-sweep-death-evidence,live-job-on-a-done-goal-unnoticed,machine-concurrency-governor,metasystem-stop-escalation-proofs,metasystem-stop-fleet-form,new-elapsed-budgets-use-explicit-hours,repo-root-paths-ride-agent-commits-unjudged,run-scoped-build-caches-have-a-janitor,skipped-test-fails-its-group-on-the-other-os,steward-catchup-livelock,steward-revives-a-done-goal,stop-batch-strands-a-resumable-goal,stop-refusal-fits-on-one-screen,suite-custody,token-spend-fence,watch-verb,watcher-repair-request-never-names-current,winddown-census-handoff-leak reason=priority-order from=1:42 to=1:41
Integrity: sha256=6dbbb4209b4f671457e08ad40d159c618c16c67d35b6f0832ea87c02015b187d
