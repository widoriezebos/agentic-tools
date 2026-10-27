# a-unit-check-runs-the-importers-too

- State: parked
- Risk: severity=1 novelty=1 exposure=2 accumulation=1 basis="Test selection inside the unit check only; a wrong selection costs check time or a static-only fallback, never a landing; the lane's full proof stays the net; the 50 percent cap is unchanged"
- Tier: 1
- Intent: A unit check runs whole every internal package that reaches a changed package through production imports, keeps test imports as a last hop only, narrows to the direct importers when the selection is capped instead of falling to a static-only check, selects a source audit by the path it reads, and runs named cmd/metasystem tests by name (finding 429; design plans/designs/the-stewards-act-on-what-they-see.md (accepted, revision 4) D2, D3, G2)
- Origin: main
- Next step: On ui after the-impact-check-selects-only-tests-this-host-can-run lands (same files): opus-designer cuts the brief from the page (BRIEF-CHECKLIST, Read each round: yes, one Astra round, the reverse-dependent packages run whole); one unit, about 60 production lines: landing/batch/goadapter/unitgate.go, impact.go, cmd/metasystem/test_impact.go; reverses part of 4e690dba7f (decision 9.1 of the page)
- OpenedAt: 2026-10-27T15:19:40Z
- Revision: 1
- BlockedBy: the-impact-check-selects-only-tests-this-host-can-run
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=360 activeJobLimit=1 reviewRoundLimit=0
- BudgetExceptions: 0
- Parked: by=human:Wido at=2026-10-27T15:19:40Z blocker=the-impact-check-selects-only-tests-this-host-can-run because=blocked by the-impact-check-selects-only-tests-this-host-can-run; returns when it is done

History:
- 2026-10-27T15:19:40Z 35NQ8H80Z4X44CJ9KW0VH9PKZV-m1e-e06ab4ba open actor=human:Wido targets=a-unit-check-runs-the-importers-too,the-impact-check-selects-only-tests-this-host-can-run
Integrity: sha256=1c2e05fdbf79dd74fddbdf88ba0010f693de85850d3833d0b20f4f9900d61182
