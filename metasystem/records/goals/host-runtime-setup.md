# host-runtime-setup

- State: done
- Priority: 1
- Sequence: 2
- Risk: severity=3 novelty=2 exposure=2 accumulation=1 basis="Severity 3: incorrect lifecycle hooks can misidentify a coordinator or lose stop enforcement. Novelty 2: extend the existing runtime registration and hook owners. Exposure 2: repository host setup and lifecycle entry. Accumulation 1: one shared installation path."
- Tier: 3
- Intent: Make this MetaSystem checkout and adopted installations discoverable and correctly supervised under Claude Code, Devin, and Codex, with repeatable automatic setup and explicit runtime selection, preserving existing Claude behavior and unrelated settings.
- Origin: human
- Next step: Source delivered on origin/main at 8ed97738; full suite and normal landing passed, canonical engine rearmed at generation 27, and all three runtime setup/authentication probes passed. Human administrative closure remains. User has made coordinator-loop-prevention the immediate priority.
- Concluded: Closed on Wido's direction 2026-09-08 ('host-runtime-setup please close') after m1b verified the delivery independently. The source is on main at 8ed97738, 'Support Claude, Codex and Devin under one MetaSystem'. plans/host-runtime-setup-verification.md records the delivery: complete validation own exit 0 after 3380 seconds with all 45 sections green, complete adoption and all nine landing scenarios, both the native implementation chain and the final critic chain closed, and a final Opus 5 xhigh review with zero material findings. m1b's own checks on this checkout rather than on that record: 8ed97738 is an ancestor of main; runtime list reports claude, codex, devin and fake; each adapter resolves its own instruction root, claude to .claude/skills and codex and devin to .agents/skills; and scripts/agents/runtime-hook-fixtures.sh exits 0 covering declared-brain all-host, hinted and unhinted delegate, deadline, cold-host, forged-hint, detached-custody and Devin delivery-repair isolation. No open review obligations and no accepted risks. Known and not closed by this goal, from its own verification page: automatic provider callbacks in an already-running coordinator remain unobserved. m1c did the work; m1b verified and closed it because the goal sat second in the ranked backlog while complete and was distorting the queue for every seat that reads it. This conclusion was typed by an agent-enrolled terminal under ruling R-88-m1b, so it records that Wido directed it, not that he typed it.
- OpenedAt: 2026-09-07T05:41:13Z
- Revision: 11
- Budget: elapsedLimit=9d attemptLimit=24 reservedJobMinutesLimit=2880 activeJobLimit=2 reviewRoundLimit=3
- BudgetExceptions: 2
- NormApproval: approvedRef=R-86-m1c minutes=2880 reviewRounds=3 goalRevision=5
- Approved: by=human:Wido at=2026-09-07T19:03:27Z revision=6 opid=9ZZ7SHCGZPWJMZ3JGHSZA381Z4-m1c-7cd0bd60 authority=proven digest=ded6f46db604b60a1a0988b39282ab1cf914b52edadcc3139971a9e85c76ac62
- Sliced: machine=m1c lineage=main-1788759014-39092-24fa5c revision=3 at=2026-09-07T05:51:33Z

History:
- 2026-09-07T05:41:13Z 627Q493CTPEA321NRM4RG0X5HH-m1c-1274caf4 open actor=m1c+main-1788759014-39092-24fa5c targets=host-runtime-setup
- 2026-09-07T05:42:17Z 40CKH10008EJAV4AR6C7NGC1QX-m1-1274caf4 approve actor=human:Wido targets=host-runtime-setup
- 2026-09-07T05:46:29Z C5QDX4WYB00BW4Q44EQ9V127FQ-m1c-1274caf4 claim actor=m1c+main-1788759014-39092-24fa5c targets=host-runtime-setup
- 2026-09-07T05:51:33Z 9MPT30W94WE6JNW8FGYBXSNZ46-m1c-1274caf4 slice-start actor=m1c+main-1788759014-39092-24fa5c targets=host-runtime-setup
- 2026-09-07T10:34:41Z AS1JDFQSA6T2NJW0BWWHC5ZYVC-m1c-7cd0bd60 set-budget actor=human:Wido targets=host-runtime-setup displaced=m1c+main-1788759014-39092-24fa5c@2026-09-07T05:46:29Z
- 2026-09-07T19:03:27Z 9ZZ7SHCGZPWJMZ3JGHSZA381Z4-m1c-7cd0bd60 set-budget actor=human:Wido targets=host-runtime-setup displaced=m1c+main-1788759014-39092-24fa5c@2026-09-07T10:34:41Z
- 2026-09-08T05:30:30Z 1AJ8YH12YD5FB205WTAC2RKP7N-m1c-1274caf4 edit actor=m1c+main-1788759014-39092-24fa5c targets=host-runtime-setup
- 2026-09-08T05:30:33Z 1SM22NR0S4GA251X9V0GPVX88E-m1c-1274caf4 release actor=m1c+main-1788759014-39092-24fa5c targets=host-runtime-setup
- 2026-09-08T15:55:25Z ZFGJK03RHKCAK03SYJF7ZETS0F-m1-7cd0bd60 set-priority actor=human:Wido targets=host-runtime-setup reason=priority-order subject=host-runtime-setup from=unranked to=1:2 requested-sequence=2
- 2026-09-08T17:01:57Z 67P1H0N6YGT5PCN53JRPFSYSP7-m1b-c6925449 claim actor=m1b+main-1788680071-18713-e76d5d targets=host-runtime-setup
- 2026-09-08T17:03:35Z YZH9GDDZJGK4DB0ENK5CGSXADW-m1b-c6925449 done actor=human:Wido targets=account-provenance,actionable-metrics,backlog-ordered-by-priority,breach-clock-and-budget-honesty,breach-stop-wedges-seat,burn-without-delivery-tripwire,delegate-job-liveness,dispatch-cap-necessity,failed-job-attention,fixture-stewards-outlive-their-suite,governed-exhaustion-reprojection,host-runtime-setup,human-goal-verbs-forgiving,job-record-birth-token,lease-sweep-death-evidence,live-job-on-a-done-goal-unnoticed,machine-concurrency-governor,proof-harness-process-custody,review-round-limit-counts-per-chain,severity-tiered-rigor,severity-tiered-rigor-p2,small-change-lane,steward-catchup-livelock,steward-revives-a-done-goal,suite-custody,token-spend-fence,turn-verdict-hardening,watch-verb,watcher-repair-request-never-names-current,winddown-census-handoff-leak
Integrity: sha256=03ee4add5737b07192b956efd6fd5fea90a299dad6f2816168fc2b65add321de
