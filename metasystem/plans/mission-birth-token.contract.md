# Intent

Land goal job-record-birth-token: every job record incarnation carries a
mandatory, immutable, machine-minted birth token, proven by the six
fixtures the landed design names (plans/job-record-birth-token-design.md
section 6). Work through the metasystem's own ladder: Sol critique of the
design, Sol build in the internal/dispatch record writers, Fable code
critique, landing with --chain. This is the fleet's first headless run
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
stream.primary=Land job-record-birth-token with its six fixtures green through the ladder.
envelope.dispatch-allow=codex:gpt-5.6-sol,claude:claude-fable-5-1
exposure=EUR:40
```
