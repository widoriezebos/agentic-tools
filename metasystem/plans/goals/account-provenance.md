# account-provenance

- State: claimed
- Priority: 1
- Sequence: 10
- Risk: severity=2 novelty=3 exposure=2 accumulation=2 basis="novelty 3: one closed account contract cut across every runtime adapter, the announcement, both job record composers and the landing evaluator; severity 2, exposure 2, accumulation 2 as before"
- Tier: 3
- Intent: ACCOUNT PROVENANCE (m0, re-opened on the fleet line at reconciliation): m0's Claude runtime signs into a different account than m1/m2 — a separate capacity pool, which is why m0 exists. m0's landings are stamped m0 while another account paid for them. The run record should carry the account identity alongside runtime and model so cost and capacity attribution survive in the record. Wido's word (decision-ask 2026-08-31): the string m0 stamps in every landing message until this lands is 'Wido@M0'.
- Origin: main
- Next step: BLOCKED ON A HUMAN RAISE (2026-09-07 00:1xZ): chain account-provenance-build1 is complete through round 4 (reviewed tree 2a4460f7; diff applies to main; Go gate green; both fixture beds green on round 2, and rounds 3-4 changed only codex.go, its test, the commit.sh stamp line and the account test file); critics crit3 and crit4 each found one item, folded in rounds 3 and 4. The last fresh critic on round 4 was refused: reservedJobMinutesLimit used=720 limit=720 (six jobs at the 120-minute cap). Tier-2 box tops out at 720, so the raise needs a tier 3 first. Ask, two commands from the metasystem directory: (1) ./bin/metasystem goal edit --id account-provenance --by Wido --risk severity=2,novelty=3,exposure=2,accumulation=2 --basis "novelty 3: one closed account contract cut across every runtime adapter, the announcement, both job record composers and the landing evaluator; severity 2, exposure 2, accumulation 2 as before" --evidence "six build and critique rounds over 22 files in eight packages"  (2) ./bin/metasystem goal set-budget --id account-provenance --by Wido --elapsed-limit 1d --attempt-limit 12 --reserved-job-minutes-limit 960 --active-job-limit 1 --review-round-limit 2 --because "the closing critic on round 4". Then m1b: dispatch the critic (--reviews account-provenance-build1-r4 --op account-provenance-crit5, brief plans/account-provenance-critique-brief.md at 8637812c), close, land --chain with the battery receipt, live up proof, receipt, done. Released by m1b at this clean point to run metasystem-stop-verb meanwhile.
- OpenedAt: 2026-08-31T19:09:19Z
- Revision: 24
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-08T06:27:59Z revision=21 opid=ZRNTXEBR7B11DE1W9GE1T0TW0F-m1b-c6925449 authority=raise=ZRNTXEBR7B11DE1W9GE1T0TW0F-m1b-c6925449 digest=01ad8354e27d0b631a3d13746d0dd38ce8f026509ac792a0bb5f9d1686e9cf79
- Sliced: machine=m0b lineage=main-1788250419-3170380-8a1fb3 revision=3 at=2026-09-02T05:51:12Z
- Claimed: machine=m1d lineage=main-1788764558-63534-a15b0d at=2026-09-07T09:14:50Z revision=21 accountingRevision=19
- StopCapability: generation=21 revision=21 machine=m1d claimEpoch=2 fenceEpoch=1
- StopFence: stopId=stop-account-provenance-r19-f1 revision=19 epoch=1 capabilityGeneration=19 closedAt=2026-09-07T21:17:15Z reason=ELAPSED_LIMIT

