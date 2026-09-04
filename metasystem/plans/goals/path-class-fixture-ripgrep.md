# path-class-fixture-ripgrep

- State: queued
- Tier: 1
- Intent: scripts/agents/path-class-fixtures.sh (TestDeletedListsHaveNoReader) calls ripgrep (rg), a command outside the declared inventory in docs/project-rules.md; on a machine without it the fixture dies 'rg: command not found' and reports a false reader of the deleted tables (m2, 2026-09-03). Replace the two rg calls with grep -rE plus --exclude-dir/--exclude so the fixture runs on every supported host.
- Origin: main
- Next step: TIER 1 per R-54-m1 (a fixture): build, run the fixture on a host without rg and on one with it, land as a declared direct fix; no design round, no review. Box 1h/3/60m/1. Origin: the m2 gate replay of the path-class manifest's second part, where the leg failed for the missing command while the equivalent grep found no reader.
- OpenedAt: 2026-09-03T13:27:19Z
- Revision: 2
- Labels: robustness

History:
- 2026-09-03T13:27:19Z 2F3FDXPRSZZTQ1A8ZPKNBVCYF6-m2-5fcf08ab open actor=m2+main-1788441779-14484-82d6ed targets=path-class-fixture-ripgrep
- 2026-09-04T06:13:55Z CNFJ5Z3RHFGGHND5FV6QBTY7Q6-m2-5fcf08ab edit actor=m2+main-1788441779-14484-82d6ed targets=path-class-fixture-ripgrep
Integrity: sha256=c09ad52d7d5e121fc8432454373cb1348441a931dda6339029e69b8ae5a5cac4
