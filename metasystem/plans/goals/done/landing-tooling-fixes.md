# landing-tooling-fixes

- State: done
- Intent: Landing tools do what agents remember around: commit.sh survives multi-path pathspecs; landing pushes origin AND transport
- Origin: main
- Next step: Reproduce the pathspec mangling as a fixture, fix commit.sh; add the dual-remote push to the landing path. Critique, then implement.
- Concluded: commit.sh --push lands origin then transport with named failures, fixture-proven on bare remotes; the pathspec hazard was already principled by the IL28 tree-proof rollback. Landed through the flag itself, whose first refusal correctly caught this checkout behind the goal-verb commits.
- OpenedAt: 2026-08-20T00:18:00Z
- Revision: 3

History:
- 2026-08-22T06:30:55Z BXWE9NXAWCGCTR3MFCE8GDC4P5-widos-m5-pro-bf243850 migrate actor=human:wido targets=landing-tooling-fixes
- 2026-08-22T09:59:45Z F5XNRZB9H33PYP4AK3MPJ1A250-widos-m5-pro-bf243850 claim actor=widos-m5-pro+coordinator targets=landing-tooling-fixes
- 2026-08-22T10:02:05Z N893D3MQH43J8B1HW536TCSS02-widos-m5-pro-bf243850 done actor=widos-m5-pro+coordinator targets=landing-tooling-fixes
Integrity: sha256=c055f37eda880d1476bde85c02cc6e65bdaafeddf5515c927d4ce3354b150a9d
