# stop-infrastructure-allows-the-seat-to-stop: code critique, round 2 of the DESIGN-BEARING root

Working Mode: design

Goal stop-infrastructure-allows-the-seat-to-stop. Round 1 of this chain
verified SIC-01 to SIC-03 resolved on the landed commit 529d8a649 and
returned SIC-06 and SIC-07 (material) with SIC-08 and SIC-09 (not
material). The seat folded all four and landed the fix-forward as the
commit named in the goal's next-step line (the child of 529d8a649 that
touches `metasystem/scripts/agents/supervision-hook.sh` and
`metasystem/internal/goal/sessionstop.go`). This round verifies that fold
against the LANDED tree on origin/main.

## What to verify

1. SIC-06: in `metasystem/scripts/agents/supervision-hook.sh`, the
   deadline parent's expiry branch prints its record-failure allowance
   with `emit_fixed_json_notice` (shell printf, no engine), and the
   log-failure prefix path falls back to fixed text when the engine cannot
   carry the prefix; the parent exits 0 with exactly one JSON object when
   the installation's `bin/metasystem` is absent. The leg after the second
   deadline run in `metasystem/scripts/agents/supervision-hook-fixtures.sh`
   moves the engine aside and asserts that.
2. SIC-07: in `metasystem/internal/goal/sessionstop.go`, a registry write
   that landed with unproven durability
   (`errSessionStopRegistryDurabilityUnknown`) is treated as consumed with
   a detail saying the record's durability is unknown, and the verdict
   then allows the authorized stop once; the next quiet stop is refused as
   already consumed. Test `TestSessionStopConsumeWithUnknownDurabilityIsSpent`
   in `metasystem/internal/goal/turnverdict_idle_test.go`.
3. SIC-08: `mkdir -p "$supervision_dir" 2>/dev/null || true` before the
   advisor exit.
4. SIC-09: the infrastructure detail in `compose_failed_stop` no longer
   says "could not prove that stopping is safe"; the unavailable-verdict
   allowance says stopping is allowed and names the owner; the template in
   `metasystem/cmd/metasystem/runtime_setup_test.go` carries the shipped
   degraded fallback; `emit_raw_stop_allowance` is the raw allowance.

## Return

Findings sorted by materiality against the landed tree. A finding is
material only if it names a path where the landed code still fails one of
the four items above or introduces a new defect. Mark SIC-06 and SIC-07
resolved or still open with the evidence read or run. The design page is
`metasystem/plans/stop-infrastructure-allows-the-seat-to-stop-design.md`.
