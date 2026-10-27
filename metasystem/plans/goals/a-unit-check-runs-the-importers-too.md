# a-unit-check-runs-the-importers-too

- State: parked
- Risk: severity=1 novelty=1 exposure=2 accumulation=1 basis="Test selection inside the unit check only; a wrong selection costs check time or a static-only fallback, never a landing; the lane's full proof stays the net; the 50 percent cap is unchanged"
- Tier: 1
- Intent: A unit check runs whole every internal package that reaches a changed package through production imports, keeps test imports as a last hop only, narrows to the direct importers when the selection is capped instead of falling to a static-only check, selects a source audit by the path it reads, and runs named cmd/metasystem tests by name (finding 429; design plans/designs/the-stewards-act-on-what-they-see.md (accepted, revision 4) D2, D3, G2)
- Origin: main
- Next step: On ui after the-impact-check-selects-only-tests-this-host-can-run lands (same files): opus-designer cuts the brief from the page (BRIEF-CHECKLIST, Read each round: yes, one Astra round, the reverse-dependent packages run whole); one unit, about 60 production lines: landing/batch/goadapter/unitgate.go, impact.go, cmd/metasystem/test_impact.go; reverses part of 4e690dba7f (decision 9.1 of the page)
- OpenedAt: 2026-10-27T15:19:40Z
- Revision: 4
- BlockedBy: the-impact-check-selects-only-tests-this-host-can-run
- Pinned: ui
- Budget: elapsedLimit=2d attemptLimit=8 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0
- NormApproval: approvedRef=8AAEPZTDKBEJ7X3A6HVAEA1CAX-m1e-e06ab4ba minutes=1200 reviewRounds=20 goalRevision=2
- Approved: by=human:Wido at=2026-10-27T15:20:22Z revision=3 opid=8AAEPZTDKBEJ7X3A6HVAEA1CAX-m1e-e06ab4ba authority=proven digest=7a7df6656225fb021fee2b293ba8eab47c7fa4fc2ee13ffabcf51047323c0ab3 episode=3
- Parked: by=human:Wido at=2026-10-27T15:19:40Z blocker=the-impact-check-selects-only-tests-this-host-can-run because=blocked by the-impact-check-selects-only-tests-this-host-can-run; returns when it is done

History:
- 2026-10-27T15:19:40Z 35NQ8H80Z4X44CJ9KW0VH9PKZV-m1e-e06ab4ba open actor=human:Wido targets=a-unit-check-runs-the-importers-too,the-impact-check-selects-only-tests-this-host-can-run
- 2026-10-27T15:20:06Z D4RDME2PGN8JJF6C76QXJC7P9C-m1e-e06ab4ba approve actor=human:Wido targets=a-unit-check-runs-the-importers-too
- 2026-10-27T15:20:22Z 8AAEPZTDKBEJ7X3A6HVAEA1CAX-m1e-e06ab4ba approve actor=human:Wido targets=a-unit-check-runs-the-importers-too
- 2026-10-27T15:20:39Z WM0HSE36YPPTY7RASKWW3NBQ7T-m1e-e06ab4ba set-pin actor=human:Wido targets=a-unit-check-runs-the-importers-too
Integrity: sha256=95de98916a5efbd1cd8a251c8777d242833a714c689e2222d6e0ef218992e7f4
