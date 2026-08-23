# s4-2-census-join

- State: claimed
- Intent: The S4-2 census custody exact join stops flaking: three sightings in thirty days make it a defect, not noise
- Origin: main
- Next step: Appetite: 4h (coordinator-ratified per the flake protocol's three-sighting rule). Diagnose why the child-custody exact join reports 'still wrong after 2 fresh census passes' under load (scanSeq drift — sightings 2026-08-21 x2, 2026-08-23 inside the adopt suite's nested validation); fix the join or widen its convergence window with a named bound; the fixture leg then passes under deliberate parallel load. Evidence trail: artifacts/agents/suite-failures/20260822T232625Z-adopt-94397.
- OpenedAt: 2026-08-23T00:29:12Z
- Revision: 2
- Claimed: machine=widos-m5-pro lineage=coordinator at=2026-08-23T00:37:58Z

History:
- 2026-08-23T00:29:12Z MH2XDKQGN6HZ0JTEKAC0PQ9R74-widos-m5-pro-bf243850 open actor=widos-m5-pro+coordinator targets=s4-2-census-join
- 2026-08-23T00:37:58Z A9X7RTQSSRBGYYBJYEJ9HZPV5Z-widos-m5-pro-bf243850 claim actor=widos-m5-pro+coordinator targets=s4-2-census-join
Integrity: sha256=849c9ec15612c738cb21459a26e5b72538165c7314f520ea73cf0ed71e9664f0
