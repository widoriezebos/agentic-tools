# steward-launches-resolve-their-model

- State: queued
- Priority: 1
- Sequence: 2
- Risk: severity=2 novelty=1 exposure=2 accumulation=1 basis="severity 2: steward continuation has delivered nothing on any seat since the launches die; novelty 1: the roster resolution exists for delegates; exposure 2: steward launches on every seat; accumulation 1: one code path"
- Tier: 2
- Intent: Steward continuation launches on m1d and m1e have died within three seconds since 2026-09-09 13:57Z with the API error that the literal model name '<model>' is not supported: nine launches, first steward-7d27d52dc2060f32, still failing 2026-09-11 15:52Z (codex-sessions.md section 3). The resolver already runs before staging a steward launch (cmd/metasystem/steward_verbs.go) and accepts the configured fallback, which is itself the template value role.default.model.codex=<model> (metasystem.conf), so a seat whose roster leaves the steward role unconfigured launches with the placeholder. DONE means: a launch whose resolved model is a template value is refused before spawning, naming the config key and the one command to set it; the shipped metasystem.conf carries no template as the fallback of a role that launches; each seat's roster names the steward role's concrete model; a fixture proves an unconfigured template refuses before launch and a configured steward completes with the expected model; every seat's steward launch has completed once after the fix. Goal 2 of plans/delivery-efficiency-plan.md.
- Origin: human
- Next step: Read steward_verbs.go launch staging and internal/dispatch/roster.go fallback acceptance; add the template refusal and the config validation; fix metasystem.conf and each seat's roster; fixture; land.
- OpenedAt: 2026-09-11T15:44:54Z
- Revision: 6
- Pinned: m1e
- BudgetExceptions: 0

History:
- 2026-09-11T15:44:54Z HF9CZF90NBZDKY1K5Z5G13AQZ9-m1e-5083721b open actor=m1e+main-1789141490-27414-b9c0d2 targets=steward-launches-resolve-their-model
- 2026-09-11T15:46:48Z XQVXPJ8T68PBAV19RD08435JFK-m1-c6925449 set-pin actor=human:Wido targets=steward-launches-resolve-their-model
- 2026-09-11T15:46:51Z GKZNG4MZDNDNVV57EFNJTGXMXF-m1-c6925449 set-priority actor=human:Wido targets=actionable-metrics,breach-clock-and-budget-honesty,burn-without-delivery-tripwire,carried-landing-debt-and-cap,delegate-job-liveness,failed-job-attention,fixture-stewards-outlive-their-suite,goal-abandoned-with-a-reason,human-carried-landing-carry,human-carried-landing-verb,human-goal-verbs-forgiving,job-record-birth-token,landing-receipt-survives-records-drift,lease-sweep-death-evidence,live-job-on-a-done-goal-unnoticed,machine-concurrency-governor,proof-harness-process-custody,severity-tiered-rigor,severity-tiered-rigor-p2,small-change-lane,steward-catchup-livelock,steward-launches-resolve-their-model,steward-revives-a-done-goal,stop-batch-strands-a-resumable-goal,suite-custody,token-spend-fence,turn-verdict-hardening,watch-verb,watcher-repair-request-never-names-current,winddown-census-handoff-leak reason=priority-order subject=steward-launches-resolve-their-model from=unranked to=1:2 requested-sequence=2
- 2026-09-11T15:49:03Z N6S7EPVKSG4ZJZWHAD0ATBM8G6-m1-c6925449 approve actor=human:Wido targets=steward-launches-resolve-their-model
- 2026-09-11T16:13:00Z RGKPCT8NZ7YN5YW8MZJ19MJ8N5-m1-c6925449 unapprove actor=human:Wido targets=steward-launches-resolve-their-model reason=Wido 2026-09-11: Codex critique round 1 of the delivery-efficiency plan accepted; intent amended before re-approval
- 2026-09-11T16:16:21Z 4NQENWEFHQWYXEGGYR3WT3NQHQ-m1e-ab5421f5 edit actor=m1e+main-1789143377-71578-b9c0d2 targets=steward-launches-resolve-their-model
Integrity: sha256=0f0792c3ddda800c841490318333030b0c05abdbe13d4952d5070d3f6d909635
