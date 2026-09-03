# Intent

Land goal job-record-birth-token: every job record incarnation carries a
mandatory, immutable, machine-minted birth token, proven by the six
fixtures the landed design names (plans/job-record-birth-token-design.md
section 6). Work through the metasystem's own ladder: Sol critique of the
design, Sol build in the internal/dispatch record writers, Fable code
critique, landing with --chain. Wido's stop rule (R-42-m0) binds: no
critique loop exceeds three rounds; a fourth is a park and an ask. This is the fleet's first headless run
(goal first-headless-run, plans/first-headless-run-plan.md): the runner
serves this goal with no Claude session on top.

# Non-goals

Do not change anything outside internal/dispatch and the two named
fixture scripts (scripts/agents/record-protocol-fixtures.sh,
scripts/agents/return-schema-fixtures.sh). Do not publish or deploy. Do
not alter the gate instruments under plans/first-headless-run/.

# Initial streams

Keep one stream active.

```mission
gate.command=bash plans/first-headless-run/gate.sh
gate.ref=first-headless-run-instruments
gate.paths=plans/first-headless-run/*.sh
truth.paths=plans/first-headless-run/fixtures.txt
truth.certification=candidate
gate.direction=max
gate.threshold.birth-token-fixtures=>=6
gate.noise-floor.birth-token-fixtures=0
guard.build.command=bash plans/first-headless-run/guard.sh
guard.build.floor=1
guard.build.noise=0
guard.cadence=1
ledger.cycle-budget=6
ledger.no-gain-budget=3
fence.wall-clock-hours=8
fence.cycles=6
fence.jobs=8
fence.concurrency=1
fence.job-cap-min=180
host.runtime=claude
host.model=claude-fable-5-1
host.turn-cap-min=60
host.max-turns=40
stream.primary=Land job-record-birth-token with its six fixtures green through the tier-3 ladder: Codex critique of the landed Fable design, one fold, one closing review, Sol build, Fable code review, landing with --chain. STOP CRITERION (R-42-m0): at most THREE critique rounds - park and ask Wido rather than dispatch a fourth.
envelope.dispatch-allow=codex:gpt-5.6-sol,claude:claude-fable-5-1
exposure=EUR:40
```

```mission-seal
sealed.version=1
sealed.at=2026-09-03T15:39:47Z
candidate.branch=main
sealed.gate-ref-sha=512c0226456d267d57de0920b7a48e94bb41461e
sealed.gate-integrity.sha256=f933d4bd74330818b7ab767b15e41021e2539f8572d9623c1ea69b0a39ffed61
sealed.truth-integrity.sha256=8836b1bc1d7d16ff7a35302c4b24dc709409dea8c8b62e56002a3c82797a59e1
sealed.baseline.candidate-sha=512c0226456d267d57de0920b7a48e94bb41461e
sealed.baseline.failure-count=1
sealed.baseline.failure-identifiers=unavailable
sealed.baseline.birth-token-fixtures=0
sealed.exposure.fence.wall-clock-hours=8
sealed.exposure.fence.cycles=6
sealed.exposure.fence.jobs=8
sealed.exposure.fence.concurrency=1
sealed.exposure.fence.job-cap-min=180
sealed.exposure.statement=EUR:40|fence.wall-clock-hours=8,fence.cycles=6,fence.jobs=8,fence.concurrency=1,fence.job-cap-min=180
```
Approval: name=Wido; date=2026-09-03; contract-sha256=7e53aa1977e3f43148174ecb51a07701a04f4060ca5ed194f788fc1bccef7559
