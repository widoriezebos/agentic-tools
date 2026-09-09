# burn-without-delivery-tripwire

- State: approved
- Priority: 1
- Sequence: 18
- Intent: The case-study day's class 1 (Wido 2026-08-25): hours and tokens burned with no product landed, noticed only by the human. The ledger already holds every fact needed - claim time, structured budget, landing receipts - but nothing compares them LIVE. ESCALATED TO HIGHEST PRIORITY by Wido 2026-09-01 after m0's 8-hour idle night (specimen on this goal): 'I need this to be fixed before you do anything else... proven with tests.' SCOPE BOUNDARY for fleet coordination (no duplicate work): this goal builds the DETECTION PRIMITIVE ONLY - a steward tick check per machine-held claim raising a steward alert through the existing alert/notify machinery when (a) a job under the claim is terminal-failed/process-lost with no landing receipt for the goal since that failure, or (b) the claim is older than 1.5x the slice norm with jobs reserved but zero landing receipts since claim. Notification only, no kill authority (the steward-watch relay delegation's bound). watch-verb (unclaimed) later CONSUMES these alerts to act on stalls; alert-escalation-channel (m3, in flight) later CARRIES them to Wido externally; ledger-attention (m2, in flight) is ledger-change noticing, disjoint. m0 builds only the primitive.
- Origin: main
- Next step: DONE twice over, landed d252c785 + aad398c2 by m0 (account Wido@M0): detection primitive live fleet-wide at next engine rebuilds - failed-job-without-recovery alarms, and burn-without-delivery now measures each slice against ITS OWN reservation budget per Wido's correction (capDeadline + capMin/2; a 30-minute slice alarms at 46 minutes). Acting layer commissioned on watch-verb with Wido's order recorded.
- OpenedAt: 2026-08-30T06:25:49Z
- Revision: 15
- Budget: elapsedLimit=1d attemptLimit=6 reservedJobMinutesLimit=360 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-06T06:53:37Z revision=12 opid=A35EHY2TGAWCCFA1Q1J0WZGS35-m1-a4f8999f authority=proven digest=e8b39fc86f5ce1cb8e4236fb6325fc4d0f2df371971f3b3ab2190a4992629ea6
- Sliced: machine=m0 lineage=main-1788178136-1684505-4ffe42 revision=8 at=2026-09-01T07:48:06Z

History:
- 2026-08-30T06:25:49Z 3JRRXNVNQF9RJAWABDDYSM7E16-m2-bc1be9cb open actor=m2+mac-coordinator targets=burn-without-delivery-tripwire
- 2026-09-01T06:38:49Z ZA805CNYYCAJ872YYWHYVR3KXP-m0-c5dbf036 edit actor=m0+main-1788178136-1684505-4ffe42 targets=burn-without-delivery-tripwire
- 2026-09-01T06:50:59Z NPC7W26TA6CPYJVAPN8S95BYMK-m0-c5dbf036 edit actor=m0+main-1788178136-1684505-4ffe42 targets=burn-without-delivery-tripwire
- 2026-09-01T06:51:10Z B2AAZBJDZJKNYY4HQF4WFN56GY-m0-c5dbf036 set-budget actor=human:Wido targets=burn-without-delivery-tripwire
- 2026-09-01T06:51:13Z 5X20MDCT05M17XV60299TRY5HC-m0-c5dbf036 claim actor=m0+main-1788178136-1684505-4ffe42 targets=burn-without-delivery-tripwire
- 2026-09-01T07:14:00Z J73M7W66438DHHKPS75JB83AMQ-m0-c5dbf036 edit actor=m0+main-1788178136-1684505-4ffe42 targets=burn-without-delivery-tripwire
- 2026-09-01T07:14:04Z VV5RCCGSE4PHGR30STZM2PR2MQ-m0-c5dbf036 release actor=m0+main-1788178136-1684505-4ffe42 targets=burn-without-delivery-tripwire
- 2026-09-01T07:47:12Z 1K7JGRRP88S9SZX9WB6CN1C3MR-m0-c5dbf036 claim actor=m0+main-1788178136-1684505-4ffe42 targets=burn-without-delivery-tripwire
- 2026-09-01T07:48:06Z PQ1MF5H6RJN58EAR71A30PZVGE-m0-c5dbf036 slice-start actor=m0+main-1788178136-1684505-4ffe42 targets=burn-without-delivery-tripwire
- 2026-09-01T08:06:35Z E2TC9Q684V81TYSWJWA39ZDAJM-m0-c5dbf036 edit actor=m0+main-1788178136-1684505-4ffe42 targets=burn-without-delivery-tripwire
- 2026-09-01T08:06:39Z F5E92C4B9HDH2X4BKXBN6GZB5Z-m0-c5dbf036 release actor=m0+main-1788178136-1684505-4ffe42 targets=burn-without-delivery-tripwire
- 2026-09-06T06:53:37Z A35EHY2TGAWCCFA1Q1J0WZGS35-m1-a4f8999f approve actor=human:Wido targets=burn-without-delivery-tripwire reason=sweep
- 2026-09-08T15:56:21Z 6Y1JKD6AQ85HV71F5CA8NAFKYC-m1-7cd0bd60 set-priority actor=human:Wido targets=burn-without-delivery-tripwire reason=priority-order subject=burn-without-delivery-tripwire from=unranked to=1:18 requested-sequence=18
- 2026-09-08T17:03:35Z YZH9GDDZJGK4DB0ENK5CGSXADW-m1b-c6925449 done actor=human:Wido targets=account-provenance,actionable-metrics,backlog-ordered-by-priority,breach-clock-and-budget-honesty,breach-stop-wedges-seat,burn-without-delivery-tripwire,delegate-job-liveness,dispatch-cap-necessity,failed-job-attention,fixture-stewards-outlive-their-suite,governed-exhaustion-reprojection,host-runtime-setup,human-goal-verbs-forgiving,job-record-birth-token,lease-sweep-death-evidence,live-job-on-a-done-goal-unnoticed,machine-concurrency-governor,proof-harness-process-custody,review-round-limit-counts-per-chain,severity-tiered-rigor,severity-tiered-rigor-p2,small-change-lane,steward-catchup-livelock,steward-revives-a-done-goal,suite-custody,token-spend-fence,turn-verdict-hardening,watch-verb,watcher-repair-request-never-names-current,winddown-census-handoff-leak reason=priority-order from=1:18 to=1:17
- 2026-09-09T06:01:27Z WBARN8TBQKTC4BW0AQZRW614PS-m1b-c6925449 set-priority actor=human:Wido targets=account-provenance,actionable-metrics,burn-without-delivery-tripwire,delegate-job-liveness,failed-job-attention,fixture-stewards-outlive-their-suite,human-goal-verbs-forgiving,job-record-birth-token,lease-sweep-death-evidence,live-job-on-a-done-goal-unnoticed,machine-concurrency-governor,proof-harness-process-custody,review-round-limit-counts-per-chain,severity-tiered-rigor,severity-tiered-rigor-p2,small-change-lane,steward-catchup-livelock,steward-revives-a-done-goal,stop-batch-strands-a-resumable-goal,suite-custody,token-spend-fence,turn-verdict-hardening,watch-verb,watcher-repair-request-never-names-current,winddown-census-handoff-leak reason=priority-order subject=stop-batch-strands-a-resumable-goal from=1:17 to=1:18 requested-sequence=7
Integrity: sha256=39e79cf7cfa0ff6ab28ff30f6bf29de142a1ee6260e2f2efb2ec33eb905c1ffd
