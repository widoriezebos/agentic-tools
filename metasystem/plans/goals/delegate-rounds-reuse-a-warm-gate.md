# delegate-rounds-reuse-a-warm-gate

- State: queued
- Risk: severity=2 novelty=1 exposure=2 accumulation=2 basis="severity 2: a skipped gate that should have run lets a red round close; novelty 1: caches and the fast gate exist; exposure 2: delegate rounds; accumulation 2: touches dispatch and the gate"
- Tier: 3
- Intent: Codex delegate jobs ran 587 go-gate invocations at 14 to 25 minutes each from a cold build cache per job, verification was 55 percent of delegate tool time (48.6 hours), and fold rounds that changed only markdown ran the full gate (codex-sessions.md section 4 of the delivery deep dive). DONE means: the rounds of one chain share a build cache keyed by the chain root; a fold round whose diff touches no Go or shell input of the gate skips the gate and records why; a round's gate runs the fast gate unless the brief names deep; proven by a chain of three rounds whose second and third gates finish in under three minutes. Goal 12 of plans/delivery-efficiency-plan.md.
- Origin: human
- Next step: Read the job environment in scripts/agents/dispatch.sh (the per-job GOCACHE), go-gate.sh --fast and the round brief format; design, critique, build, land.
- OpenedAt: 2026-09-11T15:46:36Z
- Revision: 1
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0

History:
- 2026-09-11T15:46:36Z ESAZ2BX9DC37VFYCNAXPCDG92H-m1-c6925449 open actor=human:Wido targets=delegate-rounds-reuse-a-warm-gate reason=TierOverride: derived=2 set=3 why=Wido 2026-09-11: the gate is the delegate's judge; a skipped gate is a tier-3 hazard whatever the four answers derive
Integrity: sha256=e036ecb17ab8bffc4b3fc27cdcd00f0a61639227c6540397a55ab90d50cb97fa
