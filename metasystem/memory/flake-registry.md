# Flake registry

Known flaky fixture legs as repository data — sighting counts and
dates, not one agent's memory. The protocol lives in
docs/flake-registry.md; the table is the record.

An entry earns a FIX goal at three sightings inside thirty days.
Remove an entry when its leg is fixed or three quiet months pass.

| Leg | Suite | Sightings | Last seen | Notes |
|---|---|---|---|---|
| S4 takeover arm OWNED-ELSEWHERE under detached launch | supervision-fixtures | 6 | 2026-08-24 | DETERMINISTIC and POSTURE-KEYED, not timing: a suite launched detached from any announced session ancestry (nohup, reparented to launchd) refuses at the S4-4 takeover re-arm — arm-supervision announces then requires holdership, and a reparented shell's ambient classification is UNTRUSTED. Six-for-six red when detached (three batteries, two standalone, one last-green-bytes worktree control — code exonerated, real supervision set live/down made no difference); green S4-1..16 same day, same bytes, launched with the session in ancestry. Fix owned by the suite-custody goal (suites must pass detached: cron, steward revive, CI are detached launchers); KI-43 carries the class |
| overloaded-host breaker park | missionrunner go test (nested adopt bed) | 1 | 2026-08-25 | TestInternalRunOverloadedHostStaysOffTheBreaker parked host-failure under the doubly-nested adopt suite on a busy machine (38s breaker-timing test, landed 2026-08-24 in provider-outage-posture); standalone -count=3 green (~35s each); the failing run's delta was kit/schema/docs only — no engine-behavior connection | first sighting |
| fence job-cap-min ask | dispatch-fixtures | 3 | 2026-08-26 | pre-registry count carried from memory; THIRD sighting 2026-08-26 in the batch battery (evidence suite-failures/20260826T035542Z-dispatch-36491, solo rerun green) — PROMOTED per the three-strike law to fix goal fence-ask-flake; FIXED 2026-08-26 — the true cause was three reapers, one hook: only the shell dispatch reap raised the mission fence ask on a budget-cap timeout (and only when its own CAS won); the Go standing reaper and the mission runner's drain reap stamped timeout with no ask, so whichever lost the race left the mission parked ask-less. All three paths now raise the batched ask through one shared mechanism, applied-only, loud on failure |
| silent exit-2 | supervision-fixtures | 1 | 2026-08-21 | no output captured; standalone rerun green |
| dispatch fence batched-ask assert | dispatch-fixtures | 1 | 2026-08-23 | the reap's fence-bound ask lands asynchronously to the driver's exit; the python heredoc's interpreter startup masked the window and the shell conversion exposed it; FIXED same day with a bounded wait in assert_fence_ask (10s ceiling, same failure message) — evidence artifacts/agents/suite-failures/20260823T022449Z-dispatch-39428 |
| S4-2 census custody exact join | supervision-fixtures | 3 | 2026-08-23 | scanSeq drift under load; rerun green; THIRD sighting inside 30 days (inside the adopt suite nested validation, busy machine) — fix goal s4-2-census-join CONCLUDED 2026-08-23: failing census waits now dump their snapshot into the preserved evidence, the fixture record edits are atomic (torn-read candidate eliminated), standalone and deliberate-load runs green; the mechanism is UNPROVEN until a sighting arrives with its snapshot — if one does, reopen from that evidence |
| adopt placeholder naming | adopt-fixtures | 1 | 2026-08-22 | standalone smoke red, battery's own run green same tree |
| acp turn late-window traffic | acp go test | 3 | 2026-08-29 | TestTurnLateWindowTraffic expected a post-window permission-request violation and saw none — twice in one day (custody slice-one suite, covenant rework suite); untouched package, timing-shaped window test; solo reruns -count=3 green both times; ONE MORE sighting inside 30 days earns the fix goal |
| nested-checkout mission birth cleanup | missionrunner go test | 1 | 2026-08-24 | TempDir RemoveAll raced .git/objects repopulation (background git maintenance outliving the bed) during the o19 slice-B full suite; the leg's own assertions all passed; solo rerun -count=3 green; unrelated to the slice's delta (ask/evidence note plumbing) |

## Unnamed sightings

- 2026-08-23: composite `go test ./internal/validate/ ./internal/events/
  ./internal/missionrunner/` returned rc=1 once during the wall-o10
  increment-one landing; output was discarded by an rc-only capture
  (`>/dev/null`), so the leg is unnamed. Four immediate re-runs green.
  If a named leg in these packages reds within 30 days, join it to
  this row. Lesson applied: gate runs capture output to a file first —
  an rc without its evidence cannot be diagnosed.

## goal-txn-rejected-publishes (first sighting 2026-08-26)
- Test: internal/goal TestValidationRefusalIsRejectedByName (txn_test.go:287, "a rejected transaction publishes nothing").
- Sightings: 1 (2026-08-26 12:22Z, nested go gate inside the adopt bed, package runtime 229s under three concurrent work lanes; evidence artifacts/agents/suite-failures/20260826T122246Z-adopt-28021).
- Mechanism hypothesis: timing-sensitive transaction assertion under heavy load in adopted-mode nested gates; passes 3x in 1.4s on the same tree unloaded.
- Owner: flake protocol — a FIX goal at three sightings inside thirty days.
| goal/TestValidationRefusalIsRejectedByName | 2026-08-29 | 1 | substring "bad" asserted against full ls-remote output; SHA column is hex and contains "bad" ~1% of runs | FIXED 2026-08-29 same day: assertion judges the ref-name column only |
| steward/TestRunLoopTicksUntilTheStopFile | 2026-08-29 | 2 | tick-count assertion + TempDir teardown race with the live tick goroutine; surfaces under full-parallel race-gate CPU load (~120%), passes on lightly loaded host | steward is m1's ground (L6/L7): needs a deterministic stop handshake before teardown |
| adopt/go-supervision purpose-gone teardown | 2026-08-29 | 1 | purpose-gone terminal with complete teardown not observed under post-gate load inside the adopt suite | listed area (suite-flake-supervision-watch triage goal); solo rerun per protocol |
| census enumerate-filter-resolve UNRESOLVED-CWD | 2026-08-29 | 2 | fake-agent cwd unresolvable under post-gate load (census-settle class); nested adopt validate red | second sighting same day; third promotes a fix goal per protocol |
