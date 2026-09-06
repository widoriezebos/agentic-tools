# status-next-up-only-when-claimed

- State: approved
- Risk: severity=1 novelty=1 exposure=1 accumulation=1 basis="A wrong line in a status post misleads a reader but binds nothing; the composer is established code with tests, touched by one seat, and the change is one selection rule."
- Tier: 1
- Intent: The channel status post prints 'Next up' from the ledger's ready frontier (internal/channel/report.go, ComposeReport, goal.Next(...).Ready), so it names goals no machine has claimed. Wido 2026-09-05: 'Next up is only interesting when you have completed something and will indeed pick that up, but only then. It is not interesting if you have not claimed it yet because then it means nothing. any other machine could pick it up.' DONE means the status post's Next up line names only a goal this machine has claimed, and only in a post that also carries a Delivered line; with nothing claimed there is no Next up line at all; the report tests prove both.
- Origin: main
- Next step: Tier 1: one change in ComposeReport (claimed-by-this-machine instead of the ready frontier, gated on a Delivered line) with its test in report_test.go; go test ./internal/channel/; land as a declared tier-1 direct fix. Waits for Wido's word on when.
- OpenedAt: 2026-09-05T05:37:06Z
- Revision: 2
- Labels: robustness
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=360 activeJobLimit=1 reviewRoundLimit=0
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-06T06:53:37Z revision=2 opid=A35EHY2TGAWCCFA1Q1J0WZGS35-m1-a4f8999f authority=proven digest=ab1283d94331cbec9c0dc066d9da302b2645fa809caf3a61fce7c0f0452ee3d9

History:
- 2026-09-05T05:37:06Z 7YAKEQX3127KQSG72V88T686R5-m3-a5da21ff open actor=m3+mac-m3 targets=status-next-up-only-when-claimed
- 2026-09-06T06:53:37Z A35EHY2TGAWCCFA1Q1J0WZGS35-m1-a4f8999f approve actor=human:Wido targets=status-next-up-only-when-claimed reason=sweep
Integrity: sha256=102486cabcf93e91ff0b590115902c0686b62fed32e4d1bfe6d23e25b14aeb53
