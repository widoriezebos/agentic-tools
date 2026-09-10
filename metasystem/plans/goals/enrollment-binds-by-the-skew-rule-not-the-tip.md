# enrollment-binds-by-the-skew-rule-not-the-tip

- State: approved
- Priority: 1
- Sequence: 4
- Risk: severity=2 novelty=1 exposure=3 accumulation=1 basis="severity 2: every seat rebuilds and re-arms after every ledger move or cannot land; novelty 1: the skew rule exists in dispatch.sh; exposure 3: every seat; accumulation 1: nothing built on it"
- Tier: 3
- Intent: The steward's rearm resolver (internal/steward/rearm_resolver.go around line 272) refuses 'enrollment records landed source X but the executable stamp resolves to Y' whenever the binary was built at any commit other than the enrollment's recorded tip, and the tip moves on every ledger publish (a goal edit, a claim), which changes no engine code. Seen 2026-09-10 09:00Z on m1b: a binary built three goal-edits behind the tip was refused by test verify until a rebuild on a clean tree at the exact tip and a steward arm. DONE means: the binding applies the same rule as dispatch's skew preflight (the engine is stale only when engine or agent scripts changed between its stamp and the tip), a binary built at an older commit with no such change is accepted, and a fixture proves a ledger-only move does not drift the enrollment while an engine change still does.
- Origin: main
- Next step: Small: one comparison in the rearm resolver plus a fixture. Third of the bootstrap members.
- OpenedAt: 2026-09-10T07:27:32Z
- Revision: 3
- Labels: bootstrap
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-10T07:29:14Z revision=2 opid=4ZK6CRTM3WB0139F8R1W7B9NNZ-m1b-c6925449 authority=proven digest=d3e191083c884f1144976481818c6489ad5dd538f3c60139b3438608f5997db7

History:
- 2026-09-10T07:27:32Z 74CMETNC39NSF6P3H6WX26XPMV-m1b-c6925449 open actor=m1b+main-1788680071-18713-e76d5d targets=enrollment-binds-by-the-skew-rule-not-the-tip
- 2026-09-10T07:29:14Z 4ZK6CRTM3WB0139F8R1W7B9NNZ-m1b-c6925449 approve actor=human:Wido targets=carried-landing-debt-and-cap,enrollment-binds-by-the-skew-rule-not-the-tip,human-carried-landing-verb,receipt-beds-run-the-candidate-engine,skew-preflight-knows-a-receipt-worktree,testing-contract-owns-record-paths,the-metasystem-validates-itself-with-itself
- 2026-09-10T07:29:50Z K95ARETHRMP34B6ZDJX6ECK6BT-m1b-c6925449 set-priority actor=human:Wido targets=actionable-metrics,breach-clock-and-budget-honesty,breach-stop-wedges-seat,burn-without-delivery-tripwire,delegate-job-liveness,dispatch-cap-necessity,enrollment-binds-by-the-skew-rule-not-the-tip,failed-job-attention,fixture-stewards-outlive-their-suite,goal-abandoned-with-a-reason,governed-exhaustion-reprojection,human-goal-verbs-forgiving,job-record-birth-token,landing-receipt-survives-records-drift,lease-sweep-death-evidence,live-job-on-a-done-goal-unnoticed,machine-concurrency-governor,proof-harness-process-custody,review-round-limit-counts-per-chain,severity-tiered-rigor,severity-tiered-rigor-p2,small-change-lane,steward-catchup-livelock,steward-revives-a-done-goal,stop-batch-strands-a-resumable-goal,suite-custody,token-spend-fence,turn-verdict-hardening,watch-verb,watcher-repair-request-never-names-current,winddown-census-handoff-leak reason=priority-order subject=enrollment-binds-by-the-skew-rule-not-the-tip from=unranked to=1:4 requested-sequence=4
Integrity: sha256=89656d9bbf7fce8365ee988fdd34ba368979df994efe69ae13aebb48e4dd80f4
