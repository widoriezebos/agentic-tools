# stop-infrastructure-allows-the-seat-to-stop: code critique, round 3 of the DESIGN-BEARING root

Working Mode: design

Goal stop-infrastructure-allows-the-seat-to-stop. Round 2 of this chain
verified SIC-06 and SIC-07 resolved on fed9f5d9e in prose (its SIC-14) and
returned SIC-11 (material, low: the log-failure prefix fallback printed two
JSON objects) with SIC-12 and SIC-13 (not material). The seat folded
SIC-11 and SIC-13's first note and landed them as the commit named in the
goal's next-step line (the child of fed9f5d9e touching
`metasystem/scripts/agents/supervision-hook.sh` and
`metasystem/internal/goal/sessionstop.go`). This round verifies that fold
against the LANDED tree on origin/main and closes the register.

## The register needs findings, not prose

The finding register marks an earlier finding resolved only when a later
round returns a finding with the SAME identifier and `material: false`.
Round 2's prose verdict left SIC-06 and SIC-07 open. Return, as findings:

- `SIC-06` with `material: false` if the deadline parent prints its
  record-failure allowance without the engine (`emit_fixed_json_notice`)
  and the no-engine bed leg in
  `metasystem/scripts/agents/supervision-hook-fixtures.sh` holds;
- `SIC-07` with `material: false` if a registry write of unproven
  durability is treated as spent (`errSessionStopRegistryDurabilityUnknown`
  in `metasystem/internal/goal/sessionstop.go`,
  `TestSessionStopConsumeWithUnknownDurabilityIsSpent` in
  `metasystem/internal/goal/turnverdict_idle_test.go`);
- `SIC-11` with `material: false` if the prefix fallback in the expiry
  branch now emits exactly one JSON object (one `emit_fixed_json_notice`
  carrying the log failure and the deadline's fixed words) and the
  refusing-json-engine bed leg holds.

Any of the three still failing stays `material: true` under its own
identifier with the evidence. A new defect gets a new identifier. Do not
report a prose verdict as a finding.

The design page is
`metasystem/plans/stop-infrastructure-allows-the-seat-to-stop-design.md`.
