# steward-launches-resolve-their-model

- State: approved
- Priority: 1
- Sequence: 2
- Risk: severity=2 novelty=1 exposure=2 accumulation=1 basis="severity 2: steward continuation has delivered nothing on any seat since the launches die; novelty 1: the roster resolution exists for delegates; exposure 2: steward launches on every seat; accumulation 1: one code path"
- Tier: 2
- Intent: Steward continuation launches on m1d and m1e have died within three seconds since 2026-09-09 13:57Z with the API error that the literal model name '<model>' is not supported: nine launches, first steward-7d27d52dc2060f32, still failing 2026-09-11 15:52Z (codex-sessions.md section 3 of the delivery deep dive). The roster placeholder is passed to Codex unresolved. DONE means: a steward launch resolves its model from the configured roster before spawning and refuses with a named configuration error when it cannot, never spawning with a placeholder; a fixture proves a launch with the metasystem.conf roster resolves; every seat's steward launch has completed at least once after the fix. Goal 2 of plans/delivery-efficiency-plan.md.
- Origin: human
- Next step: Find where the steward composes its delegate command (grep the '<model>' placeholder under scripts/agents and internal), resolve it through the same roster path as dispatch, add the refusal and the fixture, land.
- OpenedAt: 2026-09-11T15:44:54Z
- Revision: 4
- Pinned: m1e
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-11T15:49:03Z revision=4 opid=N6S7EPVKSG4ZJZWHAD0ATBM8G6-m1-c6925449 authority=proven digest=fd655893deb9442f90a50dc03976617cac2c743459ae308e97b07557c0e018d1

History:
- 2026-09-11T15:44:54Z HF9CZF90NBZDKY1K5Z5G13AQZ9-m1e-5083721b open actor=m1e+main-1789141490-27414-b9c0d2 targets=steward-launches-resolve-their-model
- 2026-09-11T15:46:48Z XQVXPJ8T68PBAV19RD08435JFK-m1-c6925449 set-pin actor=human:Wido targets=steward-launches-resolve-their-model
- 2026-09-11T15:46:51Z GKZNG4MZDNDNVV57EFNJTGXMXF-m1-c6925449 set-priority actor=human:Wido targets=actionable-metrics,breach-clock-and-budget-honesty,burn-without-delivery-tripwire,carried-landing-debt-and-cap,delegate-job-liveness,failed-job-attention,fixture-stewards-outlive-their-suite,goal-abandoned-with-a-reason,human-carried-landing-carry,human-carried-landing-verb,human-goal-verbs-forgiving,job-record-birth-token,landing-receipt-survives-records-drift,lease-sweep-death-evidence,live-job-on-a-done-goal-unnoticed,machine-concurrency-governor,proof-harness-process-custody,severity-tiered-rigor,severity-tiered-rigor-p2,small-change-lane,steward-catchup-livelock,steward-launches-resolve-their-model,steward-revives-a-done-goal,stop-batch-strands-a-resumable-goal,suite-custody,token-spend-fence,turn-verdict-hardening,watch-verb,watcher-repair-request-never-names-current,winddown-census-handoff-leak reason=priority-order subject=steward-launches-resolve-their-model from=unranked to=1:2 requested-sequence=2
- 2026-09-11T15:49:03Z N6S7EPVKSG4ZJZWHAD0ATBM8G6-m1-c6925449 approve actor=human:Wido targets=steward-launches-resolve-their-model
Integrity: sha256=582d53d6b64343ca50085fcacf435a9807f8266bdd7d9f53efeca9b6649a2b50
