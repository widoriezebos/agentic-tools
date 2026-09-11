# delegate-rounds-reuse-a-warm-gate

- State: queued
- Priority: 1
- Sequence: 12
- Risk: severity=2 novelty=1 exposure=2 accumulation=2 basis="severity 2: a skipped gate that should have run lets a red round close; novelty 1: caches and the fast gate exist; exposure 2: delegate rounds; accumulation 2: touches dispatch and the gate"
- Tier: 3
- Intent: Codex delegate jobs ran 587 go-gate invocations at 14 to 25 minutes each from a cold build cache per job, verification was 55 percent of delegate tool time (48.6 hours), and fold rounds that changed only markdown ran the full gate (codex-sessions.md section 4 of the delivery deep dive). DONE means: the rounds of one chain share a build cache keyed by the chain root; a fold round whose diff touches no Go or shell input of the gate skips the gate and records why; a round's gate runs the fast gate unless the brief names deep; proven by a chain of three rounds whose second and third gates finish in under three minutes. Goal 12 of plans/delivery-efficiency-plan.md.
- Origin: human
- Next step: Read the job environment in scripts/agents/dispatch.sh (the per-job GOCACHE), go-gate.sh --fast and the round brief format; design, critique, build, land.
- OpenedAt: 2026-09-11T15:46:36Z
- Revision: 3
- Pinned: m1e
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0

History:
- 2026-09-11T15:46:36Z ESAZ2BX9DC37VFYCNAXPCDG92H-m1-c6925449 open actor=human:Wido targets=delegate-rounds-reuse-a-warm-gate reason=TierOverride: derived=2 set=3 why=Wido 2026-09-11: the gate is the delegate's judge; a skipped gate is a tier-3 hazard whatever the four answers derive
- 2026-09-11T15:48:02Z YWW2DBP7XHJ494RAHNKHH24MRC-m1-c6925449 set-pin actor=human:Wido targets=delegate-rounds-reuse-a-warm-gate
- 2026-09-11T15:48:05Z RZY6J8B2FFGR3BVCN0PRD14C8P-m1-c6925449 set-priority actor=human:Wido targets=actionable-metrics,breach-clock-and-budget-honesty,burn-without-delivery-tripwire,carried-landing-debt-and-cap,delegate-job-liveness,delegate-rounds-reuse-a-warm-gate,failed-job-attention,fixture-stewards-outlive-their-suite,goal-abandoned-with-a-reason,human-carried-landing-verb,human-goal-verbs-forgiving,job-record-birth-token,landing-receipt-survives-records-drift,lease-sweep-death-evidence,live-job-on-a-done-goal-unnoticed,machine-concurrency-governor,proof-harness-process-custody,severity-tiered-rigor,severity-tiered-rigor-p2,small-change-lane,steward-catchup-livelock,steward-revives-a-done-goal,stop-batch-strands-a-resumable-goal,suite-custody,token-spend-fence,turn-verdict-hardening,watch-verb,watcher-repair-request-never-names-current,winddown-census-handoff-leak reason=priority-order subject=delegate-rounds-reuse-a-warm-gate from=unranked to=1:12 requested-sequence=12
Integrity: sha256=0211208b9f9e443667aa982c72a788e51ec729e647b76d6dba40397dab96d1f6
