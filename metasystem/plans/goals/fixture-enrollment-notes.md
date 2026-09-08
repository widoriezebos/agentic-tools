# fixture-enrollment-notes

- State: queued
- Priority: 3
- Sequence: 77
- Risk: severity=1 novelty=1 exposure=2 accumulation=1 basis="severity 1: nothing unsafe, dead code and two unpinned negatives; novelty 1: tests and an identity field that exist; exposure 2: three suites on every host that runs them; accumulation 1: fixed set of notes"
- Tier: 2
- Intent: Five low notes from the closing review of fixture-stewards-reach-the-desktop (fsrd-cc1-20260906, dispositions in plans/dispositions/fixture-stewards-reach-the-desktop-code-critique-r1.md), none material: (1) the arm function's fixture restamp for a fake-runtime root is unreachable because the runner exclusion for the same root returns first, so fixture kinds come only from the suites' hand-written identities; (2) no test pins that a real kernel terminal classifies human without the fixture-granted flag; (3) the health suite asserts the fixture kind after the arm and not after either restart, so RestartFixture is correct by inspection and unproven; (4) supervision-fixtures.sh, delegate-caps-fixtures.sh and fingerprint-harness.sh hand-write steward identities without the enrollment field, which reads as human-terminal, and are kept off the desktop today only by a configured notify command or by runner exclusion; (5) VerifyIdentity refuses an enrollment kind it does not know, so a mixed-engine host refuses loudly at arm if a kind is ever added, which is the behaviour wanted and is recorded here so it is a choice and not an accident. DONE means (1) the dead restamp is removed or made reachable with a test, (2) and (3) each gain the one test named, (4) every suite-written identity carries the fixture kind and the suites assert it, and (5) stays as recorded.
- Origin: main
- Next step: Unclaimed; awaits approval. Small build: one brief for a Sol round covering the five notes, one Fable code critique, land under the tier-2 lane.
- OpenedAt: 2026-09-06T18:05:29Z
- Revision: 2
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-06T18:05:29Z 34YVAN0DGC5F40NGV9QD2YN2G5-m1d-62183579 open actor=m1d+main-1788683763-71870-f7f607 targets=fixture-enrollment-notes
- 2026-09-08T16:03:38Z 9ARFCJX97FNCHBC0E5HPG3HA6J-m1-7cd0bd60 set-priority actor=human:Wido targets=fixture-enrollment-notes reason=priority-order subject=fixture-enrollment-notes from=unranked to=3:77 requested-sequence=77
Integrity: sha256=d11496688cecf3b806b15280c96a0e96b9b7276bcfbf72d9251f1658b09882c3
