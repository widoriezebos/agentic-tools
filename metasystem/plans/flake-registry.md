# Flake registry

Known flaky fixture legs as repository data — sighting counts and
dates, not one agent's memory. The protocol lives in
docs/flake-registry.md; the table is the record.

An entry earns a FIX goal at three sightings inside thirty days.
Remove an entry when its leg is fixed or three quiet months pass.

| Leg | Suite | Sightings | Last seen | Notes |
|---|---|---|---|---|
| fence job-cap-min ask | dispatch-fixtures | 2 | 2026-08-20 | pre-registry count carried from memory |
| silent exit-2 | supervision-fixtures | 1 | 2026-08-21 | no output captured; standalone rerun green |
| dispatch fence batched-ask assert | dispatch-fixtures | 1 | 2026-08-23 | the reap's fence-bound ask lands asynchronously to the driver's exit; the python heredoc's interpreter startup masked the window and the shell conversion exposed it; FIXED same day with a bounded wait in assert_fence_ask (10s ceiling, same failure message) — evidence artifacts/agents/suite-failures/20260823T022449Z-dispatch-39428 |
| S4-2 census custody exact join | supervision-fixtures | 3 | 2026-08-23 | scanSeq drift under load; rerun green; THIRD sighting inside 30 days (inside the adopt suite nested validation, busy machine) — fix goal s4-2-census-join CONCLUDED 2026-08-23: failing census waits now dump their snapshot into the preserved evidence, the fixture record edits are atomic (torn-read candidate eliminated), standalone and deliberate-load runs green; the mechanism is UNPROVEN until a sighting arrives with its snapshot — if one does, reopen from that evidence |
| adopt placeholder naming | adopt-fixtures | 1 | 2026-08-22 | standalone smoke red, battery's own run green same tree |
| acp turn late-window traffic | acp go test | 2 | 2026-08-24 | TestTurnLateWindowTraffic expected a post-window permission-request violation and saw none — twice in one day (custody slice-one suite, covenant rework suite); untouched package, timing-shaped window test; solo reruns -count=3 green both times; ONE MORE sighting inside 30 days earns the fix goal |
| nested-checkout mission birth cleanup | missionrunner go test | 1 | 2026-08-24 | TempDir RemoveAll raced .git/objects repopulation (background git maintenance outliving the bed) during the o19 slice-B full suite; the leg's own assertions all passed; solo rerun -count=3 green; unrelated to the slice's delta (ask/evidence note plumbing) |

## Unnamed sightings

- 2026-08-23: composite `go test ./internal/validate/ ./internal/events/
  ./internal/missionrunner/` returned rc=1 once during the wall-o10
  increment-one landing; output was discarded by an rc-only capture
  (`>/dev/null`), so the leg is unnamed. Four immediate re-runs green.
  If a named leg in these packages reds within 30 days, join it to
  this row. Lesson applied: gate runs capture output to a file first —
  an rc without its evidence cannot be diagnosed.
