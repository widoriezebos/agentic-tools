# Mission Contract Example

This is a draft contract shape, not signed authority. Replace the repository facts, run `scripts/assert-mission.sh --file <path>`, price the exposure, run `--seal`, commit the signed bytes to the shared default branch, arm supervision, and then run `--preflight`.

# Intent

Make the harness validation command pass while preserving its existing behavioral coverage.

# Non-goals

Do not publish, deploy, rewrite history, or change external API contracts.

# Initial streams

- `validation`: remove the named baseline failures.
- `documentation`: keep the operating contract replayable from shipped documentation.

```mission
gate.command=bash -c 'scripts/validate-harness.sh; status=$?; printf "metric=validation=%s\n" "$status"; exit 0'
gate.ref=main
gate.paths=scripts/validate-harness.sh
truth.paths=docs/**/*.md
truth.certification=certified
gate.direction=min
gate.threshold.validation=<=0
gate.noise-floor.validation=0
guard.audit.command=bash -c 'scripts/audit-harness.sh .; status=$?; passed=$((status == 0)); printf "metric=audit=%s\n" "$passed"; exit 0'
guard.audit.floor=1
guard.audit.noise=0
guard.cadence=1
ledger.cycle-budget=8
ledger.no-gain-budget=3
fence.wall-clock-hours=8
fence.cycles=8
fence.jobs=20
fence.concurrency=2
fence.job-cap-min=120
host.runtime=claude
host.model=project-default
host.turn-cap-min=60
stream.validation=Make scripts/validate-harness.sh exit zero.
stream.documentation=Keep mission operation replayable from shipped documentation.
envelope.dependencies=jq
exposure=EUR:25
```

After sealing, the script appends a generated `mission-seal` block. The human then adds exactly one line in this form, using the hash printed by `--seal`:

```text
Example only — remove this prefix: Approval: name=Example Person; date=2026-08-04; contract-sha256=<printed hash>
```
