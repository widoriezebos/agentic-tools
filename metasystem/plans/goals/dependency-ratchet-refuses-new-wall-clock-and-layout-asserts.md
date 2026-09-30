# dependency-ratchet-refuses-new-wall-clock-and-layout-asserts

- State: queued
- Priority: 2
- Sequence: 55
- Risk: severity=2 novelty=2 exposure=3 accumulation=3 basis="The audit changes fixture rules repository-wide; misses repeatedly leave main red, while false positives block tests rather than production behavior."
- Tier: 2
- Intent: What: One test in the suite refuses any new test that relies on the real clock or on sleeping. Why: Tests that depend on real time fail when the computer is busy, and those failures have turned main red again and again. About 190 test files use the real clock today, and nothing stops that number from growing. The shell-bed part of the old goal is obsolete: those beds are gone. Pros: The number of timing-fragile tests can only go down. Cons: Legitimate uses must be declared as allowed clock seams.
- Origin: human
- Next step: Next: Write one Go test that counts _test.go files calling time.Now, time.Sleep or time.After outside the declared clock seams, records today's count as the baseline, and fails on any rise. Done when: adding a sleep to a new test file fails the suite with a message naming that file.
- OpenedAt: 2026-09-16T21:02:59Z
- Revision: 3
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-16T21:02:59Z 30SKF3XM8D5E9482JHBHYCKREP-m1e-c6925449 open actor=human:Wido targets=dependency-ratchet-refuses-new-wall-clock-and-layout-asserts
- 2026-09-16T21:03:03Z 9MBPZS7H2VFER36MZQ5G1AJ13D-m1e-c6925449 set-priority actor=human:Wido targets=dependency-ratchet-refuses-new-wall-clock-and-layout-asserts reason=priority-order subject=dependency-ratchet-refuses-new-wall-clock-and-layout-asserts from=unranked to=2:55 requested-sequence=55
- 2026-09-30T18:48:32Z P67ZPRWH3RQQ14Y9S79D6WEB09-m1e-b6a4eb0a edit actor=human:wido targets=dependency-ratchet-refuses-new-wall-clock-and-layout-asserts
Integrity: sha256=adaf38260e0cc578a18148f891c3352b1f9577b50f4414ea0253cb2a5a06ced2
