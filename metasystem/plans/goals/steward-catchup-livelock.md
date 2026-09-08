# steward-catchup-livelock

- State: queued
- Priority: 1
- Sequence: 20
- Intent: A machine that rejoins after a long absence cannot get its steward alive: the ledger-attention stage replays every ledger commit between its last diffed tip and the accepted tip with a full tree projection each (m3 on 2026-09-03 after 48 hours offline: 405 commits at several seconds each, tens of minutes), while the runner counts as alive only through a completed steward-tick within twice the tick interval; the supervision watcher therefore stops and replaces the runner every minute (attemptSeq climbed past 17, lastCompletion never set), each replacement discards the in-memory stage, and 'metasystem up' ends every time with 'replacement runner did not complete generation N within 10s'. The catch-up is restarted from scratch forever; nothing on this machine is watched. DONE means a rejoining machine's first tick either persists its catch-up incrementally (a partial stage survives a restart and resumes from its last projected tip) or is recognised as live progress by the freshness rule, so that the runner is never replaced while it is making progress, proven by a fixture with a long ledger window and a short tick interval. Cousin of recovery-to-good-state; found by m3 at the fence takeover.
- Origin: main
- Next step: Classify at intake (R-54-m1); reproduce as a failing test in internal/steward (a ledger window longer than the freshness cap, the watcher replacing the runner mid-catch-up); then design the smallest fix (incremental persistence of the ledger-attention stage per projected tip, or a progress-aware freshness rule) and run the ladder for its tier.
- OpenedAt: 2026-09-03T13:35:53Z
- Revision: 3
- Labels: robustness, steward
- BudgetExceptions: 0

History:
- 2026-09-03T13:35:53Z PB1RC3TNGD6NF5G1ZWZF9FV2VW-m3-a5da21ff open actor=m3+mac-m3 targets=steward-catchup-livelock
- 2026-09-08T15:56:32Z TCJ3BEX7MMCN0NQY27RXH82NG8-m1-7cd0bd60 set-priority actor=human:Wido targets=steward-catchup-livelock reason=priority-order subject=steward-catchup-livelock from=unranked to=1:21 requested-sequence=21
- 2026-09-08T17:03:35Z YZH9GDDZJGK4DB0ENK5CGSXADW-m1b-c6925449 done actor=human:Wido targets=account-provenance,actionable-metrics,backlog-ordered-by-priority,breach-clock-and-budget-honesty,breach-stop-wedges-seat,burn-without-delivery-tripwire,delegate-job-liveness,dispatch-cap-necessity,failed-job-attention,fixture-stewards-outlive-their-suite,governed-exhaustion-reprojection,host-runtime-setup,human-goal-verbs-forgiving,job-record-birth-token,lease-sweep-death-evidence,live-job-on-a-done-goal-unnoticed,machine-concurrency-governor,proof-harness-process-custody,review-round-limit-counts-per-chain,severity-tiered-rigor,severity-tiered-rigor-p2,small-change-lane,steward-catchup-livelock,steward-revives-a-done-goal,suite-custody,token-spend-fence,turn-verdict-hardening,watch-verb,watcher-repair-request-never-names-current,winddown-census-handoff-leak reason=priority-order from=1:21 to=1:20
Integrity: sha256=9ce79b8f184e3f2cbec1b4bb91c467ccb1754c631a0a12a8cfca7ecf19f57dcf
