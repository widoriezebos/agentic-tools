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
| S4-2 census custody exact join | supervision-fixtures | 3 | 2026-08-23 | scanSeq drift under load; rerun green; THIRD sighting inside 30 days (inside the adopt suite nested validation, busy machine) — fix goal s4-2-census-join OPENED per the protocol |
| adopt placeholder naming | adopt-fixtures | 1 | 2026-08-22 | standalone smoke red, battery's own run green same tree |

## Unnamed sightings

- 2026-08-23: composite `go test ./internal/validate/ ./internal/events/
  ./internal/missionrunner/` returned rc=1 once during the wall-o10
  increment-one landing; output was discarded by an rc-only capture
  (`>/dev/null`), so the leg is unnamed. Four immediate re-runs green.
  If a named leg in these packages reds within 30 days, join it to
  this row. Lesson applied: gate runs capture output to a file first —
  an rc without its evidence cannot be diagnosed.
