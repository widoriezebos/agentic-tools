# status-next-up-only-when-claimed

- State: claimed
- Risk: severity=1 novelty=1 exposure=1 accumulation=1 basis="A wrong line in a status post misleads a reader but binds nothing; the composer is established code with tests, touched by one seat, and the change is one selection rule."
- Tier: 1
- Intent: The channel status post prints 'Next up' from the ledger's ready frontier (internal/channel/report.go, ComposeReport, goal.Next(...).Ready), so it names goals no machine has claimed. Wido 2026-09-05: 'Next up is only interesting when you have completed something and will indeed pick that up, but only then. It is not interesting if you have not claimed it yet because then it means nothing. any other machine could pick it up.' DONE means the status post's Next up line names only a goal this machine has claimed, and only in a post that also carries a Delivered line; with nothing claimed there is no Next up line at all; the report tests prove both.
- Origin: main
- Next step: Tier 1: one change in ComposeReport (claimed-by-this-machine instead of the ready frontier, gated on a Delivered line) with its test in report_test.go; go test ./internal/channel/; land as a declared tier-1 direct fix. Waits for Wido's word on when.
- OpenedAt: 2026-09-05T05:37:06Z
- Revision: 3
- Labels: robustness
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=360 activeJobLimit=1 reviewRoundLimit=0
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-06T06:53:37Z revision=2 opid=A35EHY2TGAWCCFA1Q1J0WZGS35-m1-a4f8999f authority=proven digest=ab1283d94331cbec9c0dc066d9da302b2645fa809caf3a61fce7c0f0452ee3d9
- Claimed: machine=m1c lineage=main-1788680061-17829-64951c at=2026-09-07T01:23:07Z revision=3 accountingRevision=3
- StopCapability: generation=3 revision=3 machine=m1c claimEpoch=1 fenceEpoch=0

History:
- 2026-09-05T05:37:06Z 7YAKEQX3127KQSG72V88T686R5-m3-a5da21ff open actor=m3+mac-m3 targets=status-next-up-only-when-claimed
- 2026-09-06T06:53:37Z A35EHY2TGAWCCFA1Q1J0WZGS35-m1-a4f8999f approve actor=human:Wido targets=status-next-up-only-when-claimed reason=sweep
- 2026-09-07T01:23:07Z XEG1YJVWT616EZDHH9ZBXPJ08P-m1c-7cd0bd60 claim actor=m1c+main-1788680061-17829-64951c targets=status-next-up-only-when-claimed
Integrity: sha256=fd99cfd3412d629203c2f35f6275dac602d65b5b9d02bcbeb2fc9b436893190b