History:
- 2026-08-31T19:09:19Z NHE37NCWZ1MATB7WFYK0PA87KP-m0-c5dbf036 open actor=m0+main-1788178136-1684505-4ffe42 targets=account-provenance
- 2026-09-01T20:26:09Z JFCW3W9VE6G7C447JZD7CED29A-m0b-6638932d set-budget actor=m0b+main-1788250419-3170380-8a1fb3 targets=account-provenance
- 2026-09-02T05:48:30Z FRWPFKA158EJ7XBMB71WV35FAN-m0b-6638932d claim actor=m0b+main-1788250419-3170380-8a1fb3 targets=account-provenance
- 2026-09-02T05:51:12Z NN2V0KAD3WYWW1KPHEAX9PQJ0B-m0b-6638932d slice-start actor=m0b+main-1788250419-3170380-8a1fb3 targets=account-provenance
- 2026-09-02T06:53:02Z M4RCPXH87FD8BTFP1GP8918PRA-m0b-6638932d edit actor=m0b+main-1788250419-3170380-8a1fb3 targets=account-provenance
- 2026-09-02T06:53:06Z 01B6Z8XKR00BGT4FFD529DJ264-m0b-6638932d release actor=m0b+main-1788250419-3170380-8a1fb3 targets=account-provenance
- 2026-09-02T17:56:01Z QVNFHBDS3J937PQW0QR9H9EDMM-m1-7bb1546e claim actor=m1+main-1788333680-2840-7f79f4 targets=account-provenance
- 2026-09-02T18:18:02Z HJSSPBFQB7G0JC97F4ENRMKM9E-m1-7bb1546e release actor=m1+main-1788333680-2840-7f79f4 targets=account-provenance
- 2026-09-06T06:53:37Z A35EHY2TGAWCCFA1Q1J0WZGS35-m1-a4f8999f approve actor=human:Wido targets=account-provenance reason=sweep
- 2026-09-06T19:49:31Z QK2TR6ZCD1PG4HXWEQHF4WP03E-m1b-c6925449 claim actor=m1b+main-1788680071-18713-e76d5d targets=account-provenance
- 2026-09-06T19:58:10Z XN7ZQ13TV2FCF2P6AW98S8KWA5-m1b-c6925449 edit actor=m1b+main-1788680071-18713-e76d5d targets=account-provenance reason=Misclassified: from=0 to=2 evidence=root:account-provenance-crit2
- 2026-09-06T20:16:08Z P1ZP6FAP79R4J3478MQM7YV7YY-m1b-c6925449 edit actor=m1b+main-1788680071-18713-e76d5d targets=account-provenance
- 2026-09-06T20:16:11Z VNDK6SV240HRH7F722QY6EQM8F-m1b-c6925449 release actor=m1b+main-1788680071-18713-e76d5d targets=account-provenance
- 2026-09-06T21:49:49Z 2A4CJFKCY0RPKJ9M2W96AT0A12-m1b-c6925449 edit actor=m1b+main-1788680071-18713-e76d5d targets=account-provenance
- 2026-09-06T22:02:12Z 2AWSYN7QHE42JXPDJ3XPWD8Z05-m1-7cd0bd60 approve actor=human:Wido targets=account-provenance
- 2026-09-06T22:12:41Z 04YBW0A0SP3D5DMBKB7GSXRYVD-m1b-c6925449 claim actor=m1b+main-1788680071-18713-e76d5d targets=account-provenance
- 2026-09-06T23:58:51Z NEHNG09FDCMQD4N8TNK66B8VD1-m1b-c6925449 edit actor=m1b+main-1788680071-18713-e76d5d targets=account-provenance
- 2026-09-06T23:58:54Z F546H7X34N34TY3CM4YYKBSSDJ-m1b-c6925449 release actor=m1b+main-1788680071-18713-e76d5d targets=account-provenance
- 2026-09-07T09:14:50Z 6E1HRWJ22AX8ASTX888QZ88519-m1d-25755dc0 claim actor=m1d+main-1788764558-63534-a15b0d targets=account-provenance
- 2026-09-07T21:17:15Z RKJHHRBPE0EY2M7317EF8SXH4S-m1d-43182c96 breach-stop actor=m1d+goal-stop-custodian targets=account-provenance
- 2026-09-08T06:27:59Z ZRNTXEBR7B11DE1W9GE1T0TW0F-m1b-c6925449 edit actor=human:Wido targets=account-provenance displaced=m1d+main-1788764558-63534-a15b0d@2026-09-07T09:14:50Z reason=Misclassified: from=2 to=3 evidence=root:account-provenance-build1
- 2026-09-08T15:55:53Z XCX3D86ZVZ4DVCHE7WWTYYJ4ND-m1-7cd0bd60 set-priority actor=human:Wido targets=account-provenance reason=priority-order subject=account-provenance from=unranked to=1:10 requested-sequence=10
- 2026-09-08T17:03:35Z YZH9GDDZJGK4DB0ENK5CGSXADW-m1b-c6925449 done actor=human:Wido targets=account-provenance,actionable-metrics,backlog-ordered-by-priority,breach-clock-and-budget-honesty,breach-stop-wedges-seat,burn-without-delivery-tripwire,delegate-job-liveness,dispatch-cap-necessity,failed-job-attention,fixture-stewards-outlive-their-suite,governed-exhaustion-reprojection,host-runtime-setup,human-goal-verbs-forgiving,job-record-birth-token,lease-sweep-death-evidence,live-job-on-a-done-goal-unnoticed,machine-concurrency-governor,proof-harness-process-custody,review-round-limit-counts-per-chain,severity-tiered-rigor,severity-tiered-rigor-p2,small-change-lane,steward-catchup-livelock,steward-revives-a-done-goal,suite-custody,token-spend-fence,turn-verdict-hardening,watch-verb,watcher-repair-request-never-names-current,winddown-census-handoff-leak reason=priority-order from=1:10 to=1:9
- 2026-09-09T06:01:27Z WBARN8TBQKTC4BW0AQZRW614PS-m1b-c6925449 set-priority actor=human:Wido targets=account-provenance,actionable-metrics,burn-without-delivery-tripwire,delegate-job-liveness,failed-job-attention,fixture-stewards-outlive-their-suite,human-goal-verbs-forgiving,job-record-birth-token,lease-sweep-death-evidence,live-job-on-a-done-goal-unnoticed,machine-concurrency-governor,proof-harness-process-custody,review-round-limit-counts-per-chain,severity-tiered-rigor,severity-tiered-rigor-p2,small-change-lane,steward-catchup-livelock,steward-revives-a-done-goal,stop-batch-strands-a-resumable-goal,suite-custody,token-spend-fence,turn-verdict-hardening,watch-verb,watcher-repair-request-never-names-current,winddown-census-handoff-leak reason=priority-order subject=stop-batch-strands-a-resumable-goal from=1:9 to=1:10 requested-sequence=7
Integrity: sha256=40d2d5ce5ee597afc99e1c9e4f85a8ff4223db3d8b3acc8fba7a1c205cc2009b
