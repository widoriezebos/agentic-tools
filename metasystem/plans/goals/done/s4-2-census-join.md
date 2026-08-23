# s4-2-census-join

- State: done
- Intent: The S4-2 census custody exact join stops flaking: three sightings in thirty days make it a defect, not noise
- Origin: main
- Next step: Appetite: 4h (coordinator-ratified per the flake protocol's three-sighting rule). Diagnose why the child-custody exact join reports 'still wrong after 2 fresh census passes' under load (scanSeq drift — sightings 2026-08-21 x2, 2026-08-23 inside the adopt suite's nested validation); fix the join or widen its convergence window with a named bound; the fixture leg then passes under deliberate parallel load. Evidence trail: artifacts/agents/suite-failures/20260822T232625Z-adopt-94397.
- Concluded: The mechanism could not be proven — every sighting's inner state had been swept — so the fix makes the next failure self-diagnosing: failing census waits dump their judged snapshot into preserved evidence, the fixture's record edits are atomic (torn-read candidate eliminated by construction), and the suite passes standalone and under deliberate parallel load. A load-kill liveness probe was tried and removed: it interrogated a synthetic pid and falsified its own theory. Reopen from the snapshot if a sighting returns. Under the 4h appetite.
- OpenedAt: 2026-08-23T00:29:12Z
- Revision: 3

History:
- 2026-08-23T00:29:12Z MH2XDKQGN6HZ0JTEKAC0PQ9R74-widos-m5-pro-bf243850 open actor=widos-m5-pro+coordinator targets=s4-2-census-join
- 2026-08-23T00:37:58Z A9X7RTQSSRBGYYBJYEJ9HZPV5Z-widos-m5-pro-bf243850 claim actor=widos-m5-pro+coordinator targets=s4-2-census-join
- 2026-08-23T00:44:35Z VE5THY2AXMJHTZVDDW78BHZ3R1-widos-m5-pro-bf243850 done actor=widos-m5-pro+coordinator targets=s4-2-census-join
Integrity: sha256=1f6ad0e7f28f4dac89568ae3b9896324888980f2666006dd220d2decd6e0f00e
