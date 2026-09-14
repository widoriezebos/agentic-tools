# seat-landings-run-the-selected-sections

- State: approved
- Priority: 1
- Sequence: 60
- Risk: severity=3 novelty=2 exposure=3 accumulation=3 basis="severity 3: regressions reach trunk unseen and stop whole verification sections for every seat; novelty 2: the landing path seats actually use bypasses the risk-selected test policy, and a human's own commit must never be refused; exposure 3: every seat landing by human commit; accumulation 3: each unseen red hides the next one in the same section"
- Tier: 3
- Intent: Seats land by human commit from the enrolled terminal (the hact land.sh path). That path never runs the risk-selected test policy (metasystem test plan / test run), so verification is whatever beds the seat picks. On 2026-09-14 and 2026-09-15 this let three reds reach trunk unseen: 157fac92 broke the supervision bed's wait-restart (found by m1b), c71f1e23 broke runtime-hook-fixtures.sh:384 and stopped section/supervision-and-census-fixtures before any scenario (found by m1e; m1c had run only two supervision beds), and witness-gate-fixtures sat red for five days after 1b12f534. m1b and m1e also found five more deep sections red on trunk. metasystem test plan refuses inside a landing worktree (ENROLLMENT_DRIFT), so a seat cannot easily ask the policy either. DONE: a seat's landing cannot reach trunk unless every section the testing policy selects for the candidate's changed paths has run on that exact tree, green or named as red on trunk by a trunk control; the landing record names the sections and their results; test plan answers for a landing worktree; a human's own commit is never refused (Wido 2026-09-04, human-carried-landing); a fixture proves a seat landing that ran only some of the selected sections does not reach trunk.
- Origin: human
- Next step: Design first (Claude Fable delegate): where the check lives given that seats land by human commit and the human is never refused, and how test plan serves a landing worktree.
- OpenedAt: 2026-09-14T23:51:08Z
- Revision: 3
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-14T23:51:14Z revision=2 opid=1AXJ8NT5J6TEN8QSNQZ3W9QBAS-m1e-c6925449 authority=proven digest=6c1d9dadf1ebc282427aad0d5e6105f4e4c4366e08ba3c5a347e03ab2bb0195d

History:
- 2026-09-14T23:51:08Z MMY4JR0VMFX2S5EV3GQ23H4TSQ-m1e-c6925449 open actor=human:Wido targets=seat-landings-run-the-selected-sections
- 2026-09-14T23:51:14Z 1AXJ8NT5J6TEN8QSNQZ3W9QBAS-m1e-c6925449 approve actor=human:Wido targets=seat-landings-run-the-selected-sections
- 2026-09-14T23:51:21Z AY2V9HG6PT92RB9D0N9A1H23F5-m1e-c6925449 set-priority actor=human:Wido targets=seat-landings-run-the-selected-sections reason=priority-order subject=seat-landings-run-the-selected-sections from=unranked to=1:60 requested-sequence=append
Integrity: sha256=c4f2f93bfc4d4a777248c66b2fef9f513b41d4329450cf88e36a087f505b5fd0
