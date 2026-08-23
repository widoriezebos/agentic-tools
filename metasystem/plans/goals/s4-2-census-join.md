# s4-2-census-join

- State: queued
- Intent: The S4-2 census custody exact join stops flaking: three sightings in thirty days make it a defect, not noise
- Origin: main
- Next step: Appetite: 4h (coordinator-ratified per the flake protocol's three-sighting rule). Diagnose why the child-custody exact join reports 'still wrong after 2 fresh census passes' under load (scanSeq drift — sightings 2026-08-21 x2, 2026-08-23 inside the adopt suite's nested validation); fix the join or widen its convergence window with a named bound; the fixture leg then passes under deliberate parallel load. Evidence trail: artifacts/agents/suite-failures/20260822T232625Z-adopt-94397.
- OpenedAt: 2026-08-23T00:29:12Z
- Revision: 1

History:
- 2026-08-23T00:29:12Z MH2XDKQGN6HZ0JTEKAC0PQ9R74-widos-m5-pro-bf243850 open actor=widos-m5-pro+coordinator targets=s4-2-census-join
Integrity: sha256=a2a4664964329b98721212274a42dda7edd2be5fd797cd7766d78c75cca3b942
