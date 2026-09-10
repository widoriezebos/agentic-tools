# enrollment-binds-by-the-skew-rule-not-the-tip

- State: claimed
- Priority: 1
- Sequence: 4
- Risk: severity=2 novelty=1 exposure=3 accumulation=1 basis="severity 2: every seat rebuilds and re-arms after every ledger move or cannot land; novelty 1: the skew rule exists in dispatch.sh; exposure 3: every seat; accumulation 1: nothing built on it"
- Tier: 3
- Intent: The steward's rearm resolver (internal/steward/rearm_resolver.go around line 272) refuses 'enrollment records landed source X but the executable stamp resolves to Y' whenever the binary was built at any commit other than the enrollment's recorded tip, and the tip moves on every ledger publish (a goal edit, a claim), which changes no engine code. Seen 2026-09-10 09:00Z on m1b: a binary built three goal-edits behind the tip was refused by test verify until a rebuild on a clean tree at the exact tip and a steward arm. DONE means: the binding applies the same rule as dispatch's skew preflight (the engine is stale only when engine or agent scripts changed between its stamp and the tip), a binary built at an older commit with no such change is accepted, and a fixture proves a ledger-only move does not drift the enrollment while an engine change still does.
- Origin: main
- Next step: CLAIMED by m1b 2026-09-10 16:20Z (member 1 parked land-ready behind the fleet's publish cadence). Build dispatched to Sol as ebsr-build1: the rearm resolver binds by dispatch's skew rule (stamp an ancestor of the landed source with no engine or script change between), the machine re-mint on a ledger-only landing keeps the enrollment valid, four fixtures. Then closing read (Opus), receipt and landing in a quiet window. With this and landing-receipt-survives-records-drift landed, a receipt and its landing stop being voided by other seats' ledger writes.
- OpenedAt: 2026-09-10T07:27:32Z
- Revision: 6
- Labels: bootstrap
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-10T07:29:14Z revision=2 opid=4ZK6CRTM3WB0139F8R1W7B9NNZ-m1b-c6925449 authority=proven digest=d3e191083c884f1144976481818c6489ad5dd538f3c60139b3438608f5997db7
- Sliced: machine=m1b lineage=main-1788680071-18713-e76d5d revision=4 at=2026-09-10T11:41:21Z
- Claimed: machine=m1b lineage=main-1788680071-18713-e76d5d at=2026-09-10T11:41:13Z revision=4 accountingRevision=4
- StopCapability: generation=4 revision=4 machine=m1b claimEpoch=1 fenceEpoch=0

History:
- 2026-09-10T07:27:32Z 74CMETNC39NSF6P3H6WX26XPMV-m1b-c6925449 open actor=m1b+main-1788680071-18713-e76d5d targets=enrollment-binds-by-the-skew-rule-not-the-tip
- 2026-09-10T07:29:14Z 4ZK6CRTM3WB0139F8R1W7B9NNZ-m1b-c6925449 approve actor=human:Wido targets=carried-landing-debt-and-cap,enrollment-binds-by-the-skew-rule-not-the-tip,human-carried-landing-verb,receipt-beds-run-the-candidate-engine,skew-preflight-knows-a-receipt-worktree,testing-contract-owns-record-paths,the-metasystem-validates-itself-with-itself
- 2026-09-10T07:29:50Z K95ARETHRMP34B6ZDJX6ECK6BT-m1b-c6925449 set-priority actor=human:Wido targets=actionable-metrics,breach-clock-and-budget-honesty,breach-stop-wedges-seat,burn-without-delivery-tripwire,delegate-job-liveness,dispatch-cap-necessity,enrollment-binds-by-the-skew-rule-not-the-tip,failed-job-attention,fixture-stewards-outlive-their-suite,goal-abandoned-with-a-reason,governed-exhaustion-reprojection,human-goal-verbs-forgiving,job-record-birth-token,landing-receipt-survives-records-drift,lease-sweep-death-evidence,live-job-on-a-done-goal-unnoticed,machine-concurrency-governor,proof-harness-process-custody,review-round-limit-counts-per-chain,severity-tiered-rigor,severity-tiered-rigor-p2,small-change-lane,steward-catchup-livelock,steward-revives-a-done-goal,stop-batch-strands-a-resumable-goal,suite-custody,token-spend-fence,turn-verdict-hardening,watch-verb,watcher-repair-request-never-names-current,winddown-census-handoff-leak reason=priority-order subject=enrollment-binds-by-the-skew-rule-not-the-tip from=unranked to=1:4 requested-sequence=4
- 2026-09-10T11:41:13Z 6TCE75DP3PM7S9V6R70T57ZV5Z-m1b-c6925449 claim actor=m1b+main-1788680071-18713-e76d5d targets=enrollment-binds-by-the-skew-rule-not-the-tip
- 2026-09-10T11:41:21Z DQ7NK8QTPXA4V4VDRVE32PES9R-m1b-c6925449 slice-start actor=m1b+main-1788680071-18713-e76d5d targets=enrollment-binds-by-the-skew-rule-not-the-tip
- 2026-09-10T11:41:32Z 44P66WJBN850H97XM5KX4G9GE2-m1b-c6925449 edit actor=m1b+main-1788680071-18713-e76d5d targets=enrollment-binds-by-the-skew-rule-not-the-tip
Integrity: sha256=bbfcfe7c5c55f0677b232158ce8fc9862168952a315203da87965b94f64f61a4
