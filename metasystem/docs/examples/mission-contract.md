# Mission Contract Example

This is a draft contract shape, not signed authority. Replace the repository facts, run `scripts/assert-mission.sh --file <path>`, price the exposure, run `--seal`, commit the signed bytes to the shared default branch, arm supervision, and then run `--preflight`.

# Intent

Make the metasystem validation command pass while preserving its existing behavioral coverage.

# Non-goals

Do not publish, deploy, rewrite history, or change external API contracts.

# Initial streams

- `validation`: remove the named baseline failures.
- `documentation`: keep the operating contract replayable from shipped documentation.

```mission
gate.command=bash -c 'scripts/validate-metasystem.sh; status=$?; printf "metric=validation=%s\n" "$status"; exit 0'
gate.ref=instruments-v1   # a tag or other non-moving ref: sealing against a branch cannot survive its own signing commit
gate.paths=scripts/validate-metasystem.sh
truth.paths=docs/**/*.md
truth.certification=certified
gate.direction=min
gate.threshold.validation=<=0
gate.noise-floor.validation=0
guard.audit.command=bash -c 'scripts/audit-metasystem.sh .; status=$?; passed=$((status == 0)); printf "metric=audit=%s\n" "$passed"; exit 0'
guard.audit.floor=1
guard.audit.noise=0
guard.cadence=1
ledger.cycle-budget=8
ledger.no-gain-budget=8
fence.wall-clock-hours=8
fence.cycles=8
fence.jobs=20
fence.concurrency=2
fence.job-cap-min=120
host.runtime=claude
host.model=project-default
host.turn-cap-min=60
stream.validation=Make scripts/validate-metasystem.sh exit zero.
stream.documentation=Keep mission operation replayable from shipped documentation.
envelope.dependencies=jq
exposure=EUR:25
```

Size `ledger.no-gain-budget` in the order of `fence.cycles`: the stop-loss is a last defense sized above any healthy runway, not a pace-setter, and the contract validator warns — never refuses — below half the cycle fence (`docs/design/stop-loss-core.md`).

After sealing, the script appends a generated `mission-seal` block. The human then adds exactly one line in this form, using the hash printed by `--seal`:

```text
Example only — remove this prefix: Approval: name=Example Person; date=2026-08-04; contract-sha256=<printed hash>
```
