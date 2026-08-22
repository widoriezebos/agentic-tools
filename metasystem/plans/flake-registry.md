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
| S4-2 census custody exact join | supervision-fixtures | 2 | 2026-08-21 | scanSeq drift under load; rerun green |
| adopt placeholder naming | adopt-fixtures | 1 | 2026-08-22 | standalone smoke red, battery's own run green same tree |
