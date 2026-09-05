# channel-local-timestamps

- State: claimed
- Risk: severity=1 novelty=1 exposure=2 accumulation=1 basis="severity 1: a misread timestamp costs the human a moment of arithmetic, nothing is authorized or lost by it; novelty 1: rendering an existing time value in a different location, with the only real question being which times are display and which are record; exposure 2: every message the fleet posts to the channel, read by one human; accumulation 1: first report of it"
- Tier: 2
- Intent: Telegram messages carry UTC timestamps, so Wido reads every fleet message in a timezone he is not in. internal/channel/report.go:84 renders the status headline as '<machine> status 2006-01-02 15:04Z' from c.Now.UTC(), and report.go:34 and question.go:158 set Now to time.Now().UTC(). DONE means the timestamps a human reads in the channel are actual local time for the machine's own timezone, the offset is unambiguous to the reader, and the records the inbox keeps stay whatever the ledger needs so nothing that is compared or sorted changes meaning
- Origin: main
- Next step: Split display from record before changing anything: internal/channel/report.go:84 and the question rendering are what a human reads and should be local with a visible offset; internal/channel/inbox.go:58-60 SentAt and ReceivedAt are record fields that other machines compare, and report.go:159 feeds a git --since, so those stay UTC. Then decide where 'local' comes from - the machine's own zone is the honest answer for a per-machine status line. Wido asked for this on 2026-09-05; it waits behind m2's supervisor repo-identity landing like everything else here
- OpenedAt: 2026-09-05T10:10:41Z
- Revision: 4
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=240 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0
- Approved: by=human:human:Wido at=2026-09-05T10:13:10Z revision=2 opid=FWHRFW8MEKGN6S95PATEVKQQ7G-m1-a4f8999f authority=relayed digest=cd7c85ba36923ba635f82d083c811c5257200df7ca014089a1abee2e1de6e8b2 reviewBy=2026-09-06
- Sliced: machine=m1 lineage=main-1788594343-3833-fb64b9 revision=3 at=2026-09-05T10:44:35Z
- Claimed: machine=m1 lineage=main-1788594343-3833-fb64b9 at=2026-09-05T10:44:30Z revision=3 accountingRevision=3
- StopCapability: generation=3 revision=3 machine=m1 claimEpoch=5 fenceEpoch=0

History:
- 2026-09-05T10:10:41Z GB3XPE0X5VV0WRKZGGBT8XTEKQ-m1-a4f8999f open actor=m1+main-1788594343-3833-fb64b9 targets=channel-local-timestamps
- 2026-09-05T10:13:10Z FWHRFW8MEKGN6S95PATEVKQQ7G-m1-a4f8999f approve actor=human:human:Wido targets=channel-local-timestamps authorityOutcome=TEMPORARY_HUMAN_WORD authorityReviewBy=2026-09-06 authorityRuling=R-32-m1 temporaryHumanWord="Instead of waiting, work on the Telegram messages: the timestamp one, and the other low-risk one that prevents a wall of text in Telegram. Maybe do these now."
- 2026-09-05T10:44:30Z QRH9S279VEDR93ZFDCPBTDZQ4Q-m1-a4f8999f claim actor=m1+main-1788594343-3833-fb64b9 targets=channel-local-timestamps
- 2026-09-05T10:44:35Z 96SMRAQS0KFEC4QN6MT23WVBNX-m1-a4f8999f slice-start actor=m1+main-1788594343-3833-fb64b9 targets=channel-local-timestamps
Integrity: sha256=60ca6dafa5f13287003601db351d3e3cd6e0ecb4cfebb1f662f2bbf9f92f0fd7
