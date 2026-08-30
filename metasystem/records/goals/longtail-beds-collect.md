# longtail-beds-collect

- State: done
- Intent: Ruling P long tail: the low-frequency fixture beds (mission, telemetry-census, return-schema, land, lease-succession, evidence-segment, flight-recorder, witness-gate, enumerate-suite, record-protocol, second-session, gate-run-freeze) still halt at first red; each runs rarely and is short, so payback sits outside the current program horizon per the payback law
- Origin: human
- Next step: Appetite: 2h — apply the scenario-level continue-and-collect pattern from the four converted beds; batch as one pass
- Concluded: Landed a3b0aaf: five beds converted to the continue-and-collect scenario harness (return-schema, land, witness-gate, mission, second-session), harness extracted to its one owner (scripts/agents/fixture-bed-scenarios.sh, sourced not copied). Two real couplings found by verification and fixed (mission supervisor-facts fabrication shared; witness frozen-helpers hoisted); one hazard recorded: bash reported an aborted child (unbound var at export) as rc=0, which had silently skipped mission's whole end-state scenario - the coupling hunt caught it. Recorded verdicts, not silent trims: six single-flow beds (telemetry-census, lease-succession, evidence-segment, flight-recorder, enumerate-suite, record-protocol) have one scenario each - nothing to continue past, conversion adds cost without learning; gate-run-freeze (11 sub-fixtures, tightly shared subject repo) needs its own slice with the mission-style coupling hunt.
- OpenedAt: 2026-08-29T05:45:22Z
- Revision: 4
- Budget: elapsedLimit=2h attemptLimit=4 reservedJobMinutesLimit=30 activeJobLimit=1

History:
- 2026-08-29T05:45:22Z ZG6ZT068QJ08M6CFWF76C8VGYB-m1-bf243850 open actor=human:wido targets=longtail-beds-collect
- 2026-08-29T23:51:21Z AV16RMBK77PJKKDBVJV3YMT0WA-m2-bc1be9cb set-budget actor=human:wido targets=longtail-beds-collect
- 2026-08-29T23:51:40Z JFNWDA9J8YXMGF0G6B64N8QWXY-m2-bc1be9cb claim actor=m2+mac-coordinator targets=longtail-beds-collect
- 2026-08-30T00:02:27Z QRSDPPKQ75AWGN5HAZEMKNKRM8-m2-bc1be9cb done actor=human:wido targets=longtail-beds-collect
Integrity: sha256=d11339650092cf6858e4c7e3425e3b7626814c6b88f7a603bd1ec553dd04ab61
