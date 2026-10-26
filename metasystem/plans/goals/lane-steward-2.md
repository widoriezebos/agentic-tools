# lane-steward-2

- State: parked
- Risk: severity=2 novelty=1 exposure=2 accumulation=1 basis="a change on the wait verb every seat steward uses and on the lane queue's record; a wrong return ends a wait early or never, caught by the unit's tests and the lane proof before main"
- Tier: 2
- Intent: The wait ends on the lane's word (lane-steward design D9, split out of lane-steward U6a2 on 2026-10-26): `work wait G --for landing` also returns when, after the wait's registration boundary, the lane records for G a return, a stop or a question, with the state word, so a seat steward's wait never sleeps through a stop. Split from lane-steward because D9 changes internal/run and the impact selection then picks Linux-only test files that do not exist on this Mac, making the frozen check red by construction (goadapter/impact.go:115, :237) until the-impact-check-selects-only-tests-this-host-can-run lands.
- Origin: main
- Next step: After the-impact-check-selects-only-tests-this-host-can-run lands: steward cuts one brief from the saved draft /Users/wido/agentic-tools-evidence/lane-test-20261009/builds/lane-steward-U6a-20261025/u6a2-d9-draft.patch (about 93 production lines measured) and the "Carried to the D9 unit" notes at the end of briefs/lane-steward-U6a2-brief.md (the command-level test asserts the persisted renewed selector keeps the first registration's LaneAfter; two small defects of the draft corrected); one Astra round; machine route on ui; lane landing. Lineage: lane-steward (D9 of its accepted design, revision 3).
- OpenedAt: 2026-10-26T10:06:51Z
- Revision: 1
- BlockedBy: the-impact-check-selects-only-tests-this-host-can-run
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0
- Parked: by=human:Wido at=2026-10-26T10:06:51Z blocker=the-impact-check-selects-only-tests-this-host-can-run because=blocked by the-impact-check-selects-only-tests-this-host-can-run; returns when it is done

History:
- 2026-10-26T10:06:51Z DM1YWYQNCHCGXX8CZHNQAMGVDQ-m1e-acb921fd open actor=human:Wido targets=lane-steward-2,the-impact-check-selects-only-tests-this-host-can-run
Integrity: sha256=4cddc1f5da2570032cb62e0767ffcfedbd3c852a3f4fa26d42ca0fa4f66b6581
